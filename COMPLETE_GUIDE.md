# Complete JSON-RPC System Guide

## Overview

This is a complete client-server JSON-RPC system for process management with real-time notifications.

- **Server**: Go implementation with thread-safe process management
- **Client**: Dart implementation with class-based architecture and async/await

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│                    Go RPC Server                         │
│  ┌────────────────────────────────────────────────┐     │
│  │  Event Registry (shutdown, download_file, etc) │     │
│  └────────────────────────────────────────────────┘     │
│  ┌────────────────────────────────────────────────┐     │
│  │  Process Manager (thread-safe, isolated)       │     │
│  │  - Process 1 (status, logs, callbacks)         │     │
│  │  - Process 2 (status, logs, callbacks)         │     │
│  │  - Process N (status, logs, callbacks)         │     │
│  └────────────────────────────────────────────────┘     │
│  ┌────────────────────────────────────────────────┐     │
│  │  Client Connections (broadcast notifications)  │     │
│  └────────────────────────────────────────────────┘     │
└──────────────────────────────────────────────────────────┘
                        ↕ TCP/JSON ↕
┌──────────────────────────────────────────────────────────┐
│                   Dart RPC Client                        │
│  ┌────────────────────────────────────────────────┐     │
│  │  Response Handler Registry                     │     │
│  │  - onSuccess, onError                          │     │
│  │  - onInvalidFormat, onInternalError            │     │
│  └────────────────────────────────────────────────┘     │
│  ┌────────────────────────────────────────────────┐     │
│  │  Notification Listener Registry                │     │
│  │  - onLog (stdout/stderr)                       │     │
│  │  - onStatusChange (process state)              │     │
│  └────────────────────────────────────────────────┘     │
│  ┌────────────────────────────────────────────────┐     │
│  │  Command/Event Methods                         │     │
│  │  - startProcess, stopProcess                   │     │
│  │  - getStatus, getLogs                          │     │
│  │  - sendEvent, healthCheck                      │     │
│  └────────────────────────────────────────────────┘     │
└──────────────────────────────────────────────────────────┘
```

## Message Flow

### 1. Command Flow (Request-Response)

```
Client                          Server
  │                               │
  ├─── start_process command ────>│
  │                               ├─ Validate
  │                               ├─ Start process
  │                               ├─ Setup callbacks
  │<─── success response ─────────┤
  │                               │
  │<─── notification: status ─────┤ (transitioning)
  │<─── notification: status ─────┤ (running)
  │<─── notification: log ────────┤ (stdout)
  │<─── notification: log ────────┤ (stdout)
  │                               │
```

### 2. Event Flow (Request-Response)

```
Client                          Server
  │                               │
  ├─── health_check event ───────>│
  │                               ├─ Find handler
  │                               ├─ Execute handler
  │<─── success response ─────────┤
  │     (with handler result)     │
  │                               │
```

### 3. Notification Flow (Server-Initiated)

```
Client                          Server
  │                               │
  │                               │ Process outputs log
  │<─── notification: log ────────┤
  │                               │
  │                               │ Process status changes
  │<─── notification: status ─────┤
  │                               │
```

## Quick Start

### 1. Start the Go Server

```bash
cd example/server
go build -o server
./server
```

Output:

```
Starting RPC server on port 8080...
Registered events: shutdown, download_file, health_check
Available commands: start, stop, get_status, get_logs
RPC Server started on port 8080
```

### 2. Run the Dart Client

```bash
cd dart_client
dart pub get
dart run example/example.dart
```

Output:

```
Connecting to RPC server...
Connected successfully!
============================================================

1️⃣  Starting a process...
------------------------------------------------------------
🟡 [STATUS] test-echo -> transitioning
Process 'test-echo' started successfully
🟢 [STATUS] test-echo -> running
📝 [LOG] test-echo (stdout): Line 1
📝 [LOG] test-echo (stdout): Line 2
...
```

## Protocol Specification

### Message Types

#### 1. Command (Client → Server)

```json
{
  "id": "req-001",
  "type": "command",
  "params": {
    "action": "start|stop|get_status|get_logs",
    "process": { ... },  // For start
    "name": "..."        // For stop, get_status, get_logs
  }
}
```

**Actions:**

- `start`: Launch a process
- `stop`: Terminate a process
- `get_status`: Query process status
- `get_logs`: Retrieve process logs

#### 2. Event (Client → Server)

```json
{
  "id": "req-002",
  "type": "event",
  "params": {
    "name": "event_name",
    "data": { ... }
  }
}
```

**Predefined Events:**

- `shutdown`: Graceful server shutdown
- `download_file`: Download file request
- `health_check`: Server health check

#### 3. Response (Server → Client)

**Success:**

```json
{
  "id": "req-001",
  "type": "success",
  "result": {
    "message": "...",
    "data": { ... }
  }
}
```

**Error:**

```json
{
	"id": "req-001",
	"type": "error",
	"result": {
		"type": "invalid_message_format|internal_error|conversion_error",
		"message": "...",
		"timestamp": 1678450800
	}
}
```

#### 4. Notification (Server → Client)

```json
{
  "type": "notification",
  "params": {
    "type": "log|status_change",
    "data": { ... }
  }
}
```

**Log Notification:**

```json
{
	"type": "notification",
	"params": {
		"type": "log",
		"data": {
			"type": "stdout|stderr",
			"process_name": "my-app",
			"log": "Application started",
			"timestamp": 1678450805000
		}
	}
}
```

**Status Change Notification:**

```json
{
	"type": "notification",
	"params": {
		"type": "status_change",
		"data": {
			"name": "my-app",
			"pid": 12345,
			"start_time": 1678450800000,
			"status": "running|stopped|transitioning",
			"exit_code": 0 // Only present when stopped
		}
	}
}
```

## Go Server Implementation

### Key Features

1. **Thread Safety**
   - `sync.RWMutex` for process manager
   - Individual mutex per process
   - Callbacks executed outside locks

2. **Process Isolation**
   - Independent goroutines
   - Separate context cancellation
   - No shared state

3. **Reliable Callbacks**
   - Status copied before callback
   - Handler reference captured
   - Lock released before invocation

### Example: Register Event

```go
server.RegisterEvent("custom_event", func(params interface{}) (interface{}, error) {
    data := params.(map[string]interface{})

    // Your logic here

    return map[string]interface{}{
        "status": "processed",
        "data": data,
    }, nil
})
```

## Dart Client Implementation

### Key Features

1. **Response Handler Registry**

   ```dart
   client.onSuccess((response) { ... });
   client.onError((response) { ... });
   client.onInvalidFormat((response) { ... });
   client.onInternalError((response) { ... });
   ```

2. **Notification Listeners**

   ```dart
   client.onLog((log) {
     print('[${log.processName}] ${log.log}');
   });

   client.onStatusChange((status) {
     print('${status.name} -> ${status.status}');
   });
   ```

3. **Async/Await API**

   ```dart
   final response = await client.startProcess(request);
   if (response.isSuccess) {
     print('Started!');
   }
   ```

4. **Type Safety**
   - All types strongly typed
   - Null safety support
   - Type-safe conversions

### Example: Full Workflow

```dart
import 'package:rpc_client/rpc_client.dart';

