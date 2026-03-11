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

type rpcFunc func(params interface{}) (interface{}, error)

type RpcServer struct {
	port          int
	eventHandlers map[string]rpcFunc
	pcsManager    *cmd.ProcessManager
	ctx           context.Context

	// Active connections for broadcasting notifications
	connections map[net.Conn]*ClientConnection
	connMu      sync.RWMutex
}

func NewRpcServer(port int, ctx context.Context) *RpcServer {
	return &RpcServer{
		port:        port,
		pcsManager:  cmd.NewProcessManager(ctx),
		ctx:         ctx,
		connections: make(map[net.Conn]*ClientConnection),
	}
}

func (s *RpcServer) RegisterEvent(event string, handler rpcFunc) {
	if s.eventHandlers == nil {
		s.eventHandlers = make(map[string]rpcFunc)
	}
	s.eventHandlers[event] = handler
}

func (s *RpcServer) SendResponse(conn net.Conn, response Response) {

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

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

// broadcastNotification sends a notification to all connected clients
func (s *RpcServer) broadcastNotification(notifType string, data interface{}) {
	s.connMu.RLock()
	defer s.connMu.RUnlock()

	for _, clientConn := range s.connections {
		if err := clientConn.SendNotification(notifType, data); err != nil {
			log.Printf("Failed to send notification to client: %v", err)
		}
	}
}
