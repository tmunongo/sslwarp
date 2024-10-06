package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/tmunongo/sslwarp/internal/client"
	"github.com/tmunongo/sslwarp/internal/config"
	"github.com/tmunongo/sslwarp/internal/server"
	"github.com/tmunongo/sslwarp/internal/web"
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

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	if *serverMode {
		wg.Add(2)
		go func ()  {
			defer wg.Done()
			runServer(ctx)
		}()
		go func ()  {
			defer wg.Done()
			web.StartWeb(ctx, "server")
		}()
	} else {
		wg.Add(2)
		go func ()  {
			defer wg.Done()
			runClient(ctx, *configFile)
		}()
		go func ()  {
			defer wg.Done()
			web.StartWeb(ctx, "client")
		}()
	}

	// Wait for interrupt signal
    <-sigChan
    log.Println("Received interrupt signal. Shutting down...")
    cancel()

    // Wait for goroutines to finish with a timeout
    waitChan := make(chan struct{})
    go func() {
        wg.Wait()
        close(waitChan)
    }()

    select {
    case <-waitChan:
        log.Println("Graceful shutdown completed")
    case <-time.After(10 * time.Second):
        log.Println("Forced shutdown after timeout")
    }
}

func runServer(ctx context.Context) {
	fmt.Println("SSLWarp is listening in server mode")

	srv := server.New()
	 
	if err := srv.Run(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func runClient(ctx context.Context, configPath string) {
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
