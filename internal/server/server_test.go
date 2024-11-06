package server

import (
	"encoding/json"
	"net"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	s := New()
	if s == nil {
		t.Fatal("New() returned nil")
	}
	if s.tunnels == nil {
		t.Error("tunnels map not initialized")
	}
	if s.subdomainMap == nil {
		t.Error("subdomainMap not initialized")
	}
}

func TestGenerateUniqueID(t *testing.T) {
	id1 := generateUniqueID()
	id2 := generateUniqueID()
	if id1 == "" || id2 == "" {
		t.Error("generateUniqueID() returned empty string")
	}
	if id1 == id2 {
		t.Error("generateUniqueID() returned duplicate IDs")
	}
}

func TestEstablishTunnel(t *testing.T) {
	s := New()
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	clientRequest := ReceivedRequest{
		Api_key: "test_api_key",
		Message: "TUNNEL_REQUEST",
		Tunnels: map[string]ServiceConfig{
			"test": {
				Name:   "test",
				Domain: "test.example.com",
				Proto:  "http",
				Addr:   8080,
			},
		},
	}

	go s.establishTunnel(serverConn, clientRequest)

	var response RequestResponse
	err := json.NewDecoder(clientConn).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Tunnel_ID == "" {
		t.Error("No tunnel ID received")
	}
	if response.Error != "" {
		t.Errorf("Unexpected error: %s", response.Error)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tunnels[response.Tunnel_ID]; !exists {
		t.Error("Tunnel not added to server's tunnels map")
	}

	if _, exists := s.subdomainMap["test.example.com"]; !exists {
		t.Error("Subdomain not added to server's subdomainMap")
	}
}

func TestHandleClientRequest(t *testing.T) {
	s := New()
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	tunnelID := generateUniqueID()
	s.tunnels[tunnelID] = &ClientTunnel{conn: serverConn}

	go s.handleClientRequest(clientConn, tunnelID)

	// Simulate sending data through the tunnel
	go func() {
		_, err := serverConn.Write([]byte("Hello, client!"))
		if err != nil {
			t.Errorf("Failed to write to server conn: %v", err)
		}
	}()

	buffer := make([]byte, 1024)
	n, err := clientConn.Read(buffer)
	if err != nil {
		t.Fatalf("Failed to read from client conn: %v", err)
	}

	received := string(buffer[:n])
	expected := "Hello, client!"
	if received != expected {
		t.Errorf("Expected %q, got %q", expected, received)
	}
}

func TestHandleInvalidClientRequest(t *testing.T) {
	s := New()
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	invalidTunnelID := "invalid-id"

	go s.handleClientRequest(serverConn, invalidTunnelID)

	buffer := make([]byte, 1024)
	n, err := clientConn.Read(buffer)
	if err != nil {
		t.Fatalf("Failed to read from client conn: %v", err)
	}

	received := string(buffer[:n])
	expected := "Provided an invalid tunnel ID"
	if received != expected {
		t.Errorf("Expected %q, got %q", expected, received)
	}
}

func TestHandleConnection(t *testing.T) {
	s := New()
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	go s.handleConnection(serverConn)

	tunnelRequest := ReceivedRequest{
		Api_key: "test_api_key",
		Message: "TUNNEL_REQUEST",
		Tunnels: map[string]ServiceConfig{
			"test": {
				Name:   "test",
				Domain: "test.example.com",
				Proto:  "http",
				Addr:   8080,
			},
		},
	}

	err := json.NewEncoder(clientConn).Encode(tunnelRequest)
	if err != nil {
		t.Fatalf("Failed to encode tunnel request: %v", err)
	}

	var response RequestResponse
	err = json.NewDecoder(clientConn).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Tunnel_ID == "" {
		t.Error("No tunnel ID received")
	}
	if response.Error != "" {
		t.Errorf("Unexpected error: %s", response.Error)
	}

	// Allow some time for the server to process the request
	time.Sleep(100 * time.Millisecond)

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tunnels[response.Tunnel_ID]; !exists {
		t.Error("Tunnel not added to server's tunnels map")
	}

	if _, exists := s.subdomainMap["test.example.com"]; !exists {
		t.Error("Subdomain not added to server's subdomainMap")
	}
}