package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for demo
	},
}

type TerminalSession struct {
	conn   *websocket.Conn
	id     string
	logger *logrus.Logger
}

func main() {
	// Initialize logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetLevel(logrus.InfoLevel)

	// Create router
	r := mux.NewRouter()

	// CORS middleware
	corsHandler := handlers.CORS(
		handlers.AllowedOrigins([]string{"*"}),
		handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		handlers.AllowedHeaders([]string{"Content-Type", "Authorization"}),
	)

	// Routes
	r.HandleFunc("/", serveIndex).Methods("GET")
	r.HandleFunc("/ws", handleWebSocket(logger)).Methods("GET")
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("web/static/"))))

	// Apply CORS
	handler := corsHandler(r)

	// Server configuration
	srv := &http.Server{
		Addr:         ":8080",
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Info("Starting portfolio terminal server on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Fatal("Server failed to start")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.WithError(err).Fatal("Server forced to shutdown")
	}

	logger.Info("Server exited")
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/templates/index.html")
}

func handleWebSocket(logger *logrus.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.WithError(err).Error("Failed to upgrade connection")
			return
		}
		defer conn.Close()

		sessionID := fmt.Sprintf("session-%d", time.Now().UnixNano())
		logger.WithField("session_id", sessionID).Info("New WebSocket connection")

		session := &TerminalSession{
			conn:   conn,
			id:     sessionID,
			logger: logger,
		}

		session.handleSession()
	}
}

func (ts *TerminalSession) handleSession() {
	// Send welcome message
	ts.sendMessage("output", "\r\n\x1b[1;36mWelcome to the Portfolio Terminal (Alternative Version)\x1b[0m\r\n")
	ts.sendMessage("output", "This version uses lightweight dependencies without Kubernetes integration.\r\n\r\n")
	ts.sendMessage("output", "Available commands:\r\n")
	ts.sendMessage("output", "  \x1b[1;32mcv\x1b[0m or \x1b[1;32mresume\x1b[0m - View CV\r\n")
	ts.sendMessage("output", "  \x1b[1;32mcontact\x1b[0m       - View contact information\r\n")
	ts.sendMessage("output", "  \x1b[1;32mprojects\x1b[0m      - List projects\r\n")
	ts.sendMessage("output", "  \x1b[1;32mhelp\x1b[0m          - Show this help\r\n")
	ts.sendMessage("output", "  \x1b[1;32mclear\x1b[0m         - Clear screen\r\n\r\n")
	ts.sendMessage("output", "guest@portfolio:~$ ")

	for {
		var msg struct {
			Type string `json:"type"`
			Data string `json:"data"`
		}

		err := ts.conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				ts.logger.WithError(err).Error("WebSocket error")
			}
			break
		}

		switch msg.Type {
		case "input":
			ts.handleCommand(msg.Data)
		case "resize":
			// Handle terminal resize if needed
			ts.logger.Debug("Terminal resize requested")
		}
	}
}

func (ts *TerminalSession) handleCommand(input string) {
	ts.logger.WithField("command", input).Debug("Command received")

	// Check for dangerous commands
	if ts.isDangerousCommand(input) {
		ts.handleSelfDestruct()
		return
	}

	// Handle commands
	switch input {
	case "cv", "resume":
		ts.showCV()
	case "contact":
		ts.showContact()
	case "projects":
		ts.showProjects()
	case "help":
		ts.showHelp()
	case "clear":
		ts.sendMessage("output", "\x1b[2J\x1b[H")
	case "":
		// Empty command, just show prompt
		ts.sendMessage("output", "guest@portfolio:~$ ")
	default:
		ts.sendMessage("output", fmt.Sprintf("Command not found: %s\r\n", input))
		ts.sendMessage("output", "Type 'help' for available commands.\r\n")
		ts.sendMessage("output", "guest@portfolio:~$ ")
	}
}

