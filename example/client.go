package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"
)

func main() {
	// Connect to RPC server
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		log.Fatalf("Failed to connect to RPC server: %v", err)
	}
	defer conn.Close()

	fmt.Println("Connected to RPC server on localhost:8080")
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

func sendAndReceive(conn net.Conn, message map[string]interface{}) {
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(message); err != nil {
		log.Printf("Failed to send message: %v", err)
		return
	}
}

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

func prettyPrint(data interface{}) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Println(string(b))
}
