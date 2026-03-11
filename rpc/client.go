// Package rpc provides a JSON-RPC client implementation for connecting to RPC servers.
// The client supports context-based lifecycle management, non-blocking message handling,
// and dedicated handlers for responses, notifications, and errors.
package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
)

// ResponseHandler is called when a response is received for a sent request.
// Parameters:
//   - response: the Response received from the server
type ResponseHandler func(response Response)

// NotificationHandler is called when a notification is received from the server.
// Parameters:
//   - notification: the Notification received from the server
type NotificationHandler func(notification Notification)

// ErrorHandler is called when an error occurs during client operations.
// Parameters:
//   - err: the error that occurred
type ErrorHandler func(err error)

// RpcClient is a JSON-RPC client that connects to an RPC server.
// It provides non-blocking message handling through channels and supports
// both TCP and local pipe connections with graceful shutdown capabilities.
type RpcClient struct {
	// Config holds the client configuration for connection details.
	Config RpcClientConfig

	// conn is the network connection to the server.
	conn net.Conn

	// connMu protects access to the connection.
	connMu sync.RWMutex

	// ctx is the client context used for managing the client lifecycle.
	ctx context.Context

	// cancel is the context cancellation function for graceful shutdown.
	cancel context.CancelFunc

	// encoder encodes and sends messages to the server.
	encoder *json.Encoder

	// decoder decodes messages received from the server.
	decoder *json.Decoder

	// responseHandlers maps request IDs to their response handlers.
	responseHandlers map[string]ResponseHandler

	// handlersMu protects concurrent access to the handlers map.
	handlersMu sync.RWMutex

	// notificationHandler is called for all received notifications.
	notificationHandler NotificationHandler

	// errorHandler is called when errors occur.
	errorHandler ErrorHandler

	// requestIDCounter generates unique request IDs.
	requestIDCounter uint64

	// shutdownWg tracks active goroutines for graceful shutdown.
	shutdownWg sync.WaitGroup

	// connected indicates whether the client is currently connected.
	connected atomic.Bool
}

// RpcClientConfig configures how the client connects to the server.
// It mirrors the server configuration structure for consistency.
type RpcClientConfig struct {
	// UseTcp indicates whether to use TCP connections (true) or local pipes (false).
	UseTcp bool `json:"use_tcp"`

	// Address is the TCP address to connect to (e.g., "localhost:8080").
	// This is only used if UseTcp is true.
	Address string `json:"address"`

	// PipeName is the name of the local pipe or socket to connect to.
	// This is only used if UseTcp is false.
	// - Unix/Linux: Path like "/tmp/go_rpc.sock"
	// - Windows: Name like "go_rpc" (becomes \\.\pipe\go_rpc)
	PipeName string `json:"pipe_name"`
}

// NewRpcClient creates a new RPC client instance.
// It initializes the client with the given configuration but does not connect yet.
// Call Start() or StartWithContext() to establish the connection.
// Parameters:
//   - config: the configuration for the RPC client
//
// Returns a pointer to a new RpcClient ready to connect.
func NewRpcClient(config RpcClientConfig) *RpcClient {
	return &RpcClient{
		Config:           config,
		responseHandlers: make(map[string]ResponseHandler),
	}
}

// Validate checks if the client configuration is valid.
// Returns an error if the configuration is invalid.
func (c *RpcClientConfig) Validate() error {
	if c.UseTcp {
		if c.Address == "" {
			return fmt.Errorf("TCP address is required when UseTcp is true")
		}
	} else {
		if c.PipeName == "" {
			return fmt.Errorf("Pipe name is required when UseTcp is false")
		}
	}
	return nil
}

// SetNotificationHandler sets the handler for all incoming notifications.
// This handler will be called for every notification received from the server.
// Parameters:
//   - handler: the function to call when notifications are received
func (c *RpcClient) SetNotificationHandler(handler NotificationHandler) {
	c.notificationHandler = handler
}

// SetErrorHandler sets the handler for client errors.
// This handler will be called when errors occur during operations.
// Parameters:
//   - handler: the function to call when errors occur
func (c *RpcClient) SetErrorHandler(handler ErrorHandler) {
	c.errorHandler = handler
}

// Start connects to the server with a background context.
// This method internally calls StartWithContext with context.Background().
// For production use, consider using StartWithContext directly for better control.
// Returns an error if connection fails.
func (c *RpcClient) Start() error {
	return c.StartWithContext(context.Background())
}

