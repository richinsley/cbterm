package app

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/calobozan/term/internal/pty"
	"github.com/calobozan/term/internal/protocol"
	"github.com/gorilla/websocket"
)

// Config holds application configuration
type Config struct {
	Shell  string
	Width  int
	Height int
}

// App is the main terminal application
type App struct {
	config   Config
	pty      *pty.PTY
	protocol *protocol.Handler
	upgrader websocket.Upgrader
	clients  map[*websocket.Conn]bool
	mu       sync.RWMutex
}

// New creates a new terminal application
func New(config Config) (*App, error) {
	if config.Width == 0 {
		config.Width = 120
	}
	if config.Height == 0 {
		config.Height = 40
	}

	return &App{
		config:  config,
		clients: make(map[*websocket.Conn]bool),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow local connections
			},
		},
		protocol: protocol.NewHandler(),
	}, nil
}

// Run starts the terminal application
func (a *App) Run() error {
	// Start the PTY
	p, err := pty.New(a.config.Shell, a.config.Width, a.config.Height)
	if err != nil {
		return fmt.Errorf("failed to create PTY: %w", err)
	}
	a.pty = p
	defer a.pty.Close()

	// Start WebSocket server for frontend communication
	http.HandleFunc("/ws", a.handleWebSocket)
	http.HandleFunc("/", a.handleIndex)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("frontend/dist"))))

	// Start PTY output reader
	go a.readPTY()

	// TODO: Initialize WebKit window and load frontend
	// For now, just serve via HTTP for development
	fmt.Println("Term server running on http://localhost:7776")
	fmt.Println("Open in browser or wait for WebKit integration...")
	
	return http.ListenAndServe(":7776", nil)
}

// handleWebSocket handles WebSocket connections from the frontend
func (a *App) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := a.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	a.mu.Lock()
	a.clients[conn] = true
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		delete(a.clients, conn)
		a.mu.Unlock()
	}()

	// Read input from frontend
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		
		// Check for protocol messages vs raw input
		if handled := a.protocol.HandleInput(msg); !handled {
			// Raw terminal input - write to PTY
			a.pty.Write(msg)
		}
	}
}

// handleIndex serves the main HTML page
func (a *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "frontend/dist/index.html")
}

// readPTY reads output from the PTY and broadcasts to clients
func (a *App) readPTY() {
	buf := make([]byte, 4096)
	for {
		n, err := a.pty.Read(buf)
		if err != nil {
			return
		}

		data := buf[:n]
		
		// Process through protocol handler (extracts our extensions)
		processed, extensions := a.protocol.ProcessOutput(data)
		
		// Broadcast to all connected clients
		a.mu.RLock()
		for client := range a.clients {
			// Send terminal data
			client.WriteMessage(websocket.BinaryMessage, processed)
			
			// Send any extracted protocol extensions as JSON
			for _, ext := range extensions {
				client.WriteMessage(websocket.TextMessage, ext)
			}
		}
		a.mu.RUnlock()
	}
}

// Resize resizes the PTY
func (a *App) Resize(width, height int) error {
	return a.pty.Resize(width, height)
}
