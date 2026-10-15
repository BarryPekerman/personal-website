# Technical Summary: Cloud-Native Terminal Portfolio

## 1. Project Scope

A high-fidelity interactive portfolio simulating a Linux environment. The system provides ephemeral, isolated container sessions to visitors via a web interface, demonstrating advanced competency in Kubernetes orchestration, Go development, and container security.

## 2. Architectural Design

**Pattern**: 3-Tier Synchronous Orchestration (Frontend ↔ Middleware ↔ Cluster).

### Frontend
- **Technology**: xterm.js over WebSocket
- **Function**: Renders terminal ANSI/VT100 sequences in the browser
- **Protocol**: WebSocket bidirectional streaming

### Middleware (The Bridge)
- **Technology**: Go-based web server (acting as a custom Kubernetes client)
- **Role**: 
  - Manages HTTP/WebSocket upgrades
  - Translates user input into Kubernetes API calls
  - Handles bidirectional streaming between browser and pod
- **Concurrency**: Utilizes Go goroutines to handle bidirectional streaming
- **Key Functions**:
  - `HandleWebSocket`: Upgrades HTTP to WebSocket, manages session lifecycle
  - `startShellSession`: Establishes bidirectional pipe (WebSocket ↔ Shell)
  - `ExecInPod`: Creates SPDY executor connection to pod

### Backend (The Engine)
- **Platform**: Single-node K3s cluster hosting ephemeral workloads
- **Workload**: Alpine Linux pods containing the portfolio filesystem (CV, projects)
- **Isolation**: One pod per visitor session

## 3. Key Technical Decisions

### Direct API Integration (Selected) vs. CRD/Operator

**Decision**: The Go server imperatively manages Pod lifecycles using `k8s.io/client-go`.

**Rationale**: 
- Avoids the "eventual consistency" latency of the Operator pattern
- A user waiting for a terminal requires synchronous execution
- The Direct API allows immediate attachment to the Pod's stdin/stdout via SPDY streams
- Provides precise control over pod lifecycle (create → wait → exec → delete)

**Implementation**:
```go
// Direct imperative pod creation
pod, err := clientset.CoreV1().Pods(Namespace).Create(ctx, podSpec, metav1.CreateOptions{})

// Synchronous wait for Running state
pod = waitForPodRunning(podName, timeout)

// Immediate exec connection
executor := remotecommand.NewSPDYExecutor(config, "POST", execURL)
executor.Stream(remotecommand.StreamOptions{...})
```

### Session Management

**1:1 Mapping**: One unique Pod per visitor session.

**On-Demand Provisioning**: Discarded "Warm Pool" optimization for V1. 
- Alpine Linux startup time on a local K3s node is negligible (<1s)
- Making the complexity of a pool manager unnecessary
- Simplifies architecture and reduces resource waste
- Pods are ephemeral and cleaned up immediately on disconnect

**Session Lifecycle**:
1. User connects → WebSocket established
2. Pod created with unique session ID
3. Pod reaches Running state
4. Shell exec connection established
5. Bidirectional streaming active
6. On disconnect → Pod deleted immediately

### Security Posture

**Network Policies**: 
- Strict deny-all egress (no internet access from user pods)
- Prevents data exfiltration and external communication
- Implemented via Kubernetes NetworkPolicy resource

**Resource Quotas**: 
- Hard limits on the target namespace to prevent DoS
- Example: Max 20 pods, Max 1 CPU total
- Per-pod limits: 100m CPU, 64Mi memory
- Prevents resource exhaustion attacks

**Security Context**:
- Pods run as unprivileged users (UID 1000)
- Read-only root filesystems (immutable OS)
- Write access limited to `/tmp` (emptyDir volume)
- All capabilities dropped
- No privilege escalation allowed

**Command Filtering**:
- Blocks dangerous commands before execution
- Examples: `rm -rf /`, fork bombs, privilege escalation attempts
- Triggers self-destruct sequence on detection

## 4. Critical Logic Flow

### Connection Phase
1. **User loads page** → Browser requests `index.html`
2. **WebSocket connection** → Browser establishes `ws://host/ws`
3. **HTTP Upgrade** → Go server upgrades HTTP to WebSocket using `gorilla/websocket`

