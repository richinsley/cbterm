package protocol

import (
	"bytes"
	"encoding/json"
)

const (
	// OSC 7776 is our private-use code ("TERM" on phone keypad)
	oscPrefix = "\x1b]7776;"
	bel       = '\x07'
	st        = "\x1b\\"
)

// Extension represents a parsed protocol extension
type Extension struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// Handler processes terminal protocol extensions
type Handler struct {
	aiContext map[string]interface{}
}

// NewHandler creates a new protocol handler
func NewHandler() *Handler {
	return &Handler{
		aiContext: make(map[string]interface{}),
	}
}

// ProcessOutput scans terminal output for protocol extensions
// Returns: (cleaned output for display, extracted extensions as JSON)
func (h *Handler) ProcessOutput(data []byte) ([]byte, [][]byte) {
	var extensions [][]byte
	var cleaned bytes.Buffer

	i := 0
	for i < len(data) {
		// Look for OSC 7776 sequence
		if i+len(oscPrefix) <= len(data) && string(data[i:i+len(oscPrefix)]) == oscPrefix {
			// Find the end of the sequence (BEL or ST)
			start := i + len(oscPrefix)
			end := -1
			
			for j := start; j < len(data); j++ {
				if data[j] == bel {
					end = j
					break
				}
				if j+1 < len(data) && string(data[j:j+2]) == st {
					end = j
					break
				}
			}

			if end > start {
				// Extract and parse the JSON payload
				payload := data[start:end]
				
				var ext Extension
				if err := json.Unmarshal(payload, &ext); err == nil {
					// Handle the extension
					h.handleExtension(&ext)
					
					// Add to extensions list for frontend
					extensions = append(extensions, payload)
				}

				// Skip past the sequence
				if end < len(data) && data[end] == bel {
					i = end + 1
				} else {
					i = end + 2 // ST is two bytes
				}
				continue
			}
		}

		// Regular byte - pass through
		cleaned.WriteByte(data[i])
		i++
	}

	return cleaned.Bytes(), extensions
}

// handleExtension processes a parsed extension
func (h *Handler) handleExtension(ext *Extension) {
	switch ext.Type {
	case "ai.context.push":
		h.handleContextPush(ext.Data)
	case "ai.context.pop":
		h.handleContextPop(ext.Data)
	case "ai.boundary":
		h.handleBoundary(ext.Data)
	// Other handlers...
	}
}

func (h *Handler) handleContextPush(data json.RawMessage) {
	var ctx struct {
		Key   string      `json:"key"`
		Value interface{} `json:"value"`
	}
	if err := json.Unmarshal(data, &ctx); err == nil {
		h.aiContext[ctx.Key] = ctx.Value
	}
}

func (h *Handler) handleContextPop(data json.RawMessage) {
	var ctx struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(data, &ctx); err == nil {
		delete(h.aiContext, ctx.Key)
	}
}

func (h *Handler) handleBoundary(data json.RawMessage) {
	// TODO: Track command boundaries for AI context
	// This will be important for semantic scrollback
}

// HandleInput checks if input is a protocol message
// Returns true if handled, false if it should go to PTY
func (h *Handler) HandleInput(data []byte) bool {
	// Check if this is a JSON protocol message from frontend
	if len(data) > 0 && data[0] == '{' {
		var msg struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(data, &msg); err == nil {
			switch msg.Type {
			case "resize":
				// Handled by app layer
				return false
			case "ai.query":
				// Handle AI queries
				return true
			}
		}
	}
	return false
}

// GetAIContext returns the current AI context
func (h *Handler) GetAIContext() map[string]interface{} {
	// Return a copy
	ctx := make(map[string]interface{})
	for k, v := range h.aiContext {
		ctx[k] = v
	}
	return ctx
}
