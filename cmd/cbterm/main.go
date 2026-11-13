// Command cbterm is an AI-native terminal emulator.
//
// The UI is an xterm.js frontend embedded in the binary and rendered by
// go-webui (github.com/webui-dev/go-webui/v2): a native webview window on
// macOS/Windows/Linux, or a pure web server that the same UI can open in a
// browser (terminal server mode, --web).
//
// JS <-> Go IPC rides go-webui's WebSocket bridge:
//
//	JS -> Go: window.cbtermToGo(jsonString)   (bound per window via ui.Bind)
//	Go -> JS: window.Run(js) fire-and-forget / window.Script(js) sync w/ reply
//
// The bind name must not be "cbterm": go-webui installs bound functions as
// window globals only when the name is still undefined, and the frontend
// already owns window.cbterm as its Go->JS API object.
//
// Modes:
//
//	cbterm            native webview window
//	cbterm --web      terminal server: prints a URL to open in a browser
//	cbterm --control  also accepts JSON commands on stdin (see docs/CONTROL.md)
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"mime"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	cbterm "github.com/calobozan/cbterm"
	"github.com/calobozan/cbterm/pkg/control"
	"github.com/calobozan/cbterm/pkg/extensions"
	"github.com/calobozan/cbterm/pkg/terminal"
	ui "github.com/webui-dev/go-webui/v2"
)

var (
	version = "dev"
	commit  = "none"
)

// App holds global application state
type App struct {
	extensions  *extensions.Manager
	controller  *control.Controller
	shell       string
	debug       bool
	controlMode bool
	webMode     bool
	public      bool
	frontendFS  fs.FS

	mu      sync.Mutex
	windows map[ui.Window]*TerminalWindow

	// For control mode - track the active window
	activeWindow *TerminalWindow
}

// TerminalWindow holds per-window state
type TerminalWindow struct {
	window    ui.Window
	terminals map[string]*terminal.Terminal
	mu        sync.Mutex
}

var app *App

func main() {
	showVersion := flag.Bool("version", false, "Show version")
	debug := flag.Bool("debug", false, "Enable debug logging")
	controlMode := flag.Bool("control", false, "Enable control mode (JSON commands via stdin)")
	shell := flag.String("shell", "", "Shell to run (default: $SHELL or /bin/zsh)")
	web := flag.Bool("web", false, "Run as a terminal server: print a URL to open in a browser")
	public := flag.Bool("public", false, "With --web: bind to all interfaces (remote browsers); localhost only by default")
	multi := flag.Bool("multi", false, "Allow multiple browser clients (terminal server tabs). Disables sync control commands (new_tab/split/get_buffer), which need a single client")
	flag.Parse()

	log.SetFlags(log.Ltime)

	if *showVersion {
		fmt.Printf("cbterm %s (%s)\n", version, commit)
		os.Exit(0)
	}

	// Embedded frontend, rooted at frontend/dist
	frontendFS, err := fs.Sub(cbterm.FrontendAssets, "frontend/dist")
	if err != nil {
		log.Fatalf("Failed to access embedded frontend: %v", err)
	}

	// Single-client mode by default: it keeps the synchronous Script()
	// path working (control-mode new_tab/split/get_buffer). Pass --multi
	// to allow several browsers on one terminal server, at the cost of
	// those sync commands.
	ui.SetConfig(ui.MultiClient, *multi)

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
		extensions:  extManager,
		shell:       *shell,
		debug:       *debug,
		controlMode: *controlMode,
		webMode:     *web,
		public:      *public,
		frontendFS:  frontendFS,
		windows:     make(map[ui.Window]*TerminalWindow),
	}

	// Start control mode if enabled
	if *controlMode {
		app.controller = control.New(app)
		app.controller.Start()
		log.Println("Control mode enabled - accepting JSON commands on stdin")
	}

	// Create first window (native webview or terminal server)
	if err := app.NewWindow(); err != nil {
		log.Fatalf("Failed to create window: %v", err)
	}

	// Block until the app is done: all webview windows closed, or
	// webui_exit() in server mode (e.g. via the control 'quit' command).
	ui.Wait()

	// Cleanup: close any PTYs still alive (server mode)
	for _, tw := range app.listWindows() {
		tw.closeTerminals()
	}
}

