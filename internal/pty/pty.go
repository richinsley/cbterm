package pty

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
)

// PTY wraps a pseudo-terminal
type PTY struct {
	cmd  *exec.Cmd
	file *os.File
}

// New creates a new PTY running the specified shell
func New(shell string, width, height int) (*PTY, error) {
	cmd := exec.Command(shell, "-l") // Login shell
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"TERM_PROTOCOL=1",      // Signal our protocol support
		"COLORTERM=truecolor",
	)

	// Start with PTY
	f, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(height),
		Cols: uint16(width),
	})
	if err != nil {
		return nil, err
	}

	return &PTY{
		cmd:  cmd,
		file: f,
	}, nil
}

// Read reads from the PTY
func (p *PTY) Read(buf []byte) (int, error) {
	return p.file.Read(buf)
}

// Write writes to the PTY
func (p *PTY) Write(data []byte) (int, error) {
	return p.file.Write(data)
}

// Resize changes the PTY size
func (p *PTY) Resize(width, height int) error {
	return pty.Setsize(p.file, &pty.Winsize{
		Rows: uint16(height),
		Cols: uint16(width),
	})
}

// Close closes the PTY and terminates the shell
func (p *PTY) Close() error {
	// Send SIGHUP to shell
	if p.cmd.Process != nil {
		p.cmd.Process.Signal(syscall.SIGHUP)
	}
	return p.file.Close()
}

// Pid returns the shell process ID
func (p *PTY) Pid() int {
	if p.cmd.Process != nil {
		return p.cmd.Process.Pid
	}
	return 0
}