func (ts *TerminalSession) isDangerousCommand(input string) bool {
	dangerousCommands := []string{
		"sudo rm /", "rm -rf /", "dd if=/dev/zero", "mkfs", "shutdown", "reboot",
		":(){ :|:& };:", "killall", "format", "fdisk",
	}

	for _, cmd := range dangerousCommands {
		if input == cmd {
			ts.logger.WithField("command", input).Warn("Dangerous command detected")
			return true
		}
	}
	return false
}

func (ts *TerminalSession) handleSelfDestruct() {
	messages := []string{
		"\r\n🚨 SECURITY BREACH DETECTED 🚨\r\n",
		"Initiating emergency self-destruct sequence...\r\n",
		"3...\r\n",
		"2...\r\n", 
		"1...\r\n",
		"💥 SYSTEM DESTROYED 💥\r\n",
		"\r\nConnection terminated for security reasons.\r\n",
	}

	for _, msg := range messages {
		time.Sleep(500 * time.Millisecond)
		ts.sendMessage("output", msg)
	}

	ts.conn.Close()
}

func (ts *TerminalSession) showCV() {
	cv := `
═══════════════════════════════════════════════════════════════════════════════
                              CURRICULUM VITAE
═══════════════════════════════════════════════════════════════════════════════

[Your Name Here]
Senior Software Engineer & Cloud Infrastructure Specialist

PROFESSIONAL SUMMARY
───────────────────────────────────────────────────────────────────────────────
Innovative software engineer with 5+ years of experience in distributed systems,
cloud-native architectures, and secure application development. Expertise in
Kubernetes, Go, and infrastructure automation.

TECHNICAL SKILLS
───────────────────────────────────────────────────────────────────────────────
Languages:         Go, Python, JavaScript/TypeScript, Rust, Java
Cloud Platforms:   AWS (EKS, ECS, Lambda), GCP (GKE), Azure (AKS)
Infrastructure:    Kubernetes, Docker, Terraform, Helm, ArgoCD
Databases:         PostgreSQL, Redis, MongoDB, Elasticsearch
Security:          OAuth2/OIDC, JWT, mTLS, network policies, RBAC

PROFESSIONAL EXPERIENCE
───────────────────────────────────────────────────────────────────────────────
Senior Software Engineer | TechCorp Inc. | 2022-Present
• Architected multi-tenant Kubernetes platform serving 100K+ users
• Led migration to microservices, reducing deployment time by 85%
• Implemented security-first infrastructure with zero-trust networking

Software Engineer | StartupXYZ | 2020-2022  
• Developed high-throughput data processing pipelines handling 10TB+ daily
• Implemented RESTful APIs and GraphQL services
• Containerized legacy applications with Kubernetes orchestration

EDUCATION
───────────────────────────────────────────────────────────────────────────────
Master of Science in Computer Science | Stanford University | 2017-2019
Bachelor of Science in Computer Science | UC Berkeley | 2013-2017

CERTIFICATIONS
───────────────────────────────────────────────────────────────────────────────
• AWS Certified Solutions Architect - Professional
• Certified Kubernetes Administrator (CKA)
• HashiCorp Certified Terraform Associate

═══════════════════════════════════════════════════════════════════════════════

`
	ts.sendMessage("output", cv)
	ts.sendMessage("output", "guest@portfolio:~$ ")
}

func (ts *TerminalSession) showContact() {
	contact := `
═══════════════════════════════════════════════════════════════════════════════
                            CONTACT INFORMATION
═══════════════════════════════════════════════════════════════════════════════

📧 Email:     your.email@example.com
🔗 LinkedIn:  linkedin.com/in/your-profile  
🐙 GitHub:    github.com/your-username
🌐 Website:   your-portfolio-site.com
📍 Location:  San Francisco, CA

💼 AVAILABILITY
───────────────────────────────────────────────────────────────────────────────
Open to opportunities in:
• Distributed Systems & Cloud Architecture
• DevOps & Infrastructure Engineering  
• Kubernetes & Container Orchestration
• Security & Compliance Engineering

Preferred work arrangements:
• Remote-first teams
• Hybrid (2-3 days in office)
• Full-time positions
• Interesting contract work (6+ months)

═══════════════════════════════════════════════════════════════════════════════

`
	ts.sendMessage("output", contact)
	ts.sendMessage("output", "guest@portfolio:~$ ")
}

