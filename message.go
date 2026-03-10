package rpc

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	// MessageTypeEvent is the type for an event message.
	MessageTypeEvent = "event"

	// MessqgeTypeCommand is the type for a command message.
	MessageTypeCommand = "command"
)

type Message struct {
	ID     string      `json:"id, omitempty"`
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
	CommandTypeStart = "start"
	CommandTypeStop  = "stop"
)

// if the type is set to the event, the params should be of type EventParams
// if the decoding fails,  then it is an error
type CommandParams struct {
	Type    string      `json:"type"`
	Command string      `json:"command"`
	Args    interface{} `json:"args,omitempty"`
}

type CommandResponse struct {
	Pid    int    `json:"pid"`
	Status string `json:"status"`
	// an map of timestamp to stdout output, the timestamp is in unix format
	Stdout map[int64]string `json:"stdout,omitempty"`

	Stderr map[int64]string `json:"stderr,omitempty"`
}

type EventParams struct {
	Name string      `json:"name"`
	Data interface{} `json:"data,omitempty"`
}
