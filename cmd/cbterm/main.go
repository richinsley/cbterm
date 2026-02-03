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

	// Create main window
	win, err := p.NewWindow(darwin.WindowOptions{
		Title:     "cbterm",
		Width:     800,
		Height:    600,
		Resizable: true,
	})
	if err != nil {
		log.Fatalf("Failed to create window: %v", err)
	}

	// Terminal instance (created after we know the size)
	var term *terminal.Terminal
	var termMu sync.Mutex

	// Handle messages from JavaScript
	win.GetWebView().OnMessage(func(msg string) {
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
			// Frontend is ready, start terminal
			log.Printf("Frontend ready, terminal size: %dx%d", m.Cols, m.Rows)
			termMu.Lock()
			if term == nil {
				var err error
				term, err = terminal.New(terminal.Options{
					Shell: *shell,
					Cols:  uint16(m.Cols),
					Rows:  uint16(m.Rows),
					OnOutput: func(data string) {
						// Write to terminal via native bridge (async, non-blocking)
						js := fmt.Sprintf("window.cbterm.write('%s')", data)
						win.GetWebView().EvalAsync(js)
					},
				})
				if err != nil {
					log.Printf("Failed to create terminal: %v", err)
				}
			}
			termMu.Unlock()

		case "input":
			// Input from user
			termMu.Lock()
			if term != nil {
				term.Write(m.Data)
			}
			termMu.Unlock()

		case "resize":
			// Terminal resized
			termMu.Lock()
			if term != nil && m.Cols > 0 && m.Rows > 0 {
				term.Resize(uint16(m.Rows), uint16(m.Cols))
				log.Printf("Resized to %dx%d", m.Cols, m.Rows)
			}
			termMu.Unlock()
		}
	})

	// Build app menu
	buildAppMenu(p, win)

	// Serve frontend
	frontendURL, err := serveFrontend()
	if err != nil {
		log.Fatalf("Failed to serve frontend: %v", err)
	}
	log.Printf("Frontend: %s", frontendURL)

	// Load frontend
	win.GetWebView().LoadURL(frontendURL)

	// Handle window close
	win.OnClose(func() bool {
		termMu.Lock()
		if term != nil {
			term.Close()
		}
		termMu.Unlock()
		return true
	})

	// Show window and run
	win.Show()

	if err := p.Run(); err != nil {
		log.Fatalf("Platform error: %v", err)
	}
}

func serveFrontend() (string, error) {
	// Find frontend dist directory
	var distPath string

	// Try relative to executable
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "..", "frontend", "dist")
		if _, err := os.Stat(candidate); err == nil {
			distPath = candidate
		}
	}

	// Try relative to current directory
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

func buildAppMenu(p *darwin.Platform, win *darwin.Window) {
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
	})
	shellMenu.Add(darwin.MenuItem{
		Label:    "New Tab",
		Shortcut: "Cmd+T",
		Enabled:  true,
	})
	shellMenu.AddSeparator()
	shellMenu.Add(darwin.MenuItem{
		Label:    "Close",
		Shortcut: "Cmd+W",
		Enabled:  true,
		Action:   func() { win.Close() },
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
		Action:   func() { win.GetWebView().Reload() },
	})
	menuBar.Add(darwin.MenuItem{Label: "View", Submenu: viewMenu})

	// Window menu
	windowMenu := p.NewMenu()
	windowMenu.Add(darwin.MenuItem{Label: "Minimize", Shortcut: "Cmd+M", Enabled: true})
	windowMenu.Add(darwin.MenuItem{Label: "Zoom", Enabled: true})
	menuBar.Add(darwin.MenuItem{Label: "Window", Submenu: windowMenu})

	p.SetAppMenu(menuBar)
}