// NewWindow creates a new terminal window
func (a *App) NewWindow() error {
	win := ui.NewWindow()

	tw := &TerminalWindow{
		window:    win,
		terminals: make(map[string]*terminal.Terminal),
	}

	a.mu.Lock()
	a.windows[win] = tw
	a.activeWindow = tw
	a.mu.Unlock()

	// Serve the embedded frontend plus the extension/theme/API namespaces
	win.SetFileHandler(func(filename string) ([]byte, int) {
		resp := a.serveFile(filename)
		return resp, len(resp)
	})

	// JS -> Go
	ui.Bind(win, "cbtermToGo", tw.handleMessage)

	// Window close: clean up PTYs, quit when the last window is gone
	win.SetCloseHandlerWebView(func(w ui.Window) bool {
		tw.closeTerminals()

		a.mu.Lock()
		delete(a.windows, w)
		if a.activeWindow == tw {
			a.activeWindow = nil
			for _, remaining := range a.windows {
				a.activeWindow = remaining
				break
			}
		}
		remaining := len(a.windows)
		a.mu.Unlock()

		log.Printf("Window closed, %d remaining", remaining)

		if remaining == 0 {
			ui.Exit()
		}
		return true
	})

	if a.webMode {
		if a.public {
			win.SetPublic(true)
		}
		url := win.StartServer("/")
		fmt.Println(url)
		log.Printf("Terminal server running — open the URL above in a browser")
		return nil
	}

	win.SetSize(800, 600)
	win.SetCenter()
	if err := win.ShowWebView("/"); err != nil {
		return err
	}
	return nil
}

