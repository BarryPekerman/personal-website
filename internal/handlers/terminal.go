// Package handlers bridges browser WebSocket connections to shell sessions.
package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"portfolio/internal/shell"
)

// maxMessageSize caps a single WebSocket message (one keystroke or a paste)
const maxMessageSize = 4096

// pingInterval keeps idle sessions alive: Cloudflare closes a WebSocket after
// 100 seconds with no traffic, so a visitor who stops to read would be dropped
const pingInterval = 30 * time.Second

// clientMessage is what the browser sends: {"type": "input"|"resize", "data": "..."}
type clientMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

// activeSessions counts the terminal sessions open right now
var activeSessions atomic.Int64

// ActiveSessions reports how many visitors have a terminal open
func ActiveSessions() int64 { return activeSessions.Load() }

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
		// The zero-value CheckOrigin only accepts same-origin requests
		upgrader: websocket.Upgrader{},
	}
}

// HandleWebSocket runs one terminal session for the lifetime of a WebSocket connection
func (h *TerminalHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	ws, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.WithError(err).Error("Failed to upgrade connection to WebSocket")
		return
	}
	defer ws.Close() //nolint:errcheck // nothing useful to do if closing fails
	ws.SetReadLimit(maxMessageSize)

	sessionID := fmt.Sprintf("session-%d", time.Now().UnixNano())
	log := h.logger.WithField("session_id", sessionID)
	log.Info("New WebSocket connection established")

	session, err := h.shellManager.CreateSession(sessionID)
	if err != nil {
		log.WithError(err).Error("Failed to create shell session")
		h.sendError(ws, "Failed to create terminal session. Please try again.")
		return
	}
	defer func() {
		if err := session.Close(); err != nil {
			log.WithError(err).Error("Error closing shell session")
		}
	}()

	if err := session.Start(); err != nil {
		log.WithError(err).Error("Failed to start shell session")
		h.sendError(ws, "Failed to start shell session.")
		return
	}
	log.Info("Shell session started")
	activeSessions.Add(1)
	defer activeSessions.Add(-1)

	done := make(chan struct{})
	defer close(done)
	go h.keepAlive(ws, done)

	go h.pumpInput(ws, session, log)
	h.pumpOutput(ws, session, log)
}

// keepAlive pings the browser until done is closed. WriteControl is safe to
// call alongside pumpOutput's writes.
func (h *TerminalHandler) keepAlive(ws *websocket.Conn, done <-chan struct{}) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if err := ws.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
				return
			}
		}
	}
}

// pumpInput forwards browser messages to the shell until the connection closes
func (h *TerminalHandler) pumpInput(ws *websocket.Conn, session shell.Session, log *logrus.Entry) {
	// Closing stdin tells the shell the visitor has gone
	defer func() {
		if closer, ok := session.Writer().(io.Closer); ok {
			_ = closer.Close()
		}
	}()

	for {
		messageType, data, err := ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.WithError(err).Debug("WebSocket closed")
			}
			return
		}
		if messageType != websocket.TextMessage {
			continue
		}

		var msg clientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		switch msg.Type {
		case "input":
			if _, err := session.Writer().Write([]byte(msg.Data)); err != nil {
				log.WithError(err).Debug("Failed to write to shell")
				return
			}
		case "resize":
			var cols, rows uint16
			if _, err := fmt.Sscanf(msg.Data, "%d,%d", &cols, &rows); err == nil {
				if err := session.Resize(cols, rows); err != nil {
					log.WithError(err).Debug("Failed to resize terminal")
				}
			}
		}
	}
}

// pumpOutput streams shell output to the browser until the shell exits.
// Output is sent as binary frames so ANSI sequences arrive untouched.
func (h *TerminalHandler) pumpOutput(ws *websocket.Conn, session shell.Session, log *logrus.Entry) {
	buf := make([]byte, 4096)
	for {
		n, err := session.Reader().Read(buf)
		if n > 0 {
			if err := ws.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				return
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				log.WithError(err).Debug("Shell read error")
			}
			return
		}
	}
}

func (h *TerminalHandler) sendError(ws *websocket.Conn, message string) {
	errMsg := fmt.Sprintf("\r\n\x1b[1;31mError: %s\x1b[0m\r\n", message)
	if err := ws.WriteMessage(websocket.BinaryMessage, []byte(errMsg)); err != nil {
		h.logger.WithError(err).Debug("Failed to send error to client")
	}
}
