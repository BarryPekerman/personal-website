package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/gorilla/websocket"
)

// Session represents a real shell session in a Docker container
type Session struct {
	ContainerID string
	Client      *client.Client
	mu          sync.Mutex
}

var (
	sessions     = make(map[string]*Session)
	sessionMu    sync.RWMutex
	dockerClient *client.Client
	upgrader     = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for MVP
		},
	}
)

func init() {
	var err error
	dockerClient, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		fmt.Printf("Warning: Could not create Docker client: %v\n", err)
		fmt.Println("Falling back to pseudo-shell mode")
	}
}

// createContainer creates a new Docker container with a persistent shell
func createContainer(sessionID string) (*Session, error) {
	if dockerClient == nil {
		return nil, fmt.Errorf("Docker client not available")
	}

	ctx := context.Background()

	// Get content directory path
	contentPath, _ := filepath.Abs("./content")
	if _, err := os.Stat(contentPath); err != nil {
		contentPath = "/app/content" // Fallback
	}

	// Create container config
	config := &types.ContainerCreateConfig{
		Config: &types.ContainerConfig{
			Image: "alpine:latest",
			Cmd:   []string{"/bin/sh", "-l"}, // Login shell to load profile
			Tty:   true,
			OpenStdin: true,
			StdinOnce: false,
			Env: []string{
				"TERM=xterm-256color",
				"PS1=guest@portfolio:\\w\\$ ",
			},
			WorkingDir: "/app/content",
		},
		HostConfig: &types.HostConfig{
			AutoRemove: true,
			NetworkMode: "bridge",
			Binds: []string{
				contentPath + ":/app/content:ro",
			},
		},
	}

	// Create the container
	resp, err := dockerClient.ContainerCreate(ctx, config.Config, config.HostConfig, nil, nil, "terminal-"+sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %v", err)
	}

	// Start the container
	if err := dockerClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return nil, fmt.Errorf("failed to start container: %v", err)
	}

	return &Session{
		ContainerID: resp.ID,
		Client:      dockerClient,
	}, nil
}

// removeContainer removes a container
func removeContainer(session *Session) {
	if session == nil || dockerClient == nil {
		return
	}
	ctx := context.Background()
	timeout := 5 * time.Second
	dockerClient.ContainerStop(ctx, session.ContainerID, &timeout)
	dockerClient.ContainerRemove(ctx, session.ContainerID, types.ContainerRemoveOptions{Force: true})
}

// attachToShell attaches to the container's main process (the persistent shell)
func (s *Session) attachToShell() (io.ReadCloser, io.WriteCloser, error) {
	if dockerClient == nil {
		return nil, nil, fmt.Errorf("Docker client not available")
	}

	ctx := context.Background()

	// Attach to the container's main process (the shell we started)
	// This gives us the persistent shell, not a new exec
	attachResp, err := dockerClient.ContainerAttach(ctx, s.ContainerID, types.ContainerAttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to attach to container: %v", err)
	}

	return attachResp.Reader, attachResp.Conn, nil
}

// execCommand executes a single command (for HTTP API fallback)
func (s *Session) execCommand(command string) (string, error) {
	if dockerClient == nil {
		return "", fmt.Errorf("Docker client not available")
	}

	ctx := context.Background()

	// Create exec config
	execConfig := types.ExecConfig{
		Cmd:          []string{"/bin/sh", "-c", command},
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
	}

	// Create exec instance
	execResp, err := dockerClient.ContainerExecCreate(ctx, s.ContainerID, execConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create exec: %v", err)
	}

	// Attach to exec
	attachResp, err := dockerClient.ContainerExecAttach(ctx, execResp.ID, types.ExecAttachOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to attach exec: %v", err)
	}
	defer attachResp.Close()

	// Read output
	var output strings.Builder
	scanner := bufio.NewScanner(attachResp.Reader)
	for scanner.Scan() {
		output.WriteString(scanner.Text())
		output.WriteString("\n")
	}

	// Inspect exec to get exit code
	inspectResp, err := dockerClient.ContainerExecInspect(ctx, execResp.ID)
	if err == nil && inspectResp.ExitCode != 0 {
		return output.String(), fmt.Errorf("exit code %d", inspectResp.ExitCode)
	}

	return output.String(), nil
}

