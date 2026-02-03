//go:build darwin

package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/calobozan/cbterm/pkg/platform/darwin"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	showVersion := flag.Bool("version", false, "Show version")
	debug := flag.Bool("debug", false, "Enable debug mode (developer tools)")
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

	// Set up message handler from JavaScript
	win.GetWebView().OnMessage(func(msg string) {
		log.Printf("Message from JS: %s", msg)
	})

	// Build app menu
	buildAppMenu(p)

	// Load a simple test page for now
	// TODO: Load xterm.js frontend
	win.GetWebView().LoadHTML(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>cbterm</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background: #1e1e1e;
            color: #d4d4d4;
            margin: 0;
            padding: 20px;
            display: flex;
            flex-direction: column;
            height: 100vh;
            box-sizing: border-box;
        }
        h1 {
            color: #569cd6;
            margin-bottom: 20px;
        }
        .terminal-placeholder {
            flex: 1;
            background: #0d0d0d;
            border-radius: 8px;
            padding: 16px;
            font-family: "SF Mono", Menlo, Monaco, monospace;
            font-size: 14px;
            overflow: auto;
        }
        .cursor {
            display: inline-block;
            width: 8px;
            height: 16px;
            background: #d4d4d4;
            animation: blink 1s step-end infinite;
        }
        @keyframes blink {
            50% { opacity: 0; }
        }
        .info {
            margin-top: 20px;
            font-size: 12px;
            color: #808080;
        }
    </style>
</head>
<body>
    <h1>🦦 cbterm</h1>
    <div class="terminal-placeholder">
        <div>Welcome to cbterm - the AI-native terminal</div>
        <div>&nbsp;</div>
        <div>Platform layer initialized successfully!</div>
        <div>&nbsp;</div>
        <div>$ <span class="cursor"></span></div>
    </div>
    <div class="info">
        Press Cmd+Option+I to open Developer Tools (if debug mode enabled)
    </div>
    <script>
        // Test message passing to Go
        if (window.webkit && window.webkit.messageHandlers && window.webkit.messageHandlers.cbterm) {
            window.webkit.messageHandlers.cbterm.postMessage("Hello from JavaScript!");
        }
    </script>
</body>
</html>`, "")

	// Handle window close
	win.OnClose(func() bool {
		log.Println("Window closing...")
		return true // Allow close
	})

	// Show window and run
	win.Show()

	if err := p.Run(); err != nil {
		log.Fatalf("Platform error: %v", err)
	}
}

func buildAppMenu(p *darwin.Platform) {
	// Main menu bar
	menuBar := p.NewMenu()

	// App menu (cbterm)
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
