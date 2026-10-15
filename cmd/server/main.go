package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	terminalHandlers "portfolio/internal/handlers"
	"portfolio/internal/k8s"
	"portfolio/internal/shell"
)

func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetLevel(logrus.DebugLevel) // Enable debug to see filter logs

	var shellManager shell.Manager

	// Determine mode from environment variable
	appMode := os.Getenv("APP_MODE")
	if appMode == "local" {
		logger.Info("Starting in LOCAL mode (using Docker/Bash)")
		
		// Get content path from environment or use default
		contentPath := os.Getenv("CONTENT_PATH")
		if contentPath == "" {
			contentPath = "./content"
		}
		
		shellManager = shell.NewLocalManager(logger, contentPath)
		logger.WithField("content_path", contentPath).Info("Local mode initialized")
	} else {
		logger.Info("Starting in KUBERNETES mode")
		
		// Create Kubernetes client
		k8sClient, err := k8s.NewClient()
		if err != nil {
			logger.WithError(err).Fatal("Failed to create Kubernetes client")
		}
		
		shellManager = shell.NewK8sManager(k8sClient, logger)
		logger.Info("Kubernetes mode initialized")
	}

	r := mux.NewRouter()

	// Serve static files
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	// Serve index.html for the root path
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/templates/index.html")
	})

	// WebSocket handler - works with both local and K8s modes via interface
	terminalHandler := terminalHandlers.NewTerminalHandler(shellManager, logger)
	r.HandleFunc("/ws", terminalHandler.HandleWebSocket)

	// Apply CORS middleware
	corsHandler := handlers.CORS(
		handlers.AllowedOrigins([]string{"*"}), // Allow all origins for development
		handlers.AllowedMethods([]string{"GET", "HEAD", "POST", "PUT", "OPTIONS"}),
		handlers.AllowedHeaders([]string{"Content-Type", "Authorization"}),
	)(r)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: corsHandler,
	}

	go func() {
		modeText := "Kubernetes"
		if appMode == "local" {
			modeText = "Local"
		}
		logger.WithField("mode", modeText).Info("Starting portfolio terminal server on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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
