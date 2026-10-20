// Command server serves the portfolio terminal website.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"portfolio/content"
	terminalHandlers "portfolio/internal/handlers"
	"portfolio/internal/k8s"
	"portfolio/internal/shell"
	"portfolio/internal/status"
)

// commit is set at build time: -ldflags "-X main.commit=<sha>"
var commit = "dev"

func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	if os.Getenv("LOG_LEVEL") == "debug" {
		logger.SetLevel(logrus.DebugLevel)
	}

	// APP_MODE picks the shell backend:
	//   virtual (default) - built-in simulated shell, nothing runs on the host
	//   local             - real bash on this machine; development only, never expose it
	//   k8s               - real shell in a throwaway pod per visitor (work in progress)
	var shellManager shell.Manager
	appMode := os.Getenv("APP_MODE")
	switch appMode {
	case "", "virtual":
		appMode = "virtual"
		shellManager = shell.NewVirtualManager(content.FS, logger)
	case "local":
		contentPath := os.Getenv("CONTENT_PATH")
		if contentPath == "" {
			contentPath = "./content"
		}
		logger.Warn("LOCAL mode runs a real shell on this host - development only, never expose it")
		shellManager = shell.NewLocalManager(logger, contentPath)
	case "k8s":
		k8sClient, err := k8s.NewClient(logger)
		if err != nil {
			logger.WithError(err).Fatal("Failed to create Kubernetes client")
		}
		shellManager = shell.NewK8sManager(k8sClient, logger)
	default:
		logger.WithField("mode", appMode).Fatal("Unknown APP_MODE (use virtual, local or k8s)")
	}

	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	// Front page: Uplink-style HUD around the terminal; /terminal is the
	// terminal alone, full screen
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/templates/home.html")
	})
	mux.HandleFunc("GET /terminal", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/templates/index.html")
	})
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
	mux.Handle("GET /api/status", status.NewHandler(statusInfo, terminalHandlers.ActiveSessions, status.Sources{
		HandoverHealthz: handoverURL,
		TunnelMetrics:   os.Getenv("TUNNEL_METRICS_URL"),
		TTL:             30 * time.Second,
	}))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /ws", terminalHandlers.NewTerminalHandler(shellManager, logger).HandleWebSocket)

	addr := ":8080"
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.WithFields(logrus.Fields{"mode": appMode, "addr": addr}).Info("Starting portfolio terminal server")
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
