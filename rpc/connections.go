package rpc

import (
	"encoding/json"
	"net"
	"sync"
)

const (
	// NotificationTypeProcessStarted indicates a process has successfully started.
	// This notification type is currently defined but not actively used in the implementation.
	NotificationTypeProcessStarted = "process_started"

	// NotificationTypeProcessStopped indicates a process has stopped executing.
	// This notification type is currently defined but not actively used in the implementation.
	NotificationTypeProcessStopped = "process_stopped"

	// NotificationTypeProcessOutput is for general process output notifications.
	// This notification type is currently defined but not actively used in the implementation.
	NotificationTypeProcessOutput = "process_output"

	// NotificationTypeProcessLog is sent when a process generates log output.
	// This is actively used to broadcast ProcessLog entries to all connected clients.
	NotificationTypeProcessLog = "process_log"

	// NotificationTypeProcessError is for process error notifications.
	// This notification type is currently defined but not actively used in the implementation.
	NotificationTypeProcessError = "process_error"

	// NotificationTypeProcessStatusChanged is sent when a process changes state.
	// This is actively used to broadcast ProcessStatus updates when processes transition between states.
	NotificationTypeProcessStatusChanged = "process_status_changed"
)

// ClientConnection represents an active client connection for bidirectional communication.
// It encapsulates the network connection and provides thread-safe notification sending.
type ClientConnection struct {
	// conn is the underlying network connection to the client.
	// This is used to identify the connection and can be used for sending data.
	conn net.Conn

	// encoder is used to serialize and send JSON messages to the client.
	// Having a dedicated encoder ensures consistent JSON formatting.
	encoder *json.Encoder

	// mu protects concurrent writes to the encoder.
	// This mutex ensures that only one goroutine can send a notification at a time,
	// preventing interleaved or corrupted messages when multiple events occur simultaneously.
	mu sync.Mutex
}

// SendNotification sends a notification message to this client connection.
// It wraps the notification data in the proper message structure and encodes it as JSON.
// This method is thread-safe and can be called from multiple goroutines concurrently.
// Parameters:
//   - notifType: the type of notification (process_log, process_status_changed, etc)
//   - data: the notification payload to send
//
// Returns an error if encoding or sending fails, nil on success.
func (cc *ClientConnection) SendNotification(notifType string, data interface{}) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	notification := Notification{
		Type: MessageTypeNotification,
		Params: NotificationParams{
			Type: notifType,
			Data: data,
		},
	}

	return cc.encoder.Encode(notification)
}