// handleWebSocket handles WebSocket connections for real-time terminal with persistent shell
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Failed to upgrade connection", http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	// Get session ID from query parameter
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		sessionID = fmt.Sprintf("session-%d", time.Now().UnixNano())
	}

	// Get or create session
	sessionMu.Lock()
	session, exists := sessions[sessionID]
	if !exists {
		var err error
		session, err = createContainer(sessionID)
		if err != nil {
			sessionMu.Unlock()
			conn.WriteJSON(map[string]string{"error": err.Error()})
			return
		}
		sessions[sessionID] = session
	}
	sessionMu.Unlock()

	// Cleanup on disconnect
	defer func() {
		sessionMu.Lock()
		delete(sessions, sessionID)
		sessionMu.Unlock()
		removeContainer(session)
	}()

	// Attach to the persistent shell
	reader, writer, err := session.attachToShell()
	if err != nil {
		conn.WriteJSON(map[string]string{"error": err.Error()})
		return
	}
	defer reader.Close()
	defer writer.Close()

	// Stream shell output to WebSocket
	go func() {
		buffer := make([]byte, 1024)
		for {
			n, err := reader.Read(buffer)
			if err != nil {
				break
			}
			if n > 0 {
				conn.WriteMessage(websocket.BinaryMessage, buffer[:n])
			}
		}
	}()

	// Stream WebSocket input to shell
	for {
		messageType, data, err := conn.ReadMessage()
		if err != nil {
			break
		}

		if messageType == websocket.TextMessage {
			var msg map[string]string
			if err := json.Unmarshal(data, &msg); err == nil {
				command := msg["command"]
				if command != "" {
					writer.Write([]byte(command))
				}
			}
		} else if messageType == websocket.BinaryMessage {
			writer.Write(data)
		}
	}
}

// handleCommand handles HTTP POST commands (fallback)
func handleCommand(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"session_id"`
		Command   string `json:"command"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Get or create session
	sessionMu.Lock()
	session, exists := sessions[req.SessionID]
	if !exists {
		var err error
		session, err = createContainer(req.SessionID)
		if err != nil {
			sessionMu.Unlock()
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		sessions[req.SessionID] = session
	}
	sessionMu.Unlock()

	// Execute command
	output, err := session.execCommand(req.Command)
	if err != nil {
		output += "\n" + err.Error()
	}

	json.NewEncoder(w).Encode(map[string]string{
		"output": output,
		"prompt": "guest@portfolio:~$ ",
	})
}

// serveHTML serves the frontend with xterm.js
func serveHTML(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Real Shell Terminal</title>
    <script src="https://cdn.jsdelivr.net/npm/xterm@5.3.0/lib/xterm.js"></script>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/xterm@5.3.0/css/xterm.css" />
    <style>
        body { margin: 0; padding: 20px; background: #000; }
        #terminal { width: 100%; height: 80vh; }
    </style>
</head>
<body>
    <div id="terminal"></div>
    <script>
        const term = new Terminal({
            theme: { background: '#000', foreground: '#0f0' },
            fontFamily: 'Courier New, monospace',
            fontSize: 14,
            cursorBlink: true
        });
        term.open(document.getElementById('terminal'));
        
        const sessionID = 'session-' + Math.random().toString(36).substring(7);
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const ws = new WebSocket(protocol + '//' + window.location.host + '/ws?session_id=' + sessionID);
        
        term.onData(data => {
            if (ws.readyState === WebSocket.OPEN) {
                ws.send(JSON.stringify({command: data}));
            }
        });
        
        ws.onmessage = (event) => {
            if (event.data instanceof ArrayBuffer) {
                const uint8Array = new Uint8Array(event.data);
                term.write(uint8Array);
            } else {
                const msg = JSON.parse(event.data);
                if (msg.output) {
                    term.write(msg.output);
                }
            }
        };
        
        ws.onopen = () => {
            term.write('Connected to real shell terminal (Docker)\\r\\n');
        };
        
        ws.onerror = (error) => {
            term.write('\\r\\nError: Could not connect to shell. Make sure Docker is running.\\r\\n');
        };
        
        ws.onclose = () => {
            term.write('\\r\\nConnection closed.\\r\\n');
        };
    </script>
</body>
</html>`
	w.Write([]byte(html))
}

func main() {
	// Check if Docker is available
	if dockerClient == nil {
		fmt.Println("WARNING: Docker client not available. Using fallback mode.")
		fmt.Println("Make sure Docker is running and the socket is accessible.")
	}

	http.HandleFunc("/ws", handleWebSocket)
	http.HandleFunc("/api/cmd", handleCommand)
	http.HandleFunc("/", serveHTML)

	fmt.Println("Real Shell Terminal Server running on http://localhost:8080")
	fmt.Println("Make sure Docker is running!")
	http.ListenAndServe(":8080", nil)
}
