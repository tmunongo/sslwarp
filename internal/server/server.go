package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
)

type Server struct {
	//
	httpsListener net.Listener
	httpListener net.Listener
	tunnels map[string]*ClientTunnel
	subdomainMap map[string]string
	mu sync.Mutex
	tlsConfig *tls.Config
}

type ClientTunnel struct {
	conn net.Conn
	services map[string]ServiceConfig
}

type ServiceConfig struct {
	Name string
	Domain string
	Proto string
	Addr int
}

type TunnelRequest struct {
	APIKey string `json:"api_key"`
	Message string `json:"message"`
	Tunnels map[string]ServiceConfig `json:"tunnels"`
}

type ReceivedRequest struct {
	Api_key string
	Subdomain string
	Local_addr string
	Message string
	Tunnels map[string]ServiceConfig
}

type RequestResponse struct {
	Tunnel_ID string
	Error string
}

func New() *Server {
	return &Server{
		tunnels: make(map[string]*ClientTunnel),
		subdomainMap: make(map[string]string),
	}
}

func (s *Server) Run() error {
	var err error

	// establish http listener
	s.httpListener, err = net.Listen("tcp", ":80")
	if err != nil {
		return fmt.Errorf("failed to start http listener: %w", err)
	}
	defer s.httpListener.Close()

	s.httpsListener, err = net.Listen("tcp", ":443")
	if err != nil {
		return fmt.Errorf("failed to start https listener: %w", err)
	}
	defer s.httpsListener.Close()

	// load tls cert
	cert, err := tls.LoadX509KeyPair("certs/server.crt", "certs/server.key")
	if err != nil {
		return fmt.Errorf("failed to load tls certificates %w", err)
	}

	s.tlsConfig = &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	log.Println("Server is listening on ports 80 (HTTP) and 443 (HTTPS)")

	// tunnel listener on separate port
	tunnelListener, err := net.Listen("tcp", ":8080")
	if err != nil {
		return fmt.Errorf("failed to start tunnel lsitener: %w", err)
	}
	defer tunnelListener.Close()

	log.Println("Tunnel listener is listening on port 8080")

	go s.listenAndServeVirtualHosts()

	// infinite loop to listen for connections
	for {
		conn, err := tunnelListener.Accept()
		if err != nil {
			log.Printf("Error accepting tunnel connections: %v", err)
			continue
		}

		// spawn a go routine to handle the connection
		go s.handleConnection(conn)
	}
}

func (s *Server) listenAndServeVirtualHosts() {
	// HTTP server
	go func() {
		httpSrv := &http.Server{
			Addr: ":80",
			Handler: http.HandlerFunc(s.handleHTTP),
		}
		if err := httpSrv.Serve(s.httpListener); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	httpSrv := &http.Server{
		Addr: ":443",
		TLSConfig: s.tlsConfig,
		Handler: http.HandlerFunc(s.handleHTTPS),
	}
	if err := httpSrv.ServeTLS(s.httpsListener, "", ""); err != nil && err != http.ErrServerClosed {
		log.Printf("HTTPS server error: %v", err)
	}
}

func (s *Server) handleHTTP(w http.ResponseWriter, r *http.Request) {
	// upgrade HTTP to HTTPS
	host := r.Host
	if host == "" {
		http.Error(w, "missing host header", http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "https://"+host+r.URL.RequestURI(), http.StatusMovedPermanently)
}

func (s *Server) handleHTTPS(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	subdomain := strings.Split(host, ".")[0]

	s.mu.Lock()
	tunnelID, exists := s.subdomainMap[subdomain]
	s.mu.Unlock()

	if !exists {
		http.Error(w, "Tunnel not found", http.StatusNotFound)
		return
	}

	s.mu.Lock()
	clientTunnel, exists := s.tunnels[tunnelID]
	s.mu.Unlock()

	var service ServiceConfig
	for _, svc := range clientTunnel.services {
		if svc.Domain == subdomain {
			service = svc
			break
		}
	}

	if service.Name == "" {
		http.Error(w, "Service not found for the subdomain", http.StatusNotFound)
		return
	}

	// notify the client of the new connection
	s.notifyNewConnection(clientTunnel, service)

	// create a new connection to handle this request
	clientConn, serverConn := net.Pipe()
	go s.handleFullDuplexCommunication(serverConn, clientTunnel.conn)

	s.forwardRequest(w, r, clientConn)
}

func (s *Server) forwardRequest(w http.ResponseWriter, r *http.Request, clientConnection net.Conn) {
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return clientConnection, nil
			},
		},
	}

	outReq, err := http.NewRequest(r.Method, r.URL.String(), r.Body)
	if err != nil {
		http.Error(w, "error creating forwarded request", http.StatusInternalServerError)
		return
	}

	// set x-forwarded for header
	if clientIP, _, err := net.SplitHostPort(r.RemoteAddr); err != nil {
		if prior, ok := outReq.Header["X-Forwarded-For"]; ok {
			clientIP = strings.Join(prior, ", ") + ", " + clientIP
		}
		outReq.Header.Set("X-Forwarded-For", clientIP)
	}

	resp, err := client.Do(outReq)
	if err != nil {
		http.Error(w, "error forwarding request", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		w.Header()[k] = v
	}

	w.WriteHeader(resp.StatusCode)

	io.Copy(w, resp.Body)
}

