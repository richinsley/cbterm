// Package control provides a JSON-based control interface for cbterm.
// This enables programmatic control by agents, scripts, and tests.
package control

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

// Command represents an incoming control command.
// terminal_id / tab_id aliases are accepted for convenience (they match the
// names used in responses and events); the documented names are
// "terminal" and "tab".
type Command struct {
	Cmd        string `json:"cmd"`
	TerminalID string `json:"terminal,omitempty"`
	Terminal   string `json:"terminal_id,omitempty"` // alias
	TabID      string `json:"tab,omitempty"`
	Tab        string `json:"tab_id,omitempty"`      // alias
	Text       string `json:"text,omitempty"`
	Direction  string `json:"direction,omitempty"` // "horizontal" or "vertical"
	Scrollback int    `json:"scrollback,omitempty"` // get_buffer: limit to last N lines
	JS         string `json:"js,omitempty"`         // eval_js: JS statements to run in the UI
	RequestID  string `json:"request_id,omitempty"` // For correlating responses
}

// Response represents an outgoing response or event.
type Response struct {
	Type      string      `json:"type"` // "response", "event", "error"
	RequestID string      `json:"request_id,omitempty"`
	Cmd       string      `json:"cmd,omitempty"`
	Success   bool        `json:"success,omitempty"`
	Error     string      `json:"error,omitempty"`
	Data      interface{} `json:"data,omitempty"`
}

// Event types
const (
	EventTerminalOutput  = "terminal_output"
	EventTerminalCreated = "terminal_created"
	EventTerminalClosed  = "terminal_closed"
	EventTabCreated      = "tab_created"
	EventTabClosed       = "tab_closed"
	EventPaneSplit       = "pane_split"
	EventStateChanged    = "state_changed"
)

// Handler processes control commands.
type Handler interface {
	// Terminal operations
	NewTab() (tabID string, terminalID string, err error)
	CloseTab(tabID string) error
	SplitPane(direction string) (newTerminalID string, err error)
	ClosePane(terminalID string) error
	
	// Input/Output
	TypeText(terminalID string, text string) error
	GetTerminalBuffer(terminalID string, scrollback int) (string, error)
	EvalJS(js string) (string, error)
	
	// State
	GetState() (interface{}, error)
	
	// Window
	Focus() error
	Close() error
}

// Controller manages the control interface.
type Controller struct {
	handler Handler
	input   io.Reader
	output  io.Writer
	mu      sync.Mutex
	done    chan struct{}
}

// New creates a new controller.
func New(handler Handler) *Controller {
	return &Controller{
		handler: handler,
		input:   os.Stdin,
		output:  os.Stdout,
		done:    make(chan struct{}),
	}
}

// SetIO sets custom input/output streams.
func (c *Controller) SetIO(input io.Reader, output io.Writer) {
	c.input = input
	c.output = output
}

// Start begins listening for commands.
func (c *Controller) Start() {
	go c.readLoop()
}

// Stop stops the controller.
func (c *Controller) Stop() {
	close(c.done)
}

// Emit sends an event to the output.
func (c *Controller) Emit(eventType string, data interface{}) {
	c.send(Response{
		Type: "event",
		Cmd:  eventType,
		Data: data,
	})
}

// EmitTerminalOutput sends terminal output event.
func (c *Controller) EmitTerminalOutput(terminalID string, data string) {
	c.Emit(EventTerminalOutput, map[string]string{
		"terminal": terminalID,
		"data":     data,
	})
}

func (c *Controller) readLoop() {
	scanner := bufio.NewScanner(c.input)
	// Increase buffer size for large commands
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	
	for scanner.Scan() {
		select {
		case <-c.done:
			return
		default:
		}
		
		line := scanner.Text()
		if line == "" {
			continue
		}
		
		var cmd Command
		if err := json.Unmarshal([]byte(line), &cmd); err != nil {
			c.sendError("", "parse_error", fmt.Sprintf("invalid JSON: %v", err))
			continue
		}
		// Resolve the terminal_id / tab_id aliases
		if cmd.TerminalID == "" {
			cmd.TerminalID = cmd.Terminal
		}
		if cmd.TabID == "" {
			cmd.TabID = cmd.Tab
		}
		
		c.handleCommand(cmd)
	}
	
	if err := scanner.Err(); err != nil {
		log.Printf("Control input error: %v", err)
	}
}

func (c *Controller) handleCommand(cmd Command) {
	var resp Response
	resp.RequestID = cmd.RequestID
	resp.Cmd = cmd.Cmd
	resp.Type = "response"
	
	switch cmd.Cmd {
	case "ping":
		resp.Success = true
		resp.Data = map[string]string{"pong": "cbterm"}
		
	case "new_tab":
		tabID, terminalID, err := c.handler.NewTab()
		if err != nil {
			resp.Error = err.Error()
		} else {
			resp.Success = true
			resp.Data = map[string]string{
				"tab_id":      tabID,
				"terminal_id": terminalID,
			}
		}
		
	case "close_tab":
		if err := c.handler.CloseTab(cmd.TabID); err != nil {
			resp.Error = err.Error()
		} else {
			resp.Success = true
		}
		
	case "split":
		direction := cmd.Direction
		if direction == "" {
			direction = "vertical"
		}
		terminalID, err := c.handler.SplitPane(direction)
		if err != nil {
			resp.Error = err.Error()
		} else {
			resp.Success = true
			resp.Data = map[string]string{"terminal_id": terminalID}
		}
		
	case "close_pane":
		if err := c.handler.ClosePane(cmd.TerminalID); err != nil {
			resp.Error = err.Error()
		} else {
			resp.Success = true
		}
		
	case "type":
		if err := c.handler.TypeText(cmd.TerminalID, cmd.Text); err != nil {
			resp.Error = err.Error()
		} else {
			resp.Success = true
		}
		
	case "get_buffer":
		buffer, err := c.handler.GetTerminalBuffer(cmd.TerminalID, cmd.Scrollback)
		if err != nil {
			resp.Error = err.Error()
		} else {
			resp.Success = true
			resp.Data = map[string]string{"buffer": buffer}
		}
		
	case "eval_js":
		result, err := c.handler.EvalJS(cmd.JS)
		if err != nil {
			resp.Error = err.Error()
		} else {
			resp.Success = true
			resp.Data = map[string]string{"result": result}
		}
		
	case "get_state":
		state, err := c.handler.GetState()
		if err != nil {
			resp.Error = err.Error()
		} else {
			resp.Success = true
			resp.Data = state
		}
		
	case "focus":
		if err := c.handler.Focus(); err != nil {
			resp.Error = err.Error()
		} else {
			resp.Success = true
		}
		
	case "quit", "exit", "close":
		resp.Success = true
		c.send(resp)
		c.handler.Close()
		return
		
	default:
		resp.Error = fmt.Sprintf("unknown command: %s", cmd.Cmd)
	}
	
	c.send(resp)
}

func (c *Controller) send(resp Response) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Failed to marshal response: %v", err)
		return
	}
	
	fmt.Fprintln(c.output, string(data))
}

func (c *Controller) sendError(requestID, cmd, message string) {
	c.send(Response{
		Type:      "error",
		RequestID: requestID,
		Cmd:       cmd,
		Error:     message,
	})
}
