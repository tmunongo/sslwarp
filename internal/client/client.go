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
		return fmt.Errorf("Failed to connect to the server: %w", err)
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
	localConn, err := net.Dial("tcp", fmt.Sprintf("%s", c.config.Tunnels["Addr"]))
	if err != nil {
		return fmt.Errorf("Failed to collect to local service: %w", err)
	}
	defer localConn.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	// Server to local
	go func() {
		defer wg.Done()
		if _, err := io.Copy(localConn, serverConn); err != nil {
			log.Printf("Error in server -> local: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		if _, err := io.Copy(serverConn, localConn); err != nil {
			log.Printf("Error in local -> tunnel: %v", err)
		}
	}()

	wg.Wait()
	return nil
}