// handleMessage processes messages from the frontend (bound as
// window.cbtermToGo by go-webui)
func (tw *TerminalWindow) handleMessage(e ui.Event) ui.Void {
	msg, err := ui.GetArg[string](e)
	if err != nil {
		log.Printf("Bind argument error: %v", err)
		return nil
	}

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
		return nil
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

	case "new_window":
		if err := app.NewWindow(); err != nil {
			log.Printf("Failed to create window: %v", err)
		}

	case "ready", "new_terminal":
		// Create a new terminal for this terminal ID
		terminalId := m.TerminalId
		if terminalId == "" {
			terminalId = "term-1" // backwards compat
		}

		tw.mu.Lock()
		if tw.terminals == nil {
			tw.terminals = make(map[string]*terminal.Terminal)
		}

		if tw.terminals[terminalId] != nil {
			// Duplicate request (e.g. frontend sent both ready and
			// new_terminal for the first tab) — the terminal exists.
			tw.mu.Unlock()
			return nil
		}

		log.Printf("Creating terminal %s (%dx%d)", terminalId, m.Cols, m.Rows)
			tid := terminalId // capture for closure
			term, err := terminal.New(terminal.Options{
				Shell: app.shell,
				Cols:  uint16(m.Cols),
				Rows:  uint16(m.Rows),
				OnOutput: func(data string) {
					// Send output to specific terminal
					// data is base64 — template literal avoids escaping issues
					if app.debug {
						log.Printf("Terminal %s output: %d bytes", tid, len(data))
					}
					js := fmt.Sprintf("window.cbterm && window.cbterm.write(`%s`, `%s`)", tid, data)
					tw.window.Run(js)

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

						if remaining == 0 {
							// Close window when last terminal exits
							tw.window.Close()
							app.fallbackExitIfNoWindows()
						} else {
							// Tell frontend to close this pane
							js := fmt.Sprintf("window.cbterm.closePaneByTerminal && window.cbterm.closePaneByTerminal('%s')", tid)
							tw.window.Run(js)
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

	return nil
}

// closeTerminals closes all PTYs for this window
func (tw *TerminalWindow) closeTerminals() {
	tw.mu.Lock()
	toClose := make([]*terminal.Terminal, 0, len(tw.terminals))
	for _, term := range tw.terminals {
		toClose = append(toClose, term)
	}
	tw.terminals = nil // Clear map so OnExit callbacks don't try to modify
	tw.mu.Unlock()

	for _, term := range toClose {
		term.Close()
	}
	if len(toClose) > 0 {
		log.Printf("Closed %d terminals", len(toClose))
	}
}

// listWindows returns a snapshot of registered windows
func (a *App) listWindows() []*TerminalWindow {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]*TerminalWindow, 0, len(a.windows))
	for _, tw := range a.windows {
		out = append(out, tw)
	}
	return out
}

// fallbackExitIfNoWindows exits the app if no windows remain after a
// programmatic close (the close handler may not fire for programmatic
// closes in server mode).
func (a *App) fallbackExitIfNoWindows() {
	go func() {
		time.Sleep(500 * time.Millisecond)
		a.mu.Lock()
		remaining := len(a.windows)
		a.mu.Unlock()
		if remaining == 0 {
			ui.Exit()
		}
	}()
}

// serveFile answers go-webui file requests. filename is the request URI
// ("/", "/index.html", "/assets/...", "/extensions/...", "/themes/...",
// "/api/..."). The library intercepts /webui.js before the handler.
//
// Response routing (replaces the old cbterm:// scheme):
//
//	/                -> embedded frontend
//	/extensions/...  -> ~/.config/cbterm/extensions/...
//	/themes/...      -> ~/.config/cbterm/themes/...
//	/api/...         -> virtual API endpoints
func (a *App) serveFile(filename string) []byte {
	p := strings.TrimPrefix(filename, "/")
	if i := strings.IndexByte(p, '?'); i >= 0 {
		p = p[:i]
	}

	switch {
	case p == "" || p == "index.html":
		data, err := fs.ReadFile(a.frontendFS, "index.html")
		if err != nil {
			return httpNotFound(p)
		}
		return httpResponse(data, "text/html; charset=utf-8")

	case strings.HasPrefix(p, "extensions/"):
		return a.serveFromUserDir("extensions", p[len("extensions/"):])

	case strings.HasPrefix(p, "themes/"):
		return a.serveFromUserDir("themes", p[len("themes/"):])

	case p == "api/extensions/list.json":
		if a.extensions != nil {
			data, err := a.extensions.ToJSON()
			if err != nil {
				log.Printf("Failed to serialize extensions: %v", err)
				return httpResponse([]byte("[]"), "application/json")
			}
			if a.debug {
				log.Printf("API: extensions/list.json -> %s", string(data))
			}
			return httpResponse(data, "application/json")
		}
		return httpResponse([]byte("[]"), "application/json")

	case p == "api/terminal.js":
		// Terminal API module for extensions. Uses window.cbtermBridge
		// (exposed by the frontend bundle's bridge module) so the virtual
		// module works on both the go-webui and legacy WKWebView backends.
		return httpResponse([]byte(`
			// Terminal API
			// Terminal.onData(callback): callback receives (base64Data, terminalId)
			// for every output chunk from any terminal, after it has been
			// written to the terminal.
			export const Terminal = {
				write(data) {
					window.cbtermBridge.send(JSON.stringify({type: 'input', data: data}));
				},
				onData(callback) {
					window.cbterm._onDataCallbacks = window.cbterm._onDataCallbacks || [];
					window.cbterm._onDataCallbacks.push(callback);
				}
			};
		`), "application/javascript")

	case p == "api/config.js":
		// Config API module (placeholder)
		return httpResponse([]byte(`
			// Config API
			export const Config = {
				async get(key) { return null; },
				async set(key, value) { }
			};
		`), "application/javascript")

	case strings.HasPrefix(p, "api/"):
		log.Printf("Unknown API endpoint: %s", p)
		return httpNotFound(p)
	}

	// Default: embedded frontend (index.html, assets/...)
	if strings.Contains(p, "..") {
		log.Printf("Blocked path traversal attempt: %s", p)
		return httpNotFound(p)
	}

	data, err := fs.ReadFile(a.frontendFS, p)
	if err != nil {
		if a.debug {
			log.Printf("404: %s", p)
		}
		return httpNotFound(p)
	}

	if a.debug {
		log.Printf("200: %s (%d bytes, %s)", p, len(data), a.mimeFor(p))
	}
	return httpResponse(data, a.mimeFor(p))
}

// serveFromUserDir serves a file from ~/.config/cbterm/<namespace>/ with
// directory-traversal protection
func (a *App) serveFromUserDir(namespace, resourcePath string) []byte {
	home, _ := os.UserHomeDir()
	base := filepath.Join(home, ".config", "cbterm", namespace)

	filePath := filepath.Join(base, resourcePath)

	// Security: prevent directory traversal
	absPath, err := filepath.Abs(filePath)
	if err != nil || !strings.HasPrefix(absPath, base) {
		log.Printf("Blocked %s path traversal: %s", namespace, resourcePath)
		return httpNotFound(resourcePath)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if a.debug {
			log.Printf("%s 404: %s", namespace, resourcePath)
		}
		return httpNotFound(resourcePath)
	}

	if a.debug {
		log.Printf("%s 200: %s (%d bytes)", namespace, resourcePath, len(data))
	}
	return httpResponse(data, a.mimeFor(resourcePath))
}

// mimeFor returns a content type for a path, with sane defaults for
// JavaScript variants the stdlib mime table misses
func (a *App) mimeFor(p string) string {
	switch path.Ext(p) {
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".json":
		return "application/json"
	case ".html":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".wasm":
		return "application/wasm"
	}
	if ct := mime.TypeByExtension(path.Ext(p)); ct != "" {
		return ct
	}
	return "application/octet-stream"
}

// httpResponse builds a full HTTP response (headers + body) as required by
// go-webui's file handler
func httpResponse(body []byte, contentType string) []byte {
	var sb strings.Builder
	sb.WriteString("HTTP/1.1 200 OK\r\n")
	sb.WriteString("Content-Type: ")
	sb.WriteString(contentType)
	sb.WriteString("\r\n")
	sb.WriteString("Content-Length: ")
	sb.WriteString(fmt.Sprintf("%d", len(body)))
	sb.WriteString("\r\n\r\n")
	return append([]byte(sb.String()), body...)
}

func httpNotFound(p string) []byte {
	body := []byte("404 not found: " + p)
	var sb strings.Builder
	sb.WriteString("HTTP/1.1 404 Not Found\r\n")
	sb.WriteString("Content-Type: text/plain\r\n")
	sb.WriteString(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body)))
	return append([]byte(sb.String()), body...)
}

