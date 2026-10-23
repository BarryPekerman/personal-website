package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/gorilla/websocket"
	"portfolio/internal/shell"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv, _ := newLimitedServer(t, DefaultLimits)
	return srv
}

func newLimitedServer(t *testing.T, limits Limits) (*httptest.Server, *TerminalHandler) {
	t.Helper()
	logger := slog.New(slog.DiscardHandler)

	fsys := fstest.MapFS{"cv.txt": {Data: []byte("CV-CONTENT\n")}}
	handler := NewTerminalHandler(shell.NewVirtualManager(fsys, logger), logger, limits)
	srv := httptest.NewServer(http.HandlerFunc(handler.HandleWebSocket))
	t.Cleanup(srv.Close)
	return srv, handler
}

// dial opens a session as the visitor at ip (sent as CF-Connecting-IP)
func dial(t *testing.T, srv *httptest.Server, ip string) *websocket.Conn {
	t.Helper()
	header := http.Header{}
	if ip != "" {
		header.Set("CF-Connecting-IP", ip)
	}
	ws, resp, err := websocket.DefaultDialer.Dial(wsURL(srv), header)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	t.Cleanup(func() { _ = ws.Close() })
	return ws
}

// closeReason reads until the server closes the connection and returns the
// close frame's code and text
func closeReason(t *testing.T, ws *websocket.Conn) (int, string) {
	t.Helper()
	if err := ws.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	for {
		_, _, err := ws.ReadMessage()
		if err == nil {
			continue
		}
		var ce *websocket.CloseError
		if !errors.As(err, &ce) {
			t.Fatalf("connection ended without a close frame: %v", err)
		}
		return ce.Code, ce.Text
	}
}

// waitSessions waits until the handler counts want sessions
func waitSessions(t *testing.T, h *TerminalHandler, want int64) {
	t.Helper()
	for end := time.Now().Add(3 * time.Second); time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
		if h.ActiveSessions() == want {
			return
		}
	}
	t.Fatalf("sessions = %d, want %d", h.ActiveSessions(), want)
}

func wsURL(srv *httptest.Server) string {
	return "ws" + strings.TrimPrefix(srv.URL, "http")
}

// readUntil reads output frames until they contain want
func readUntil(t *testing.T, ws *websocket.Conn, want string) {
	t.Helper()
	var out strings.Builder
	if err := ws.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	for !strings.Contains(out.String(), want) {
		_, data, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("waiting for %q: %v; output: %q", want, err, out.String())
		}
		out.Write(data)
	}
}

func TestSessionOverWebSocket(t *testing.T) {
	srv := newTestServer(t)
	ws, resp, err := websocket.DefaultDialer.Dial(wsURL(srv), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	defer func() { _ = ws.Close() }()

	readUntil(t, ws, "$ ")
	if err := ws.WriteJSON(clientMessage{Type: "resize", Data: "120,40"}); err != nil {
		t.Fatal(err)
	}
	// One message per keystroke, the way xterm.js sends them
	for _, key := range "cat cv.txt\r" {
		if err := ws.WriteJSON(clientMessage{Type: "input", Data: string(key)}); err != nil {
			t.Fatal(err)
		}
	}
	readUntil(t, ws, "CV-CONTENT")

	if err := ws.WriteJSON(clientMessage{Type: "input", Data: "exit\r"}); err != nil {
		t.Fatal(err)
	}
	readUntil(t, ws, "logout")
	if err := ws.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	for {
		if _, _, err := ws.ReadMessage(); err != nil {
			break // server closed the connection after exit
		}
	}
}

func TestCrossOriginRejected(t *testing.T) {
	srv := newTestServer(t)
	header := http.Header{"Origin": {"https://evil.example"}}
	ws, resp, err := websocket.DefaultDialer.Dial(wsURL(srv), header)
	if err == nil {
		_ = ws.Close()
		t.Fatal("cross-origin WebSocket handshake should fail")
	}
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
		}
	}
}

