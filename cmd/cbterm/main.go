//go:build darwin

package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/calobozan/cbterm/pkg/platform/darwin"
	"github.com/calobozan/cbterm/pkg/terminal"
)

// Frontend will be served from filesystem for now
// TODO: Embed frontend in binary for single-file distribution

var (
	version = "dev"
	commit  = "none"
)

func main() {
	showVersion := flag.Bool("version", false, "Show version")
	debug := flag.Bool("debug", false, "Enable debug mode (developer tools)")
	shell := flag.String("shell", "", "Shell to run (default: $SHELL or /bin/zsh)")
	devMode := flag.Bool("dev", false, "Development mode (load frontend from disk)")
	flag.Parse()

	if *showVersion {
		fmt.Printf("cbterm %s (%s)\n", version, commit)
		os.Exit(0)
	}

	// Start terminal WebSocket server
	termServer := terminal.NewServer(terminal.ServerOptions{
		Shell: *shell,
	})

	wsAddr, err := termServer.Start()
	if err != nil {
		log.Fatalf("Failed to start terminal server: %v", err)
	}
	log.Printf("Terminal server listening on %s", wsAddr)

	// Start HTTP server for frontend (in dev mode or embedded)
	var frontendURL string
	if *devMode {
		// Serve from disk for hot reload during development
		frontendURL = "http://localhost:3000"
		log.Printf("Dev mode: expecting frontend at %s (run 'npm run dev' in frontend/)", frontendURL)
	} else {
		// Serve embedded frontend
		httpAddr, err := serveFrontend(wsAddr)
		if err != nil {
			log.Fatalf("Failed to serve frontend: %v", err)
		}
		frontendURL = fmt.Sprintf("http://%s", httpAddr)
		log.Printf("Frontend served at %s", frontendURL)
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

	// Set up message handler from JavaScript
	win.GetWebView().OnMessage(func(msg string) {
		log.Printf("Message from JS: %s", msg)
	})

	// Build app menu
	buildAppMenu(p, win)

	// Load frontend
	// Add WebSocket URL as query parameter
	loadURL := fmt.Sprintf("%s?ws=ws://%s/ws", frontendURL, wsAddr)
	log.Printf("Loading %s", loadURL)
	win.GetWebView().LoadURL(loadURL)

	// Handle window close
	win.OnClose(func() bool {
		log.Println("Window closing...")
		termServer.Stop()
		return true
	})

	// Show window and run
	win.Show()

	if err := p.Run(); err != nil {
		log.Fatalf("Platform error: %v", err)
	}
}

func serveFrontend(wsAddr string) (string, error) {
	// Find frontend dist directory
	// Try relative to executable first, then current directory
	var distPath string

	// Try relative to executable
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidate := filepath.Join(exeDir, "..", "frontend", "dist")
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
		return "", fmt.Errorf("frontend not found (run 'cd frontend && npm install && npm run build')")
	}

	log.Printf("Serving frontend from %s", distPath)

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(distPath)))

	// Start on random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}

	go http.Serve(listener, mux)
	return listener.Addr().String(), nil
}

func buildAppMenu(p *darwin.Platform, win *darwin.Window) {
	menuBar := p.NewMenu()

	// App menu
	appMenu := p.NewMenu()
	appMenu.Add(darwin.MenuItem{
		Label:   "About cbterm",
		Enabled: true,
		Action: func() {
			log.Println("About clicked")
		},
	})
	appMenu.AddSeparator()
	appMenu.Add(darwin.MenuItem{
		Label:    "Quit cbterm",
		Shortcut: "Cmd+Q",
		Enabled:  true,
		Action: func() {
			p.Quit()
		},
	})
	menuBar.Add(darwin.MenuItem{
		Label:   "cbterm",
		Submenu: appMenu,
	})

	// Shell menu
	shellMenu := p.NewMenu()
	shellMenu.Add(darwin.MenuItem{
		Label:    "New Window",
		Shortcut: "Cmd+N",
		Enabled:  true,
		Action: func() {
			log.Println("New window - TODO")
		},
	})
	shellMenu.Add(darwin.MenuItem{
		Label:    "New Tab",
		Shortcut: "Cmd+T",
		Enabled:  true,
		Action: func() {
			log.Println("New tab - TODO")
		},
	})
	shellMenu.AddSeparator()
	shellMenu.Add(darwin.MenuItem{
		Label:    "Close Window",
		Shortcut: "Cmd+W",
		Enabled:  true,
		Action: func() {
			win.Close()
		},
	})
	menuBar.Add(darwin.MenuItem{
		Label:   "Shell",
		Submenu: shellMenu,
	})

	// Edit menu
	editMenu := p.NewMenu()
	editMenu.Add(darwin.MenuItem{Label: "Undo", Shortcut: "Cmd+Z", Enabled: true})
	editMenu.Add(darwin.MenuItem{Label: "Redo", Shortcut: "Cmd+Shift+Z", Enabled: true})
	editMenu.AddSeparator()
	editMenu.Add(darwin.MenuItem{Label: "Cut", Shortcut: "Cmd+X", Enabled: true})
	editMenu.Add(darwin.MenuItem{Label: "Copy", Shortcut: "Cmd+C", Enabled: true})
	editMenu.Add(darwin.MenuItem{Label: "Paste", Shortcut: "Cmd+V", Enabled: true})
	editMenu.Add(darwin.MenuItem{Label: "Select All", Shortcut: "Cmd+A", Enabled: true})
	menuBar.Add(darwin.MenuItem{
		Label:   "Edit",
		Submenu: editMenu,
	})

	// View menu
	viewMenu := p.NewMenu()
	viewMenu.Add(darwin.MenuItem{
		Label:    "Reload",
		Shortcut: "Cmd+R",
		Enabled:  true,
		Action: func() {
			win.GetWebView().Reload()
		},
	})
	menuBar.Add(darwin.MenuItem{
		Label:   "View",
		Submenu: viewMenu,
	})

	// Window menu
	windowMenu := p.NewMenu()
	windowMenu.Add(darwin.MenuItem{Label: "Minimize", Shortcut: "Cmd+M", Enabled: true})
	windowMenu.Add(darwin.MenuItem{Label: "Zoom", Enabled: true})
	menuBar.Add(darwin.MenuItem{
		Label:   "Window",
		Submenu: windowMenu,
	})

	p.SetAppMenu(menuBar)
}
