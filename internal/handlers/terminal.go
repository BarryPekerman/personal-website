package handlers

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"portfolio/internal/shell"
	wsMsg "portfolio/internal/websocket"
)

// TerminalHandler handles WebSocket connections for terminal sessions
type TerminalHandler struct {
	shellManager shell.Manager
	logger       *logrus.Logger
	upgrader     websocket.Upgrader
}

// NewTerminalHandler creates a new terminal handler with the given shell manager
func NewTerminalHandler(shellManager shell.Manager, logger *logrus.Logger) *TerminalHandler {
	return &TerminalHandler{
		shellManager: shellManager,
		logger:       logger,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for development
			},
		},
	}
}

// HandleWebSocket handles incoming WebSocket connections
func (h *TerminalHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	ws, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.WithError(err).Error("Failed to upgrade connection to WebSocket")
		return
	}
	defer ws.Close()

	sessionID := fmt.Sprintf("session-%d", time.Now().UnixNano())
	h.logger.WithField("session_id", sessionID).Info("New WebSocket connection established")

	// Create shell session
	session, err := h.shellManager.CreateSession(sessionID)
	if err != nil {
		h.logger.WithError(err).WithField("session_id", sessionID).Error("Failed to create shell session")
		h.sendError(ws, "Failed to create terminal session. Please try again.")
		return
	}

	// Ensure cleanup on exit
	defer func() {
		if err := session.Close(); err != nil {
			h.logger.WithError(err).WithField("session_id", sessionID).Error("Error closing shell session")
		}
	}()

	// Start the shell session
	if err := session.Start(); err != nil {
		h.logger.WithError(err).WithField("session_id", sessionID).Error("Failed to start shell session")
		h.sendError(ws, "Failed to start shell session.")
		return
	}

	h.logger.WithFields(logrus.Fields{
		"session_id": sessionID,
		"mode":       session.Mode(),
	}).Info("Shell session started successfully")

	// Start interactive shell session (let shell handle its own prompt)
	h.startShellSession(ws, session, sessionID)
}

// startShellSession handles the bidirectional streaming between WebSocket and shell
func (h *TerminalHandler) startShellSession(ws *websocket.Conn, session shell.Session, sessionID string) {
	// Handle WebSocket messages (user input) → Shell stdin
	go func() {
		defer func() {
			if closer, ok := session.Writer().(io.Closer); ok {
				closer.Close()
			}
		}()

		for {
			messageType, message, err := ws.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					h.logger.WithError(err).WithField("session_id", sessionID).Debug("WebSocket closed")
				}
				return
			}

			// Handle text messages (JSON)
			if messageType == websocket.TextMessage {
				var msg wsMsg.Message
				if err := wsMsg.UnmarshalMessage(message, &msg); err != nil {
					continue
				}

				switch msg.Type {
				case "input":
					// Check for dangerous commands
					if h.isDangerousCommand(msg.Data) {
						h.handleSelfDestruct(ws, sessionID, session)
						return
					}

					// Send input to shell immediately
					if _, err := session.Writer().Write([]byte(msg.Data)); err != nil {
						h.logger.WithError(err).WithField("session_id", sessionID).Debug("Failed to write to shell")
						return
					}

				case "resize":
					var cols, rows uint16
					if _, err := fmt.Sscanf(msg.Data, "%d,%d", &cols, &rows); err == nil {
						session.Resize(cols, rows)
					}
				}
			}
		}
	}()

	// Stream shell output → WebSocket (binary to preserve ANSI)
	// Use smaller buffer for better responsiveness
	// Filter initial unwanted output (sudo messages, ANSI sequences)
	reader := newFilteredReader(session.Reader(), h.logger)
	buf := make([]byte, 1024)
	writeCount := 0
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			// Debug: log first few writes to see what's actually being sent
			if writeCount < 5 {
				h.logger.WithField("writeCount", writeCount).Debugf("Sending %d bytes to WebSocket", n)
				h.logger.Debugf("WebSocket send hex (first 200 bytes): %x", buf[:min(n, 200)])
				h.logger.Debugf("WebSocket send content (first 200 bytes): %q", string(buf[:min(n, 200)]))
			}
			if err := ws.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				return
			}
			writeCount++
		}
		if err != nil {
			if err != io.EOF {
				h.logger.WithError(err).WithField("session_id", sessionID).Debug("Shell read error")
			}
			return
		}
	}
}

func (h *TerminalHandler) isDangerousCommand(input string) bool {
	dangerous := []string{
		"sudo rm",
		"rm -rf /",
		":(){ :|:& };:",
		"mkfs",
		"dd if=/dev/zero",
		"shutdown",
		"reboot",
		"halt",
		"init 0",
		"kill -9 -1",
	}

	input = strings.ToLower(strings.TrimSpace(input))
	for _, cmd := range dangerous {
		if strings.Contains(input, cmd) {
			return true
		}
	}
	return false
}

func (h *TerminalHandler) handleSelfDestruct(ws *websocket.Conn, sessionID string, session shell.Session) {
	messages := []string{
		"\r\n\x1b[1;31m🚨 SECURITY VIOLATION DETECTED! 🚨\x1b[0m\r\n",
		"\x1b[1;33m⚠️  Dangerous command intercepted\x1b[0m\r\n",
		"\x1b[1;31m🔥 INITIATING SELF-DESTRUCT SEQUENCE...\x1b[0m\r\n",
		"\r\n\x1b[5;31m💥 3... 💥\x1b[0m\r\n",
		"\x1b[5;31m💥 2... 💥\x1b[0m\r\n",
		"\x1b[5;31m💥 1... 💥\x1b[0m\r\n",
		"\r\n\x1b[1;37;41m 💀 SESSION TERMINATED 💀 \x1b[0m\r\n",
	}

	for _, msg := range messages {
		ws.WriteMessage(websocket.BinaryMessage, []byte(msg))
		time.Sleep(500 * time.Millisecond)
	}

	session.Close()
}

func (h *TerminalHandler) sendError(ws *websocket.Conn, message string) {
	errMsg := fmt.Sprintf("\r\n\x1b[1;31mError: %s\x1b[0m\r\n", message)
	ws.WriteMessage(websocket.BinaryMessage, []byte(errMsg))
}
