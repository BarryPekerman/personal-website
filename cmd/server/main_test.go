package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sirupsen/logrus"
)

func TestNoListing(t *testing.T) {
	fsys := fstest.MapFS{
		"cv.txt":        {Data: []byte("cv")},
		"projects/a.md": {Data: []byte("a")},
	}
	h := http.StripPrefix("/api/content/", noListing(http.FileServerFS(fsys)))

	for path, want := range map[string]int{
		"/api/content/cv.txt":        http.StatusOK,
		"/api/content/projects/a.md": http.StatusOK,
		"/api/content/":              http.StatusNotFound,
		"/api/content/projects/":     http.StatusNotFound,
		"/api/content/projects":      http.StatusMovedPermanently, // to projects/, a 404
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != want {
			t.Errorf("GET %s: %d, want %d", path, rec.Code, want)
		}
	}
}

func TestRoutes(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	// Nothing to reach: the status reports the game as unavailable
	h, err := newHandler(config{commit: "abc123", devVisitor: "IL/TLV", region: "il-jerusalem-1"}, logger)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	for _, tt := range []struct {
		path, want string
		code       int
	}{
		{"/", "BARRY PEKERMAN", http.StatusOK},
		{"/terminal", `id="terminal"`, http.StatusOK},
		{"/healthz", "ok", http.StatusOK},
		{"/static/js/terminal.js?v=abc123", "PortfolioTerminal", http.StatusOK},
		{"/static/vendor/land-110m.json", `"land"`, http.StatusOK},
		{"/api/content/projects/the-handover.md", "the-handover", http.StatusOK},
		{"/api/status", `"commit":"abc123"`, http.StatusOK},
		{"/api/status", `"label":"TLV"`, http.StatusOK},
		{"/static/", "", http.StatusNotFound},
		{"/api/content/", "", http.StatusNotFound},
		{"/nope", "", http.StatusNotFound},
		// content/ holds only what visitors may read; the Go file isn't embedded
		{"/api/content/content.go", "", http.StatusNotFound},
	} {
		resp, err := http.Get(srv.URL + tt.path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != tt.code || !strings.Contains(string(body), tt.want) {
			t.Errorf("GET %s: %d, body %.80q; want %d containing %q", tt.path, resp.StatusCode, body, tt.code, tt.want)
		}
		if resp.Header.Get("Content-Security-Policy") == "" {
			t.Errorf("GET %s: no security headers", tt.path)
		}
	}

	// POST isn't routed anywhere
	resp, err := http.Post(srv.URL+"/", "text/plain", strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST /: %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}
}

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("POD_NAME", "portfolio-abc")
	t.Setenv("HANDOVER_HEALTHZ_URL", "")
	c := configFromEnv()
	if c.pod != "portfolio-abc" || c.handoverURL != "https://play.barrypekerman.com/healthz" {
		t.Errorf("config: %+v", c)
	}
}