func (s *Server) notifyNewConnection(clientTunnel *ClientTunnel, service ServiceConfig) {
	notification := struct {
		Type string `json:"type"`
		Service ServiceConfig `json:"service"`
	}{
		Type: "new_connection",
		Service: service,
	}

	json.NewEncoder(clientTunnel.conn).Encode(&notification)
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

	if clientRequest.Api_key == "" {
        log.Println("Missing API key in tunnel request")
        return
    }

    // TODO: Validate API key

    // Check if the message starts with "TUNNEL_REQUEST"
    if (clientRequest.Message == "TUNNEL_REQUEST") {
		s.establishTunnel(conn, *clientRequest)
	} else {
		s.handleClientRequest(conn, clientRequest.Message)
	}
}

func (s *Server) establishTunnel(conn net.Conn, clientRequest ReceivedRequest) {
	tunnelID := generateUniqueID();

	log.Printf("Tunnels: %v", clientRequest.Tunnels)

	clientTunnel := &ClientTunnel{
		conn: conn,
		services: clientRequest.Tunnels,
	}

	s.mu.Lock()
	s.tunnels[tunnelID] = clientTunnel
	for _, service := range clientRequest.Tunnels {
		s.subdomainMap[service.Domain] = tunnelID
	}
	s.mu.Unlock()

	log.Printf("Connection established with ID: %s", tunnelID)

	// send tunnelID back
	response := RequestResponse {
		Tunnel_ID: tunnelID,
		Error: "",
	}
	err := json.NewEncoder(conn).Encode(response)
	if err != nil {
		log.Printf("failed to encode JSON to connection: %v", err)
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
			for domain, id := range s.subdomainMap {
				if id == tunnelID {
					delete(s.subdomainMap, domain)
				}
			}
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

	s.handleFullDuplexCommunication(conn, tunnelConn.conn)	
}

func (s *Server) handleFullDuplexCommunication(clientConn, tunnelConn net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	// Create a channel to signal when it's time to close connections
	done := make(chan struct{})

	// client to tunnel
	go func() {
		defer wg.Done()
		s.pipe(tunnelConn, clientConn, "client -> tunnel", done)
	}()

	// tunnel to client
	go func() {
		defer wg.Done()
		s.pipe(clientConn, tunnelConn, "tunnel -> client", done)
	}()

	// Wait for both directions to complete
	wg.Wait()

	// Close the connections after both directions are done
	clientConn.Close()
	tunnelConn.Close()
	log.Println("Client request handled!")
}

func (s *Server) pipe(dst, src net.Conn, direction string, done chan struct{}) {
	defer func() {
		select {
		case <-done:
			// Channel is already closed, do nothing
		default:
			close(done)
		}
	}()

	buf := make([]byte, 32*1024) // 32KB buffer
	for {
		nr, err := src.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading in %s: %v", direction, err)
			}
			break
		}
		if nr > 0 {
			nw, err := dst.Write(buf[0:nr])
			if err != nil {
				log.Printf("Error writing in %s: %v", direction, err)
				break
			}
			if nw != nr {
				log.Printf("Error: short write in %s", direction)
				break
			}
		}

		select {
		case <-done:
			return
		default:
			// Continue the loop
		}
	}
}

func (s *Server) extractTunnelID(message string) (string, error) {
	// This function should parse the initial message and return the tunnel ID, or an error if the message is invalid.
	// this function is protocol dependent
	panic("Unimplemented!")
}