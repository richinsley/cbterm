// Package extensions handles extension discovery, validation, and lifecycle.
package extensions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Manifest describes an extension's metadata and requirements.
type Manifest struct {
	Name             string   `json:"name"`
	Version          string   `json:"version"`
	Description      string   `json:"description"`
	Main             string   `json:"main"`
	Permissions      []string `json:"permissions"`
	ActivationEvents []string `json:"activationEvents"`
}

// Extension represents a discovered extension.
type Extension struct {
	Manifest Manifest `json:"manifest"`
	Path     string   `json:"path"`     // Absolute path to extension directory
	Enabled  bool     `json:"enabled"`  // Whether extension is enabled
	Error    string   `json:"error,omitempty"` // Load error, if any
}

// Manager handles extension discovery and state.
type Manager struct {
	basePath   string
	extensions map[string]*Extension
	mu         sync.RWMutex
}

// NewManager creates an extension manager for the given base path.
func NewManager(basePath string) *Manager {
	return &Manager{
		basePath:   basePath,
		extensions: make(map[string]*Extension),
	}
}

// DefaultBasePath returns the default extensions directory.
func DefaultBasePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "cbterm", "extensions")
}

// Discover scans the extensions directory and loads manifests.
func (m *Manager) Discover() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear existing
	m.extensions = make(map[string]*Extension)

	// Ensure base path exists
	if _, err := os.Stat(m.basePath); os.IsNotExist(err) {
		// No extensions directory - that's fine
		return nil
	}

	entries, err := os.ReadDir(m.basePath)
	if err != nil {
		return fmt.Errorf("failed to read extensions directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		extPath := filepath.Join(m.basePath, entry.Name())
		manifestPath := filepath.Join(extPath, "manifest.json")

		ext := &Extension{
			Path:    extPath,
			Enabled: true,
		}

		// Try to load manifest
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			ext.Error = fmt.Sprintf("failed to read manifest: %v", err)
			ext.Enabled = false
			m.extensions[entry.Name()] = ext
			continue
		}

		if err := json.Unmarshal(data, &ext.Manifest); err != nil {
			ext.Error = fmt.Sprintf("invalid manifest: %v", err)
			ext.Enabled = false
			m.extensions[entry.Name()] = ext
			continue
		}

		// Validate manifest
		if ext.Manifest.Name == "" {
			ext.Manifest.Name = entry.Name()
		}
		if ext.Manifest.Main == "" {
			ext.Manifest.Main = "index.js"
		}

		// Verify main file exists
		mainPath := filepath.Join(extPath, ext.Manifest.Main)
		if _, err := os.Stat(mainPath); os.IsNotExist(err) {
			ext.Error = fmt.Sprintf("main file not found: %s", ext.Manifest.Main)
			ext.Enabled = false
		}

		m.extensions[ext.Manifest.Name] = ext
	}

	return nil
}

// List returns all discovered extensions.
func (m *Manager) List() []*Extension {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Extension, 0, len(m.extensions))
	for _, ext := range m.extensions {
		result = append(result, ext)
	}
	return result
}

// ListEnabled returns only enabled extensions.
func (m *Manager) ListEnabled() []*Extension {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Extension, 0)
	for _, ext := range m.extensions {
		if ext.Enabled && ext.Error == "" {
			result = append(result, ext)
		}
	}
	return result
}

// Get returns an extension by name.
func (m *Manager) Get(name string) *Extension {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.extensions[name]
}

// SetEnabled enables or disables an extension.
func (m *Manager) SetEnabled(name string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ext, ok := m.extensions[name]
	if !ok {
		return fmt.Errorf("extension not found: %s", name)
	}
	ext.Enabled = enabled
	return nil
}

// ToJSON returns the enabled extensions as JSON for the frontend.
func (m *Manager) ToJSON() ([]byte, error) {
	return json.Marshal(m.ListEnabled())
}
