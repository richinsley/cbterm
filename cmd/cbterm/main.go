//go:build darwin

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/calobozan/cbterm/pkg/control"
	"github.com/calobozan/cbterm/pkg/extensions"
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
	extensions  *extensions.Manager
	controller  *control.Controller
	shell       string
	debug       bool
	controlMode bool
	frontendURL string

	mu      sync.Mutex
	windows map[*darwin.Window]*TerminalWindow
	
	// For control mode - track the active window
	activeWindow *TerminalWindow
}

// TerminalWindow holds per-window state
type TerminalWindow struct {
	window    *darwin.Window
	terminals map[string]*terminal.Terminal
	mu        sync.Mutex
}

var app *App

func main() {
	showVersion := flag.Bool("version", false, "Show version")
	debug := flag.Bool("debug", false, "Enable debug mode (developer tools)")
	controlMode := flag.Bool("control", false, "Enable control mode (JSON commands via stdin)")
	shell := flag.String("shell", "", "Shell to run (default: $SHELL or /bin/zsh)")
	flag.Parse()

	if *showVersion {
		fmt.Printf("cbterm %s (%s)\n", version, commit)
		os.Exit(0)
	}

	// Register custom URI scheme for frontend assets
	// Must be done BEFORE creating the platform/windows
	frontendURL, err := registerFrontendScheme()
	if err != nil {
		log.Fatalf("Failed to register frontend scheme: %v", err)
	}

	// Initialize platform
	p, err := darwin.New(darwin.Options{
		Name:  "cbterm",
		Debug: *debug,
	})
	if err != nil {
		log.Fatalf("Failed to initialize platform: %v", err)
	}
	log.Printf("Frontend: %s", frontendURL)

	// Discover extensions
	extManager := extensions.NewManager(extensions.DefaultBasePath())
	if err := extManager.Discover(); err != nil {
		log.Printf("Warning: failed to discover extensions: %v", err)
	}
	for _, ext := range extManager.ListEnabled() {
		log.Printf("Extension: %s v%s", ext.Manifest.Name, ext.Manifest.Version)
	}

	// Create app
	app = &App{
		platform:    p,
		extensions:  extManager,
		shell:       *shell,
		debug:       *debug,
		controlMode: *controlMode,
		frontendURL: frontendURL,
		windows:     make(map[*darwin.Window]*TerminalWindow),
	}
	
	// Start control mode if enabled
	if *controlMode {
		app.controller = control.New(app)
		app.controller.Start()
		log.Println("Control mode enabled - accepting JSON commands on stdin")
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
		window:    win,
		terminals: make(map[string]*terminal.Terminal),
	}

	// Register window
	a.mu.Lock()
	a.windows[win] = tw
	a.activeWindow = tw
	a.mu.Unlock()

	// Handle messages from JavaScript
	win.GetWebView().OnMessage(func(msg string) {
		tw.handleMessage(msg)
	})

	// Handle window close
	win.OnClose(func() bool {
		// Collect terminals to close (don't hold lock while closing)
		tw.mu.Lock()
		toClose := make([]*terminal.Terminal, 0, len(tw.terminals))
		for _, term := range tw.terminals {
			toClose = append(toClose, term)
		}
		tw.terminals = nil // Clear map so OnExit callbacks don't try to modify
		tw.mu.Unlock()

		// Close terminals without holding lock
		for _, term := range toClose {
			term.Close()
		}
		log.Printf("Closed %d terminals", len(toClose))

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

	// Load frontend
	win.GetWebView().LoadURL(a.frontendURL)
	win.Show()

	return nil
}

// handleMessage processes messages from the frontend
func (tw *TerminalWindow) handleMessage(msg string) {
	var m struct {
		Type       string `json:"type"`
		Data       string `json:"data"`
		Cols       int    `json:"cols"`
		Rows       int    `json:"rows"`
		Count      int    `json:"count"`
		Error      string `json:"error"`
		Msg        string `json:"msg"`
		Name       string `json:"name"`
		Version    string `json:"version"`
		TerminalId string `json:"terminalId"`
	}
	if err := json.Unmarshal([]byte(msg), &m); err != nil {
		log.Printf("Invalid message: %s", msg)
		return
	}

	switch m.Type {
	case "debug":
		log.Printf("[JS] %s", m.Msg)

	case "extension_loaded":
		log.Printf("Loaded extension: %s v%s", m.Name, m.Version)

	case "extension_error":
		log.Printf("Extension %s failed: %s", m.Name, m.Error)

	case "extensions_loaded":
		log.Printf("Extensions loaded: %d", m.Count)

	case "extensions_error":
		log.Printf("Extensions error: %s", m.Error)

	case "terminal_mounted":
		log.Printf("Terminal %s mounted in frontend", m.TerminalId)

	case "ready", "new_terminal":
		// Create a new terminal for this terminal ID
		terminalId := m.TerminalId
		if terminalId == "" {
			terminalId = "term-1" // backwards compat
		}
		
		log.Printf("Creating terminal %s (%dx%d)", terminalId, m.Cols, m.Rows)
		
		tw.mu.Lock()
		if tw.terminals == nil {
			tw.terminals = make(map[string]*terminal.Terminal)
		}
		
		if tw.terminals[terminalId] == nil {
			tid := terminalId // capture for closure
			term, err := terminal.New(terminal.Options{
				Shell: app.shell,
				Cols:  uint16(m.Cols),
				Rows:  uint16(m.Rows),
				OnOutput: func(data string) {
					// Send output to specific terminal
					// Use template literal to avoid escaping issues with base64
					if app.debug {
						log.Printf("Terminal %s output: %d bytes", tid, len(data))
					}
					js := fmt.Sprintf("window.cbterm.write(`%s`, `%s`)", tid, data)
					tw.window.GetWebView().EvalAsync(js)
					
					// Emit event for control mode
					if app.controller != nil {
						app.controller.EmitTerminalOutput(tid, data)
					}
				},
				OnExit: func(exitCode int) {
					log.Printf("Terminal %s exited with code %d", tid, exitCode)
					
					tw.mu.Lock()
					// Only process if terminal is still in map (not already removed by close_terminal)
					if tw.terminals != nil && tw.terminals[tid] != nil {
						delete(tw.terminals, tid)
						remaining := len(tw.terminals)
						tw.mu.Unlock()
						
						// Close window when last terminal exits
						if remaining == 0 {
							app.platform.Invoke(func() {
								tw.window.Close()
							})
						}
					} else {
						tw.mu.Unlock()
					}
				},
			})
			if err != nil {
				log.Printf("Failed to create terminal %s: %v", terminalId, err)
			} else {
				tw.terminals[terminalId] = term
				// Emit event for control mode
				if app.controller != nil {
					app.controller.Emit(control.EventTerminalCreated, map[string]interface{}{
						"terminal_id": terminalId,
						"cols":        m.Cols,
						"rows":        m.Rows,
					})
				}
			}
		}
		tw.mu.Unlock()

	case "close_terminal":
		terminalId := m.TerminalId
		// Extract terminal from map, then close without holding lock
		tw.mu.Lock()
		term := tw.terminals[terminalId]
		if term != nil {
			delete(tw.terminals, terminalId)
		}
		tw.mu.Unlock()
		
		if term != nil {
			term.Close()
			log.Printf("Closed terminal %s", terminalId)
		}

	case "input":
		terminalId := m.TerminalId
		if terminalId == "" {
			terminalId = "term-1"
		}
		tw.mu.Lock()
		if term := tw.terminals[terminalId]; term != nil {
			term.Write(m.Data)
		}
		tw.mu.Unlock()

	case "resize":
		terminalId := m.TerminalId
		if terminalId == "" {
			terminalId = "term-1"
		}
		tw.mu.Lock()
		if term := tw.terminals[terminalId]; term != nil && m.Cols > 0 && m.Rows > 0 {
			term.Resize(uint16(m.Rows), uint16(m.Cols))
		}
		tw.mu.Unlock()
	}
}

// registerFrontendScheme sets up the "cbterm://" scheme handler for serving
// frontend assets. This bypasses browser security restrictions on file:// URLs
// and allows full ES module support, fetch(), etc.
//
// URL structure:
//   cbterm://app/           → main frontend (index.html, assets)
//   cbterm://extensions/    → user extensions (future)
//   cbterm://themes/        → custom themes (future)
func registerFrontendScheme() (string, error) {
	var distPath string

	// Try relative to executable
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "..", "frontend", "dist")
		if _, err := os.Stat(filepath.Join(candidate, "index.html")); err == nil {
			distPath = candidate
		}
	}

	// Try relative to current directory
	if distPath == "" {
		cwd, _ := os.Getwd()
		candidate := filepath.Join(cwd, "frontend", "dist")
		if _, err := os.Stat(filepath.Join(candidate, "index.html")); err == nil {
			distPath = candidate
		}
	}

	if distPath == "" {
		return "", fmt.Errorf("frontend not found (run 'cd frontend && npm run build')")
	}

	log.Printf("Frontend dir: %s", distPath)

	// Register the cbterm:// scheme handler
	darwin.RegisterScheme("cbterm", func(url string) ([]byte, string) {
		// Parse: cbterm://app/path/to/file.js
		path := strings.TrimPrefix(url, "cbterm://")

		// Route based on first path segment
		parts := strings.SplitN(path, "/", 2)
		namespace := parts[0]
		resourcePath := ""
		if len(parts) > 1 {
			resourcePath = parts[1]
		}

		switch namespace {
		case "app":
			// Serve from frontend dist
			if resourcePath == "" {
				resourcePath = "index.html"
			}
			filePath := filepath.Join(distPath, resourcePath)

			// Security: prevent directory traversal
			absPath, err := filepath.Abs(filePath)
			if err != nil || !strings.HasPrefix(absPath, distPath) {
				log.Printf("Blocked path traversal attempt: %s", path)
				return nil, ""
			}

			data, err := os.ReadFile(filePath)
			if err != nil {
				if app != nil && app.debug {
					log.Printf("404: %s", resourcePath)
				}
				return nil, ""
			}

			if app != nil && app.debug {
				log.Printf("200: %s (%d bytes, %s)", resourcePath, len(data), darwin.MimeTypeForPath(filePath))
			}
			return data, darwin.MimeTypeForPath(filePath)

		case "extensions":
			// Load from user extensions directory
			// ~/.config/cbterm/extensions/{name}/...
			home, _ := os.UserHomeDir()
			extBase := filepath.Join(home, ".config", "cbterm", "extensions")

			filePath := filepath.Join(extBase, resourcePath)

			// Security: prevent directory traversal
			absPath, err := filepath.Abs(filePath)
			if err != nil || !strings.HasPrefix(absPath, extBase) {
				log.Printf("Blocked extension path traversal: %s", path)
				return nil, ""
			}

			data, err := os.ReadFile(filePath)
			if err != nil {
				if app != nil && app.debug {
					log.Printf("Extension 404: %s", resourcePath)
				}
				return nil, ""
			}

			if app != nil && app.debug {
				log.Printf("Extension 200: %s (%d bytes)", resourcePath, len(data))
			}
			return data, darwin.MimeTypeForPath(filePath)

		case "themes":
			// Load from user themes directory
			// ~/.config/cbterm/themes/{name}/...
			home, _ := os.UserHomeDir()
			themeBase := filepath.Join(home, ".config", "cbterm", "themes")

			filePath := filepath.Join(themeBase, resourcePath)

			// Security: prevent directory traversal
			absPath, err := filepath.Abs(filePath)
			if err != nil || !strings.HasPrefix(absPath, themeBase) {
				log.Printf("Blocked theme path traversal: %s", path)
				return nil, ""
			}

			data, err := os.ReadFile(filePath)
			if err != nil {
				if app != nil && app.debug {
					log.Printf("Theme 404: %s", resourcePath)
				}
				return nil, ""
			}

			if app != nil && app.debug {
				log.Printf("Theme 200: %s (%d bytes)", resourcePath, len(data))
			}
			return data, darwin.MimeTypeForPath(filePath)

		case "api":
			// Virtual API endpoints - serve JavaScript API modules
			// These provide the extension API surface
			switch resourcePath {
			case "extensions/list.json":
				// Return list of enabled extensions
				if app != nil && app.extensions != nil {
					data, err := app.extensions.ToJSON()
					if err != nil {
						log.Printf("Failed to serialize extensions: %v", err)
						return []byte("[]"), "application/json"
					}
					if app.debug {
						log.Printf("API: extensions/list.json -> %s", string(data))
					}
					return data, "application/json"
				}
				return []byte("[]"), "application/json"

			case "terminal.js":
				// Terminal API module
				return []byte(`
					// Terminal API
					export const Terminal = {
						write(data) {
							window.webkit.messageHandlers.cbterm.postMessage(
								JSON.stringify({type: 'input', data: data})
							);
						},
						onData(callback) {
							window.cbterm._onDataCallbacks = window.cbterm._onDataCallbacks || [];
							window.cbterm._onDataCallbacks.push(callback);
						}
					};
				`), "application/javascript"

			case "config.js":
				// Config API module (placeholder)
				return []byte(`
					// Config API
					export const Config = {
						async get(key) { return null; },
						async set(key, value) { }
					};
				`), "application/javascript"

			default:
				log.Printf("Unknown API endpoint: %s", resourcePath)
				return nil, ""
			}

		default:
			log.Printf("Unknown cbterm namespace: %s", namespace)
			return nil, ""
		}
	})

	return "cbterm://app/", nil
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
		Action: func() {
			app.evalActiveWindow(`window.cbterm.createTab()`)
		},
	})
	shellMenu.AddSeparator()
	shellMenu.Add(darwin.MenuItem{
		Label:    "Split Vertically",
		Shortcut: "Cmd+D",
		Enabled:  true,
		Action: func() {
			app.evalActiveWindow(`window.cbterm.splitVertical()`)
		},
	})
	shellMenu.Add(darwin.MenuItem{
		Label:    "Split Horizontally",
		Shortcut: "Cmd+Shift+D",
		Enabled:  true,
		Action: func() {
			app.evalActiveWindow(`window.cbterm.splitHorizontal()`)
		},
	})
	shellMenu.AddSeparator()
	shellMenu.Add(darwin.MenuItem{
		Label:    "Close Pane",
		Shortcut: "Cmd+W",
		Enabled:  true,
		Action: func() {
			app.evalActiveWindow(`window.cbterm.closeActivePane && window.cbterm.closeActivePane()`)
		},
	})
	menuBar.Add(darwin.MenuItem{Label: "Shell", Submenu: shellMenu})
	
	// View menu
	viewMenu := p.NewMenu()
	viewMenu.Add(darwin.MenuItem{
		Label:    "Previous Tab",
		Shortcut: "Cmd+[",
		Enabled:  true,
		Action: func() {
			app.evalActiveWindow(`window.cbterm.prevTab && window.cbterm.prevTab()`)
		},
	})
	viewMenu.Add(darwin.MenuItem{
		Label:    "Next Tab",
		Shortcut: "Cmd+]",
		Enabled:  true,
		Action: func() {
			app.evalActiveWindow(`window.cbterm.nextTab && window.cbterm.nextTab()`)
		},
	})
	viewMenu.AddSeparator()
	viewMenu.Add(darwin.MenuItem{
		Label:    "Reload",
		Shortcut: "Cmd+R",
		Enabled:  true,
		Action: func() {
			if app.activeWindow != nil {
				app.activeWindow.window.GetWebView().Reload()
			}
		},
	})
	menuBar.Add(darwin.MenuItem{Label: "View", Submenu: viewMenu})

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

	// Window menu
	windowMenu := p.NewMenu()
	windowMenu.Add(darwin.MenuItem{Label: "Minimize", Shortcut: "Cmd+M", Enabled: true})
	windowMenu.Add(darwin.MenuItem{Label: "Zoom", Enabled: true})
	menuBar.Add(darwin.MenuItem{Label: "Window", Submenu: windowMenu})

	p.SetAppMenu(menuBar)
}

