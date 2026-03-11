//go:build !windows

package rpc

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/utsav-56/go-json-rpc/cmd"
)

// Start begins listening for client connections with a background context.
// This method internally calls StartWithContext with context.Background().
// For production use, consider using StartWithContext directly for better control.
func (s *RpcServer) Start() error {
	return s.StartWithContext(context.Background())
}

// StartWithContext begins listening for client connections on the configured port or pipe.
// This method blocks and runs the server loop until the context is cancelled.
// For each incoming connection, it spawns a goroutine to handle that client.
// The goroutine approach allows multiple clients to connect simultaneously and operate independently.
// Each client connection is handled concurrently without blocking other clients.
// When the context is cancelled, the server performs a graceful shutdown.
// Parameters:
//   - ctx: context for managing server lifecycle and graceful shutdown
//
// Returns an error if the server cannot start or if shutdown encounters issues.
func (s *RpcServer) StartWithContext(ctx context.Context) error {
	var err error
	err = s.Config.Validate()
	if err != nil {
		return fmt.Errorf("Invalid server configuration: %v", err)
	}

	// Create a cancellable context for the server
	s.ctx, s.cancel = context.WithCancel(ctx)

	// Initialize process manager with server context
	s.pcsManager = cmd.NewProcessManager(s.ctx)

	var ln net.Listener

	if s.Config.UseTcp {
		ln, err = net.Listen("tcp", fmt.Sprintf(":%d", s.Config.Port))
		if err != nil {
			return fmt.Errorf("Failed to start TCP listener: %v", err)
		}
		log.Printf("Server listening on TCP port %d", s.Config.Port)
	} else {
		// Remove existing Unix socket if it exists
		os.Remove(s.Config.PipeName)
		ln, err = net.Listen("unix", s.Config.PipeName)
		if err != nil {
			return fmt.Errorf("Failed to start Unix socket listener: %v", err)
		}
		log.Printf("Server listening on Unix socket: %s", s.Config.PipeName)
	}

	s.listenerMu.Lock()
	s.listener = ln
	s.listenerMu.Unlock()

	// Channel to signal when accept loop exits
	acceptDone := make(chan struct{})

	// Start accept loop in goroutine
	go func() {
		defer close(acceptDone)
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-s.ctx.Done():
					// Server is shutting down, this is expected
					return
				default:
					// If accepting a connection fails, log the error and continue accepting other connections.
					// we dont return here because a single failed connection should not bring down the entire server.
					log.Printf("Failed to accept connection: %v", err)
					continue
				}
			}

			s.shutdownWg.Add(1)
			go func(c net.Conn) {
				defer s.shutdownWg.Done()
				s.handleConnection(c)
			}(conn)
		}
	}()

	// Wait for context cancellation
	<-s.ctx.Done()

	log.Println("Server shutdown initiated...")
	return s.shutdown()
}

// Shutdown performs a graceful shutdown of the server.
// It closes the listener, disconnects all clients, and cleans up resources.
// Returns an error if cleanup encounters issues.
func (s *RpcServer) Shutdown() error {
	if s.cancel != nil {
		s.cancel()
	}
	return s.shutdown()
}

// shutdown performs the actual shutdown operations.
// This is called internally by StartWithContext and Shutdown.
func (s *RpcServer) shutdown() error {
	// Close listener to stop accepting new connections
	s.listenerMu.Lock()
	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			log.Printf("Error closing listener: %v", err)
		}
		s.listener = nil
	}
	s.listenerMu.Unlock()

	// Close all client connections
	s.connMu.Lock()
	for conn := range s.connections {
		if err := conn.Close(); err != nil {
			log.Printf("Error closing client connection: %v", err)
		}
	}
	// Clear connections map
	s.connections = make(map[net.Conn]*ClientConnection)
	s.connMu.Unlock()

	// Wait for all connection handlers to finish
	s.shutdownWg.Wait()

	// Clean up Unix socket file if not using TCP
	if !s.Config.UseTcp {
		os.Remove(s.Config.PipeName)
	}

	log.Println("Server shutdown complete")
	return nil
}
