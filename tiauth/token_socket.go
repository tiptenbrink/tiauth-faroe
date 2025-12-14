package tiauth

import (
	"encoding/json"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

// TokenMessage represents a token event sent over the socket
type TokenMessage struct {
	Type      string `json:"type"`
	Email     string `json:"email"`
	Code      string `json:"code,omitempty"`
	Timestamp string `json:"timestamp"`
}

// TokenBroadcaster manages a Unix domain socket that broadcasts token events
type TokenBroadcaster struct {
	socketPath string
	listener   net.Listener
	clients    map[net.Conn]bool
	clientsMu  sync.RWMutex
	stopChan   chan struct{}
	errChan    chan error
}

// NewTokenBroadcaster creates a new token broadcaster with the given socket path
// If socketPath is empty, broadcasting is disabled
func NewTokenBroadcaster(socketPath string) *TokenBroadcaster {
	return &TokenBroadcaster{
		socketPath: socketPath,
		clients:    make(map[net.Conn]bool),
		stopChan:   make(chan struct{}),
		errChan:    make(chan error, 1),
	}
}

// Start begins listening on the Unix domain socket
func (tb *TokenBroadcaster) Start() error {
	if tb.socketPath == "" {
		log.Println("Token socket path not configured, broadcasting disabled")
		return nil
	}

	// Remove existing socket file if it exists (stale from previous run)
	// This works on both Unix and Windows
	if _, err := os.Stat(tb.socketPath); err == nil {
		if err := os.Remove(tb.socketPath); err != nil {
			return err
		}
	}

	listener, err := net.Listen("unix", tb.socketPath)
	if err != nil {
		return err
	}
	tb.listener = listener

	log.Printf("Token broadcaster listening on %s", tb.socketPath)

	// Accept connections in a goroutine
	go tb.acceptLoop()

	return nil
}

// acceptLoop handles incoming connections
func (tb *TokenBroadcaster) acceptLoop() {
	for {
		conn, err := tb.listener.Accept()
		if err != nil {
			select {
			case <-tb.stopChan:
				return
			default:
				log.Printf("Token socket accept error: %v", err)
				continue
			}
		}

		tb.clientsMu.Lock()
		tb.clients[conn] = true
		tb.clientsMu.Unlock()

		log.Printf("Token socket client connected (total: %d)", len(tb.clients))

		// Handle client disconnection in a goroutine
		go tb.handleClient(conn)
	}
}

// handleClient monitors a client connection for disconnection
func (tb *TokenBroadcaster) handleClient(conn net.Conn) {
	buf := make([]byte, 1)
	for {
		// Read will return an error when the client disconnects
		conn.SetReadDeadline(time.Now().Add(time.Minute * 5))
		_, err := conn.Read(buf)
		if err != nil {
			// Check if it's a timeout - if so, keep the connection alive
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			// Actual error (client disconnected)
			tb.removeClient(conn)
			return
		}
	}
}

// removeClient removes a client from the broadcast list
func (tb *TokenBroadcaster) removeClient(conn net.Conn) {
	tb.clientsMu.Lock()
	delete(tb.clients, conn)
	clientCount := len(tb.clients)
	tb.clientsMu.Unlock()
	conn.Close()
	log.Printf("Token socket client disconnected (remaining: %d)", clientCount)
}

// Broadcast sends a token message to all connected clients
func (tb *TokenBroadcaster) Broadcast(msg TokenMessage) {
	if tb.socketPath == "" || tb.listener == nil {
		return
	}

	msg.Timestamp = time.Now().UTC().Format(time.RFC3339)

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Token socket marshal error: %v", err)
		return
	}
	data = append(data, '\n')

	tb.clientsMu.RLock()
	clients := make([]net.Conn, 0, len(tb.clients))
	for conn := range tb.clients {
		clients = append(clients, conn)
	}
	tb.clientsMu.RUnlock()

	for _, conn := range clients {
		conn.SetWriteDeadline(time.Now().Add(time.Second * 5))
		_, err := conn.Write(data)
		if err != nil {
			tb.removeClient(conn)
		}
	}
}

// BroadcastSignupVerification broadcasts a signup verification code
func (tb *TokenBroadcaster) BroadcastSignupVerification(email, code string) {
	tb.Broadcast(TokenMessage{
		Type:  "signup_verification",
		Email: email,
		Code:  code,
	})
}

// BroadcastEmailUpdateVerification broadcasts an email update verification code
func (tb *TokenBroadcaster) BroadcastEmailUpdateVerification(email, code string) {
	tb.Broadcast(TokenMessage{
		Type:  "email_update_verification",
		Email: email,
		Code:  code,
	})
}

// BroadcastPasswordReset broadcasts a password reset temporary password
func (tb *TokenBroadcaster) BroadcastPasswordReset(email, temporaryPassword string) {
	tb.Broadcast(TokenMessage{
		Type:  "password_reset",
		Email: email,
		Code:  temporaryPassword,
	})
}

// Close shuts down the broadcaster
func (tb *TokenBroadcaster) Close() error {
	if tb.listener == nil {
		return nil
	}

	close(tb.stopChan)

	tb.clientsMu.Lock()
	for conn := range tb.clients {
		conn.Close()
	}
	tb.clients = make(map[net.Conn]bool)
	tb.clientsMu.Unlock()

	err := tb.listener.Close()

	// Clean up socket file
	if tb.socketPath != "" {
		os.Remove(tb.socketPath)
	}

	return err
}
