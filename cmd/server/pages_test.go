package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"portfolio/internal/status"
)

func TestPagesUseAssetVersion(t *testing.T) {
	p, err := newPages("0123abc", noVisitor)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"home.html", "index.html"} {
		body := render(t, p, name)
		if !strings.Contains(body, "?v=0123abc") {
			t.Errorf("%s: no asset version in the static URLs", name)
		}
		if strings.Contains(body, "{{") {
			t.Errorf("%s: unrendered template action", name)
		}
		// The CSP allows no inline script and no other origin's scripts
		if m := regexp.MustCompile(`<script(?:\s[^>]*)?>`).FindAllString(body, -1); len(m) == 0 {
			t.Errorf("%s: no scripts", name)
		} else {
			for _, tag := range m {
				if !strings.Contains(tag, `src="/static/`) {
					t.Errorf("%s: script not from /static/: %s", name, tag)
				}
			}
		}
		if regexp.MustCompile(`\son[a-z]+=`).MatchString(body) || strings.Contains(body, "https://cdn.") {
			t.Errorf("%s: inline handler or CDN reference", name)
		}
	}

	dev, err := newPages("dev", noVisitor)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(render(t, dev, "home.html"), "?v=dev") {
		t.Error("a dev build should version assets by start time, not \"dev\"")
	}
}

func noVisitor(*http.Request) status.Visitor { return status.Visitor{} }

func render(t *testing.T, p *pages, name string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	p.handler(name).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	return rec.Body.String()
}

func TestHomeCarriesItsEdge(t *testing.T) {
	p, err := newPages("abc", func(r *http.Request) status.Visitor {
		return status.Visitor{Country: "IL", Colo: r.Header.Get("X-Colo")}
	})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Colo", `"><script>`)
	p.handler("home.html").ServeHTTP(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, `data-country="IL"`) || strings.Contains(body, `"><script>`) {
		t.Errorf("edge not carried, or not escaped: %s", body[:min(len(body), 400)])
	}
}

func TestSecurityHeaders(t *testing.T) {
	p, err := newPages("abc", noVisitor)
	if err != nil {
		t.Fatal(err)
	}
	h := securityHeaders(p.handler("home.html"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	for header, want := range map[string]string{
		"Content-Security-Policy":   "script-src 'self' https://static.cloudflareinsights.com;",
		"Strict-Transport-Security": "max-age=",
		"X-Content-Type-Options":    "nosniff",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
		"Content-Type":              "text/html",
		"Cache-Control":             "no-cache",
	} {
		if got := rec.Header().Get(header); !strings.Contains(got, want) {
			t.Errorf("%s = %q, want it to contain %q", header, got, want)
		}
	}
	if !strings.Contains(rec.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
		t.Error("CSP should forbid framing")
	}
	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), "BARRY PEKERMAN") {
		t.Error("home page not served")
	}
}

func TestCacheForSuccessOnly(t *testing.T) {
	h := cacheFor(time.Hour, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	for path, want := range map[string]string{"/ok": "public, max-age=3600", "/missing": ""} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if got := rec.Header().Get("Cache-Control"); got != want {
			t.Errorf("%s: Cache-Control %q, want %q", path, got, want)
		}
	}
}