// Helper to evaluate JS on the active window
func (a *App) evalActiveWindow(js string) {
	a.mu.Lock()
	tw := a.activeWindow
	a.mu.Unlock()
	
	if tw != nil {
		tw.window.GetWebView().EvalAsync(js)
	}
}

// Control mode Handler interface implementation

// NewTab creates a new tab in the active window.
func (a *App) NewTab() (tabID string, terminalID string, err error) {
	a.mu.Lock()
	tw := a.activeWindow
	a.mu.Unlock()
	
	if tw == nil {
		return "", "", fmt.Errorf("no active window")
	}
	
	// Tell frontend to create a new tab
	js := `(function() {
		const tab = window.cbterm.createTab();
		return JSON.stringify({tabId: tab.id, terminalId: tab.rootPane.terminal?.id || ''});
	})()`
	
	result, err := tw.window.GetWebView().Eval(js)
	if err != nil {
		return "", "", fmt.Errorf("failed to create tab: %v", err)
	}
	
	var resp struct {
		TabID      string `json:"tabId"`
		TerminalID string `json:"terminalId"`
	}
	if err := json.Unmarshal([]byte(result), &resp); err != nil {
		return "", "", fmt.Errorf("failed to parse response: %v", err)
	}
	
	return resp.TabID, resp.TerminalID, nil
}

