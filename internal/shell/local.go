package shell

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/creack/pty"
	"github.com/sirupsen/logrus"
)

// LocalManager creates local shell sessions using PTY
type LocalManager struct {
	logger      *logrus.Logger
	contentPath string
}

// NewLocalManager creates a new local shell manager
func NewLocalManager(logger *logrus.Logger, contentPath string) *LocalManager {
	if contentPath == "" {
		contentPath = "./content"
	}
	return &LocalManager{
		logger:      logger,
		contentPath: contentPath,
	}
}

// CreateSession creates a new local shell session
func (m *LocalManager) CreateSession(sessionID string) (Session, error) {
	m.logger.WithField("session_id", sessionID).Debug("Creating local shell session")

	// Prefer bash (history, tab completion); fall back to sh
	shellPath, err := exec.LookPath("bash")
	args := []string{"-i", "-l"}
	if err != nil {
		shellPath, args = "/bin/sh", []string{"-l"}
	}

	absContentPath, err := filepath.Abs(m.contentPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve content path: %w", err)
	}

	// Local mode deliberately runs the developer's own shell
	cmd := exec.Command(shellPath, args...) // #nosec G204
	cmd.Dir = absContentPath
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"HOME="+absContentPath,
		"USER=guest",
		"SHELL="+shellPath,
		"PS1=guest@portfolio:\\w\\$ ",
	)

	return &LocalSession{
		cmd:    cmd,
		logger: m.logger,
	}, nil
}

// LocalSession represents a local shell session wrapped in a PTY
type LocalSession struct {
	cmd    *exec.Cmd
	ptmx   *os.File
	logger *logrus.Logger
}

// Start launches the command with a PTY
func (s *LocalSession) Start() error {
	ptmx, err := pty.Start(s.cmd)
	if err != nil {
		return fmt.Errorf("failed to start PTY: %w", err)
	}

	s.ptmx = ptmx
	return nil
}

// Resize handles terminal window size changes
func (s *LocalSession) Resize(cols, rows uint16) error {
	if s.ptmx == nil {
		return fmt.Errorf("PTY not initialized")
	}
	return pty.Setsize(s.ptmx, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	})
}

// Reader returns the PTY master file descriptor for reading shell output
func (s *LocalSession) Reader() io.Reader {
	return s.ptmx
}

// Writer returns the PTY master file descriptor for writing shell input
func (s *LocalSession) Writer() io.Writer {
	return s.ptmx
}

// Wait blocks until the shell process exits
func (s *LocalSession) Wait() error {
	if s.cmd == nil {
		return fmt.Errorf("command not started")
	}
	return s.cmd.Wait()
}

// Close cleans up the PTY and terminates the process
func (s *LocalSession) Close() error {
	var err error
	if s.ptmx != nil {
		err = s.ptmx.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		// The process may already have exited; that is fine
		_ = s.cmd.Process.Kill()
	}
	return err
}

// Mode returns the session mode
func (s *LocalSession) Mode() string {
	return "local"
}
