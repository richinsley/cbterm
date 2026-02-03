// Package terminal provides PTY management for cbterm.
package terminal

import (
	"encoding/base64"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
)

// Terminal represents a PTY connected to a shell with callbacks for I/O.
type Terminal struct {
	cmd    *exec.Cmd
	file   *os.File
	mu     sync.Mutex
	closed bool

	// OnOutput is called when data is available from the PTY.
	// The data is base64 encoded for safe transport.
	OnOutput func(data string)

	// OnExit is called when the shell process exits.
	// The callback receives the exit code (0 = success).
	OnExit func(exitCode int)
}

// Options configures a new terminal.
type Options struct {
	// Shell is the shell to run (default: $SHELL or /bin/zsh)
	Shell string

	// Args are additional arguments to pass to the shell
	Args []string

	// Env is additional environment variables
	Env []string

	// Dir is the working directory
	Dir string

	// Rows and Cols set the initial terminal size
	Rows uint16
	Cols uint16

	// OnOutput is called when data is available from the PTY
	OnOutput func(data string)

	// OnExit is called when the shell process exits
	OnExit func(exitCode int)
}

// New creates and starts a new terminal with a shell.
func New(opts Options) (*Terminal, error) {
	shell := opts.Shell
	if shell == "" {
		shell = os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/zsh"
		}
	}

	args := append([]string{shell}, opts.Args...)
	cmd := exec.Command(args[0], args[1:]...)

	// Set environment
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "TERM=xterm-256color")
	cmd.Env = append(cmd.Env, "COLORTERM=truecolor")
	cmd.Env = append(cmd.Env, opts.Env...)

	if opts.Dir != "" {
		cmd.Dir = opts.Dir
	}

	// Set initial size
	rows := opts.Rows
	cols := opts.Cols
	if rows == 0 {
		rows = 24
	}
	if cols == 0 {
		cols = 80
	}

	// Start PTY
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	})
	if err != nil {
		return nil, err
	}

	t := &Terminal{
		cmd:      cmd,
		file:     ptmx,
		OnOutput: opts.OnOutput,
		OnExit:   opts.OnExit,
	}

	// Start reading from PTY
	go t.readLoop()

	// Monitor for process exit
	go t.waitForExit()

	return t, nil
}

// readLoop reads from PTY and calls OnOutput.
func (t *Terminal) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := t.file.Read(buf)
		if err != nil {
			if err != io.EOF && !t.closed {
				// Log error but don't crash
			}
			return
		}
		if n > 0 && t.OnOutput != nil {
			// Base64 encode for safe transport through JS
			encoded := base64.StdEncoding.EncodeToString(buf[:n])
			t.OnOutput(encoded)
		}
	}
}

// waitForExit waits for the shell process to exit and calls OnExit.
func (t *Terminal) waitForExit() {
	err := t.cmd.Wait()
	
	t.mu.Lock()
	t.closed = true
	t.mu.Unlock()

	if t.OnExit != nil {
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = 1
			}
		}
		t.OnExit(exitCode)
	}
}

// Write sends input to the terminal.
// Data should be base64 encoded.
func (t *Terminal) Write(base64Data string) error {
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		// Try as raw string (for simple ASCII input)
		data = []byte(base64Data)
	}
	_, err = t.file.Write(data)
	return err
}

// WriteRaw sends raw bytes to the terminal.
func (t *Terminal) WriteRaw(data []byte) error {
	_, err := t.file.Write(data)
	return err
}

// Resize changes the terminal size.
func (t *Terminal) Resize(rows, cols uint16) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	return pty.Setsize(t.file, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	})
}

// Close terminates the terminal.
func (t *Terminal) Close() error {
	t.mu.Lock()
	t.closed = true
	t.mu.Unlock()

	if err := t.file.Close(); err != nil {
		return err
	}
	return t.cmd.Wait()
}
