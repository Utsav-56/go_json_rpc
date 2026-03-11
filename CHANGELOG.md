# Changelog

## [2.0.0] - 2026-03-11

### Breaking Changes

- **Constructor Signature Change**: `NewRpcServer` now accepts only `RpcServerConfig` instead of separate port and context parameters
   - Old: `NewRpcServer(port int, ctx context.Context) *RpcServer`
   - New: `NewRpcServer(config RpcServerConfig) *RpcServer`

### Added

- **RPC Client Library**: Complete client implementation with comprehensive features:
   - `RpcClient` struct with configuration-based initialization
   - Non-blocking, handler-based API for all operations
   - `SendCommand()` and `SendEvent()` methods with response handlers
   - `SetNotificationHandler()` for real-time notification streaming
   - `SetErrorHandler()` for centralized error handling
   - Automatic request ID generation and response tracking
   - Context-based lifecycle with `Start()` and `StartWithContext()`
   - Graceful shutdown with `Shutdown()` method
   - `IsConnected()` status checking
   - Cross-platform support (TCP, Unix sockets, Windows pipes)
   - Thread-safe operations with proper synchronization
   - Platform-specific implementations in `client_unix.go` and `client_windows.go`

- **Client Configuration Structure**:

   ```go
   type RpcClientConfig struct {
       UseTcp   bool   // Use TCP (true) or local pipes (false)
       Address  string // TCP address (when UseTcp is true)
       PipeName string // Socket/pipe name (when UseTcp is false)
   }
   ```

- **Cross-Platform Communication Support**:
   - TCP connections for network access (works on all platforms)
   - Unix sockets for Linux/Mac (faster local communication)
   - Named pipes for Windows (native Windows IPC)
- **Context-Based Lifecycle Management**:
   - New `StartWithContext(ctx context.Context)` method for better control over server lifecycle
   - Graceful shutdown when context is cancelled
   - The existing `Start()` method now internally calls `StartWithContext` with `context.Background()`

- **Enhanced Shutdown Capabilities**:
   - New `Shutdown()` method for explicit graceful shutdown
   - Properly closes listener to stop accepting new connections
   - Waits for all active connection handlers to complete
   - Cleans up Unix sockets and other resources
   - Handles Windows named pipes cleanup

- **Server Configuration Structure**:

   ```go
   type RpcServerConfig struct {
       UseTcp   bool   // Use TCP (true) or local pipes (false)
       Port     int    // TCP port (when UseTcp is true)
       PipeName string // Socket/pipe name (when UseTcp is false)
   }
   ```

- **Internal Improvements**:
   - Added `listener` field to RpcServer for shutdown management
   - Added `shutdownWg` WaitGroup to track active connections
   - Added `cancel` context function for internal cancellation
   - Separate `serve_unix.go` and `serve_windows.go` for platform-specific implementations

### Changed

- Server now properly initializes process manager with context in `StartWithContext`
- Accept loop runs in separate goroutine to allow context-based shutdown
- Connection handlers are tracked via WaitGroup for graceful shutdown
- Error handling differentiates between shutdown and actual errors

### Fixed

- Resource leaks during server shutdown
- Unix socket files not being cleaned up on shutdown
- Race conditions during concurrent shutdown
- Better handling of context cancellation throughout the server

### Migration Guide

**Step 1: Update Server Creation**

```go
// Before
ctx := context.Background()
server := rpc.NewRpcServer(8080, ctx)

// After
config := rpc.RpcServerConfig{
    UseTcp:   true,
    Port:     8080,
    PipeName: "",
}
server := rpc.NewRpcServer(config)
```

**Step 2: Update Server Startup (Recommended)**

```go
// Before
server.Start() // Blocks forever

// After (with graceful shutdown)
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

go func() {
    if err := server.StartWithContext(ctx); err != nil {
        log.Printf("Server error: %v", err)
    }
}()

// Handle signals for graceful shutdown
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
<-sigChan

cancel()
server.Shutdown()
```

**Step 3: Optional - Use RPC Client Library**

```go
// Create client
client := rpc.NewRpcClient(rpc.RpcClientConfig{
    UseTcp:  true,
    Address: "localhost:8080",
})

// Set handlers
client.SetNotificationHandler(func(notif rpc.Notification) {
    log.Printf("Notification: %+v", notif)
})

// Connect
if err := client.StartWithContext(ctx); err != nil {
    log.Fatal(err)
}
defer client.Shutdown()

// Send commands (non-blocking)
client.SendCommand(rpc.CommandParams{
    Action: rpc.CommandTypeStart,
    Process: &cmd.ProcessRequest{Name: "test", Command: "echo", Args: []string{"hello"}},
}, func(resp rpc.Response) {
    log.Printf("Response: %+v", resp)
})
```

**Step 4: Optional - Use Local Pipes for Better Performance**

```go
// Unix/Linux
config := rpc.RpcServerConfig{
    UseTcp:   false,
    Port:     0,
    PipeName: "/tmp/my_app.sock",
}

// Windows
config := rpc.RpcServerConfig{
    UseTcp:   false,
    Port:     0,
    PipeName: "my_app", // Becomes \\.\pipe\my_app
}
```

### Examples Added

- **example/advanced_client.go**: Comprehensive client example demonstrating:
   - Handler-based non-blocking operations
   - Notification streaming with dedicated handler
   - Error handling with error handler
   - Graceful shutdown with signal handling
   - All command types (start, stop, get_status, get_logs)
   - Custom event invocation
   - Pretty-printed responses

### Platform-Specific Notes

**Linux/Mac (Unix Sockets)**:

- Socket file is automatically removed on server start
- Socket file is cleaned up during graceful shutdown
- Faster than TCP for local communication
- Server: `net.Listen("unix", "/tmp/my_app.sock")`
- Client: `net.Dial("unix", "/tmp/my_app.sock")` or use `RpcClient`

**Windows (Named Pipes)**:

- Uses `github.com/Microsoft/go-winio` for pipe support
- Pipe names automatically prefixed with `\\.\pipe\`
- Server: `winio.ListenPipe(`\\.\pipe\my_app`, nil)`
- Client: `winio.DialPipe(`\\.\pipe\my_app`, nil)` or use `RpcClient`

**Cross-Platform (TCP)**:

- Works on all platforms without additional dependencies
- Required for network communication
- Server: `net.Listen("tcp", ":8080")`
- Client: `net.Dial("tcp", "localhost:8080")` or use `RpcClient`

### Examples Updated

- [example/main.go](example/main.go): Updated server with graceful shutdown
- [example/client.go](example/client.go): Added connection configuration options
- Both examples now show signal handling and proper cleanup

### Documentation Updated

- README.md: Complete rewrite with new API documentation
- Added configuration examples for all platforms
- Added graceful shutdown examples
- Added migration guide from v1.x
- Added "Recent Changes" section

## [1.x.x] - Previous Versions

Initial implementation with:

- TCP-only server support
- Basic process management
- Real-time log streaming
- Custom event handlers
- Multi-client support
