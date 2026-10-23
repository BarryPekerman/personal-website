// Package shell provides the terminal behind a visitor session: a simulated
// shell that only reads files embedded in the binary.
package shell

import (
	"io"
)

// Session is what the WebSocket handler needs from a terminal session
type Session interface {
	// Start launches the shell
	Start() error

	// Resize handles window size changes from xterm.js
	Resize(cols, rows uint16) error

	// Wait blocks until the shell exits
	Wait() error

	// Reader returns the stream to read shell output from
	Reader() io.Reader

	// Writer returns the stream to write shell input to
	Writer() io.Writer

	// Close cleans up resources (closes streams, ends the shell)
	Close() error
}

// Manager creates new shell sessions
type Manager interface {
	// CreateSession creates a new shell session with the given ID
	CreateSession(sessionID string) (Session, error)
}
