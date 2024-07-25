package main

import (
	"fmt"
	"flag"
)

func main() {
	fmt.Println("hello world")

	serverMode := flag.Bool("server", false, "Start sslwarp in server mode")
	configFile := flag.String("config", "config.yml", "Path to your configuration file")

	flag.Parse()

	if *serverMode {
		runServer()
	} else {
		runClient()
	}
}

func runServer() {
	fmt.Println("SSLWarp is listening on port :8000")

	srv := server.New()

	if err != srv.Run(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

fun runClient() {
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
