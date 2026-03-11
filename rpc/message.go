package rpc

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/utsav-56/go-json-rpc/cmd"
)

const (
	// MessageTypeEvent identifies messages that invoke custom event handlers.
	// Event messages are used to trigger application-specific functionality registered with the server.
	MessageTypeEvent = "event"

	// MessageTypeCommand identifies messages that control process management.
	// Command messages are used for built-in operations like starting, stopping, and querying processes.
	MessageTypeCommand = "command"

	// MessageTypeNotification identifies server-to-client messages for real-time updates.
	// Notification messages are pushed from the server when processes generate logs or change status.
	MessageTypeNotification = "notification"
)

// Message represents a client-to-server message in the JSON-RPC protocol.
// It can be either an event message to invoke custom handlers or a command message for process management.
// The Params field is initially decoded as interface{} and then type-cast based on the Type field.
type Message struct {
	// ID is an optional identifier that clients can use to match responses to requests.
	// This helps correlate responses when multiple requests are in flight.
	ID string `json:"id,omitempty"`

	// Type specifies the message category: event, command, or notification.
	// Valid values are MessageTypeEvent, MessageTypeCommand, or MessageTypeNotification.
	Type string `json:"type"`

	// Params contains the message-specific parameters.
	// This is initially decoded as interface{} and then converted to EventParams or CommandParams.
	Params interface{} `json:"params,omitempty"`
}

// NewMarshallError creates an ErrorResponse for JSON marshalling or unmarshalling failures.
// This is a helper method to standardize error reporting when parameter conversion fails.
// Parameters:
//   - err: the marshalling error that occurred
//   - isMarshalling: true if the error happened during marshalling, false if during unmarshalling
//
// Returns a pointer to an ErrorResponse with details about the conversion failure.
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

// TypeCast converts the generic Params interface{} to the appropriate concrete type.
// It performs a two-step conversion: first marshalling to JSON bytes, then unmarshalling to the target type.
// This approach handles the dynamic nature of JSON parameters received from clients.
// For event messages, it converts to EventParams.
// For command messages, it converts to CommandParams.
// Parameters are modified in place within the Message struct.
// Returns nil if successful, or an ErrorResponse if the type is unknown or conversion fails.
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
	// ResponseTypeSuccess indicates that a request was processed successfully.
	// The Result field will contain the successful response data.
	ResponseTypeSuccess = "success"

	// ResponseTypeError indicates that a request failed or encountered an error.
	// The Result field will contain an ErrorResponse with details about the failure.
	ResponseTypeError = "error"
)

// Response represents a server-to-client response message.
// It includes the request ID for correlation, a type indicating success or error,
// and the result data or error information.
type Response struct {
	// ID matches the ID from the original request to correlate responses.
	// Clients use this to match responses with their pending requests.
	ID string `json:"id"`

	// Type indicates whether the request succeeded or failed.
	// Valid values are ResponseTypeSuccess or ResponseTypeError.
	Type string `json:"type"`

	// Result contains either the success data or an ErrorResponse.
	// The content depends on the Type field.
	Result interface{} `json:"result,omitempty"`
}

const (
	// ErrorTypeInvalidFormat indicates the message format was incorrect or invalid.
	// This occurs when the message structure does not match expected schemas or contains unknown types.
	ErrorTypeInvalidFormat = "invalid_message_format"

	// ErrorTypeInternalError indicates an error occurred within the server.
	// This covers unexpected failures, process start/stop errors, and other internal issues.
	ErrorTypeInternalError = "internal_error"

	// ErrorTypeConversionError indicates a type conversion or casting failure.
	// This happens when parameters cannot be converted to the expected types.
	ErrorTypeConversionError = "conversion_error"
)