// StartWithContext connects to the server and begins message processing.
// This method is non-blocking and starts goroutines to handle incoming messages.
// When the context is cancelled, the client performs a graceful shutdown.
// Parameters:
//   - ctx: context for managing client lifecycle and graceful shutdown
//
// Returns an error if the connection cannot be established.
func (c *RpcClient) StartWithContext(ctx context.Context) error {
	// Validate configuration
	if err := c.Config.Validate(); err != nil {
		return fmt.Errorf("invalid client configuration: %w", err)
	}

	// Create a cancellable context for the client
	c.ctx, c.cancel = context.WithCancel(ctx)

	// Establish connection
	var err error
	c.connMu.Lock()
	if c.Config.UseTcp {
		c.conn, err = net.Dial("tcp", c.Config.Address)
		if err != nil {
			c.connMu.Unlock()
			return fmt.Errorf("failed to connect to TCP address %s: %w", c.Config.Address, err)
		}
		log.Printf("Connected to RPC server at %s", c.Config.Address)
	} else {
		// Platform-specific connection
		c.conn, err = c.connectPipe()
		if err != nil {
			c.connMu.Unlock()
			return fmt.Errorf("failed to connect to pipe %s: %w", c.Config.PipeName, err)
		}
		log.Printf("Connected to RPC server via pipe: %s", c.Config.PipeName)
	}

	// Initialize encoder and decoder
	c.encoder = json.NewEncoder(c.conn)
	c.decoder = json.NewDecoder(c.conn)
	c.connected.Store(true)
	c.connMu.Unlock()

	// Start message receiver goroutine
	c.shutdownWg.Add(1)
	go c.receiveMessages()

	// Monitor context cancellation
	go func() {
		<-c.ctx.Done()
		log.Println("Client context cancelled, initiating shutdown...")
		c.Shutdown()
	}()

	return nil
}

// IsConnected returns whether the client is currently connected to the server.
func (c *RpcClient) IsConnected() bool {
	return c.connected.Load()
}

// Shutdown performs a graceful shutdown of the client.
// It closes the connection and waits for all goroutines to finish.
// Returns an error if cleanup encounters issues.
func (c *RpcClient) Shutdown() error {
	if !c.connected.Load() {
		return nil // Already disconnected
	}

	c.connected.Store(false)

	// Cancel the context
	if c.cancel != nil {
		c.cancel()
	}

	// Close the connection
	c.connMu.Lock()
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			log.Printf("Error closing connection: %v", err)
		}
		c.conn = nil
	}
	c.connMu.Unlock()

	// Wait for all goroutines to finish
	c.shutdownWg.Wait()

	log.Println("Client shutdown complete")
	return nil
}

// receiveMessages continuously reads and dispatches messages from the server.
// This runs in a goroutine and handles responses, notifications, and errors.
func (c *RpcClient) receiveMessages() {
	defer c.shutdownWg.Done()

	for {
		// Check if context is cancelled
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		// Read message
		var rawMsg map[string]interface{}
		if err := c.decoder.Decode(&rawMsg); err != nil {
			if c.connected.Load() {
				// Only report error if we're supposed to be connected
				if c.errorHandler != nil {
					c.errorHandler(fmt.Errorf("failed to decode message: %w", err))
				} else {
					log.Printf("Failed to decode message: %v", err)
				}
			}
			return
		}

		// Dispatch based on message type
		msgType, _ := rawMsg["type"].(string)
		switch msgType {
		case MessageTypeNotification:
			c.handleNotification(rawMsg)
		case ResponseTypeSuccess, ResponseTypeError:
			c.handleResponse(rawMsg)
		default:
			// Try to handle as response (responses don't always have explicit type)
			if _, hasID := rawMsg["id"]; hasID {
				c.handleResponse(rawMsg)
			}
		}
	}
}

// handleNotification processes incoming notifications from the server.
func (c *RpcClient) handleNotification(rawMsg map[string]interface{}) {
	// Convert to Notification struct
	msgBytes, err := json.Marshal(rawMsg)
	if err != nil {
		if c.errorHandler != nil {
			c.errorHandler(fmt.Errorf("failed to marshal notification: %w", err))
		}
		return
	}

	var notification Notification
	if err := json.Unmarshal(msgBytes, &notification); err != nil {
		if c.errorHandler != nil {
			c.errorHandler(fmt.Errorf("failed to unmarshal notification: %w", err))
		}
		return
	}

	// Call notification handler
	if c.notificationHandler != nil {
		c.notificationHandler(notification)
	}
}

