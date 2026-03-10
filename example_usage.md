# RPC System Usage Guide

## Overview

This RPC system provides process management capabilities with real-time notifications for logs and status changes.

## Message Types

### 1. Commands

Commands are requests from client to server that expect a response.

### 2. Events

Events are custom user-defined actions handled by registered event handlers.

### 3. Notifications (Server to Client)

Notifications are server-initiated messages (no response expected) for logs and status changes.

## Command Examples

### Start a Process

**Request:**

```json
{
	"id": "req-001",
	"type": "command",
	"params": {
		"action": "start",
		"process": {
			"name": "my-app",
			"command": "python",
			"args": ["-u", "app.py"],
			"work_dir": "/path/to/project",
			"logfile_path": "/path/to/logs/app.log"
		}
	}
}
```

**Response:**

```json
{
	"id": "req-001",
	"type": "success",
	"result": {
		"message": "Process 'my-app' started successfully"
	}
}
```

### Stop a Process

**Request:**

```json
{
	"id": "req-002",
	"type": "command",
	"params": {
		"action": "stop",
		"name": "my-app"
	}
}
```

**Response:**

```json
{
	"id": "req-002",
	"type": "success",
	"result": {
		"message": "Process 'my-app' stopped successfully"
	}
}
```

### Get Process Status

**Request (specific process):**

```json
{
	"id": "req-003",
	"type": "command",
	"params": {
		"action": "get_status",
		"name": "my-app"
	}
}
```

**Response:**

```json
{
	"id": "req-003",
	"type": "success",
	"result": {
		"message": "Status for process 'my-app' retrieved",
		"data": {
			"name": "my-app",
			"pid": 12345,
			"start_time": 1678450800000,
			"status": "running"
		}
	}
}
```

**Request (all processes):**

```json
{
	"id": "req-004",
	"type": "command",
	"params": {
		"action": "get_status"
	}
}
```

**Response:**

```json
{
	"id": "req-004",
	"type": "success",
	"result": {
		"message": "All process statuses retrieved",
		"data": {
			"my-app": {
				"name": "my-app",
				"pid": 12345,
				"start_time": 1678450800000,
				"status": "running"
			},
			"worker": {
				"name": "worker",
				"pid": 12346,
				"start_time": 1678450900000,
				"status": "running"
			}
		}
	}
}
```

### Get Process Logs

**Request:**

```json
{
	"id": "req-005",
	"type": "command",
	"params": {
		"action": "get_logs",
		"name": "my-app"
	}
}
```

**Response:**

```json
{
	"id": "req-005",
	"type": "success",
	"result": {
		"message": "Logs for process 'my-app' retrieved",
		"data": [
			"[2026-03-10T10:30:00Z] stdout: Application starting...",
			"[2026-03-10T10:30:01Z] stdout: Server listening on port 8080",
			"[2026-03-10T10:30:05Z] stderr: Warning: config file not found"
		]
	}
}
```

## Event Examples

### Register and Call Custom Events

**Server Setup:**

```go
server.RegisterEvent("shutdown", func(params interface{}) (interface{}, error) {
    // Handle graceful shutdown
    return map[string]string{"status": "shutting down"}, nil
})

server.RegisterEvent("download_file", func(params interface{}) (interface{}, error) {
    data := params.(map[string]interface{})
    url := data["url"].(string)
    // Download file logic
    return map[string]string{"status": "downloading", "url": url}, nil
})
```

**Request:**

```json
{
	"id": "req-006",
	"type": "event",
	"params": {
		"name": "shutdown",
		"data": {
			"timeout": 30
		}
	}
}
```

**Response:**

```json
{
	"id": "req-006",
	"type": "success",
	"result": {
		"message": "Event 'shutdown' processed successfully",
		"data": {
			"status": "shutting down"
		}
	}
}
```

## Notification Examples (Server to Client)

### Log Notification

When a process outputs a log line, the server sends:

```json
{
	"type": "notification",
	"params": {
		"type": "log",
		"data": {
			"type": "stdout",
			"process_name": "my-app",
			"log": "Request processed successfully",
			"timestamp": 1678450805000
		}
	}
}
```

### Status Change Notification

When a process status changes, the server sends:

```json
{
	"type": "notification",
	"params": {
		"type": "status_change",
		"data": {
			"name": "my-app",
			"pid": 12345,
			"start_time": 1678450800000,
			"status": "stopped",
			"exit_code": 0
		}
	}
}
```

## Status Flow

1. **transitioning** - Process start command issued, but process not yet running
2. **running** - Process is actively running
3. **stopped** - Process has exited (with exit_code)

## Error Responses

**Format:**

```json
{
	"id": "req-xxx",
	"type": "error",
	"result": {
		"type": "error_type",
		"message": "Error description",
		"timestamp": 1678450800
	}
}
```

**Error Types:**

- `invalid_message_format` - Message structure is invalid
- `internal_error` - Server-side error (e.g., process already exists)
- `conversion_error` - Parameter type conversion failed

## Go Server Usage Example

```go
package main

import (
    "context"
    "log"

    "github.com/utsav-56/go-json-rpc/rpc"
)

func main() {
    ctx := context.Background()
    server := rpc.NewRpcServer(8080, ctx)

    // Register custom events
    server.RegisterEvent("shutdown", func(params interface{}) (interface{}, error) {
        log.Println("Shutdown event received")
        return map[string]string{"status": "ok"}, nil
    })

    server.RegisterEvent("download_file", func(params interface{}) (interface{}, error) {
        data := params.(map[string]interface{})
        url := data["url"].(string)
        log.Printf("Downloading file from: %s", url)
        return map[string]string{"status": "started", "url": url}, nil
    })

    // Start RPC server
    log.Println("Starting RPC server on port 8080...")
    server.Start()
}
```

## Key Features

**Thread-safe process management** - Each process runs independently
**Real-time log streaming** - Logs are sent as notifications immediately
**Status tracking** - Receive notifications when process status changes
**Process isolation** - One process cannot interfere with another
**Custom event registry** - Register and handle custom events
**Proper error handling** - Structured error responses with types
**Reliable callbacks** - No missed logs or status updates
