package shell

import (
	"fmt"
	"io"

	"github.com/sirupsen/logrus"
	"portfolio/internal/k8s"
)

// K8sManager creates Kubernetes pod-based shell sessions
type K8sManager struct {
	k8sClient *k8s.Client
	logger    *logrus.Logger
}

// NewK8sManager creates a new Kubernetes shell manager
func NewK8sManager(k8sClient *k8s.Client, logger *logrus.Logger) *K8sManager {
	return &K8sManager{
		k8sClient: k8sClient,
		logger:    logger,
	}
}

// CreateSession creates a new Kubernetes pod and returns a session
func (m *K8sManager) CreateSession(sessionID string) (Session, error) {
	m.logger.WithField("session_id", sessionID).Info("Creating Kubernetes shell session")

	// Create the pod
	pod, err := m.k8sClient.CreateTerminalPod(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to create terminal pod: %w", err)
	}

	return &K8sSession{
		k8sClient: m.k8sClient,
		podName:   pod.Name,
		sessionID: sessionID,
		logger:    m.logger,
	}, nil
}

// K8sSession represents a shell session running in a Kubernetes pod
type K8sSession struct {
	k8sClient *k8s.Client
	podName   string
	sessionID string
	logger    *logrus.Logger

	// Streams for communication
	stdinReader  io.Reader
	stdinWriter  io.Writer
	stdoutReader io.Reader
	stdoutWriter io.Writer

	// Exec goroutine management
	execDone chan error
}

// Start establishes the exec connection to the pod and starts the shell
func (s *K8sSession) Start() error {
	// Create pipes for stdin/stdout communication
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()

	s.stdinReader = stdinReader
	s.stdinWriter = stdinWriter
	s.stdoutReader = stdoutReader
	s.stdoutWriter = stdoutWriter
	s.execDone = make(chan error, 1)

	// Start the exec connection in a goroutine
	go func() {
		err := s.k8sClient.ExecInPod(s.podName, k8s.PodExecOptions{
			Command: []string{"/bin/sh", "-l"}, // Login shell to load .profile
			Stdin:   stdinReader,
			Stdout:  stdoutWriter,
			Stderr:  stdoutWriter,
			TTY:     true,
		})
		s.execDone <- err
	}()

	s.logger.WithFields(logrus.Fields{
		"pod_name":   s.podName,
		"session_id": s.sessionID,
	}).Debug("Kubernetes shell session started")

	return nil
}

// Resize handles terminal window size changes
// Note: Kubernetes remotecommand executor doesn't support resize in the current implementation
// This would require implementing TerminalSizeQueue - left as TODO for now
func (s *K8sSession) Resize(cols, rows uint16) error {
	// TODO: Implement terminal resize using remotecommand.TerminalSizeQueue
	// This requires refactoring ExecInPod to accept a size queue
	s.logger.WithFields(logrus.Fields{
		"cols":       cols,
		"rows":       rows,
		"session_id": s.sessionID,
	}).Debug("Terminal resize requested (not yet implemented for K8s mode)")
	return nil
}

// Reader returns the stream to read shell output from
func (s *K8sSession) Reader() io.Reader {
	return s.stdoutReader
}

// Writer returns the stream to write shell input to
func (s *K8sSession) Writer() io.Writer {
	return s.stdinWriter
}

// Wait blocks until the shell process exits
func (s *K8sSession) Wait() error {
	if s.execDone == nil {
		return fmt.Errorf("session not started")
	}
	return <-s.execDone
}

// Close cleans up the session and deletes the pod
func (s *K8sSession) Close() error {
	// Close the pipes
	if s.stdinWriter != nil {
		if closer, ok := s.stdinWriter.(io.Closer); ok {
			closer.Close()
		}
	}
	if s.stdoutWriter != nil {
		if closer, ok := s.stdoutWriter.(io.Closer); ok {
			closer.Close()
		}
	}

	// Delete the pod
	if err := s.k8sClient.DeletePod(s.podName); err != nil {
		s.logger.WithError(err).WithField("pod_name", s.podName).Error("Failed to delete pod during cleanup")
		return fmt.Errorf("failed to delete pod: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"pod_name":   s.podName,
		"session_id": s.sessionID,
	}).Info("Kubernetes shell session closed and pod deleted")

	return nil
}

// GetPodName returns the pod name (useful for logging/debugging)
func (s *K8sSession) GetPodName() string {
	return s.podName
}

// Mode returns the session mode
func (s *K8sSession) Mode() string {
	return "k8s"
}

