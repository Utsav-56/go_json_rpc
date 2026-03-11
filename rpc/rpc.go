// Package rpc provides a JSON-RPC server implementation for managing processes and handling custom events.
// It allows clients to send commands to start and stop processes, query their status, retrieve logs,
// and invoke custom event handlers. The server broadcasts notifications to all connected clients
// when processes generate logs or change status.
package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/utsav-56/go-json-rpc/cmd"
)

// rpcFunc is a function type for handling custom RPC events.
// It accepts arbitrary parameters and returns a result or an error.
// Event handlers registered with the server must match this signature.
// Parameters:
//   - params: the event parameters passed from the client (can be any type)
//
// Returns:
//   - interface{}: the result to send back to the client
//   - error: an error if the event handling failed
type rpcFunc func(params interface{}) (interface{}, error)

// RpcServer is a JSON-RPC server that manages processes and handles custom events.
// It listens for client connections, processes incoming messages, and broadcasts notifications.
// The server supports both commands for process management and custom event handlers.
type RpcServer struct {
	// port is the TCP port number where the server listens for connections.
	port int

	// eventHandlers maps event names to their handler functions.
	// Custom events can be registered to extend the server functionality.
	eventHandlers map[string]rpcFunc

	// pcsManager handles the lifecycle of all managed processes.
	// It provides methods to start, stop, and monitor processes.
	pcsManager *cmd.ProcessManager

	// ctx is the server context used for managing the server lifecycle.
	ctx context.Context

	// connections holds all active client connections for broadcasting.
	// Each connection has its own encoder for sending notifications.
	connections map[net.Conn]*ClientConnection

	// connMu protects concurrent access to the connections map.
	// This ensures thread-safe operations when clients connect or disconnect.
	connMu sync.RWMutex
}

// NewRpcServer creates a new RPC server instance.
// It initializes the process manager and connection tracking.
// Parameters:
//   - port: the TCP port number where the server will listen
//   - ctx: the context for managing server and process lifecycles
//
// Returns a pointer to a new RpcServer ready to accept connections.
func NewRpcServer(port int, ctx context.Context) *RpcServer {
	return &RpcServer{
		port:        port,
		pcsManager:  cmd.NewProcessManager(ctx),
		ctx:         ctx,
		connections: make(map[net.Conn]*ClientConnection),
	}
}

// RegisterEvent registers a custom event handler with the server.
// Clients can trigger these handlers by sending event messages with the registered name.
// If an event with the same name was already registered, it will be replaced.
// Parameters:
//   - event: the name of the event that clients will use to invoke this handler
//   - handler: the function to call when this event is received
func (s *RpcServer) RegisterEvent(event string, handler rpcFunc) {
	if s.eventHandlers == nil {
		s.eventHandlers = make(map[string]rpcFunc)
	}
	s.eventHandlers[event] = handler
}

// SendResponse encodes and sends a response message to a client.
// This is a helper method for sending structured responses over a connection.
// If encoding fails, the error is logged but the connection remains open.
// Parameters:
//   - conn: the network connection to send the response to
//   - response: the Response struct to encode and send
func (s *RpcServer) SendResponse(conn net.Conn, response Response) {

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// SendError creates and sends an error response to a client.
// This is a convenience method for quickly sending error messages.
// Parameters:
//   - conn: the network connection to send the error to
//   - errMsg: the error message text to include in the response
//   - errType: the error type constant (ErrorTypeInvalidFormat, ErrorTypeInternalError, etc)
func (s *RpcServer) SendError(conn net.Conn, errMsg string, errType string) {

	errorResponse := Response{
		ID:   "",
		Type: ResponseTypeError,
		Result: ErrorResponse{
			Type:      errType,
			Message:   errMsg,
			Timestamp: time.Now().Unix(),
		},
	}
	s.SendResponse(conn, errorResponse)
}

// Start begins listening for client connections on the configured port.
// This method blocks and runs the server loop indefinitely.
// For each incoming connection, it spawns a goroutine to handle that client.
// The goroutine approach allows multiple clients to connect simultaneously and operate independently.
// Each client connection is handled concurrently without blocking other clients.
// The server will log a fatal error and exit if it cannot bind to the port.
func (s *RpcServer) Start() {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer ln.Close()
	log.Printf("RPC Server started on port %d", s.port)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}
		go s.handleConnection(conn)
	}

}

// handleConnection processes messages from a single client connection.
// This runs in a goroutine because each client connection needs to be handled concurrently.
// Running it in a goroutine allows the server to accept and serve multiple clients at the same time.
// It registers the connection for notifications, reads messages in a loop, processes them,
// and cleans up when the client disconnects.
// Parameters:
//   - conn: the network connection to the client
func (s *RpcServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Register client connection
	clientConn := &ClientConnection{
		conn:    conn,
		encoder: json.NewEncoder(conn),
	}

	s.connMu.Lock()
	s.connections[conn] = clientConn
	s.connMu.Unlock()

	defer func() {
		s.connMu.Lock()
		delete(s.connections, conn)
		s.connMu.Unlock()
	}()

	decoder := json.NewDecoder(conn)

	for {
		var msg Message
		if err := decoder.Decode(&msg); err != nil {
			log.Printf("Failed to decode message: %v", err)
			return
		}
		log.Printf("Received message: %+v", msg)

		response := s.handleMessage(msg, clientConn)
		if err := clientConn.encoder.Encode(response); err != nil {
			log.Printf("Failed to encode response: %v", err)
			return
		}
	}
}