void main() async {
  final client = RpcClient(host: 'localhost', port: 8080);

  try {
    // Connect
    await client.connect();

    // Register handlers
    client.onLog((log) => print(log));
    client.onStatusChange((status) => print(status));
    client.onSuccess((response) => print('${response.successData?.message}'));
    client.onError((response) => print('❌ ${response.error?.message}'));

    // Start process
    final process = ProcessRequest(
      name: 'worker',
      command: 'python',
      args: ['-u', 'worker.py'],
      workDir: '/path/to/project',
    );

    await client.startProcess(process);

    // Wait for logs...
    await Future.delayed(Duration(seconds: 5));

    // Get status
    final status = await client.getProcessStatus('worker');
    print(status.successData?.data);

    // Stop process
    await client.stopProcess('worker');

  } finally {
    await client.disconnect();
  }
}
```

## Error Handling

### Server-Side (Go)

```go
// Returns error response
if _, exists := pm.processes[name]; !exists {
    return fmt.Errorf("process not found")
}

// Handled in RPC layer and sent as ErrorResponse
```

### Client-Side (Dart)

```dart
try {
  final response = await client.startProcess(request);

  if (response.isSuccess) {
    // Success path
    print(response.successData?.message);
  } else {
    // Error path
    final error = response.error!;

    switch (error.type) {
      case ErrorType.invalidFormat:
        print('Invalid message');
        break;
      case ErrorType.internalError:
        print('Server error: ${error.message}');
        break;
      case ErrorType.conversionError:
        print('Type error');
        break;
    }
  }
} on SocketException catch (e) {
  print('Connection error: $e');
} catch (e) {
  print('Unexpected error: $e');
}
```

## Performance Characteristics

### Go Server

- O(1) process lookup (map-based)
- Non-blocking I/O
- Concurrent client handling
- Minimal lock contention
- Efficient log buffering (last 100 lines)

### Dart Client

- Stream-based socket reading
- Efficient buffer management (StringBuffer)
- Async/await non-blocking
- No external dependencies
- Small memory footprint

## Testing

### Manual Testing

1. **Start Server**

   ```bash
   cd example/server && ./server
   ```

2. **Run Client Example**

   ```bash
   cd dart_client && dart run example/example.dart
   ```

3. **Expected Flow**
   - Connection established
   - Process started (see status: transitioning → running)
   - Logs streamed in real-time
   - Status changes received
   - Process stopped (see status: stopped with exit code)

### Using netcat (Low-Level Testing)

```bash
# Connect to server
nc localhost 8080

# Send start command
{"id":"1","type":"command","params":{"action":"start","process":{"name":"test","command":"echo","args":["hello"]}}}

# Receive responses and notifications
```

## Best Practices

### Server

1. Register events before starting server
2. Use context for graceful shutdown
3. Handle process cleanup properly
4. Log errors appropriately

### Client

1. Always call `disconnect()` in finally block
2. Register handlers before connecting
3. Check response types before accessing data
4. Handle both success and error cases
5. Clear handlers when reusing client

## File Structure

```
go_json_rpc/
├── rpc/                    # RPC server package
│   ├── rpc.go             # Server implementation
│   └── message.go         # Message types
├── cmd/                    # Process management
│   └── cmd.go             # Process manager
├── example/               # Go examples
│   ├── server/
│   │   └── main.go
│   └── client/
│       └── client.go
├── dart_client/           # Dart client package
│   ├── lib/
│   │   ├── rpc_client.dart  # Main client
│   │   ├── types.dart       # Type definitions
│   │   └── handlers.dart    # Handler registries
│   ├── example/
│   │   └── example.dart     # Complete example
│   ├── pubspec.yaml
│   ├── analysis_options.yaml
│   └── README.md
├── go.mod
├── RPC_IMPLEMENTATION.md
└── example_usage.md
```

## Common Issues & Solutions

### Issue: Connection Refused

**Solution**: Ensure Go server is running on specified port

### Issue: No Notifications Received

**Solution**: Register handlers before sending commands

### Issue: Process Already Exists

**Solution**: Each process name must be unique

### Issue: Dart Analysis Warnings

**Solution**: Run `dart fix --apply` to auto-fix

## License

See LICENSE file for details.
