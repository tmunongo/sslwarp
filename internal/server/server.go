package server

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"sync"

	"github.com/google/uuid"
)

type Server struct {
	//
	listener net.Listener
	tunnels map[string]net.Conn
	mu sync.Mutex
}

type ReceivedRequest struct {
	Api_key string
	Subdomain string
	Local_addr string
	Message string
}

func New() *Server {
	return &Server{
		tunnels: make(map[string]net.Conn),
	}
}

func (s *Server) Run() error {
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		return err
	}

	s.listener = listener
	defer s.listener.Close()

	log.Println("Server is listening on port :8080")

	// infinite loop to listen for connections
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			log.Printf("Error accepting connections: %v", err)
			continue
		}

		// spawn a go routine to handle the connection
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		log.Printf("Error reading from connection: %v", err)
		return
	}

	jsonRequest := buffer[:n]

	clientRequest := &ReceivedRequest{}

	err = json.Unmarshal([]byte(jsonRequest), clientRequest)
	if err != nil {
        log.Printf("Error unmarshalling JSON: %v", err)
        return
    }

	// this is empty for some reason, must fix
	log.Printf("Received request %v", clientRequest)

	if clientRequest.Message == "TUNNEL_REQUEST" {
		log.Println("Establishing")
		s.establishTunnel(conn)
	} else {
		s.handleClientRequest(conn, clientRequest.Message)
	}
}

func (s *Server) establishTunnel(conn net.Conn) {
	tunnelID := generateUniqueID();

	s.mu.Lock()
	s.tunnels[tunnelID] = conn
	s.mu.Unlock()

	log.Printf("Connection established with ID: %s", tunnelID)

	// send tunnelID back
	err := json.NewEncoder(conn).Encode(tunnelID)
	if err != nil {
		log.Println("Failed to encode JSON to connection: %v", err)
	}
	// conn.Write([]byte(tunnelID))

	// keep the tunnel open
	for {
		buffer := make([]byte, 1024)

		_, err := conn.Read(buffer)

		if err != nil {
			log.Printf("Tunnel %s closed: %v", tunnelID, err)

			s.mu.Lock()
			delete(s.tunnels, tunnelID)
			s.mu.Unlock()
			return
		}
	}
}

func generateUniqueID() string {
	return uuid.NewString()
}

func (s *Server) handleClientRequest(conn net.Conn, msg string) {
	s.mu.Lock()
	tunnelID := msg
	s.mu.Unlock()

	// extract
	// tunnelID, err := s.extractTunnelID(tunnelID)
	// if err != nil {
	// 	log.Printf("Tunnel ID not found!")
	// 	conn.Write([]byte("No tunnel ID was found in the request"))
	// 	return
	// }

	err := uuid.Validate(tunnelID)
	if err != nil {
		log.Printf("Tunnel ID %s is invalid", tunnelID)
		conn.Write([]byte("Provided an invalid tunnel ID"))
		return
	}

	// find the tunnel
	s.mu.Lock()
	tunnelConn, exists := s.tunnels[tunnelID]
	s.mu.Unlock()

	if !exists {
		log.Printf("Tunnel %s not found", tunnelID)
		conn.Write([]byte("Tunnel not found"))
		return
	}

	s.handleFullDuplexCommunication(conn, tunnelConn)	
}

func (s *Server) handleFullDuplexCommunication(clientConn, tunnelConn net.Conn) {
	defer clientConn.Close()
	defer tunnelConn.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	// client to tunnel
	go func() {
		defer wg.Done()
		if err := s.pipe(tunnelConn, clientConn); err != nil {
			log.Printf("Error in client -> tunnel: %v", err)
		}
	}()

	// tunnel to client
	go func() {
		defer wg.Done()
		if err := s.pipe(clientConn, tunnelConn); err != nil {
			log.Printf("Error in tunnel -> client: %v", err)
		}
	}()

	wg.Wait()
	log.Println("Client request handled!")
}

func (s *Server) pipe(dst, src net.Conn) error {
	_, err := io.Copy(dst, src)
	return err
}

func (s *Server) extractTunnelID(message string) (string, error) {
	// This function should parse the initial message and return the tunnel ID, or an error if the message is invalid.
	// this function is protocol dependent
	panic("Unimplemented!")
}