// Package handlers bridges browser WebSocket connections to shell sessions.
package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"portfolio/internal/shell"
)

// maxMessageSize caps a single WebSocket message (one keystroke or a paste)
const maxMessageSize = 4096

// Terminal session metrics, served on /metrics
var (
	sessionsStarted = promauto.NewCounter(prometheus.CounterOpts{
		Name: "portfolio_terminal_sessions_started_total",
		Help: "Terminal sessions started.",
	})
	sessionsRefused = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "portfolio_terminal_sessions_refused_total",
		Help: "Terminal sessions refused, by limit: full (all sessions) or address (per visitor).",
	}, []string{"limit"})
	sessionsEnded = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "portfolio_terminal_sessions_ended_total",
		Help: "Terminal sessions ended, by reason: idle (no input), timeout (the browser stopped answering) or closed (the visitor left or typed exit).",
	}, []string{"reason"})
	sessionErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "portfolio_terminal_errors_total",
		Help: "Failures setting up a terminal session, by stage: upgrade, create or start.",
	}, []string{"stage"})
)

func init() {
	// Every label value shows from the start, at zero
	for _, l := range []string{"full", "address"} {
		sessionsRefused.WithLabelValues(l)
	}
	for _, r := range []string{"idle", "timeout", "closed"} {
		sessionsEnded.WithLabelValues(r)
	}
	for _, st := range []string{"upgrade", "create", "start"} {
		sessionErrors.WithLabelValues(st)
	}
}

// writeWait is how long a write may block on a peer that stopped reading
const writeWait = 10 * time.Second

// Limits keep one visitor, or a script, from holding every session: each
// costs memory on a small pod, and they last until the visitor leaves
type Limits struct {
	MaxSessions int           // across all visitors
	MaxPerIP    int           // per address (per /64 for IPv6)
	Idle        time.Duration // no input for this long ends the session
	// Ping keeps quiet sessions alive (Cloudflare closes a WebSocket after
	// 100 seconds with no traffic) and finds dead peers: a browser silent for
	// three ping intervals (two unanswered pings) is gone
	Ping time.Duration
}

// DefaultLimits are sized for one pod with a 128Mi memory limit; a session
// costs a few tens of KiB
var DefaultLimits = Limits{
	MaxSessions: 200,
	MaxPerIP:    8,
	Idle:        15 * time.Minute,
	Ping:        30 * time.Second,
}

// clientMessage is what the browser sends: {"type": "input"|"resize", "data": "..."}
type clientMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

// edgeMessage tells the browser where its WebSocket entered Cloudflare
type edgeMessage struct {
	Type    string `json:"type"`
	Country string `json:"country"`
	Colo    string `json:"colo"`
}

// TerminalHandler handles WebSocket connections for terminal sessions
type TerminalHandler struct {
	shellManager shell.Manager
	logger       *slog.Logger
	upgrader     websocket.Upgrader
	limits       Limits

	// Edge, when set, names the visitor's country and the Cloudflare edge
	// that took the WebSocket: sent to the browser as the session's first
	// frame, {"type":"edge",...}, for the route map. It is the only text
	// frame; terminal output is binary.
	Edge func(r *http.Request) (country, colo string)

	mu     sync.Mutex
	total  int
	perIP  map[netip.Prefix]int
	nextID uint64
}

// NewTerminalHandler creates a new terminal handler with the given shell
// manager and limits
func NewTerminalHandler(shellManager shell.Manager, logger *slog.Logger, limits Limits) *TerminalHandler {
	return &TerminalHandler{
		shellManager: shellManager,
		logger:       logger,
		// The zero-value CheckOrigin only accepts same-origin requests
		upgrader: websocket.Upgrader{},
		limits:   limits,
		perIP:    map[netip.Prefix]int{},
	}
}

// ActiveSessions reports how many visitors have a terminal open
func (h *TerminalHandler) ActiveSessions() int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return int64(h.total)
}