// script runs a synchronous Go→JS script on the active window's client,
// with timing logged so hung calls are visible in the app log.
func (a *App) script(tw *TerminalWindow, label, js string, timeout uint) (string, error) {
	log.Printf("script %s: begin", label)
	start := time.Now()
	res, err := tw.window.Script(js, ui.ScriptOptions{Timeout: timeout})
	log.Printf("script %s: done in %v, err=%v", label, time.Since(start), err)
	return res, err
}

// Control mode Handler interface implementation

// NewTab creates a new tab in the active window.
func (a *App) NewTab() (tabID string, terminalID string, err error) {
	tw := a.active()
	if tw == nil {
		return "", "", fmt.Errorf("no active window")
	}

	// Tell frontend to create a new tab
	js := `return (function() {
		const tab = window.cbterm.createTab();
		return JSON.stringify({tabId: tab.id, terminalId: tab.rootPane.terminal?.id || ''});
	})()`

	result, err := a.script(tw, "new_tab", js, 5)
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
	tw := a.active()
	if tw == nil {
		return fmt.Errorf("no active window")
	}

	js := fmt.Sprintf(`return (window.cbterm.closeTab && window.cbterm.closeTab('%s'))`, tabID)
	_, err := a.script(tw, "close_tab", js, 5)
	return err
}

// SplitPane splits the active pane.
func (a *App) SplitPane(direction string) (newTerminalID string, err error) {
	tw := a.active()
	if tw == nil {
		return "", fmt.Errorf("no active window")
	}

	var fn string
	if direction == "horizontal" {
		fn = "splitHorizontal"
	} else {
		fn = "splitVertical"
	}
	js := fmt.Sprintf(`return (function() {
		const pane = window.cbterm.%s();
		return pane?.terminal?.id || '';
	})()`, fn)

	result, err := a.script(tw, "split", js, 5)
	if err != nil {
		return "", fmt.Errorf("failed to split pane: %v", err)
	}

	return strings.Trim(result, `"`), nil
}