// handleMessage processes a decoded message and routes it to the appropriate handler.
// It first validates the message type by calling TypeCast, then dispatches to either
// event or command handlers based on the message type.
// Parameters:
//   - msg: the decoded Message from the client
//   - clientConn: the ClientConnection for sending responses
//
// Returns a Response struct to send back to the client.
func (s *RpcServer) handleMessage(msg Message, clientConn *ClientConnection) Response {
	errResp := msg.TypeCast()
	if errResp != nil {
		return Response{
			ID:     msg.ID,
			Type:   ResponseTypeError,
			Result: *errResp,
		}
	}

	switch msg.Type {
	case MessageTypeEvent:
		return s.handleEvent(msg)
	case MessageTypeCommand:
		return s.handleCommand(msg, clientConn)
	default:
		return Response{
			ID:   msg.ID,
			Type: ResponseTypeError,
			Result: ErrorResponse{
				Type:      ErrorTypeInvalidFormat,
				Message:   fmt.Sprintf("Unknown message type: %s", msg.Type),
				Timestamp: time.Now().Unix(),
			},
		}
	}
}

// handleEvent processes custom event messages from clients.
// It looks up the registered handler for the event name and invokes it with the event data.
// If no handler is registered for the event name, an error response is returned.
// Parameters:
//   - msg: the Message containing event parameters
//
// Returns a Response with the handler result or an error.
func (s *RpcServer) handleEvent(msg Message) Response {
	eventParams, ok := msg.Params.(EventParams)
	if !ok {
		return Response{
			ID:     msg.ID,
			Type:   ResponseTypeError,
			Result: NewErrorResponse(ErrorTypeConversionError, "Invalid event params"),
		}
	}

	handler, exists := s.eventHandlers[eventParams.Name]
	if !exists {
		return Response{
			ID:     msg.ID,
			Type:   ResponseTypeError,
			Result: NewErrorResponse(ErrorTypeInvalidFormat, fmt.Sprintf("Event '%s' not registered", eventParams.Name)),
		}
	}

	result, err := handler(eventParams.Data)
	if err != nil {
		return Response{
			ID:     msg.ID,
			Type:   ResponseTypeError,
			Result: NewErrorResponse(ErrorTypeInternalError, err.Error()),
		}
	}

	return Response{
		ID:   msg.ID,
		Type: ResponseTypeSuccess,
		Result: CommandResponse{
			Message: fmt.Sprintf("Event '%s' processed successfully", eventParams.Name),
			Data:    result,
		},
	}
}

// handleCommand processes command messages for process management.
// It routes the command to the appropriate handler based on the action type:
// start, stop, get_status, or get_logs.
// Parameters:
//   - msg: the Message containing command parameters
//   - clientConn: the ClientConnection for sending notifications
//
// Returns a Response with the command result or an error.
func (s *RpcServer) handleCommand(msg Message, clientConn *ClientConnection) Response {
	cmdParams, ok := msg.Params.(CommandParams)
	if !ok {
		return Response{
			ID:     msg.ID,
			Type:   ResponseTypeError,
			Result: NewErrorResponse(ErrorTypeConversionError, "Invalid command params"),
		}
	}

	switch cmdParams.Action {
	case CommandTypeStart:
		return s.handleStartProcess(msg.ID, cmdParams, clientConn)
	case CommandTypeStop:
		return s.handleStopProcess(msg.ID, cmdParams)
	case CommandTypeGetStatus:
		return s.handleGetStatus(msg.ID, cmdParams)
	case CommandTypeGetLogs:
		return s.handleGetLogs(msg.ID, cmdParams)
	default:
		return Response{
			ID:     msg.ID,
			Type:   ResponseTypeError,
			Result: NewErrorResponse(ErrorTypeInvalidFormat, fmt.Sprintf("Unknown command action: %s", cmdParams.Action)),
		}
	}
}