// CloseTab closes a tab.
func (a *App) CloseTab(tabID string) error {
	a.mu.Lock()
	tw := a.activeWindow
	a.mu.Unlock()
	
	if tw == nil {
		return fmt.Errorf("no active window")
	}
	
	js := fmt.Sprintf(`window.cbterm.closeTab && window.cbterm.closeTab('%s')`, tabID)
	_, err := tw.window.GetWebView().Eval(js)
	return err
}

// SplitPane splits the active pane.
func (a *App) SplitPane(direction string) (newTerminalID string, err error) {
	a.mu.Lock()
	tw := a.activeWindow
	a.mu.Unlock()
	
	if tw == nil {
		return "", fmt.Errorf("no active window")
	}
	
	var js string
	if direction == "horizontal" {
		js = `(function() {
			const pane = window.cbterm.splitHorizontal();
			return pane?.terminal?.id || '';
		})()`
	} else {
		js = `(function() {
			const pane = window.cbterm.splitVertical();
			return pane?.terminal?.id || '';
		})()`
	}
	
	result, err := tw.window.GetWebView().Eval(js)
	if err != nil {
		return "", fmt.Errorf("failed to split pane: %v", err)
	}
	
	return strings.Trim(result, `"`), nil
}

// ClosePane closes a pane.
func (a *App) ClosePane(terminalID string) error {
	a.mu.Lock()
	tw := a.activeWindow
	a.mu.Unlock()
	
	if tw == nil {
		return fmt.Errorf("no active window")
	}
	
	js := fmt.Sprintf(`window.cbterm.closePane && window.cbterm.closePane('%s')`, terminalID)
	_, err := tw.window.GetWebView().Eval(js)
	return err
}

