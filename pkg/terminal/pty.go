// Package terminal provides PTY management for cbterm.
package terminal

import (
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
)

// PTY represents a pseudo-terminal connected to a shell.
type PTY struct {
	cmd  *exec.Cmd
	file *os.File
	mu   sync.Mutex
}

// Options configures a new PTY.
type Options struct {
	// Shell is the shell to run (default: $SHELL or /bin/zsh)
	Shell string

	// Args are additional arguments to pass to the shell
	Args []string

	// Env is additional environment variables
	Env []string

	// Dir is the working directory
	Dir string

	// InitialRows and InitialCols set the initial terminal size
	InitialRows uint16
	InitialCols uint16
}

// New creates and starts a new PTY with a shell.
func New(opts Options) (*PTY, error) {
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
	cmd.Env = append(cmd.Env, opts.Env...)

	if opts.Dir != "" {
		cmd.Dir = opts.Dir
	}

	// Set initial size
	rows := opts.InitialRows
	cols := opts.InitialCols
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

	return &PTY{
		cmd:  cmd,
		file: ptmx,
	}, nil
}

// Read reads from the PTY.
func (p *PTY) Read(buf []byte) (int, error) {
	return p.file.Read(buf)
}

// Write writes to the PTY.
func (p *PTY) Write(data []byte) (int, error) {
	return p.file.Write(data)
}

// Resize changes the terminal size.
func (p *PTY) Resize(rows, cols uint16) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	return pty.Setsize(p.file, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	})
}

// Close terminates the PTY and shell process.
func (p *PTY) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Close the PTY file
	if err := p.file.Close(); err != nil {
		return err
	}

	// Wait for the process to exit
	return p.cmd.Wait()
}

// File returns the underlying PTY file descriptor.
func (p *PTY) File() *os.File {
	return p.file
}

// Pid returns the shell process ID.
func (p *PTY) Pid() int {
	return p.cmd.Process.Pid
}

// Reader returns an io.Reader for the PTY.
func (p *PTY) Reader() io.Reader {
	return p.file
}

// Writer returns an io.Writer for the PTY.
func (p *PTY) Writer() io.Writer {
	return p.file
}
