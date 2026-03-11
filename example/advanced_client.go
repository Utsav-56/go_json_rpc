// Package main provides a comprehensive example of using the RPC client.
// It demonstrates connecting to the server, sending commands, handling responses,
// and receiving real-time notifications with graceful shutdown.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/utsav-56/go-json-rpc/cmd"
	"github.com/utsav-56/go-json-rpc/rpc"
)

func main() {
	// Create a context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Configure client connection
	// Option 1: TCP connection (cross-platform)
	config := rpc.RpcClientConfig{
		UseTcp:   true,
		Address:  "localhost:8080",
		PipeName: "",
	}

	// Option 2: Unix socket (Linux/Mac)
	// config := rpc.RpcClientConfig{
	// 	UseTcp:   false,
	// 	Address:  "",
	// 	PipeName: "/tmp/go_rpc.sock",
	// }

	// Option 3: Named pipe (Windows)
	// config := rpc.RpcClientConfig{
	// 	UseTcp:   false,
	// 	Address:  "",
	// 	PipeName: "go_rpc",
	// }

	// Create RPC client
	client := rpc.NewRpcClient(config)

	// Set up notification handler
	client.SetNotificationHandler(func(notification rpc.Notification) {
		handleNotification(notification)
	})

	// Set up error handler
	client.SetErrorHandler(func(err error) {
		log.Printf("Client error: %v", err)
	})

	// Start the client
	if err := client.StartWithContext(ctx); err != nil {
		log.Fatalf("Failed to start client: %v", err)
	}

	// Give the client a moment to connect
	time.Sleep(100 * time.Millisecond)

	fmt.Println("RPC Client Examples")
	fmt.Println("===================")

	// Example 1: Start a process
	fmt.Println("\n1. Starting a process...")
	startProcess(client)

	// Wait for process to generate some output
	time.Sleep(3 * time.Second)

	// Example 2: Get process status
	fmt.Println("\n2. Getting process status...")
	getProcessStatus(client, "test-process")

	// Example 3: Call custom event
	fmt.Println("\n3. Calling health_check event...")
	callHealthCheck(client)

	// Wait for process to complete
	time.Sleep(3 * time.Second)

	// Example 4: Stop the process
	fmt.Println("\n4. Stopping process...")
	stopProcess(client, "test-process")

	// Example 5: Get all process statuses
	fmt.Println("\n5. Getting all process statuses...")
	getAllStatuses(client)

	// Example 6: Get process logs
	fmt.Println("\n6. Getting process logs...")
	getProcessLogs(client, "test-process")

	fmt.Println("\nWaiting for more notifications...")
	fmt.Println("Press Ctrl+C to exit")

	// Wait for interrupt signal
	<-sigChan
	fmt.Println("\nShutting down client...")

	// Shutdown the client
	cancel()
	if err := client.Shutdown(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	fmt.Println("Client stopped")
}

// startProcess demonstrates starting a new process
func startProcess(client *rpc.RpcClient) {
	cmdParams := rpc.CommandParams{
		Action: rpc.CommandTypeStart,
		Process: &cmd.ProcessRequest{
			Name:    "test-process",
			Command: "bash",
			Args:    []string{"-c", "for i in {1..5}; do echo \"Output line $i\"; sleep 1; done"},
		},
	}

	_, err := client.SendCommand(cmdParams, func(response rpc.Response) {
		fmt.Printf("✓ Start Process Response:\n")
		printResponse(response)
	})

	if err != nil {
		log.Printf("Failed to send start command: %v", err)
	}
}

// stopProcess demonstrates stopping a running process
func stopProcess(client *rpc.RpcClient, processName string) {
	cmdParams := rpc.CommandParams{
		Action: rpc.CommandTypeStop,
		Name:   processName,
	}

	_, err := client.SendCommand(cmdParams, func(response rpc.Response) {
		fmt.Printf("✓ Stop Process Response:\n")
		printResponse(response)
	})

	if err != nil {
		log.Printf("Failed to send stop command: %v", err)
	}
}

// getProcessStatus demonstrates getting status of a specific process
func getProcessStatus(client *rpc.RpcClient, processName string) {
	cmdParams := rpc.CommandParams{
		Action: rpc.CommandTypeGetStatus,
		Name:   processName,
	}

	_, err := client.SendCommand(cmdParams, func(response rpc.Response) {
		fmt.Printf("✓ Process Status Response:\n")
		printResponse(response)
	})

	if err != nil {
		log.Printf("Failed to send get status command: %v", err)
	}
}

// getAllStatuses demonstrates getting status of all processes
func getAllStatuses(client *rpc.RpcClient) {
	cmdParams := rpc.CommandParams{
		Action: rpc.CommandTypeGetStatus,
		// Leave Name empty to get all statuses
	}

	_, err := client.SendCommand(cmdParams, func(response rpc.Response) {
		fmt.Printf("✓ All Process Statuses Response:\n")
		printResponse(response)
	})

	if err != nil {
		log.Printf("Failed to send get all statuses command: %v", err)
	}
}

// getProcessLogs demonstrates retrieving process logs
func getProcessLogs(client *rpc.RpcClient, processName string) {
	cmdParams := rpc.CommandParams{
		Action: rpc.CommandTypeGetLogs,
		Name:   processName,
	}

	_, err := client.SendCommand(cmdParams, func(response rpc.Response) {
		fmt.Printf("✓ Process Logs Response:\n")
		printResponse(response)
	})

	if err != nil {
		log.Printf("Failed to send get logs command: %v", err)
	}
}

// callHealthCheck demonstrates calling a custom event
func callHealthCheck(client *rpc.RpcClient) {
	eventParams := rpc.EventParams{
		Name: "health_check",
		Data: nil,
	}

	_, err := client.SendEvent(eventParams, func(response rpc.Response) {
		fmt.Printf("✓ Health Check Response:\n")
		printResponse(response)
	})

	if err != nil {
		log.Printf("Failed to send health check event: %v", err)
	}
}

// handleNotification processes incoming notifications from the server
func handleNotification(notification rpc.Notification) {
	params, ok := notification.Params.(map[string]interface{})
	if !ok {
		return
	}

	notifType, _ := params["type"].(string)
	data := params["data"]

	switch notifType {
	case rpc.NotificationTypeProcessLog:
		handleLogNotification(data)
	case rpc.NotificationTypeProcessStatusChanged:
		handleStatusChangeNotification(data)
	default:
		fmt.Printf("📢 [NOTIFICATION] %s: %+v\n", notifType, data)
	}
}

// handleLogNotification processes process log notifications
func handleLogNotification(data interface{}) {
	logData, ok := data.(map[string]interface{})
	if !ok {
		return
	}

	processName, _ := logData["process_name"].(string)
	logType, _ := logData["type"].(string)
	logMsg, _ := logData["log"].(string)

	icon := "📝"
	if logType == "stderr" {
		icon = "⚠️"
	}

	fmt.Printf("%s [LOG] %s (%s): %s\n", icon, processName, logType, logMsg)
}

// handleStatusChangeNotification processes process status change notifications
func handleStatusChangeNotification(data interface{}) {
	statusData, ok := data.(map[string]interface{})
	if !ok {
		return
	}

	processName, _ := statusData["name"].(string)
	status, _ := statusData["status"].(string)
	pid, _ := statusData["pid"].(float64)

	icon := "🔄"
	switch status {
	case "running":
		icon = "✅"
	case "stopped":
		icon = "🛑"
	}

	if pid > 0 {
		fmt.Printf("%s [STATUS] %s -> %s (PID: %.0f)\n", icon, processName, status, pid)
	} else {
		fmt.Printf("%s [STATUS] %s -> %s\n", icon, processName, status)
	}
}

// printResponse pretty-prints a response
func printResponse(response rpc.Response) {
	fmt.Printf("  ID: %s\n", response.ID)
	fmt.Printf("  Type: %s\n", response.Type)

	if response.Type == rpc.ResponseTypeError {
		if errResp, ok := response.Result.(map[string]interface{}); ok {
			fmt.Printf("  Error Type: %v\n", errResp["type"])
			fmt.Printf("  Error Message: %v\n", errResp["message"])
		} else {
			fmt.Printf("  Error: %+v\n", response.Result)
		}
	} else {
		if cmdResp, ok := response.Result.(map[string]interface{}); ok {
			fmt.Printf("  Message: %v\n", cmdResp["message"])
			if data, hasData := cmdResp["data"]; hasData && data != nil {
				fmt.Printf("  Data: %+v\n", data)
			}
		} else {
			fmt.Printf("  Result: %+v\n", response.Result)
		}
	}
	fmt.Println()
}
