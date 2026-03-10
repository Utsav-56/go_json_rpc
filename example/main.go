package main

import (
	"context"
	"log"

	"github.com/utsav-56/go-json-rpc/rpc"
)

func main() {
	ctx := context.Background()

	// Create RPC server on port 8080
	server := rpc.NewRpcServer(8080, ctx)

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

	// Start the RPC server
	log.Println("Starting RPC server on port 8080...")
	log.Println("Registered events: shutdown, download_file, health_check")
	log.Println("Available commands: start, stop, get_status, get_logs")
	server.Start()
}
