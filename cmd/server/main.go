// Command server serves the portfolio terminal website.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"portfolio/content"
	terminalHandlers "portfolio/internal/handlers"
	"portfolio/internal/k8s"
	"portfolio/internal/shell"
)

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
	// Front page: the Uplink-style design (static for now); the working
	// terminal is at /terminal until the front page embeds it
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/static/mockup.html")
	})
	mux.HandleFunc("GET /terminal", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/templates/index.html")
	})
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
