package rpc

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"
)

type rpcFunc func(params interface{}) (interface{}, error)

type RpcServer struct {
	port          int
	eventHandlers map[string]rpcFunc

	// actually we never handle the command we will just run them
}

func NewRpcServer(port int) *RpcServer {
	return &RpcServer{port: port}

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
	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	for {
		var msg Message
		if err := decoder.Decode(&msg); err != nil {
			log.Printf("Failed to decode message: %v", err)
			return
		}
		log.Printf("Received message: %+v", msg)

		response := s.handleMessage(msg)
		if err := encoder.Encode(response); err != nil {
			log.Printf("Failed to encode response: %v", err)
			return
		}
	}
}

func (s *RpcServer) handleMessage(msg Message) Response {
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
	}

	return Response{
		ID:     msg.ID,
		Type:   ResponseTypeSuccess,
		Result: "Event processed successfully",
	}
}
