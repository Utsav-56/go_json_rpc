# JSON-RPC Process Manager

A robust JSON-RPC server implementation in Go with process management capabilities, real-time log streaming, and status notifications.

## Features

**Thread-Safe Process Management**

- Each process runs independently with its own goroutines
- Proper mutex locking prevents race conditions
- Process isolation ensures one process cannot interfere with another

**Real-Time Notifications**

- Log streaming: Receive stdout/stderr as they happen
- Status updates: Get notified when processes start, run, or stop
- Broadcast to all connected clients

**Event Registry System**

- Register custom event handlers (e.g., `shutdown`, `download_file`)
- Flexible event handling with custom parameters
- Clean separation between commands and events

**Comprehensive Command Support**

- `start`: Launch processes with custom arguments
- `stop`: Gracefully terminate processes
- `get_status`: Query status of one or all processes
- `get_logs`: Retrieve process logs

**Reliable Status Tracking**

- **transitioning**: Process starting up
- **running**: Process active and executing
- **stopped**: Process terminated (with exit code)

## Architecture

```
┌─────────────────────┐
│   RPC Server        │
│  (Port 8080)        │
└──────────┬──────────┘
           │
           ├─── Event Registry (custom handlers)
           │
           ├─── Process Manager
           │      ├── Process 1 (isolated)
           │      ├── Process 2 (isolated)
           │      └── Process N (isolated)
           │
           └─── Client Connections
                 ├── Client 1 (notifications)
                 ├── Client 2 (notifications)
                 └── Client N (notifications)
```

## Package Structure

```
.
├── rpc/
│   ├── rpc.go        # RPC server implementation
│   └── message.go    # Message types and structures
├── cmd/
│   └── cmd.go        # Process management logic
└── example/
    ├── server/
    │   └── main.go   # Example server
    └── client/
        └── client.go # Example client
```

## Quick Start

### 1. Start the Server

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
        return map[string]string{"status": "ok"}, nil
    })

    log.Println("Starting RPC server on port 8080...")
    server.Start()
}
```

### 2. Connect and Send Commands

#### Start a Process

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
			"work_dir": "/path/to/project"
		}
	}
}
```

Response:

```json
{
	"id": "req-001",
	"type": "success",
	"result": {
		"message": "Process 'my-app' started successfully"
	}
}
```

#### Receive Log Notification

```json
{
	"type": "notification",
	"params": {
		"type": "log",
		"data": {
			"type": "stdout",
			"process_name": "my-app",
			"log": "Application started",
			"timestamp": 1678450805000
		}
	}
}
```

#### Receive Status Change Notification

```json
{
	"type": "notification",
	"params": {
		"type": "status_change",
		"data": {
			"name": "my-app",
			"pid": 12345,
			"start_time": 1678450800000,
			"status": "running"
		}
	}
}
```

## API Reference

### Message Types

1. **command**: Client → Server (expects response)
2. **event**: Client → Server (custom events, expects response)
3. **notification**: Server → Client (no response expected)

### Command Actions

| Action       | Parameters        | Description               |
| ------------ | ----------------- | ------------------------- |
| `start`      | `process`         | Start a new process       |
| `stop`       | `name`            | Stop a running process    |
| `get_status` | `name` (optional) | Get status of process(es) |
| `get_logs`   | `name`            | Retrieve process logs     |

### Notification Types

| Type            | Description                  |
| --------------- | ---------------------------- |
| `log`           | Process stdout/stderr output |
| `status_change` | Process status transition    |

## Thread Safety & Process Isolation

### How Thread Safety is Ensured

1. **Process Manager Level**
   - `sync.RWMutex` protects the process map
   - Read locks for queries, write locks for modifications

2. **Controlled Process Level**
   - Each process has its own `sync.RWMutex`
   - Protects status, logs, and logfile access
   - Callbacks executed **outside of locks** to prevent deadlocks

3. **Client Connection Level**
   - Each client connection has its own mutex
   - Prevents concurrent writes to the same connection

### Process Isolation

- Each process runs in its own goroutine context
- Separate `context.Context` with independent cancellation
- Isolated log buffers (last 100 lines in memory)
- Independent logfile handles
- No shared state between processes

### Callback Reliability

```go
// ❌ BAD: Holding lock during callback (can deadlock)
cp.mu.Lock()
cp.Status = newStatus
if cp.onStatusChange != nil {
    cp.onStatusChange(cp.Status)  // Deadlock risk!
}
cp.mu.Unlock()

// GOOD: Copy data, release lock, then callback
cp.mu.Lock()
cp.Status = newStatus
statusCopy := cp.Status
handler := cp.onStatusChange
cp.mu.Unlock()

if handler != nil {
    handler(statusCopy)  // Safe!
}
```

## Error Handling

### Error Types

- `invalid_message_format`: Malformed JSON or wrong structure
- `internal_error`: Process errors, not found, already exists, etc.
- `conversion_error`: Type casting failed

### Example Error Response

```json
{
	"id": "req-xxx",
	"type": "error",
	"result": {
		"type": "internal_error",
		"message": "process already exists",
		"timestamp": 1678450800
	}
}
```

## Building

```bash
# Build all packages
go build ./...

# Build server example
cd example/server
go build -o server

# Build client example
cd example/client
go build -o client
```

## Testing

Run the server:

```bash
./example/server/server
```

In another terminal, run the client:

```bash
./example/client/client
```

## Advanced Usage

### Custom Event Registration

```go
server.RegisterEvent("download_file", func(params interface{}) (interface{}, error) {
    data := params.(map[string]interface{})
    url := data["url"].(string)

    // Your download logic here

    return map[string]interface{}{
        "status": "started",
        "url": url,
    }, nil
})
```

### Process with Logfile

```json
{
	"action": "start",
	"process": {
		"name": "logger-app",
		"command": "./myapp",
		"logfile_path": "/var/log/myapp.log"
	}
}
```

## License

See LICENSE file for details.