### Provisioning Phase
4. **Pod Creation** → Go server constructs `v1.Pod` struct with:
   - Unique session ID as pod name
   - Alpine Linux image with portfolio content
   - Security context (non-root, read-only FS)
   - Resource limits
   - ConfigMap volume mount for CV/projects
5. **API Call** → `clientset.CoreV1().Pods().Create(ctx, podSpec, opts)`
6. **Wait Loop** → Go server watches Pod status until `Phase == Running`
   - Polls pod status every 1 second
   - 30-second timeout
   - Handles PodFailed state

### Bridge Phase
7. **SPDY Executor** → Go server initiates `remotecommand.NewSPDYExecutor`
   - Creates SPDY connection to Kubernetes API server
   - API server proxies to kubelet
   - Kubelet connects to container runtime
8. **Stream Setup** → Connects WebSocket stream to Pod's `/bin/sh` process:
   - WebSocket → `io.Pipe` → Shell stdin
   - Shell stdout → `io.Pipe` → WebSocket
   - TTY mode enabled for proper terminal behavior
9. **Bidirectional Streaming**:
   - **Input**: Browser → WebSocket → Pipe → Shell stdin
   - **Output**: Shell stdout → Pipe → WebSocket → Browser
   - Handled by separate goroutines for concurrency

### Teardown Phase
10. **Disconnect Detection** → WebSocket close or timeout
11. **Pod Deletion** → Go server calls `Pods().Delete(podName, DeleteOptions{GracePeriodSeconds: 0})`
12. **Cleanup** → Pod resources released immediately

## 5. Skill Demonstration

This architecture validates skills beyond standard "YAML Engineering":

### Kubernetes Internals
- **Programmatic Control Plane**: Manipulating the Control Plane programmatically via client-go
- **Pod Lifecycle Management**: Understanding pod states, scheduling, and resource allocation
- **SPDY Protocol**: Direct use of Kubernetes remotecommand executor for streaming
- **RBAC Configuration**: ServiceAccount permissions for pod create/delete/exec

### Systems Programming
- **Stream Handling**: Managing bidirectional streams (WebSocket ↔ stdin/stdout)
- **Buffer Management**: Efficient data transfer between protocols
- **Signal Handling**: SIGINT/SIGTERM graceful shutdown
- **Concurrency**: Goroutines for parallel input/output handling
- **Error Handling**: Robust error recovery and logging

### Platform Engineering
- **Bespoke Runtime Platform**: Building a custom runtime platform rather than just deploying apps
- **Security Architecture**: Defense-in-depth with multiple security layers
- **Resource Management**: Efficient resource allocation and cleanup
- **Observability**: Comprehensive logging and monitoring

### Web Technologies
- **WebSocket Protocol**: Real-time bidirectional communication
- **Terminal Emulation**: ANSI/VT100 sequence rendering in browser
- **Protocol Translation**: WebSocket ↔ SPDY ↔ stdin/stdout

## 6. Implementation Status

### ✅ Completed Components

1. **WebSocket Handler** (`internal/handlers/terminal.go`)
   - HTTP to WebSocket upgrade
   - Session management
   - Bidirectional streaming setup

2. **Kubernetes Client** (`internal/k8s/client.go`)
   - Pod creation with proper security context
   - Pod status polling and wait logic
   - SPDY executor connection
   - Pod deletion with immediate cleanup

3. **Security Features**
   - Command filtering
   - Self-destruct sequence
   - Resource limits
   - Network policies

### 🔧 Potential Improvements

1. **Terminal Resize Handling**
   - Currently logged but not implemented
   - Should send resize events to SPDY executor
   - Requires `remotecommand.TerminalSizeQueue`

2. **Binary Data Handling**
   - Current implementation uses `bufio.Scanner` (line-based)
   - Should handle raw binary for ANSI sequences
   - Consider using `io.Copy` with buffered channels

3. **Error Recovery**
   - Better handling of pod creation failures
   - Retry logic for transient errors
   - User-friendly error messages

4. **Performance Optimization**
   - Connection pooling (if warm pool added)
   - Stream buffering optimization
   - Reduced logging overhead