func (ts *TerminalSession) showProjects() {
	projects := `
═══════════════════════════════════════════════════════════════════════════════
                                PROJECTS
═══════════════════════════════════════════════════════════════════════════════

🚀 PROJECT 1: Distributed Task Scheduler
───────────────────────────────────────────────────────────────────────────────
A high-performance task scheduling system built with Go and Redis.

Technologies: Go 1.21, Redis Cluster, gRPC, Docker & Kubernetes
Features: Fault-tolerant execution, real-time monitoring, horizontal scaling
GitHub: https://github.com/your-username/task-scheduler

🔐 PROJECT 2: Secure API Gateway  
───────────────────────────────────────────────────────────────────────────────
Enterprise-grade API gateway with advanced security features.

Technologies: Rust, Tokio async runtime, JWT authentication, Redis rate limiting
Features: OAuth 2.0/OIDC support, request transformation, circuit breaker
GitHub: https://github.com/your-username/api-gateway

☁️ PROJECT 3: Infrastructure as Code Platform
───────────────────────────────────────────────────────────────────────────────
Terraform-based platform for managing cloud infrastructure.

Technologies: Terraform 1.5+, AWS CDK, GitHub Actions, ArgoCD GitOps
Features: Multi-cloud support, automated testing, cost optimization
GitHub: https://github.com/your-username/iac-platform

🖥️ PROJECT 4: Portfolio Terminal (This Project!)
───────────────────────────────────────────────────────────────────────────────
Kubernetes-native terminal portfolio with security features.

Technologies: Go, WebSockets, Kubernetes, Docker, Alpine Linux
Features: Pod isolation, network policies, self-destruct sequences
GitHub: https://github.com/your-username/portfolio-terminal

═══════════════════════════════════════════════════════════════════════════════

`
	ts.sendMessage("output", projects)
	ts.sendMessage("output", "guest@portfolio:~$ ")
}

func (ts *TerminalSession) showHelp() {
	help := `
═══════════════════════════════════════════════════════════════════════════════
                               HELP COMMANDS
═══════════════════════════════════════════════════════════════════════════════

Available commands:

📄 CONTENT COMMANDS
───────────────────────────────────────────────────────────────────────────────
cv, resume    Display curriculum vitae
contact       Show contact information  
projects      List portfolio projects
help          Show this help message

🖥️ TERMINAL COMMANDS
───────────────────────────────────────────────────────────────────────────────
clear         Clear the terminal screen

⚠️ SECURITY FEATURES
───────────────────────────────────────────────────────────────────────────────
This terminal includes security monitoring. Dangerous commands like:
• sudo rm /
• rm -rf /  
• shutdown/reboot commands
• Fork bombs

Will trigger the self-destruct sequence! 💥

═══════════════════════════════════════════════════════════════════════════════
Built with: Go + WebSockets + Gorilla Mux (Lightweight Alternative)
No Kubernetes dependencies required!
═══════════════════════════════════════════════════════════════════════════════

`
	ts.sendMessage("output", help)
	ts.sendMessage("output", "guest@portfolio:~$ ")
}

func (ts *TerminalSession) sendMessage(msgType, data string) {
	msg := struct {
		Type string `json:"type"`
		Data string `json:"data"`
	}{
		Type: msgType,
		Data: data,
	}

	if err := ts.conn.WriteJSON(msg); err != nil {
		ts.logger.WithError(err).Error("Failed to send message")
	}
}
