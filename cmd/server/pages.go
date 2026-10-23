package main

import (
	"bytes"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"portfolio/internal/status"
	"portfolio/web"
)

// pages renders the HTML pages. The templates put the asset version on every
// static URL (?v=...), so a deploy never serves a page with an old script
// that a cache still holds, and the front page carries the Cloudflare edge
// that served it, for the route map.
type pages struct {
	tmpl    *template.Template
	version string
	visitor func(*http.Request) status.Visitor
}

// pageData is what the templates see
type pageData struct {
	V       string // asset version
	Country string // the visitor's country and the edge that served the page
	Colo    string
}

// newPages loads the templates. The asset version is the commit, or in a dev
// build (no commit) the start time, so each restart busts the cache.
func newPages(commit string, visitor func(*http.Request) status.Visitor) (*pages, error) {
	version := commit
	if version == "dev" {
		version = strconv.FormatInt(time.Now().Unix(), 36)
	}
	tmpl, err := template.ParseFS(web.FS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &pages{tmpl: tmpl, version: version, visitor: visitor}, nil
}

func (p *pages) handler(name string) http.Handler {
	if p.tmpl.Lookup(name) == nil {
		panic("no template " + name)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v := p.visitor(r)
		var b bytes.Buffer
		if err := p.tmpl.ExecuteTemplate(&b, name, pageData{V: p.version, Country: v.Country, Colo: v.Colo}); err != nil {
			http.Error(w, "page unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// Always revalidate: the page names the current assets and its edge
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(b.Bytes())
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