func TestPerIPLimit(t *testing.T) {
	limits := DefaultLimits
	limits.MaxPerIP = 2
	srv, h := newLimitedServer(t, limits)

	// One IPv6 visitor holds a whole /64: two addresses in it count together
	a := dial(t, srv, "2001:db8:1:2::1")
	readUntil(t, a, "$ ")
	b := dial(t, srv, "2001:db8:1:2::ffff")
	readUntil(t, b, "$ ")
	c := dial(t, srv, "2001:db8:1:2:aaaa::1")
	if code, text := closeReason(t, c); code != websocket.CloseTryAgainLater || !strings.Contains(text, "your address") {
		t.Errorf("third session from the /64: close %d %q, want %d about the address", code, text, websocket.CloseTryAgainLater)
	}
	// Another visitor isn't affected
	d := dial(t, srv, "203.0.113.7")
	readUntil(t, d, "$ ")

	// Closing a session frees its slot
	_ = a.Close()
	waitSessions(t, h, 2)
	e := dial(t, srv, "2001:db8:1:2::2")
	readUntil(t, e, "$ ")
}

func TestGlobalLimit(t *testing.T) {
	limits := DefaultLimits
	limits.MaxSessions = 2
	srv, _ := newLimitedServer(t, limits)

	readUntil(t, dial(t, srv, "203.0.113.1"), "$ ")
	readUntil(t, dial(t, srv, "203.0.113.2"), "$ ")
	if code, text := closeReason(t, dial(t, srv, "203.0.113.3")); code != websocket.CloseTryAgainLater || !strings.Contains(text, "full") {
		t.Errorf("session over the cap: close %d %q, want %d, server full", code, text, websocket.CloseTryAgainLater)
	}
}

func TestIdleSessionClosed(t *testing.T) {
	limits := DefaultLimits
	limits.Idle = 300 * time.Millisecond
	srv, h := newLimitedServer(t, limits)

	ws := dial(t, srv, "")
	readUntil(t, ws, "$ ")
	// Typing keeps it open past the idle time
	for i := 0; i < 4; i++ {
		time.Sleep(150 * time.Millisecond)
		if err := ws.WriteJSON(clientMessage{Type: "input", Data: "x"}); err != nil {
			t.Fatal(err)
		}
	}
	readUntil(t, ws, "xxxx")
	if code, text := closeReason(t, ws); code != websocket.CloseNormalClosure || !strings.Contains(text, "without input") {
		t.Errorf("idle close: %d %q", code, text)
	}
	waitSessions(t, h, 0)
}

func TestDeadPeerDropped(t *testing.T) {
	limits := DefaultLimits
	limits.Ping = 100 * time.Millisecond
	srv, h := newLimitedServer(t, limits)

	// A client that never reads never answers pings: gone within three intervals
	dial(t, srv, "")
	waitSessions(t, h, 1)
	waitSessions(t, h, 0)

	// One that reads (and so answers pings) stays
	ws := dial(t, srv, "")
	go func() {
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				return
			}
		}
	}()
	time.Sleep(600 * time.Millisecond)
	if n := h.ActiveSessions(); n != 1 {
		t.Errorf("live client dropped: sessions = %d", n)
	}
}

func TestClientAddr(t *testing.T) {
	for _, tt := range []struct{ header, remote, want string }{
		{"203.0.113.9", "10.0.0.5:1234", "203.0.113.9/32"},
		{"2001:db8:1:2:3:4:5:6", "10.0.0.5:1234", "2001:db8:1:2::/64"},
		{"::ffff:203.0.113.9", "10.0.0.5:1234", "203.0.113.9/32"},
		{"", "10.0.0.5:1234", "10.0.0.5/32"},
		{"not an ip", "[2001:db8::1]:1234", "2001:db8::/64"},
	} {
		r := httptest.NewRequest(http.MethodGet, "/ws", nil)
		r.RemoteAddr = tt.remote
		if tt.header != "" {
			r.Header.Set("CF-Connecting-IP", tt.header)
		}
		if got := clientAddr(r); got != netip.MustParsePrefix(tt.want) {
			t.Errorf("CF-Connecting-IP %q, peer %q: %v, want %s", tt.header, tt.remote, got, tt.want)
		}
	}
}

func TestEdgeIsFirstFrame(t *testing.T) {
	srv, h := newLimitedServer(t, DefaultLimits)
	h.Edge = func(r *http.Request) (string, string) { return "IL", r.Header.Get("X-Test-Colo") }
	header := http.Header{"X-Test-Colo": {"TLV"}}
	ws, resp, err := websocket.DefaultDialer.Dial(wsURL(srv), header)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	defer func() { _ = ws.Close() }()

	kind, data, err := ws.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if kind != websocket.TextMessage || string(data) != `{"type":"edge","country":"IL","colo":"TLV"}`+"\n" {
		t.Errorf("first frame: type %d, %q", kind, data)
	}
	readUntil(t, ws, "$ ")
}
