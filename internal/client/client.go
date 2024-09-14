package client

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/tmunongo/sslwarp/internal/config"
)

const (
	// possible use env variables
	// ServerAddr = "api.sslwarp.com:443"
	ServerAddr = "localhost:8080"
)

type Client struct {
	//
	config *config.Config
	tunnelID string
}

func New(cfg *config.Config) (*Client, error) {
	return &Client {
		config: cfg,
	}, nil
}

type TunnelRequest struct {
	APIKey string `json:"api_key"`
	Tunnels map[string]config.TunnelConfig `json:"tunnels"`
	Message string `json:"message"`
}

func (c *Client) Run() error {
	for {
		if err := c.establishAndMaintainTunnel(); err != nil {
			log.Printf("Tunnel error: %v", err)
			log.Println("Retrying in 5 seconds...")
			time.Sleep(5 * time.Second)
			continue
		}
	}
}

func (c *Client) establishAndMaintainTunnel() error {
	// connect to server
	conn, err := net.Dial("tcp", ServerAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to the server: %w", err)
	}
	// defer conn.Close()

	log.Printf("config tunnels: %v", c.config.Tunnels)

	request := TunnelRequest{
        APIKey: c.config.ApiKey,
		Message: "TUNNEL_REQUEST",
		Tunnels: c.config.Tunnels,
    }

	if err := json.NewEncoder(conn).Encode(request); err != nil {
		return fmt.Errorf("failed to send tunnel request: %w", err)
	}

	var response struct {

		TunnelID string `json:"tunnel_id"`
		Error string `json:"error",omitempty`
	}
	if err := json.NewDecoder(conn).Decode(&response); err != nil {
		return fmt.Errorf("failed to read server response: %w", err)
	}

	if response.Error != "" {
		return fmt.Errorf("server error: %s", response.Error)
	}

	c.tunnelID = response.TunnelID
	log.Printf("Tunnel established with ID: %s", c.tunnelID)

	return c.handleTunnel(conn)	
}

type ServiceConfig struct {
	Name string
	Domain string
	Proto string
	Addr int
}

func (c *Client) handleTunnel(serverConn net.Conn) error {
	defer serverConn.Close()

	for {
		var request struct {
			Service ServiceConfig `json:"service"`
		}

		if err := json.NewDecoder(serverConn).Decode(&request); err != nil {
			if err == io.EOF {
				return fmt.Errorf("remote closed the connection")
			}
			return fmt.Errorf("failed to read server request %w", err)
		}

		log.Printf("service name: %s", request.Service.Name)

		tunnelConfig, ok := c.config.Tunnels[request.Service.Name]
		if !ok {
			log.Printf("warning, received request for unknown service %s", request.Service.Name)
			continue
		}

		localAddr := fmt.Sprintf("%s.sslwarp.local:%s", tunnelConfig.Domain, tunnelConfig.Proto)
		localConn, err := net.Dial("tcp", localAddr)
		if err != nil {
			log.Printf("failed to connect to local service %s, %v", request.Service.Name, err)
			continue
		}

		go c.handleConnection(serverConn, localConn)

		break
	}

	return nil
}

func (c *Client) handleConnection(serverConn net.Conn, localConn net.Conn) {
	defer localConn.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	copy := func (dst, src net.Conn)  {
		defer wg.Done()

		_, err := io.Copy(dst, src)
		if err != nil {
			if err != io.EOF && !isConnectionClosed(err) {
				log.Printf("error in connection: %v", err)
			}
		}
		dst.Close()
		src.Close()
	}

	go copy(localConn, serverConn)
	go copy(serverConn, localConn)

	wg.Wait()
}

func isConnectionClosed(err error) bool {
	if err == nil {
		return false
	}
	if opErr, ok := err.(*net.OpError); ok {
		return opErr.Err.Error() == "use of closed network connection"
	}
	return false
}