// Command server serves the portfolio terminal website.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"portfolio/content"
	terminalHandlers "portfolio/internal/handlers"
	"portfolio/internal/shell"
	"portfolio/internal/status"
	"portfolio/web"
)

// commit is set at build time: -ldflags "-X main.commit=<sha>"
var commit = "dev"

func main() {
	level := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

	handler, err := newHandler(configFromEnv(), logger)
	if err != nil {
		logger.Error("Failed to set up the server", "err", err)
		os.Exit(1)
	}

	addr := ":8080"
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("Starting portfolio terminal server", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Failed to start server", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", "err", err)
		os.Exit(1)
	}

	logger.Info("Server exited")
}

// config is what the server reads from its environment
type config struct {
	commit            string
	pod, node, region string // Downward API and the cluster's region
	devVisitor        string // DEV_VISITOR=IL/TLV
	handoverURL       string // The Handover's /healthz
	tunnelMetricsURL  string // cloudflared's /metrics
}

func configFromEnv() config {
	c := config{
		commit:           commit,
		pod:              os.Getenv("POD_NAME"),
		node:             os.Getenv("NODE_NAME"),
		region:           os.Getenv("REGION"),
		devVisitor:       os.Getenv("DEV_VISITOR"),
		handoverURL:      os.Getenv("HANDOVER_HEALTHZ_URL"),
		tunnelMetricsURL: os.Getenv("TUNNEL_METRICS_URL"),
	}
	if c.handoverURL == "" {
		c.handoverURL = "https://play.barrypekerman.com/healthz"
	}
	return c
}

// newHandler builds every route, wrapped in the security headers
func newHandler(c config, logger *slog.Logger) (http.Handler, error) {
	// The shell is simulated: nothing a visitor types runs anywhere
	shellManager := shell.NewVirtualManager(content.FS, logger)
	terminal := terminalHandlers.NewTerminalHandler(shellManager, logger, terminalHandlers.DefaultLimits)

	statusInfo := status.Info{
		Pod:     c.pod,
		Node:    c.node,
		Region:  c.region,
		Commit:  c.commit,
		Started: time.Now().UTC(),
	}
	// DEV_VISITOR=IL/TLV previews the route map locally, where requests carry
	// no Cloudflare headers
	if country, colo, ok := strings.Cut(c.devVisitor, "/"); ok {
		statusInfo.DevVisitor = status.Visitor{Country: country, Colo: colo}
	}
	// The tunnel metrics URL is unset locally: the map then shows no tunnel
	// connections
	statusHandler := status.NewHandler(statusInfo, terminal.ActiveSessions, status.Sources{
		HandoverHealthz: c.handoverURL,
		TunnelMetrics:   c.tunnelMetricsURL,
		TTL:             30 * time.Second,
	})
	// The page and the terminal name the edge that served them
	terminal.Edge = func(r *http.Request) (string, string) {
		v := statusHandler.Visitor(r)
		return v.Country, v.Colo
	}
	pages, err := newPages(c.commit, statusHandler.Visitor)
	if err != nil {
		return nil, err
	}
	static, err := fs.Sub(web.FS, "static")
	if err != nil {
		return nil, err
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
	// The project list, built from the write-ups in content/projects
	projects, err := content.Projects(content.FS)
	if err != nil {
		return nil, err
	}
	projectsJSON, err := json.Marshal(projects)
	if err != nil {
		return nil, err
	}
	mux.HandleFunc("GET /api/projects", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(projectsJSON)
	})
	mux.Handle("GET /api/status", statusHandler)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /ws", terminal.HandleWebSocket)
	return securityHeaders(mux), nil
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
