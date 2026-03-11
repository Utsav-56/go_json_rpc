// Package main provides an example client implementation for the JSON-RPC server.
// It demonstrates how to connect to the server, send commands, receive responses,
// and handle real-time notifications from the server.
// The client shows examples of starting processes, querying status, retrieving logs,
// and calling custom events.
// NOTE: This file should be built separately from main.go
// Build with: go build -o client client.go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"
)

// connectToServer connects to the RPC server based on configuration.
// It supports both TCP and Unix socket/named pipe connections.
// Parameters:
//   - useTcp: whether to use TCP connection
//   - address: TCP address (e.g., "localhost:8080") or Unix socket/pipe path
//
// Returns the network connection or an error.
func connectToServer(useTcp bool, address string) (net.Conn, error) {
	if useTcp {
		return net.Dial("tcp", address)
	}
	// For Unix systems, use "unix" network
	// For Windows, use the named pipe path directly with "tcp" would require different handling
	// This example focuses on TCP for cross-platform compatibility
	return net.Dial("unix", address)
}

// main connects to the RPC server and demonstrates various client operations.
// It starts a background goroutine to receive notifications and then sends
// a series of example commands including starting a process, checking status,
// calling a custom event, and stopping the process.
func main() {
	// Configure connection - change these values based on server configuration
	useTcp := true
	address := "localhost:8080" // For TCP
	// address := "/tmp/go_rpc.sock" // For Unix socket (Linux/Mac)
	// address := `\\.\pipe\go_rpc` // For Windows named pipe

	// Connect to RPC server
	conn, err := connectToServer(useTcp, address)
	if err != nil {
		log.Fatalf("Failed to connect to RPC server: %v", err)
	}
	defer conn.Close()

	fmt.Printf("Connected to RPC server at %s\n", address)
	fmt.Println("--------------------------------------------------")

	// Start receiving notifications in background
	go receiveNotifications(conn)

	// Give some time for goroutine to start
	time.Sleep(100 * time.Millisecond)

	// Example 1: Start a process
	fmt.Println("\n1. Starting a process...")
	startCmd := map[string]interface{}{
		"id":   "req-001",
		"type": "command",
		"params": map[string]interface{}{
			"action": "start",
			"process": map[string]interface{}{
				"name":    "test-echo",
				"command": "bash",
				"args":    []string{"-c", "for i in {1..5}; do echo \"Line $i\"; sleep 1; done"},
			},
		},
	}
	sendAndReceive(conn, startCmd)

	// Wait for some logs
	time.Sleep(3 * time.Second)

	// Example 2: Get process status
	fmt.Println("\n2. Getting process status...")
	statusCmd := map[string]interface{}{
		"id":   "req-002",
		"type": "command",
		"params": map[string]interface{}{
			"action": "get_status",
			"name":   "test-echo",
		},
	}
	sendAndReceive(conn, statusCmd)

	// Example 3: Call custom event
	fmt.Println("\n3. Calling health_check event...")
	healthCmd := map[string]interface{}{
		"id":   "req-003",
		"type": "event",
		"params": map[string]interface{}{
			"name": "health_check",
		},
	}
	sendAndReceive(conn, healthCmd)

	// Wait for process to complete
	time.Sleep(3 * time.Second)

	// Example 4: Stop the process
	fmt.Println("\n4. Stopping process...")
	stopCmd := map[string]interface{}{
		"id":   "req-004",
		"type": "command",
		"params": map[string]interface{}{
			"action": "stop",
			"name":   "test-echo",
		},
	}
	sendAndReceive(conn, stopCmd)

	// Example 5: Get all statuses
	fmt.Println("\n5. Getting all process statuses...")
	allStatusCmd := map[string]interface{}{
		"id":   "req-005",
		"type": "command",
		"params": map[string]interface{}{
			"action": "get_status",
		},
	}
	sendAndReceive(conn, allStatusCmd)

	// Keep connection alive to receive final notifications
	fmt.Println("\nWaiting for final notifications...")
	time.Sleep(2 * time.Second)
}

// sendAndReceive encodes and sends a message to the server.
// This is a helper function that handles JSON encoding and error logging.
// Parameters:
//   - conn: the network connection to the server
//   - message: a map representing the JSON message to send
func sendAndReceive(conn net.Conn, message map[string]interface{}) {
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(message); err != nil {
		log.Printf("Failed to send message: %v", err)
		return
	}
}

// receiveNotifications continuously reads and handles messages from the server.
// This function runs in a goroutine to receive notifications without blocking the main thread.
// Running it in a goroutine allows the client to both send requests and receive asynchronous
// notifications simultaneously. It distinguishes between notification messages (pushed by the server)
// and response messages (replies to client requests) and formats them appropriately.
// Parameters:
//   - conn: the network connection to read from
func receiveNotifications(conn net.Conn) {
	decoder := json.NewDecoder(conn)

	for {
		var rawMsg map[string]interface{}
		if err := decoder.Decode(&rawMsg); err != nil {
			log.Printf("Connection closed or error: %v", err)
			return
		}

		msgType, _ := rawMsg["type"].(string)

		if msgType == "notification" {
			// Handle notification
			params, _ := rawMsg["params"].(map[string]interface{})
			notifType, _ := params["type"].(string)
			data := params["data"]

			switch notifType {
			case "log":
				logData, _ := data.(map[string]interface{})
				processName, _ := logData["process_name"].(string)
				logType, _ := logData["type"].(string)
				logMsg, _ := logData["log"].(string)
				fmt.Printf("📝 [LOG] %s (%s): %s\n", processName, logType, logMsg)

			case "status_change":
				statusData, _ := data.(map[string]interface{})
				processName, _ := statusData["name"].(string)
				status, _ := statusData["status"].(string)
				fmt.Printf("🔄 [STATUS] %s -> %s\n", processName, status)
			}
		} else {
			// Handle response
			prettyPrint(rawMsg)
		}
	}
}

// prettyPrint formats and displays data as indented JSON for better readability.
// This helper function makes it easier to read complex response structures.
// Parameters:
//   - data: the data to format and print
func prettyPrint(data interface{}) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Println(string(b))
}
