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
	// ServerAddr = "api.webbe.dev:443"
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
	Subdomain string `json:"subdomain"`
	LocalAddr string `json:"local_addr"`
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
	defer conn.Close()

	request := TunnelRequest{
        APIKey: c.config.ApiKey,
		Message: "TUNNEL_REQUEST",
        Subdomain: fmt.Sprintf("%s", c.config.Tunnels["Addr"]),
        LocalAddr: fmt.Sprintf("%s:%d", c.config.Tunnels["Domain"], c.config.Tunnels["Proto"]),
    }

	if err := json.NewEncoder(conn).Encode(request); err != nil {
		return fmt.Errorf("failed to send tunnel request: %w", err)
	}

	// current error happens with reading the tunnel id in the response here
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

func (c *Client) handleTunnel(serverConn net.Conn) error {
	for {
		var request struct {
			ServiceName string `json:"service_name"`
		}

		if err := json.NewDecoder(serverConn).Decode(&request); err != nil {
			if err == io.EOF {
				return fmt.Errorf("remote closed the connection")
			}
			return fmt.Errorf("failed to read server request %w", err)
		}

		tunnelConfig, ok := c.config.Tunnels[request.ServiceName]
		if !ok {
			log.Printf("warning, received request for unknown service %s", request.ServiceName)
			continue
		}

		localAddr := fmt.Sprintf("%s.local:%s", tunnelConfig.Domain, tunnelConfig.Proto)
		localConn, err := net.Dial("tcp", localAddr)
		if err != nil {
			log.Printf("failed to connect to local service %s, %v", request.ServiceName, err)
			continue
		}

		go c.handleConnection(serverConn, localConn)
	}
}

func (c *Client) handleConnection(serverConn net.Conn, localConn net.Conn) {
	defer localConn.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if _, err := io.Copy(localConn, serverConn); err != nil {
			log.Printf("error in server -> local: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		if _, err := io.Copy(serverConn, localConn); err != nil {
			log.Printf("error in local -> server: %v", err)
		}
	}()
}

// func (c *Client) handleTunnelOld(serverConn net.Conn) error {
// 	// log.Println(serverConn.)
// 	localConn, err := net.Dial("tcp", fmt.Sprintf("%s", c.config.Tunnels["Proto"]))
// 	if err != nil {
// 		return fmt.Errorf("failed to collect to local service: %w", err)
// 	}
// 	defer localConn.Close()

// 	var wg sync.WaitGroup
// 	wg.Add(2)

// 	// Server to local
// 	go func() {
// 		defer wg.Done()
// 		if _, err := io.Copy(localConn, serverConn); err != nil {
// 			log.Printf("Error in server -> local: %v", err)
// 		}
// 	}()

// 	go func() {
// 		defer wg.Done()
// 		if _, err := io.Copy(serverConn, localConn); err != nil {
// 			log.Printf("Error in local -> tunnel: %v", err)
// 		}
// 	}()

// 	wg.Wait()
// 	return nil
// }