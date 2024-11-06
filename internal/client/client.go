package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
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
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    defer serverConn.Close()
    defer localConn.Close()

    errChan := make(chan error, 2)

    copy := func(dst, src net.Conn, direction string) {
        defer func() {
            // Attempt half-close if supported
            if closer, ok := dst.(interface{ CloseWrite() error }); ok {
                _ = closer.CloseWrite()
            }
            // Signal completion
            errChan <- nil
        }()

        buf := make([]byte, 32*1024) // 32KB buffer for better performance
        for {
            select {
            case <-ctx.Done():
                return
            default:
                nr, err := src.Read(buf)
                if err != nil {
                    if !isConnectionClosed(err) && err != io.EOF {
                        errChan <- fmt.Errorf("%s read error: %v", direction, err)
                    }
                    return
                }
                if nr > 0 {
                    nw, err := dst.Write(buf[0:nr])
                    if err != nil {
                        if !isConnectionClosed(err) {
                            errChan <- fmt.Errorf("%s write error: %v", direction, err)
                        }
                        return
                    }
                    if nw != nr {
                        errChan <- fmt.Errorf("%s incomplete write: %d != %d", direction, nw, nr)
                        return
                    }
                }
            }
        }
    }

    // Start the copy operations
    go copy(localConn, serverConn, "server->local")
    go copy(serverConn, localConn, "local->server")

    // Wait for both copies to complete or error
    for i := 0; i < 2; i++ {
        if err := <-errChan; err != nil {
            // Only log if it's not a normal closure
            if !isConnectionClosed(err) {
                log.Printf("Connection error: %v", err)
            }
            // Cancel context to signal other goroutine to stop
            cancel()
        }
    }
}

// isConnectionClosed checks various types of connection closure errors
func isConnectionClosed(err error) bool {
    if err == nil {
        return false
    }

    // Check for various forms of connection closure
    if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
        return true
    }

    errStr := err.Error()
    if strings.Contains(errStr, "use of closed network connection") ||
        strings.Contains(errStr, "broken pipe") ||
        strings.Contains(errStr, "connection reset by peer") ||
        strings.Contains(errStr, "io: read/write on closed pipe") {
        return true
    }

    // Check for operation errors
    if opErr, ok := err.(*net.OpError); ok {
        return opErr.Err.Error() == "use of closed network connection" ||
            strings.Contains(opErr.Err.Error(), "broken pipe") ||
            strings.Contains(opErr.Err.Error(), "connection reset by peer")
    }

    return false
}