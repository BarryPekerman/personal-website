package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"portfolio/internal/shell"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	fsys := fstest.MapFS{"cv.txt": {Data: []byte("CV-CONTENT\n")}}
	handler := NewTerminalHandler(shell.NewVirtualManager(fsys, logger), logger)
	srv := httptest.NewServer(http.HandlerFunc(handler.HandleWebSocket))
	t.Cleanup(srv.Close)
	return srv
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
