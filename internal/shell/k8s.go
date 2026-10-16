package shell

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"portfolio/internal/k8s"
)

const (
	podStartTimeout  = 60 * time.Second
	podDeleteTimeout = 10 * time.Second
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

	ctx, cancel := context.WithTimeout(context.Background(), podStartTimeout)
	defer cancel()
	pod, err := m.k8sClient.CreateTerminalPod(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to create terminal pod: %w", err)
	}

	return &K8sSession{
		pipes:     newPipes(),
		k8sClient: m.k8sClient,
		podName:   pod.Name,
		sessionID: sessionID,
		logger:    m.logger,
		done:      make(chan error, 1),
	}, nil
}

// K8sSession represents a shell session running in a Kubernetes pod
type K8sSession struct {
	*pipes

	k8sClient *k8s.Client
	podName   string
	sessionID string
	logger    *logrus.Logger

	cancel context.CancelFunc
	done   chan error
}

// Start establishes the exec connection to the pod and starts the shell
func (s *K8sSession) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	go func() {
		err := s.k8sClient.ExecInPod(ctx, s.podName, k8s.PodExecOptions{
			Command: []string{"/bin/sh", "-l"}, // Login shell to load .profile
			Stdin:   s.inR,
			Stdout:  s.outW,
			Stderr:  s.outW,
			TTY:     true,
		})
		// The shell has exited: give the handler EOF
		_ = s.outW.Close()
		s.done <- err
	}()

	s.logger.WithFields(logrus.Fields{
		"pod_name":   s.podName,
		"session_id": s.sessionID,
	}).Debug("Kubernetes shell session started")
	return nil
}

// Resize handles terminal window size changes
func (s *K8sSession) Resize(cols, rows uint16) error {
	// TODO: pass a remotecommand.TerminalSizeQueue to ExecInPod
	s.logger.WithFields(logrus.Fields{
		"cols":       cols,
		"rows":       rows,
		"session_id": s.sessionID,
	}).Debug("Terminal resize requested (not yet implemented for K8s mode)")
	return nil
}

// Wait blocks until the shell process exits
func (s *K8sSession) Wait() error {
	return <-s.done
}

// Close stops the exec stream and deletes the pod
func (s *K8sSession) Close() error {
	if s.cancel != nil {
		s.cancel()
	}
	s.closePipes()

	ctx, cancel := context.WithTimeout(context.Background(), podDeleteTimeout)
	defer cancel()
	if err := s.k8sClient.DeletePod(ctx, s.podName); err != nil {
		return fmt.Errorf("failed to delete pod: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"pod_name":   s.podName,
		"session_id": s.sessionID,
	}).Info("Kubernetes shell session closed and pod deleted")
	return nil
}

// Mode returns the session mode
func (s *K8sSession) Mode() string {
	return "k8s"
}
