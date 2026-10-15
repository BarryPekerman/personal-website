package shell

import (
	"io"
)

// Session defines what our web server needs to interact with a terminal session.
// This abstraction allows us to swap between Kubernetes pods and local processes
// without changing the WebSocket handling code.
type Session interface {
	// Start launches the process (Pod exec or Local Command)
	Start() error

	// Resize handles window size changes from xterm.js
	Resize(cols, rows uint16) error

	// Wait blocks until the shell exits
	Wait() error

	// Reader returns the stream to read shell output from
	Reader() io.Reader

	// Writer returns the stream to write shell input to
	Writer() io.Writer

	// Close cleans up resources (closes streams, terminates process)
	Close() error

	// Mode returns the session mode ("local" or "k8s")
	Mode() string
}

// Manager creates new shell sessions
type Manager interface {
	// CreateSession creates a new shell session with the given ID
	CreateSession(sessionID string) (Session, error)
}