// acquire reserves a session slot for addr. It returns the session's ID, or
// the reason the visitor can't have one.
func (h *TerminalHandler) acquire(addr netip.Prefix) (string, string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.total >= h.limits.MaxSessions {
		sessionsRefused.WithLabelValues("full").Inc()
		return "", "The server is full. Try again in a few minutes."
	}
	if h.perIP[addr] >= h.limits.MaxPerIP {
		sessionsRefused.WithLabelValues("address").Inc()
		return "", "Too many terminals open from your address. Close one and reload."
	}
	h.total++
	h.perIP[addr]++
	h.nextID++
	return fmt.Sprintf("session-%d", h.nextID), ""
}

func (h *TerminalHandler) release(addr netip.Prefix) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.total--
	if h.perIP[addr]--; h.perIP[addr] <= 0 {
		delete(h.perIP, addr)
	}
}

// clientAddr is the visitor's address, as a prefix: an IPv6 visitor
// controls a whole /64, so that counts as one. Behind the tunnel the TCP
// peer is cloudflared, and Cloudflare names the visitor in CF-Connecting-IP.
func clientAddr(r *http.Request) netip.Prefix {
	addr, err := netip.ParseAddr(r.Header.Get("CF-Connecting-IP"))
	if err != nil {
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		addr, _ = netip.ParseAddr(host)
	}
	addr = addr.Unmap()
	bits := addr.BitLen() // 0 for an unparseable address: all of those share one prefix
	if addr.Is6() {
		bits = 64
	}
	p, _ := addr.Prefix(bits)
	return p
}

// HandleWebSocket runs one terminal session for the lifetime of a WebSocket connection
func (h *TerminalHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	ws, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade connection to WebSocket", "err", err)
		sessionErrors.WithLabelValues("upgrade").Inc()
		return
	}
	defer ws.Close() //nolint:errcheck // nothing useful to do if closing fails
	ws.SetReadLimit(maxMessageSize)

	addr := clientAddr(r)
	sessionID, refused := h.acquire(addr)
	if refused != "" {
		h.logger.Warn("Terminal session refused", "reason", refused, "addr", addr.String())
		h.closeWith(ws, websocket.CloseTryAgainLater, refused)
		return
	}
	defer h.release(addr)
	log := h.logger.With("session_id", sessionID)
	log.Info("New WebSocket connection established")

	session, err := h.shellManager.CreateSession(sessionID)
	if err != nil {
		log.Error("Failed to create shell session", "err", err)
		sessionErrors.WithLabelValues("create").Inc()
		h.sendError(ws, "Failed to create terminal session. Please try again.")
		return
	}
	defer func() {
		if err := session.Close(); err != nil {
			log.Error("Error closing shell session", "err", err)
		}
	}()

	if err := session.Start(); err != nil {
		log.Error("Failed to start shell session", "err", err)
		sessionErrors.WithLabelValues("start").Inc()
		h.sendError(ws, "Failed to start shell session.")
		return
	}
	log.Info("Shell session started")
	sessionsStarted.Inc()
	// keepAlive or pumpInput may name why the session ended; otherwise the
	// visitor closed it
	var ended atomic.Value
	ended.Store("closed")
	defer func() { sessionsEnded.WithLabelValues(ended.Load().(string)).Inc() }()

	// Nothing else writes yet: the output pump starts below
	if h.Edge != nil {
		country, colo := h.Edge(r)
		_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
		if err := ws.WriteJSON(edgeMessage{Type: "edge", Country: country, Colo: colo}); err != nil {
			return
		}
	}

	// A peer that answers neither pings nor anything else is gone: the read
	// deadline ends pumpInput, which ends the shell
	deadline := func() error { return ws.SetReadDeadline(time.Now().Add(3 * h.limits.Ping)) }
	_ = deadline()
	ws.SetPongHandler(func(string) error { return deadline() })

	input := make(chan struct{}, 1)
	done := make(chan struct{})
	defer close(done)
	go h.keepAlive(ws, input, done, &ended, log)

	go h.pumpInput(ws, session, input, deadline, &ended, log)
	h.pumpOutput(ws, session, log)
}