## 7. Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│  Browser (xterm.js)                                         │
│  - Renders ANSI/VT100 sequences                             │
│  - Sends keystrokes via WebSocket                          │
└────────────────────┬────────────────────────────────────────┘
                     │ WebSocket (ws://)
                     ▼
┌─────────────────────────────────────────────────────────────┐
│  Go Middleware (The Bridge)                                │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  HandleWebSocket()                                    │  │
│  │  - Upgrades HTTP → WebSocket                         │  │
│  │  - Creates session ID                                │  │
│  └──────────────────────────────────────────────────────┘  │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  CreateTerminalPod()                                  │  │
│  │  - Constructs v1.Pod spec                           │  │
│  │  - Calls K8s API: Pods().Create()                   │  │
│  │  - Waits for Phase == Running                        │  │
│  └──────────────────────────────────────────────────────┘  │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  ExecInPod()                                          │  │
│  │  - Creates SPDY executor                             │  │
│  │  - Connects to /bin/sh in pod                        │  │
│  │  - Sets up bidirectional streams                     │  │
│  └──────────────────────────────────────────────────────┘  │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  startShellSession()                                  │  │
│  │  - Goroutine: WebSocket → Shell stdin               │  │
│  │  - Goroutine: Shell stdout → WebSocket              │  │
│  │  - Handles command filtering                         │  │
│  └──────────────────────────────────────────────────────┘  │
└────────────────────┬────────────────────────────────────────┘
                     │ SPDY (via K8s API)
                     ▼
┌─────────────────────────────────────────────────────────────┐
│  Kubernetes API Server                                      │
│  - Validates RBAC permissions                               │
│  - Proxies to kubelet                                      │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│  Kubelet                                                    │
│  - Manages container runtime                                │
│  - Establishes exec connection                              │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│  Ephemeral Pod (Alpine Linux)                              │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  /bin/sh process                                       │  │
│  │  - Executes user commands                             │  │
│  │  - Reads from stdin                                   │  │
│  │  - Writes to stdout/stderr                            │  │
│  └──────────────────────────────────────────────────────┘  │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Portfolio Content (ConfigMap)                      │  │
│  │  - cv.txt                                            │  │
│  │  - projects/                                          │  │
│  │  - contact_info.md                                    │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

## 8. Code Structure

```
portfolio-terminal/
├── cmd/server/
│   └── main.go                    # Entry point, HTTP server setup
├── internal/
│   ├── handlers/
│   │   └── terminal.go            # WebSocket handler, bridge logic
│   ├── k8s/
│   │   └── client.go              # Kubernetes client, pod management
│   └── websocket/
│       └── message.go             # WebSocket message types
├── k8s/
│   ├── deployment.yaml            # Backend pod deployment
│   ├── rbac.yaml                  # ServiceAccount permissions
│   ├── network-policy.yaml        # Network isolation
│   └── resource-quota.yaml        # Resource limits
└── web/
    ├── static/
    │   └── js/terminal.js         # xterm.js frontend
    └── templates/
        └── index.html             # HTML page
```

## 9. Next Steps

### Recommended Improvements

1. **Enhance Terminal Resize**
   ```go
   // Add terminal size queue to ExecInPod
   sizeQueue := remotecommand.TerminalSizeQueue{}
   // Send resize events from WebSocket resize handler
   ```

2. **Improve Binary Streaming**
   ```go
   // Replace bufio.Scanner with io.Copy for raw binary
   go io.Copy(ws, stdoutReader)
   go io.Copy(stdinWriter, ws)
   ```

3. **Add Metrics & Observability**
   - Prometheus metrics for pod creation time
   - Session duration tracking
   - Error rate monitoring

4. **Optimize Resource Usage**
   - Consider connection pooling for high traffic
   - Implement session timeout
   - Add health checks

---

**Built with**: Go, Kubernetes (K3s), Docker, WebSockets, xterm.js  
**Architecture Pattern**: 3-Tier Synchronous Orchestration  
**Security Model**: Defense-in-Depth (Network Policies, RBAC, Resource Quotas, Command Filtering)



