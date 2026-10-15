# Your Current Setup Explained

## What You Actually Have

### Two Separate Containers:

1. **Backend Container** (`Dockerfile.backend`)
   - Image: `portfolio-k8s:latest` (or `portfolio-backend`)
   - Contains: Go binary + web files
   - Runs: Go web server on port 8080
   - Base: `alpine:latest` (has `/bin/sh` from BusyBox)

2. **Guest Container** (`Dockerfile.guest`)
   - Image: `portfolio-guest:latest`
   - Contains: Your CV files, projects, bash config
   - Runs: Nothing (just sits there waiting)
   - Base: `alpine:latest` (has `/bin/sh` from BusyBox)

## Current Architecture (Pod-per-User)

```
┌─────────────────────────────────────────────────────────┐
│  Kubernetes Cluster                                     │
│                                                          │
│  ┌──────────────────────────────────────────────────┐  │
│  │  Backend Pod (portfolio-terminal)                │  │
│  │  ┌────────────────────────────────────────────┐  │  │
│  │  │  Go Web App (main.go)                     │  │  │
│  │  │  - Listens on :8080                       │  │  │
│  │  │  - Handles /ws WebSocket                  │  │  │
│  │  │  - Uses Kubernetes client-go              │  │  │
│  │  └────────────────────────────────────────────┘  │  │
│  └──────────────────────────────────────────────────┘  │
│                          │                             │
│                          │ kubectl exec                │
│                          │ (via K8s API)               │
│                          ▼                             │
│  ┌──────────────────────────────────────────────────┐  │
│  │  Guest Pod (terminal-xxx)                       │  │
│  │  ┌────────────────────────────────────────────┐  │  │
│  │  │  Alpine Container                         │  │  │
│  │  │  - /bin/sh (BusyBox shell)                │  │  │
│  │  │  - cv.txt, projects/                      │  │  │
│  │  │  - .bashrc config                          │  │  │
│  │  └────────────────────────────────────────────┘  │  │
│  └──────────────────────────────────────────────────┘  │
│                                                          │
└─────────────────────────────────────────────────────────┘
         ▲
         │ WebSocket
         │
┌────────┴────────┐
│   Browser       │
│   (xterm.js)    │
└─────────────────┘
```

## The Flow (Step by Step)

### 1. User Visits Website
```
Browser → http://yoursite.com
         ↓
Go App serves index.html (web page with xterm.js)
```

### 2. Browser Connects WebSocket
```
Browser → ws://yoursite.com/ws
         ↓
Go App: HandleWebSocket() receives connection
```

### 3. Go App Creates Guest Pod
```go
// In internal/k8s/client.go
pod, err := k8sClient.CreateTerminalPod(sessionID)
// Creates a NEW pod with portfolio-guest:latest image
```

**What happens:**
- Go app calls Kubernetes API
- Kubernetes creates a new pod
- Pod runs `portfolio-guest:latest` container
- Container has your CV files, projects, bash config
- Pod waits (doesn't run anything yet)

### 4. Go App Executes Shell in Pod
```go
// In internal/handlers/terminal.go
h.k8sClient.ExecInPod(podName, k8s.PodExecOptions{
    Command: []string{"/bin/sh", "-l"},  // Runs shell in guest pod
    Stdin:   stdinReader,
    Stdout:  stdoutWriter,
    TTY:     true,
})
```

**What happens:**
- Go app uses `kubectl exec` (via Kubernetes API)
- Executes `/bin/sh` inside the guest pod
- Connects stdin/stdout to Go app
- Shell loads `.bashrc` (your aliases, prompt, etc.)

### 5. User Types "ls"
```
Browser sends: "ls\n" via WebSocket
         ↓
Go App receives WebSocket message
         ↓
Go App writes to shell stdin: "ls\n"
         ↓
Shell in guest pod executes: ls
         ↓
Shell outputs: "cv.txt\nprojects/\n"
         ↓
Go App reads from shell stdout
         ↓
Go App sends via WebSocket: "cv.txt\nprojects/\n"
         ↓
Browser displays output
```

## Why Two Containers?

### Backend Container (Go App)
- **Purpose**: Web server
- **Why separate**: Needs Kubernetes API access, different security context
- **Contains**: Go binary, web files (HTML/CSS/JS)
- **Does**: Handles HTTP/WebSocket, creates pods, manages connections

### Guest Container (Terminal Environment)
- **Purpose**: Terminal environment for users
- **Why separate**: Isolated, can be destroyed/recreated per user
- **Contains**: CV files, projects, bash config, shell commands
- **Does**: Runs shell, executes user commands

## The "BusyBox" Confusion

**Alpine Linux uses BusyBox!**

When you use `alpine:latest`:
- It includes BusyBox utilities (`ls`, `cat`, `grep`, etc.)
- `/bin/sh` is BusyBox's shell
- You get all the standard Linux commands

So when you run `/bin/sh` in Alpine, you're running BusyBox shell!

## Why Pod-per-User Doesn't Work

You mentioned: **"pod-per-user doesn't work rn"**

### Common Issues:

1. **Kubernetes API Access**
   - Go app needs ServiceAccount with RBAC permissions
   - Check: `k8s/rbac.yaml` - does it have correct permissions?

2. **Image Availability**
   - Guest image (`portfolio-guest:latest`) must exist
   - Check: Is it built? Is it in the registry?

3. **Pod Creation**
   - NetworkPolicy might block
   - Resource quotas might prevent creation
   - Check: `kubectl get pods -n public-sessions`

4. **Exec Connection**
   - Pod must be in "Running" state
   - ServiceAccount needs `pods/exec` permission
   - Check: `kubectl exec` works manually?

## The Simpler Alternative (Process-per-User)

Instead of creating separate pods, you could:

### Single Container Approach:
```
┌─────────────────────────────────────────┐
│  Backend Pod                            │
│  ┌───────────────────────────────────┐  │
│  │  Go Web App                      │  │
│  │  - Handles WebSocket            │  │
│  │  - Spawns /bin/sh processes     │  │
│  └───────────────────────────────────┘  │
│  ┌───────────────────────────────────┐  │
│  │  CV files, projects/             │  │
│  │  (mounted from ConfigMap)        │  │
│  └───────────────────────────────────┘  │
│  ┌───────────────────────────────────┐  │
│  │  Shell Process 1 (User A)        │  │
│  │  Shell Process 2 (User B)        │  │
│  │  Shell Process 3 (User C)        │  │
│  └───────────────────────────────────┘  │
└─────────────────────────────────────────┘
```

**Benefits:**
- No Kubernetes API calls needed
- No pod creation/deletion
- Instant shell spawn (<50ms vs 3-9s)
- Simpler code

**Trade-off:**
- All users share same container (less isolation)

## Summary

**Your Current Setup:**
- ✅ Go app = Web server (handles HTTP/WebSocket)
- ✅ Guest pod = Terminal environment (has commands + files)
- ✅ Go app creates guest pod per user
- ✅ Go app executes shell in guest pod via `kubectl exec`
- ✅ Go app pipes WebSocket ↔ shell stdin/stdout

**Why It's Complex:**
- Requires Kubernetes API access
- Requires pod creation/deletion logic
- Requires RBAC setup
- Multiple failure points

**Why Process-per-User Is Simpler:**
- No Kubernetes API needed
- Just spawn processes in same container
- Same security (read-only FS, network policy)
- Much faster and simpler

The key insight: **You don't need separate pods for isolation if you're just showing a portfolio!** The read-only filesystem + network policy + resource limits provide sufficient security.



