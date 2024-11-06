package client

import (
	"encoding/json"
	"net"
	"testing"
	"time"
	"fmt"

	"github.com/tmunongo/sslwarp/internal/config"
)

func TestNew(t *testing.T) {
	cfg := &config.Config{
		ApiKey: "test-api-key",
		Tunnels: map[string]config.TunnelConfig{
			"test-service": {
				Domain: "test.com",
				Proto:  "http",
			},
		},
	}

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New() returned an error: %v", err)
	}

	if client.config != cfg {
		t.Errorf("New() did not set the config correctly")
	}

	if client.tunnelID != "" {
		t.Errorf("New() should not set a tunnelID")
	}
}

func TestEstablishAndMaintainTunnel(t *testing.T) {
	// Create a mock server
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to create mock server: %v", err)
	}
	defer listener.Close()

	// ServerAddr = listener.Addr().String()

	// Start mock server
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			t.Errorf("Mock server failed to accept connection: %v", err)
			return
		}
		defer conn.Close()

		// Read request
		var request TunnelRequest
		if err := json.NewDecoder(conn).Decode(&request); err != nil {
			t.Errorf("Mock server failed to decode request: %v", err)
			return
		}

		// Send response
		response := struct {
			TunnelID string `json:"tunnel_id"`
			Error    string `json:"error,omitempty"`
		}{
			TunnelID: "test-tunnel-id",
		}
		if err := json.NewEncoder(conn).Encode(response); err != nil {
			t.Errorf("Mock server failed to send response: %v", err)
			return
		}

		// Keep connection open for a short while
		time.Sleep(100 * time.Millisecond)
	}()

	// Create client
	cfg := &config.Config{
		ApiKey: "test-api-key",
		Tunnels: map[string]config.TunnelConfig{
			"test-service": {
				Domain: "test.com",
				Proto:  "http",
			},
		},
	}
	client, _ := New(cfg)

	// Run establishAndMaintainTunnel in a goroutine
	errChan := make(chan error)
	go func() {
		errChan <- client.establishAndMaintainTunnel()
	}()

	// Wait for the function to complete or timeout
	select {
	case err := <-errChan:
		if err != nil {
			t.Errorf("establishAndMaintainTunnel() returned an error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Errorf("establishAndMaintainTunnel() timed out")
	}

	if client.tunnelID != "test-tunnel-id" {
		t.Errorf("establishAndMaintainTunnel() did not set the correct tunnelID")
	}
}

func TestIsConnectionClosed(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "non-OpError",
			err:  fmt.Errorf("some error"),
			want: false,
		},
		{
			name: "closed connection OpError",
			err:  &net.OpError{Err: fmt.Errorf("use of closed network connection")},
			want: true,
		},
		{
			name: "other OpError",
			err:  &net.OpError{Err: fmt.Errorf("some other network error")},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isConnectionClosed(tt.err); got != tt.want {
				t.Errorf("isConnectionClosed() = %v, want %v", got, tt.want)
			}
		})
	}
}