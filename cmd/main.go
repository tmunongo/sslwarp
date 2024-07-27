package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/joho/godotenv"
	"github.com/tmunongo/sslwarp/internal/client"
	"github.com/tmunongo/sslwarp/internal/config"
	"github.com/tmunongo/sslwarp/internal/server"
)

func main() {
	// load env
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	serverMode := flag.Bool("server", false, "Start sslwarp in server mode")
	configFile := flag.String("config", "config.example.yml", "Path to your configuration file")

	flag.Parse()

	if *serverMode {
		runServer()
	} else {
		runClient(*configFile)
	}
}

func runServer() {
	fmt.Println("SSLWarp is listening on port :8000")

	srv := server.New()
	 
	if err := srv.Run(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func runClient(configPath string) {
	fmt.Println("SSLWarp is running in client mode")

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	cli, err := client.New(cfg)
	if err != nil {
		log.Fatalf("Failed to start client with config: %v", err)
	}

	if err := cli.Run(); err != nil {
		log.Fatalf("Client error: %v", err)
	}
}
