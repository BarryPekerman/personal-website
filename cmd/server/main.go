// Command server serves the portfolio terminal website.
package main

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"portfolio/content"
	terminalHandlers "portfolio/internal/handlers"
	"portfolio/internal/shell"
	"portfolio/internal/status"
	"portfolio/web"
)

// commit is set at build time: -ldflags "-X main.commit=<sha>"
var commit = "dev"

func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	if os.Getenv("LOG_LEVEL") == "debug" {
		logger.SetLevel(logrus.DebugLevel)
	}

	// The shell is simulated: nothing a visitor types runs anywhere
	shellManager := shell.NewVirtualManager(content.FS, logger)
	terminal := terminalHandlers.NewTerminalHandler(shellManager, logger, terminalHandlers.DefaultLimits)

	pages, err := newPages(commit)
	if err != nil {
		logger.WithError(err).Fatal("Failed to load the page templates")
	}
	static, err := fs.Sub(web.FS, "static")
	if err != nil {
		logger.WithError(err).Fatal("Failed to load the static files")
	}

	mux := http.NewServeMux()
	// Static files are requested with ?v=<asset version>, so a deploy changes
	// every URL and they can be cached for long
	mux.Handle("GET /static/", cacheFor(24*time.Hour, http.StripPrefix("/static/", noListing(http.FileServerFS(static)))))
	// Front page: Uplink-style HUD around the terminal; /terminal is the
	// terminal alone, full screen
	mux.Handle("GET /{$}", pages.handler("home.html"))
	mux.Handle("GET /terminal", pages.handler("index.html"))
	// The same files the terminal shows, read-only, for the phone layout
	// (which renders them as text instead of opening a terminal)
	mux.Handle("GET /api/content/", http.StripPrefix("/api/content/", noListing(http.FileServerFS(content.FS))))
	statusInfo := status.Info{
		Pod:     os.Getenv("POD_NAME"),
		Node:    os.Getenv("NODE_NAME"),
		Region:  os.Getenv("REGION"),
		Commit:  commit,
		Started: time.Now().UTC(),
	}
	// DEV_VISITOR=IL/TLV previews the route map locally, where requests carry
	// no Cloudflare headers
	if country, colo, ok := strings.Cut(os.Getenv("DEV_VISITOR"), "/"); ok {
		statusInfo.DevVisitor = status.Visitor{Country: country, Colo: colo}
	}
	handoverURL := os.Getenv("HANDOVER_HEALTHZ_URL")
	if handoverURL == "" {
		handoverURL = "https://play.barrypekerman.com/healthz"
	}
	// TUNNEL_METRICS_URL is cloudflared's /metrics in the cluster (unset
	// locally: the map then shows no tunnel connections)
	mux.Handle("GET /api/status", status.NewHandler(statusInfo, terminal.ActiveSessions, status.Sources{
		HandoverHealthz: handoverURL,
		TunnelMetrics:   os.Getenv("TUNNEL_METRICS_URL"),
		TTL:             30 * time.Second,
	}))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /ws", terminal.HandleWebSocket)

	addr := ":8080"
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           securityHeaders(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.WithField("addr", addr).Info("Starting portfolio terminal server")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.WithError(err).Fatal("Failed to start server")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.WithError(err).Fatal("Server forced to shutdown")
	}

	logger.Info("Server exited")
}

// noListing serves files but not directory listings: a path naming a
// directory (http.FileServer redirects those to end in "/") is a 404, so
// visitors get the files the pages link to and can't browse for others.
func noListing(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "" || strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		h.ServeHTTP(w, r)
	})
}