// handleResponse processes incoming responses from the server.
func (c *RpcClient) handleResponse(rawMsg map[string]interface{}) {
	// Convert to Response struct
	msgBytes, err := json.Marshal(rawMsg)
	if err != nil {
		if c.errorHandler != nil {
			c.errorHandler(fmt.Errorf("failed to marshal response: %w", err))
		}
		return
	}

	var response Response
	if err := json.Unmarshal(msgBytes, &response); err != nil {
		if c.errorHandler != nil {
			c.errorHandler(fmt.Errorf("failed to unmarshal response: %w", err))
		}
		return
	}

	// Find and call the corresponding handler
	c.handlersMu.RLock()
	handler, exists := c.responseHandlers[response.ID]
	c.handlersMu.RUnlock()

	if exists {
		handler(response)
		// Remove the handler after calling it
		c.handlersMu.Lock()
		delete(c.responseHandlers, response.ID)
		c.handlersMu.Unlock()
	}
}

// generateRequestID generates a unique request ID for tracking responses.
func (c *RpcClient) generateRequestID() string {
	id := atomic.AddUint64(&c.requestIDCounter, 1)
	return fmt.Sprintf("req-%d", id)
}

// SendCommand sends a command to the server and registers a response handler.
// This is a non-blocking operation - the handler will be called when the response arrives.
// Parameters:
//   - cmdParams: the CommandParams containing the command details
//   - handler: the function to call when the response is received
//
// Returns the request ID and any error that occurred during sending.
func (c *RpcClient) SendCommand(cmdParams CommandParams, handler ResponseHandler) (string, error) {
	if !c.connected.Load() {
		return "", fmt.Errorf("client is not connected")
	}

	requestID := c.generateRequestID()

	msg := Message{
		ID:     requestID,
		Type:   MessageTypeCommand,
		Params: cmdParams,
	}

	// Register handler
	if handler != nil {
		c.handlersMu.Lock()
		c.responseHandlers[requestID] = handler
		c.handlersMu.Unlock()
	}

	// Send message
	c.connMu.RLock()
	err := c.encoder.Encode(msg)
	c.connMu.RUnlock()

	if err != nil {
		// Remove handler if send failed
		c.handlersMu.Lock()
		delete(c.responseHandlers, requestID)
		c.handlersMu.Unlock()
		return "", fmt.Errorf("failed to send command: %w", err)
	}

	return requestID, nil
}

// SendEvent sends a custom event to the server and registers a response handler.
// This is a non-blocking operation - the handler will be called when the response arrives.
// Parameters:
//   - eventParams: the EventParams containing the event details
//   - handler: the function to call when the response is received
//
// Returns the request ID and any error that occurred during sending.
func (c *RpcClient) SendEvent(eventParams EventParams, handler ResponseHandler) (string, error) {
	if !c.connected.Load() {
		return "", fmt.Errorf("client is not connected")
	}

	requestID := c.generateRequestID()

	msg := Message{
		ID:     requestID,
		Type:   MessageTypeEvent,
		Params: eventParams,
	}

	// Register handler
	if handler != nil {
		c.handlersMu.Lock()
		c.responseHandlers[requestID] = handler
		c.handlersMu.Unlock()
	}

	// Send message
	c.connMu.RLock()
	err := c.encoder.Encode(msg)
	c.connMu.RUnlock()

	if err != nil {
		// Remove handler if send failed
		c.handlersMu.Lock()
		delete(c.responseHandlers, requestID)
		c.handlersMu.Unlock()
		return "", fmt.Errorf("failed to send event: %w", err)
	}

	return requestID, nil
}

// SendMessage sends a raw message to the server.
// This is a lower-level method for advanced use cases.
// Parameters:
//   - msg: the Message to send
//
// Returns any error that occurred during sending.
func (c *RpcClient) SendMessage(msg Message) error {
	if !c.connected.Load() {
		return fmt.Errorf("client is not connected")
	}

	c.connMu.RLock()
	err := c.encoder.Encode(msg)
	c.connMu.RUnlock()

	return err
}