// TypeText sends text to a terminal.
func (a *App) TypeText(terminalID string, text string) error {
	a.mu.Lock()
	tw := a.activeWindow
	a.mu.Unlock()
	
	if tw == nil {
		return fmt.Errorf("no active window")
	}
	
	tw.mu.Lock()
	term := tw.terminals[terminalID]
	tw.mu.Unlock()
	
	if term == nil {
		return fmt.Errorf("terminal not found: %s", terminalID)
	}
	
	return term.WriteRaw([]byte(text))
}

// GetTerminalBuffer gets the visible buffer from a terminal.
func (a *App) GetTerminalBuffer(terminalID string) (string, error) {
	a.mu.Lock()
	tw := a.activeWindow
	a.mu.Unlock()
	
	if tw == nil {
		return "", fmt.Errorf("no active window")
	}
	
	// Ask the frontend for the terminal's serialized content
	js := fmt.Sprintf(`(function() {
		// Find terminal by ID and serialize its buffer
		// This requires the serialize addon which we have loaded
		return ''; // TODO: implement buffer serialization
	})()`)
	
	result, err := tw.window.GetWebView().Eval(js)
	if err != nil {
		return "", err
	}
	
	return result, nil
}

// GetState returns the current app state.
func (a *App) GetState() (interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	state := map[string]interface{}{
		"windows": len(a.windows),
	}
	
	if a.activeWindow != nil {
		a.activeWindow.mu.Lock()
		terminals := make([]string, 0, len(a.activeWindow.terminals))
		for id := range a.activeWindow.terminals {
			terminals = append(terminals, id)
		}
		a.activeWindow.mu.Unlock()
		state["terminals"] = terminals
	}
	
	return state, nil
}

// Focus focuses the window.
func (a *App) Focus() error {
	a.mu.Lock()
	tw := a.activeWindow
	a.mu.Unlock()
	
	if tw == nil {
		return fmt.Errorf("no active window")
	}
	
	tw.window.Focus()
	return nil
}

// Close closes the application.
func (a *App) Close() error {
	a.platform.Quit()
	return nil
}
