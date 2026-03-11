//go:build !windows

package rpc

import (
	"net"
)

// connectPipe establishes a connection to a Unix socket.
// This is the Unix/Linux implementation for local pipe connections.
func (c *RpcClient) connectPipe() (net.Conn, error) {
	return net.Dial("unix", c.Config.PipeName)
}
