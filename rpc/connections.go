package rpc

import (
	"encoding/json"
	"net"
	"sync"
)

const (
	NotificationTypeProcessStarted = "process_started"
	NotificationTypeProcessStopped = "process_stopped"

	NotificationTypeProcessOutput = "process_output"
	NotificationTypeProcessLog    = "process_log"
	NotificationTypeProcessError  = "process_error"

	NotificationTypeProcessStatusChanged = "process_status_changed"
)

// ClientConnection represents an active client connection
type ClientConnection struct {
	conn    net.Conn
	encoder *json.Encoder
	mu      sync.Mutex
}

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
