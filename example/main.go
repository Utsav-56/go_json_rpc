// Package main provides an example server implementation using the JSON-RPC framework.
// It demonstrates how to create an RPC server, register custom event handlers,
// and start the server to handle client connections.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/utsav-56/go-json-rpc/rpc"
)

// main initializes and starts the JSON-RPC server with example event handlers.
// It demonstrates registering three custom events: shutdown, download_file, and health_check.
// The server listens on port 8080 and supports both custom events and built-in commands
// for process management (start, stop, get_status, get_logs).
func main() {
	// Create a context that can be cancelled on interrupt signals
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Create RPC server configuration
	config := rpc.RpcServerConfig{
		UseTcp:   true,
		Port:     8080,
		PipeName: "", // Not used when UseTcp is true
	}

	// Create RPC server with configuration
	server := rpc.NewRpcServer(config)

	// Register custom event handlers
	server.RegisterEvent("shutdown", func(params interface{}) (interface{}, error) {
		log.Println("Shutdown event received")
		// Perform graceful shutdown logic here
		return map[string]interface{}{
			"status":  "shutting_down",
			"message": "Server is gracefully shutting down",
		}, nil
	})

	server.RegisterEvent("download_file", func(params interface{}) (interface{}, error) {
		data, ok := params.(map[string]interface{})
		if !ok {
			return nil, nil
		}

		url, _ := data["url"].(string)
		destination, _ := data["destination"].(string)

		log.Printf("Download request - URL: %s, Destination: %s", url, destination)

		// Simulate download logic
		return map[string]interface{}{
			"status":      "started",
			"url":         url,
			"destination": destination,
			"progress":    0,
		}, nil
	})

	server.RegisterEvent("health_check", func(params interface{}) (interface{}, error) {
		return map[string]interface{}{
			"status":  "healthy",
			"uptime":  "5h 32m",
			"version": "1.0.0",
		}, nil
	})

	// Start the RPC server in a goroutine
	go func() {
		log.Println("Starting RPC server on port 8080...")
		log.Println("Registered events: shutdown, download_file, health_check")
		log.Println("Available commands: start, stop, get_status, get_logs")
		if err := server.StartWithContext(ctx); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-sigChan
	log.Println("Interrupt signal received, shutting down...")

	// Cancel the context to trigger graceful shutdown
	cancel()

	// Optionally call Shutdown explicitly for immediate cleanup
	if err := server.Shutdown(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("Server stopped")
}
