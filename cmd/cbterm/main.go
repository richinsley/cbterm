//go:build darwin

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/calobozan/cbterm/pkg/platform/darwin"
	"github.com/calobozan/cbterm/pkg/terminal"
)

var (
	version = "dev"
	commit  = "none"
)

// App holds global application state
type App struct {
	platform    *darwin.Platform
	shell       string
	debug       bool
	frontendURL string

	mu      sync.Mutex
	windows map[*darwin.Window]*TerminalWindow
}

// TerminalWindow holds per-window state
type TerminalWindow struct {
	window   *darwin.Window
	terminal *terminal.Terminal
	mu       sync.Mutex
}

var app *App

func main() {
	showVersion := flag.Bool("version", false, "Show version")
	debug := flag.Bool("debug", false, "Enable debug mode (developer tools)")
	shell := flag.String("shell", "", "Shell to run (default: $SHELL or /bin/zsh)")
	flag.Parse()

	if *showVersion {
		fmt.Printf("cbterm %s (%s)\n", version, commit)
		os.Exit(0)
	}

	// Initialize platform
	p, err := darwin.New(darwin.Options{
		Name:  "cbterm",
		Debug: *debug,
	})
	if err != nil {
		log.Fatalf("Failed to initialize platform: %v", err)
	}

	// Serve frontend (shared across all windows)
	frontendURL, err := serveFrontend()
	if err != nil {
		log.Fatalf("Failed to serve frontend: %v", err)
	}
	log.Printf("Frontend: %s", frontendURL)

	// Create app
	app = &App{
		platform:    p,
		shell:       *shell,
		debug:       *debug,
		frontendURL: frontendURL,
		windows:     make(map[*darwin.Window]*TerminalWindow),
	}

	// Build app menu
	buildAppMenu()

	// Create first window
	if err := app.NewWindow(); err != nil {
		log.Fatalf("Failed to create window: %v", err)
	}

	// Run
	if err := p.Run(); err != nil {
		log.Fatalf("Platform error: %v", err)
	}
}

// NewWindow creates a new terminal window
func (a *App) NewWindow() error {
	win, err := a.platform.NewWindow(darwin.WindowOptions{
		Title:     "cbterm",
		Width:     800,
		Height:    600,
		Resizable: true,
	})
	if err != nil {
		return err
	}

	tw := &TerminalWindow{
		window: win,
	}

	// Register window
	a.mu.Lock()
	a.windows[win] = tw
	a.mu.Unlock()

	// Handle messages from JavaScript
	win.GetWebView().OnMessage(func(msg string) {
		tw.handleMessage(msg)
	})

	// Handle window close
	win.OnClose(func() bool {
		tw.mu.Lock()
		if tw.terminal != nil {
			tw.terminal.Close()
		}
		tw.mu.Unlock()

		a.mu.Lock()
		delete(a.windows, win)
		remaining := len(a.windows)
		a.mu.Unlock()

		log.Printf("Window closed, %d remaining", remaining)

		// Quit app when last window closes
		if remaining == 0 {
			a.platform.Quit()
		}

		return true
	})

	// Load frontend and show
	win.GetWebView().LoadURL(a.frontendURL)
	win.Show()

	return nil
}

// handleMessage processes messages from the frontend
func (tw *TerminalWindow) handleMessage(msg string) {
	var m struct {
		Type string `json:"type"`
		Data string `json:"data"`
		Cols int    `json:"cols"`
		Rows int    `json:"rows"`
	}
	if err := json.Unmarshal([]byte(msg), &m); err != nil {
		log.Printf("Invalid message: %s", msg)
		return
	}

	switch m.Type {
	case "ready":
		log.Printf("Frontend ready, terminal size: %dx%d", m.Cols, m.Rows)
		tw.mu.Lock()
		if tw.terminal == nil {
			var err error
			tw.terminal, err = terminal.New(terminal.Options{
				Shell: app.shell,
				Cols:  uint16(m.Cols),
				Rows:  uint16(m.Rows),
				OnOutput: func(data string) {
					js := fmt.Sprintf("window.cbterm.write('%s')", data)
					tw.window.GetWebView().EvalAsync(js)
				},
				OnExit: func(exitCode int) {
					log.Printf("Shell exited with code %d", exitCode)
					// Close window when shell exits
					// Use Invoke to run on main thread
					app.platform.Invoke(func() {
						tw.window.Close()
					})
				},
			})
			if err != nil {
				log.Printf("Failed to create terminal: %v", err)
			}
		}
		tw.mu.Unlock()

	case "input":
		tw.mu.Lock()
		if tw.terminal != nil {
			tw.terminal.Write(m.Data)
		}
		tw.mu.Unlock()

	case "resize":
		tw.mu.Lock()
		if tw.terminal != nil && m.Cols > 0 && m.Rows > 0 {
			tw.terminal.Resize(uint16(m.Rows), uint16(m.Cols))
		}
		tw.mu.Unlock()
	}
}

