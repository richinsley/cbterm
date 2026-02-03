package terminal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// Server provides a WebSocket interface to PTY sessions.
type Server struct {
	addr     string
	listener net.Listener
	server   *http.Server
	ptyOpts  Options

	mu      sync.Mutex
	clients map[*websocket.Conn]*PTY
}

// ServerOptions configures the terminal server.
type ServerOptions struct {
	// Addr is the address to listen on (default: "127.0.0.1:0" for random port)
	Addr string

	// PTY options
	Shell string
	Dir   string
}

// NewServer creates a new terminal WebSocket server.
func NewServer(opts ServerOptions) *Server {
	addr := opts.Addr
	if addr == "" {
		addr = "127.0.0.1:0"
	}

	return &Server{
		addr: addr,
		ptyOpts: Options{
			Shell: opts.Shell,
			Dir:   opts.Dir,
		},
		clients: make(map[*websocket.Conn]*PTY),
	}
}

// Start starts the server and returns the address it's listening on.
func (s *Server) Start() (string, error) {
	var err error
	s.listener, err = net.Listen("tcp", s.addr)
	if err != nil {
		return "", fmt.Errorf("failed to listen: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	s.server = &http.Server{Handler: mux}

	go func() {
		if err := s.server.Serve(s.listener); err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	return s.listener.Addr().String(), nil
}

// Stop stops the server and closes all connections.
func (s *Server) Stop() error {
	s.mu.Lock()
	for conn, pty := range s.clients {
		conn.Close()
		pty.Close()
	}
	s.clients = make(map[*websocket.Conn]*PTY)
	s.mu.Unlock()

	if s.server != nil {
		return s.server.Shutdown(context.Background())
	}
	return nil
}

// Addr returns the address the server is listening on.
func (s *Server) Addr() string {
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return ""
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for local connections
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	log.Printf("New WebSocket connection from %s", conn.RemoteAddr())

	// Create PTY for this connection
	pty, err := New(s.ptyOpts)
	if err != nil {
		log.Printf("Failed to create PTY: %v", err)
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Error: %v", err)))
		conn.Close()
		return
	}

	s.mu.Lock()
	s.clients[conn] = pty
	s.mu.Unlock()

	// Clean up on exit
	defer func() {
		s.mu.Lock()
		delete(s.clients, conn)
		s.mu.Unlock()
		conn.Close()
		pty.Close()
		log.Printf("Connection closed from %s", conn.RemoteAddr())
	}()

	// PTY -> WebSocket
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := pty.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Printf("PTY read error: %v", err)
				}
				conn.Close()
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, buf[:n]); err != nil {
				log.Printf("WebSocket write error: %v", err)
				return
			}
		}
	}()

	// WebSocket -> PTY
	for {
		messageType, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			return
		}

		switch messageType {
		case websocket.TextMessage:
			// Check if it's a control message (JSON)
			if len(data) > 0 && data[0] == '{' {
				var msg struct {
					Type string `json:"type"`
					Cols int    `json:"cols"`
					Rows int    `json:"rows"`
				}
				if err := json.Unmarshal(data, &msg); err == nil && msg.Type == "resize" {
					if msg.Cols > 0 && msg.Rows > 0 {
						pty.Resize(uint16(msg.Rows), uint16(msg.Cols))
						log.Printf("Resized to %dx%d", msg.Cols, msg.Rows)
					}
					continue
				}
			}
			// Regular terminal input
			if _, err := pty.Write(data); err != nil {
				log.Printf("PTY write error: %v", err)
				return
			}

		case websocket.BinaryMessage:
			if _, err := pty.Write(data); err != nil {
				log.Printf("PTY write error: %v", err)
				return
			}
		}
	}
}
