# Local Development Mode

This project supports two modes of operation:

1. **Kubernetes Mode** (Production): Creates isolated pods per user session
2. **Local Mode** (Development): Uses Docker containers locally for fast iteration

## Why Local Mode?

Local mode allows you to:
- ✅ Develop without a Kubernetes cluster running
- ✅ Test WebSocket and terminal emulation logic instantly
- ✅ Iterate on CSS/frontend changes without pod scheduling delays
- ✅ Work from anywhere (coffee shop, airplane) without cluster access
- ✅ Debug faster with immediate feedback

## Architecture

Local mode uses the **Interface Pattern** in Go to swap the "Engine" without changing frontend or WebSocket handling code:

```
Browser (xterm.js)
    ↓ WebSocket
Go App (main.go)
    ↓ PTY (github.com/creack/pty)
Docker Container (alpine:latest)
    ↓ /bin/sh
Shell Process
```

The same `shell.Session` interface is used for both modes, allowing seamless switching.

## Prerequisites

1. **Docker** installed and running
2. **Go 1.21+** installed
3. **Portfolio content** in `./content/` directory:
   ```
   content/
   ├── cv.txt
   ├── contact_info.md
   └── projects/
       └── ...
   ```

## Running in Local Mode

### Option 1: Using the Run Script (Easiest)

```bash
./run-local.sh
```

### Option 2: Environment Variable

```bash
export APP_MODE=local
export CONTENT_PATH=./content  # Optional, defaults to ./content
go run cmd/server/main.go
```

### Option 3: Inline

```bash
APP_MODE=local CONTENT_PATH=./content go run cmd/server/main.go
```

**Note**: You must specify `cmd/server/main.go` (not just `cmd/server`) because there are multiple main files in that directory.

## How It Works

### Docker Container Setup

Local mode runs:
```bash
docker run \
  --rm \                    # Remove container on exit
  -it \                     # Interactive TTY
  --network=none \          # No network (matches NetworkPolicy)
  --read-only \             # Read-only root filesystem (matches prod)
  --tmpfs /tmp:rw \         # Writable /tmp (matches prod)
  --user 1000:1000 \        # Non-root user (matches prod)
  -v $(pwd)/content:/home/guest:ro \  # Mount content read-only
  -w /home/guest \          # Working directory
  alpine:latest \
  /bin/sh -l                # Login shell
```

This closely mimics the production Kubernetes environment:
- Same base image (Alpine)
- Same security constraints (read-only FS, no network, non-root)
- Same content mounting (your local `content/` directory)

### PTY (Pseudo-Terminal)

The `github.com/creack/pty` library wraps the Docker command in a pseudo-terminal, providing:
- ✅ Tab completion
- ✅ Color output (ANSI sequences)
- ✅ Interactive programs (vi, top, etc.)
- ✅ Proper terminal behavior

Without PTY, you'd lose these interactive features.

## Switching Between Modes

### Local Mode → Kubernetes Mode

```bash
# Unset or remove APP_MODE
unset APP_MODE
go run cmd/server/main.go
```

### Kubernetes Mode → Local Mode

```bash
export APP_MODE=local
go run cmd/server/main.go
```

The code automatically detects the mode and uses the appropriate implementation.

## Development Workflow

### 1. Start Local Mode

```bash
APP_MODE=local go run cmd/server/main.go
```

### 2. Open Browser

Navigate to `http://localhost:8080`

### 3. Test Changes

- Edit `web/static/css/style.css` → Refresh browser (instant)
- Edit `web/templates/index.html` → Refresh browser (instant)
- Edit `internal/handlers/terminal.go` → Restart server (fast, no K8s overhead)

### 4. Test Terminal Features

```bash
# In the terminal:
ls                    # Should show cv.txt, projects/
cat cv.txt           # Should display your CV
cat projects/...     # Should display project files
```

## Troubleshooting

### Docker Not Running

**Error**: `exec: "docker": executable file not found`

**Solution**: Start Docker daemon
```bash
# Linux
sudo systemctl start docker

# macOS
open -a Docker
```

### Content Directory Not Found

**Error**: `failed to resolve content path`

**Solution**: Ensure `content/` directory exists
```bash
ls -la content/
# Should show: cv.txt, contact_info.md, projects/
```

### Permission Denied

**Error**: Docker permission denied

**Solution**: Add user to docker group or use sudo
```bash
# Add to docker group (Linux)
sudo usermod -aG docker $USER
# Log out and back in

# Or use sudo (not recommended)
sudo docker ...
```

### Port Already in Use

**Error**: `bind: address already in use`

**Solution**: Change port or kill existing process
```bash
# Find process using port 8080
lsof -i :8080
# Kill it
kill -9 <PID>

# Or change port in main.go
```

## Comparison: Local vs Kubernetes Mode

| Feature | Local Mode | Kubernetes Mode |
|---------|------------|-----------------|
| **Startup Time** | <1 second | 3-9 seconds |
| **Requires K8s** | ❌ No | ✅ Yes |
| **Isolation** | Container-level | Pod-level |
| **Network** | None (--network=none) | NetworkPolicy |
| **Filesystem** | Read-only + /tmp | Read-only + /tmp |
| **User** | 1000:1000 | 1000:1000 |
| **Best For** | Development | Production |

## Code Structure

```
internal/
├── shell/
│   ├── interface.go      # Session and Manager interfaces
│   ├── local.go          # Local Docker implementation
│   └── k8s.go            # Kubernetes implementation
├── handlers/
│   └── terminal.go      # WebSocket handler (uses interface)
└── k8s/
    └── client.go         # Kubernetes client (used by k8s.go)
```

The handler code (`terminal.go`) doesn't know or care which implementation is used - it just works with the `shell.Session` interface.

## Benefits of Interface Pattern

1. **Testability**: Easy to mock for unit tests
2. **Flexibility**: Swap implementations without changing handler code
3. **Maintainability**: Clear separation of concerns
4. **Professional**: Demonstrates senior-level Go design patterns

## Next Steps

1. **Try Local Mode**: `APP_MODE=local go run cmd/server/main.go`
2. **Test Terminal**: Open `http://localhost:8080` and try commands
3. **Iterate Fast**: Make CSS/JS changes and see them instantly
4. **Switch to K8s**: When ready, test in Kubernetes mode

---

**Happy Coding!** 🚀