func serveFrontend() (string, error) {
	var distPath string

	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "..", "frontend", "dist")
		if _, err := os.Stat(candidate); err == nil {
			distPath = candidate
		}
	}

	if distPath == "" {
		cwd, _ := os.Getwd()
		candidate := filepath.Join(cwd, "frontend", "dist")
		if _, err := os.Stat(candidate); err == nil {
			distPath = candidate
		}
	}

	if distPath == "" {
		return "", fmt.Errorf("frontend not found (run 'cd frontend && npm run build')")
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(distPath)))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}

	go http.Serve(listener, mux)
	return fmt.Sprintf("http://%s", listener.Addr().String()), nil
}

func buildAppMenu() {
	p := app.platform
	menuBar := p.NewMenu()

	// App menu
	appMenu := p.NewMenu()
	appMenu.Add(darwin.MenuItem{
		Label:   "About cbterm",
		Enabled: true,
	})
	appMenu.AddSeparator()
	appMenu.Add(darwin.MenuItem{
		Label:    "Quit cbterm",
		Shortcut: "Cmd+Q",
		Enabled:  true,
		Action:   func() { p.Quit() },
	})
	menuBar.Add(darwin.MenuItem{Label: "cbterm", Submenu: appMenu})

	// Shell menu
	shellMenu := p.NewMenu()
	shellMenu.Add(darwin.MenuItem{
		Label:    "New Window",
		Shortcut: "Cmd+N",
		Enabled:  true,
		Action: func() {
			if err := app.NewWindow(); err != nil {
				log.Printf("Failed to create window: %v", err)
			}
		},
	})
	shellMenu.Add(darwin.MenuItem{
		Label:    "New Tab",
		Shortcut: "Cmd+T",
		Enabled:  true,
		// TODO: Implement tabs
	})
	shellMenu.AddSeparator()
	shellMenu.Add(darwin.MenuItem{
		Label:    "Close",
		Shortcut: "Cmd+W",
		Enabled:  true,
		// Close is handled by the window itself
	})
	menuBar.Add(darwin.MenuItem{Label: "Shell", Submenu: shellMenu})

	// Edit menu
	editMenu := p.NewMenu()
	editMenu.Add(darwin.MenuItem{Label: "Undo", Shortcut: "Cmd+Z", Enabled: true})
	editMenu.Add(darwin.MenuItem{Label: "Redo", Shortcut: "Cmd+Shift+Z", Enabled: true})
	editMenu.AddSeparator()
	editMenu.Add(darwin.MenuItem{Label: "Cut", Shortcut: "Cmd+X", Enabled: true})
	editMenu.Add(darwin.MenuItem{Label: "Copy", Shortcut: "Cmd+C", Enabled: true})
	editMenu.Add(darwin.MenuItem{Label: "Paste", Shortcut: "Cmd+V", Enabled: true})
	editMenu.Add(darwin.MenuItem{Label: "Select All", Shortcut: "Cmd+A", Enabled: true})
	menuBar.Add(darwin.MenuItem{Label: "Edit", Submenu: editMenu})

	// View menu
	viewMenu := p.NewMenu()
	viewMenu.Add(darwin.MenuItem{
		Label:    "Reload",
		Shortcut: "Cmd+R",
		Enabled:  true,
		// TODO: Reload focused window
	})
	menuBar.Add(darwin.MenuItem{Label: "View", Submenu: viewMenu})

	// Window menu
	windowMenu := p.NewMenu()
	windowMenu.Add(darwin.MenuItem{Label: "Minimize", Shortcut: "Cmd+M", Enabled: true})
	windowMenu.Add(darwin.MenuItem{Label: "Zoom", Enabled: true})
	menuBar.Add(darwin.MenuItem{Label: "Window", Submenu: windowMenu})

	p.SetAppMenu(menuBar)
}
