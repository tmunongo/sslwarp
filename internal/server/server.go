package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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
    conn      net.Conn
    services  map[string]ServiceConfig
    lastPing  time.Time
    closed    atomic.Bool
    closeOnce sync.Once
    done      chan struct{}
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
		MinVersion:  tls.VersionTLS10,
		NextProtos:  []string{"http/1.1"},
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
	clientTunnel := s.tunnels[tunnelID]
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

	targetURL := &url.URL{
        Scheme:   "https",
        Host:     r.Host,
        Path:     r.URL.Path,
        RawPath:  r.URL.RawPath,
        RawQuery: r.URL.RawQuery,
        Fragment: r.URL.Fragment,
    }

	outReq, err := http.NewRequest(r.Method, targetURL.String(), r.Body)
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
		http.Error(w, fmt.Sprintf("error forwarding request in server: %v", err), http.StatusBadGateway)
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
    tunnelID := generateUniqueID()
    
    // Create new tunnel instance with monitoring channels
    clientTunnel := &ClientTunnel{
        conn:     conn,
        services: clientRequest.Tunnels,
        lastPing: time.Now(),
        done:     make(chan struct{}),
    }

    // Register tunnel and services
    s.registerTunnel(tunnelID, clientTunnel)

    // Send tunnel ID back to client
    if err := s.sendTunnelResponse(conn, tunnelID); err != nil {
        s.cleanupTunnel(tunnelID, clientTunnel)
        log.Printf("Failed to send tunnel response: %v", err)
        return
    }

    // Start tunnel monitoring
    go s.monitorTunnel(tunnelID, clientTunnel)

    // Start heartbeat checker
    go s.checkTunnelHeartbeat(tunnelID, clientTunnel)

    // Wait for tunnel termination
    <-clientTunnel.done
}

func (s *Server) registerTunnel(tunnelID string, tunnel *ClientTunnel) {
    s.mu.Lock()
    defer s.mu.Unlock()

    s.tunnels[tunnelID] = tunnel
    for _, service := range tunnel.services {
        s.subdomainMap[service.Domain] = tunnelID
    }
    
    log.Printf("Tunnel established - ID: %s, Services: %v", tunnelID, tunnel.services)
}

func (s *Server) sendTunnelResponse(conn net.Conn, tunnelID string) error {
    response := RequestResponse{
        Tunnel_ID: tunnelID,
        Error:     "",
    }

    // Set write deadline for response
    if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
        return fmt.Errorf("failed to set write deadline: %w", err)
    }
    defer conn.SetWriteDeadline(time.Time{})

    if err := json.NewEncoder(conn).Encode(response); err != nil {
        return fmt.Errorf("failed to encode response: %w", err)
    }

    return nil
}

func (s *Server) monitorTunnel(tunnelID string, tunnel *ClientTunnel) {
    defer s.cleanupTunnel(tunnelID, tunnel)

    // Create a buffer pool for better memory management
    bufferPool := sync.Pool{
        New: func() interface{} {
            return make([]byte, 32*1024) // 32KB buffer
        },
    }

    for {
        buffer := bufferPool.Get().([]byte)
        
        // Set read deadline for periodic connection check
        if err := tunnel.conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
            if !isConnectionClosed(err) {
                log.Printf("Failed to set read deadline for tunnel %s: %v", tunnelID, err)
            }
            bufferPool.Put(&buffer)
            return
        }

        _, err := tunnel.conn.Read(buffer)
        bufferPool.Put(&buffer)

        if err != nil {
            if !isConnectionClosed(err) && !strings.Contains(err.Error(), "timeout") {
                log.Printf("Tunnel %s read error: %v", tunnelID, err)
            }
            return
        }

        // Update last ping time
        tunnel.lastPing = time.Now()
    }
}

func (s *Server) checkTunnelHeartbeat(tunnelID string, tunnel *ClientTunnel) {
    ticker := time.NewTicker(15 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-tunnel.done:
            return
        case <-ticker.C:
            if time.Since(tunnel.lastPing) > 45*time.Second {
                log.Printf("Tunnel %s heartbeat timeout", tunnelID)
                s.cleanupTunnel(tunnelID, tunnel)
                return
            }
        }
    }
}

