package rpc

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/utsav-56/go-json-rpc/cmd"
)

const (
	// MessageTypeEvent is the type for an event message.
	MessageTypeEvent = "event"

	// MessageTypeCommand is the type for a command message.
	MessageTypeCommand = "command"

	// MessageTypeNotification is for server-to-client notifications (logs, status changes)
	MessageTypeNotification = "notification"
)

type Message struct {
	ID     string      `json:"id,omitempty"`
	Type   string      `json:"type"`
	Params interface{} `json:"params,omitempty"`
}

func (m *Message) NewMarshallError(err error, isMarshalling bool) *ErrorResponse {
	var operation string = "marshal"
	if !isMarshalling {
		operation = "Unmarshal"
	}

	return &ErrorResponse{
		Type:      ErrorTypeInvalidFormat,
		Message:   fmt.Sprintf("failed to %s params: %v", operation, err),
		Timestamp: time.Now().Unix(),
	}
}

func (m *Message) TypeCast() *ErrorResponse {
	switch m.Type {
	case MessageTypeEvent:
		var eventParams EventParams
		paramsBytes, err := json.Marshal(m.Params)
		if err != nil {
			return m.NewMarshallError(err, true)
		}
		if err := json.Unmarshal(paramsBytes, &eventParams); err != nil {
			return m.NewMarshallError(err, false)

		}
		m.Params = eventParams
	case MessageTypeCommand:
		var commandParams CommandParams

		paramsBytes, err := json.Marshal(m.Params)
		if err != nil {
			return m.NewMarshallError(err, true)
		}
		if err := json.Unmarshal(paramsBytes, &commandParams); err != nil {
			return m.NewMarshallError(err, false)

		}
		m.Params = commandParams
	default:
		return &ErrorResponse{
			Type:      ErrorTypeInvalidFormat,
			Message:   fmt.Sprintf("unknown message type: %s, Only %s or %s are allowed", m.Type, MessageTypeEvent, MessageTypeCommand),
			Timestamp: time.Now().Unix(),
		}
	}

	return nil
}

const (
	ResponseTypeSuccess = "success"
	ResponseTypeError   = "error"
)

type Response struct {
	ID     string      `json:"id"`
	Type   string      `json:"type"`
	Result interface{} `json:"result,omitempty"`
}

const (
	ErrorTypeInvalidFormat   = "invalid_message_format"
	ErrorTypeInternalError   = "internal_error"
	ErrorTypeConversionError = "conversion_error"
)

type ErrorResponse struct {
	Type      string `json:"type"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

func NewErrorResponse(errType, errMsg string) ErrorResponse {
	return ErrorResponse{
		Type:      errType,
		Message:   errMsg,
		Timestamp: time.Now().Unix(),
	}
}

const (
	CommandTypeStart     = "start"
	CommandTypeStop      = "stop"
	CommandTypeGetStatus = "get_status"
	CommandTypeGetLogs   = "get_logs"
)

type EventParams struct {
	Name string      `json:"name"`
	Data interface{} `json:"data,omitempty"`
}

// CommandParams represents command parameters
type CommandParams struct {
	Action  string                `json:"action"` // start, stop, get_status, get_logs
	Process *cmd.ProcessRequest   `json:"process,omitempty"`
	Name    string                `json:"name,omitempty"` // For stop, get_status, get_logs
}

// NotificationParams for server-to-client notifications
type NotificationParams struct {
	Type string      `json:"type"` // "log" or "status_change"
	Data interface{} `json:"data"`
}

// Notification message structure
type Notification struct {
	Type   string      `json:"type"` // Always "notification"
	Params interface{} `json:"params"`
}

// CommandResponse for successful command execution
type CommandResponse struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
