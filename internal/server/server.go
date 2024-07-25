package server

import (
	"log"
	"net"
	"sync"
)

type Server struct {
	//
	listener net.Listener
	tunnels map[string]net.Conn
	mu sync.Mutex
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

	log.Println("Server is lsitening on port :8080")

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
		log.Println("Error reading from connection: %v", err)
		return
	}

	message := string(buffer[:n])

	if message == "TUNNEL_REQUEST" {
		s.establishTunnel()
	} else {
		s.handleClientRequest(conn, message)
	}
}

func (s *Server) establishTunnel() {
	return
}

func (s *Server) handleClientRequest(conn net.Conn, msg string) {
	return
}