func (s *Server) cleanupTunnel(tunnelID string, tunnel *ClientTunnel) {
    tunnel.closeOnce.Do(func() {
        // Mark as closed
        tunnel.closed.Store(true)

        // Close the connection
        if err := tunnel.conn.Close(); err != nil && !isConnectionClosed(err) {
            log.Printf("Error closing tunnel %s connection: %v", tunnelID, err)
        }

        // Clean up tunnel mappings
        s.mu.Lock()
        delete(s.tunnels, tunnelID)
        for domain, id := range s.subdomainMap {
            if id == tunnelID {
                delete(s.subdomainMap, domain)
            }
        }
        s.mu.Unlock()

        // Signal tunnel is done
        close(tunnel.done)

        log.Printf("Tunnel %s closed and cleaned up", tunnelID)
    })
}

func generateUniqueID() string {
	return uuid.NewString()
}

func (s *Server) handleClientRequest(conn net.Conn, msg string) {
	s.mu.Lock()
	tunnelID := msg
	s.mu.Unlock()

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
    _, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Create channels for signaling when each direction is done
    clientDone := make(chan error, 1)
    tunnelDone := make(chan error, 1)

    // Copy from client to tunnel
    go func() {
        defer func() {
            // Half-close the tunnel connection (if it implements CloseWrite)
            if conn, ok := tunnelConn.(interface{ CloseWrite() error }); ok {
                _ = conn.CloseWrite()
            }
        }()

        _, err := io.Copy(tunnelConn, clientConn)
        if err != nil {
            if !errors.Is(err, io.EOF) && !isConnectionClosed(err) {
                clientDone <- fmt.Errorf("client->tunnel error: %w", err)
                return
            }
        }
        clientDone <- nil
    }()

    // Copy from tunnel to client
    go func() {
        defer func() {
            // Half-close the client connection (if it implements CloseWrite)
            if conn, ok := clientConn.(interface{ CloseWrite() error }); ok {
                _ = conn.CloseWrite()
            }
        }()

        _, err := io.Copy(clientConn, tunnelConn)
        if err != nil {
            if !errors.Is(err, io.EOF) && !isConnectionClosed(err) {
                tunnelDone <- fmt.Errorf("tunnel->client error: %w", err)
                return
            }
        }
        tunnelDone <- nil
    }()

    // Helper function for graceful connection closure
    closeConn := func(conn net.Conn) {
        if conn != nil {
            _ = conn.Close()
        }
    }

    // Wait for both directions to complete and handle any errors
    var clientErr, tunnelErr error
    for i := 0; i < 2; i++ {
        select {
        case clientErr = <-clientDone:
            if clientErr != nil {
                cancel() // Signal other goroutine to stop
                closeConn(tunnelConn)
            }
        case tunnelErr = <-tunnelDone:
            if tunnelErr != nil {
                cancel() // Signal other goroutine to stop
                closeConn(clientConn)
            }
        }
    }

    // Ensure both connections are closed
    closeConn(clientConn)
    closeConn(tunnelConn)

    // Log any non-connection-closed errors
    if clientErr != nil && !isConnectionClosed(clientErr) {
        log.Println("Error in client->tunnel direction", "error", clientErr)
    }
    if tunnelErr != nil && !isConnectionClosed(tunnelErr) {
        log.Println("Error in tunnel->client direction", "error", tunnelErr)
    }
}

// isConnectionClosed checks if the error is related to a closed connection
// isConnectionClosed checks various connection closure conditions
func isConnectionClosed(err error) bool {
    if err == nil {
        return false
    }

    return errors.Is(err, io.EOF) ||
        errors.Is(err, net.ErrClosed) ||
        strings.Contains(err.Error(), "use of closed network connection") ||
        strings.Contains(err.Error(), "connection reset by peer") ||
        strings.Contains(err.Error(), "broken pipe")
}

func (s *Server) pipe(dst, src net.Conn, direction string, done chan struct{}) {
	defer func() {
		log.Printf("Exiting pipe function for %s", direction)
		select {
		case <-done:
			log.Printf("Done channel already closed for %s", direction)
		default:
			log.Printf("Closing done channel for %s", direction)
			close(done)
		}
	}()

	buf := make([]byte, 32*1024) // 32KB buffer
	for {
		nr, err := src.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading in %s: %v", direction, err)
			} else if ne, ok := err.(net.Error); ok && ne.Timeout() {
				log.Printf("Timeout error in %s: %v", direction, err)
				continue
			} else {
				log.Printf("Error reading in %s: %v", direction, err)
			}
			return
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
	}
}