// keepAlive pings the browser until done is closed, and ends the session
// after limits.Idle without input. Closing the connection unblocks both
// pumps. WriteControl and Close are safe to call alongside pumpOutput's writes.
func (h *TerminalHandler) keepAlive(ws *websocket.Conn, input <-chan struct{}, done <-chan struct{}, ended *atomic.Value, log *slog.Logger) {
	ticker := time.NewTicker(h.limits.Ping)
	defer ticker.Stop()
	idle := time.NewTimer(h.limits.Idle)
	defer idle.Stop()
	for {
		select {
		case <-done:
			return
		case <-input:
			idle.Reset(h.limits.Idle)
		case <-idle.C:
			log.Info("Closing idle session")
			ended.Store("idle")
			h.closeWith(ws, websocket.CloseNormalClosure,
				fmt.Sprintf("Session closed after %v without input.", h.limits.Idle))
			return
		case <-ticker.C:
			if err := ws.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
				_ = ws.Close()
				return
			}
		}
	}
}

// closeWith tells the browser why the session ends (the terminal prints the
// reason), then closes the connection
func (h *TerminalHandler) closeWith(ws *websocket.Conn, code int, reason string) {
	msg := websocket.FormatCloseMessage(code, reason)
	if err := ws.WriteControl(websocket.CloseMessage, msg, time.Now().Add(writeWait)); err != nil {
		h.logger.Debug("Failed to send close message", "err", err)
	}
	_ = ws.Close()
}

// pumpInput forwards browser messages to the shell until the connection
// closes. Each message pushes back the read deadline and the idle timer.
func (h *TerminalHandler) pumpInput(ws *websocket.Conn, session shell.Session, input chan<- struct{}, deadline func() error, ended *atomic.Value, log *slog.Logger) {
	// Closing stdin tells the shell the visitor has gone
	defer func() {
		if closer, ok := session.Writer().(io.Closer); ok {
			_ = closer.Close()
		}
	}()

	for {
		messageType, data, err := ws.ReadMessage()
		if err != nil {
			// The read deadline passed: the browser stopped answering pings
			if ne, ok := errors.AsType[net.Error](err); ok && ne.Timeout() {
				ended.Store("timeout")
			}
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Debug("WebSocket closed", "err", err)
			}
			return
		}
		_ = deadline()
		if messageType != websocket.TextMessage {
			continue
		}

		var msg clientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		switch msg.Type {
		case "input":
			select {
			case input <- struct{}{}:
			default: // keepAlive has a reset pending already
			}
			if _, err := session.Writer().Write([]byte(msg.Data)); err != nil {
				log.Debug("Failed to write to shell", "err", err)
				return
			}
		case "resize":
			var cols, rows uint16
			if _, err := fmt.Sscanf(msg.Data, "%d,%d", &cols, &rows); err == nil {
				if err := session.Resize(cols, rows); err != nil {
					log.Debug("Failed to resize terminal", "err", err)
				}
			}
		}
	}
}

// pumpOutput streams shell output to the browser until the shell exits.
// Output is sent as binary frames so ANSI sequences arrive untouched.
func (h *TerminalHandler) pumpOutput(ws *websocket.Conn, session shell.Session, log *slog.Logger) {
	buf := make([]byte, 4096)
	for {
		n, err := session.Reader().Read(buf)
		if n > 0 {
			if err := ws.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			if err := ws.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				return
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				log.Debug("Shell read error", "err", err)
			}
			return
		}
	}
}

func (h *TerminalHandler) sendError(ws *websocket.Conn, message string) {
	errMsg := fmt.Sprintf("\r\n\x1b[1;31mError: %s\x1b[0m\r\n", message)
	_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
	if err := ws.WriteMessage(websocket.BinaryMessage, []byte(errMsg)); err != nil {
		h.logger.Debug("Failed to send error to client", "err", err)
	}
}
