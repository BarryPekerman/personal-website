package main

import (
	"bytes"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"portfolio/web"
)

// pages are the HTML pages, rendered once at startup. The templates put the
// asset version on every static URL (?v=...), so a deploy never serves a page
// with an old script that a cache still holds.
type pages map[string][]byte

// newPages renders every template with the asset version: the commit, or in
// a dev build (no commit) the start time, so each restart busts the cache
func newPages(commit string) (pages, error) {
	version := commit
	if version == "dev" {
		version = strconv.FormatInt(time.Now().Unix(), 36)
	}
	tmpl, err := template.ParseFS(web.FS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	p := pages{}
	for _, t := range tmpl.Templates() {
		var b bytes.Buffer
		if err := t.Execute(&b, struct{ V string }{version}); err != nil {
			return nil, err
		}
		p[t.Name()] = b.Bytes()
	}
	return p, nil
}

func (p pages) handler(name string) http.Handler {
	body, ok := p[name]
	if !ok {
		panic("no template " + name)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// Always revalidate: the page names the current assets
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(body)
	})
}

// cacheFor lets browsers and Cloudflare keep a successful response for d;
// errors (a 404 for a file a deploy adds) aren't cached
func cacheFor(d time.Duration, h http.Handler) http.Handler {
	value := "public, max-age=" + strconv.Itoa(int(d.Seconds()))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(&cacheOnSuccess{ResponseWriter: w, value: value}, r)
	})
}

type cacheOnSuccess struct {
	http.ResponseWriter
	value       string
	wroteHeader bool
}

func (c *cacheOnSuccess) WriteHeader(code int) {
	if !c.wroteHeader {
		c.wroteHeader = true
		if code == http.StatusOK || code == http.StatusNotModified {
			c.Header().Set("Cache-Control", c.value)
		}
	}
	c.ResponseWriter.WriteHeader(code)
}

func (c *cacheOnSuccess) Write(b []byte) (int, error) {
	if !c.wroteHeader {
		c.WriteHeader(http.StatusOK)
	}
	return c.ResponseWriter.Write(b)
}

// contentSecurityPolicy allows only this origin's own scripts, styles,
// requests (the WebSocket included) and images, and no framing. Styles need
// 'unsafe-inline' because xterm.js themes the terminal by adding <style>
// elements; scripts don't, as the pages have no inline script.
const contentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"

// securityHeaders sets the headers every response carries
func securityHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hdr := w.Header()
		hdr.Set("Content-Security-Policy", contentSecurityPolicy)
		// Browsers only honour it over HTTPS, which Cloudflare always serves
		hdr.Set("Strict-Transport-Security", "max-age=31536000")
		hdr.Set("X-Content-Type-Options", "nosniff")
		hdr.Set("X-Frame-Options", "DENY")
		hdr.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		hdr.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		h.ServeHTTP(w, r)
	})
}
