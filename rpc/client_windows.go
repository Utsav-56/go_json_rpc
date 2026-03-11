//go:build windows

package rpc

import (
	"fmt"
	"net"

	"github.com/Microsoft/go-winio"
)

// connectPipe establishes a connection to a Windows named pipe.
// This is the Windows implementation for local pipe connections.
func (c *RpcClient) connectPipe() (net.Conn, error) {
	pipePath := fmt.Sprintf(`\\.\pipe\%s`, c.Config.PipeName)
	return winio.DialPipe(pipePath, nil)
}