// handleStartProcess processes a command to start a new process.
// It validates that process request parameters are provided, sets up callbacks for logs
// and status changes, and starts the process through the process manager.
// The callbacks enable real-time broadcasting of process events to all connected clients.
// Parameters:
//   - msgID: the message ID for the response
//   - cmdParams: the CommandParams containing the process configuration
//   - clientConn: the ClientConnection (currently unused but available for future features)
//
// Returns a Response indicating success or failure of the start operation.
func (s *RpcServer) handleStartProcess(msgID string, cmdParams CommandParams, clientConn *ClientConnection) Response {
	if cmdParams.Process == nil {
		return Response{
			ID:     msgID,
			Type:   ResponseTypeError,
			Result: NewErrorResponse(ErrorTypeInvalidFormat, "Process request is required for start command"),
		}
	}

	// Set up callbacks for this process
	cmdParams.Process.SetOnLog(func(log cmd.ProcessLog) {
		s.broadcastNotification(NotificationTypeProcessLog, log)
	})

	cmdParams.Process.SetOnStatusChange(func(status cmd.ProcessStatus) {
		s.broadcastNotification(NotificationTypeProcessStatusChanged, status)
	})

	if err := s.pcsManager.StartProcess(cmdParams.Process); err != nil {
		return Response{
			ID:     msgID,
			Type:   ResponseTypeError,
			Result: NewErrorResponse(ErrorTypeInternalError, err.Error()),
		}
	}

	return Response{
		ID:   msgID,
		Type: ResponseTypeSuccess,
		Result: CommandResponse{
			Message: fmt.Sprintf("Process '%s' started successfully", cmdParams.Process.Name),
		},
	}
}

// handleStopProcess processes a command to stop a running process.
// It validates that a process name was provided and attempts to stop the process.
// Parameters:
//   - msgID: the message ID for the response
//   - cmdParams: the CommandParams containing the process name to stop
//
// Returns a Response indicating success or failure of the stop operation.
func (s *RpcServer) handleStopProcess(msgID string, cmdParams CommandParams) Response {
	if cmdParams.Name == "" {
		return Response{
			ID:     msgID,
			Type:   ResponseTypeError,
			Result: NewErrorResponse(ErrorTypeInvalidFormat, "Process name is required for stop command"),
		}
	}

	if err := s.pcsManager.StopProcess(cmdParams.Name); err != nil {
		return Response{
			ID:     msgID,
			Type:   ResponseTypeError,
			Result: NewErrorResponse(ErrorTypeInternalError, err.Error()),
		}
	}

	return Response{
		ID:   msgID,
		Type: ResponseTypeSuccess,
		Result: CommandResponse{
			Message: fmt.Sprintf("Process '%s' stopped successfully", cmdParams.Name),
		},
	}
}

// handleGetStatus processes a command to retrieve process status information.
// If no process name is provided, it returns the status of all processes.
// If a process name is provided, it returns the status of only that specific process.
// Parameters:
//   - msgID: the message ID for the response
//   - cmdParams: the CommandParams optionally containing a specific process name
//
// Returns a Response with status information or an error.
func (s *RpcServer) handleGetStatus(msgID string, cmdParams CommandParams) Response {
	if cmdParams.Name == "" {
		// Get all statuses
		statuses := s.pcsManager.GetAllStatuses()
		return Response{
			ID:   msgID,
			Type: ResponseTypeSuccess,
			Result: CommandResponse{
				Message: "All process statuses retrieved",
				Data:    statuses,
			},
		}
	}

	// Get specific process status
	status, err := s.pcsManager.GetStatus(cmdParams.Name)
	if err != nil {
		return Response{
			ID:     msgID,
			Type:   ResponseTypeError,
			Result: NewErrorResponse(ErrorTypeInternalError, err.Error()),
		}
	}

	return Response{
		ID:   msgID,
		Type: ResponseTypeSuccess,
		Result: CommandResponse{
			Message: fmt.Sprintf("Status for process '%s' retrieved", cmdParams.Name),
			Data:    status,
		},
	}
}

// handleGetLogs processes a command to retrieve log entries from a process.
// It requires a process name and returns all available logs for that process.
// Logs may come from memory or from a log file depending on the process configuration.
// Parameters:
//   - msgID: the message ID for the response
//   - cmdParams: the CommandParams containing the process name
//
// Returns a Response with log entries or an error.
func (s *RpcServer) handleGetLogs(msgID string, cmdParams CommandParams) Response {
	if cmdParams.Name == "" {
		return Response{
			ID:     msgID,
			Type:   ResponseTypeError,
			Result: NewErrorResponse(ErrorTypeInvalidFormat, "Process name is required for get_logs command"),
		}
	}

	logs, err := s.pcsManager.GetProcessLogs(cmdParams.Name)
	if err != nil {
		return Response{
			ID:     msgID,
			Type:   ResponseTypeError,
			Result: NewErrorResponse(ErrorTypeInternalError, err.Error()),
		}
	}

	return Response{
		ID:   msgID,
		Type: ResponseTypeSuccess,
		Result: CommandResponse{
			Message: fmt.Sprintf("Logs for process '%s' retrieved", cmdParams.Name),
			Data:    logs,
		},
	}
}

// broadcastNotification sends a notification to all currently connected clients.
// This allows the server to push real-time updates about process events to all clients.
// If sending to a client fails, the error is logged but other clients still receive the notification.
// Parameters:
//   - notifType: the type of notification (process_log, process_status_changed, etc)
//   - data: the notification payload to send to clients
func (s *RpcServer) broadcastNotification(notifType string, data interface{}) {
	s.connMu.RLock()
	defer s.connMu.RUnlock()

	for _, clientConn := range s.connections {
		if err := clientConn.SendNotification(notifType, data); err != nil {
			log.Printf("Failed to send notification to client: %v", err)
		}
	}
}