// ErrorResponse contains detailed information about an error that occurred.
// It provides a categorized error type, descriptive message, and timestamp.
type ErrorResponse struct {
	// Type categorizes the error using predefined constants.
	// Valid values are ErrorTypeInvalidFormat, ErrorTypeInternalError, or ErrorTypeConversionError.
	Type string `json:"type"`

	// Message provides a human-readable description of what went wrong.
	// This explains the error in detail to help with debugging.
	Message string `json:"message"`

	// Timestamp is the Unix timestamp in seconds when the error occurred.
	// This helps track when errors happen for debugging and logging purposes.
	Timestamp int64 `json:"timestamp"`
}

// NewErrorResponse creates a new ErrorResponse with the current timestamp.
// This is a convenience function for quickly creating error responses.
// Parameters:
//   - errType: the error type constant (ErrorTypeInvalidFormat, etc)
//   - errMsg: a descriptive error message
//
// Returns an ErrorResponse struct ready to be sent to the client.
func NewErrorResponse(errType, errMsg string) ErrorResponse {
	return ErrorResponse{
		Type:      errType,
		Message:   errMsg,
		Timestamp: time.Now().Unix(),
	}
}

const (
	// CommandTypeStart is the action for starting a new process.
	// This command requires process configuration in the CommandParams.
	CommandTypeStart = "start"

	// CommandTypeStop is the action for stopping a running process.
	// This command requires the process name in the CommandParams.
	CommandTypeStop = "stop"

	// CommandTypeGetStatus is the action for retrieving process status.
	// If no process name is provided, returns status for all processes.
	CommandTypeGetStatus = "get_status"

	// CommandTypeGetLogs is the action for retrieving process logs.
	// This command requires the process name in the CommandParams.
	CommandTypeGetLogs = "get_logs"
)

// EventParams contains parameters for custom event invocations.
// Events allow clients to trigger application-specific handlers registered with the server.
type EventParams struct {
	// Name is the identifier of the event handler to invoke.
	// This must match an event name registered with RegisterEvent.
	Name string `json:"name"`

	// Data contains arbitrary parameters to pass to the event handler.
	// The structure depends on what the specific event handler expects.
	Data interface{} `json:"data,omitempty"`
}

// CommandParams contains parameters for process management commands.
// Different actions require different fields to be populated.
type CommandParams struct {
	// Action specifies which process management operation to perform.
	// Valid values are CommandTypeStart, CommandTypeStop, CommandTypeGetStatus, or CommandTypeGetLogs.
	Action string `json:"action"`

	// Process contains the configuration for starting a new process.
	// This is required for CommandTypeStart actions and ignored for other actions.
	Process *cmd.ProcessRequest `json:"process,omitempty"`

	// Name specifies which process to operate on.
	// This is required for CommandTypeStop, CommandTypeGetStatus (optional), and CommandTypeGetLogs.
	Name string `json:"name,omitempty"`
}

// NotificationParams contains parameters for server-to-client notifications.
// Notifications are pushed from the server to inform clients about process events.
type NotificationParams struct {
	// Type identifies what kind of notification this is.
	// Common types include process_log and process_status_changed.
	Type string `json:"type"`

	// Data contains the notification payload.
	// For process_log notifications, this is a ProcessLog.
	// For process_status_changed notifications, this is a ProcessStatus.
	Data interface{} `json:"data"`
}

// Notification represents a server-to-client notification message.
// Unlike responses, notifications are not tied to a specific request and can be sent at any time.
type Notification struct {
	// Type is always MessageTypeNotification to identify this as a notification.
	Type string `json:"type"`

	// Params contains the NotificationParams with the notification type and data.
	Params interface{} `json:"params"`
}

// CommandResponse represents a successful command execution result.
// It provides a human-readable message and optional data payload.
type CommandResponse struct {
	// Message is a human-readable description of what happened.
	// This confirms the operation and provides context about the result.
	Message string `json:"message"`

	// Data contains the result payload if applicable.
	// For get_status commands, this might be a ProcessStatus or map of statuses.
	// For get_logs commands, this would be a slice of log strings.
	Data interface{} `json:"data,omitempty"`
}