// ClosePane closes a pane by terminal ID.
func (a *App) ClosePane(terminalID string) error {
	tw := a.active()
	if tw == nil {
		return fmt.Errorf("no active window")
	}

	js := fmt.Sprintf(`return (window.cbterm.closePaneByTerminal && window.cbterm.closePaneByTerminal(%s))`, mustJSONString(terminalID))
	_, err := a.script(tw, "close_pane", js, 5)
	return err
}

// mustJSONString marshals a Go string as a JSON string literal (with quotes
// and escaping) for safe embedding in generated JS.
func mustJSONString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `"unknown"`
	}
	return string(b)
}

// resolveTerminal returns the terminal for id, or the window's first
// terminal when id is empty (control commands may omit it).
func (tw *TerminalWindow) resolveTerminal(terminalID string) *terminal.Terminal {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if terminalID != "" {
		return tw.terminals[terminalID]
	}
	ids := make([]string, 0, len(tw.terminals))
	for id := range tw.terminals {
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil
	}
	sort.Strings(ids)
	return tw.terminals[ids[0]]
}

// TypeText sends text to a terminal.
func (a *App) TypeText(terminalID string, text string) error {
	tw := a.active()
	if tw == nil {
		return fmt.Errorf("no active window")
	}

	term := tw.resolveTerminal(terminalID)
	if term == nil {
		return fmt.Errorf("terminal not found: %s", terminalID)
	}

	return term.WriteRaw([]byte(text))
}

// GetTerminalBuffer gets the serialized buffer from a terminal.
// Returns the terminal content (with ANSI escapes) as it currently renders:
// visible rows always, plus the last N lines of scrollback when
// scrollback > 0.
func (a *App) GetTerminalBuffer(terminalID string, scrollback int) (string, error) {
	tw := a.active()
	if tw == nil {
		return "", fmt.Errorf("no active window")
	}
	if tw.resolveTerminal(terminalID) == nil {
		return "", fmt.Errorf("terminal not found: %s", terminalID)
	}

	// Ask the frontend for the terminal's serialized content
	idJSON, _ := json.Marshal(terminalID)
	js := fmt.Sprintf(`return (window.cbterm.getBuffer ? window.cbterm.getBuffer(%s, %d) : '')`, idJSON, scrollback)

	result, err := a.script(tw, "get_buffer", js, 5)
	if err != nil {
		return "", err
	}

	return result, nil
}

// EvalJS runs raw JS statements in the active window's UI. The script body
// executes as an async function body, so a top-level `return` produces the
// result (returned as a string; use JSON.stringify for structured values).
// Only available while a window exists; requires the single-client
// Script() path (not usable with --multi).
func (a *App) EvalJS(js string) (string, error) {
	tw := a.active()
	if tw == nil {
		return "", fmt.Errorf("no active window")
	}
	result, err := a.script(tw, "eval_js", js, 5)
	if err != nil {
		return "", err
	}
	return result, nil
}

// GetState returns the current app state.
func (a *App) GetState() (interface{}, error) {
	a.mu.Lock()
	state := map[string]interface{}{
		"windows": len(a.windows),
	}

	tw := a.activeWindow
	if tw != nil {
		tw.mu.Lock()
		terminals := make([]string, 0, len(tw.terminals))
		for id := range tw.terminals {
			terminals = append(terminals, id)
		}
		tw.mu.Unlock()
		state["terminals"] = terminals
	}
	a.mu.Unlock()

	return state, nil
}

// Focus focuses the window. go-webui exposes no window-focus API, so this
// is a no-op (kept for the control protocol).
func (a *App) Focus() error {
	return nil
}

// Close closes the application.
func (a *App) Close() error {
	ui.Exit()
	return nil
}

// active returns the active window's state
func (a *App) active() *TerminalWindow {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.activeWindow
}
