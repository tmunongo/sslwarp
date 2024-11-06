package main

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestLoadEnv(t *testing.T) {
	// Create a temporary .env file for testing
	err := os.WriteFile(".env", []byte("TEST_VAR=test_value"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test .env file: %v", err)
	}
	defer os.Remove(".env")

	// Call the main function (which loads the .env file)
	oldArgs := os.Args
	os.Args = []string{"cmd", "-server=false", "-config=config.example.yml"}
	defer func() { os.Args = oldArgs }()

	go main()

	// Allow some time for the .env file to be loaded
	time.Sleep(100 * time.Millisecond)

	// Check if the environment variable was loaded
	if os.Getenv("TEST_VAR") != "test_value" {
		t.Errorf("Environment variable TEST_VAR not loaded correctly")
	}
}

func TestRunServer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan bool)
	go func() {
		runServer(ctx)
		done <- true
	}()

	select {
	case <-done:
		t.Error("runServer returned unexpectedly")
	case <-ctx.Done():
		// This is the expected behavior
	}
}

func TestRunClient(t *testing.T) {
	// Create a temporary config file for testing
	configContent := `
server_address: "localhost:8080"
local_address: "localhost:8081"
`
	err := os.WriteFile("test_config.yml", []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}
	defer os.Remove("test_config.yml")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan bool)
	go func() {
		runClient(ctx, "test_config.yml")
		done <- true
	}()

	select {
	case <-done:
		t.Error("runClient returned unexpectedly")
	case <-ctx.Done():
		// This is the expected behavior
	}
}

// func TestConfigLoad(t *testing.T) {
// 	// Create a temporary config file for testing
// 	configContent := `
// 	api_key: 123
// 	tunnels:
// 		service:
// 			name: service
// 			domain: scrollwize
// 			proto: "http"
// 			addr: 8000
// 	`
// 	err := os.WriteFile("test_config.yml", []byte(configContent), 0644)
// 	if err != nil {
// 		t.Fatalf("Failed to create test config file: %v", err)
// 	}
// 	defer os.Remove("test_config.yml")

// 	cfg, err := config.Load("test_config.yml")
// 	if err != nil {
// 		t.Fatalf("Failed to load config: %v", err)
// 	}

// 	if cfg.Tunnels["service"] != "localhost:8080" {
// 		t.Errorf("Expected ServerAddress to be 'localhost:8080', got '%s'", cfg.ServerAddress)
// 	}

// 	if cfg.LocalAddress != "localhost:8081" {
// 		t.Errorf("Expected LocalAddress to be 'localhost:8081', got '%s'", cfg.LocalAddress)
// 	}
// }