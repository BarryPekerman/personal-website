# Real Shell Implementation with Docker Compose

This implementation provides a **real shell** experience using Docker containers. Each user session gets a persistent shell process running in an isolated Docker container.

## Architecture

```
Browser (xterm.js)
  ↓ WebSocket
Go Backend
  ↓ Docker API
Docker Container (Alpine Linux)
  ↓ /bin/sh -l (persistent shell process)
Shell maintains all state internally
```

## Features

✅ **Real Persistent Shell** - Each session has a `/bin/sh` process that stays alive
✅ **Full Shell Features** - Job control, aliases, functions, all shell features work
✅ **Environment Persistence** - Variables, directory, history all maintained by shell
✅ **Isolated Containers** - Each user gets their own container
✅ **Real-time Streaming** - WebSocket streams directly to/from shell stdin/stdout

## Prerequisites

1. **Docker** installed and running
2. **Docker Compose** (usually comes with Docker)
3. **Go 1.21+** (for building)

## Setup

### 1. Install Dependencies

```bash
# Copy the go.mod file
cp go.mod.real-shell go.mod

# Download dependencies
go mod download
```

### 2. Build and Run with Docker Compose

```bash
# Build and start
docker-compose up --build

# Or run in background
docker-compose up -d --build
```

### 3. Access the Terminal

Open `http://localhost:8080` in your browser.

## Manual Setup (Without Docker Compose)

If you prefer to run manually:

```bash
# 1. Make sure Docker is running
docker ps

# 2. Build the Go binary
go build -o real-shell-server real-shell-main.go

# 3. Run it (needs Docker socket access)
sudo ./real-shell-server
# Or if your user is in docker group:
./real-shell-server
```

## How It Works

### 1. Container Creation
- When a user connects, a new Alpine Linux container is created
- Container runs `/bin/sh -l` (login shell) which stays alive
- Content directory is mounted read-only into `/app/content`

### 2. Shell Attachment
- Go backend attaches to the container's shell via Docker Exec API
- Creates a TTY (pseudo-terminal) for interactive shell
- Streams stdin/stdout between WebSocket and shell

### 3. Real Shell Features
- **Environment Variables**: `export VAR=value` persists
- **Directory Navigation**: `cd` works, state maintained
- **Job Control**: Background processes (`&`), `fg`, `bg` work
- **Aliases**: `alias ll='ls -l'` works
- **Functions**: Shell functions work
- **History**: Command history maintained by shell
- **Pipes**: `cmd1 | cmd2` works with process groups

### 4. Cleanup
- When user disconnects, container is automatically removed
- Uses `AutoRemove: true` so containers clean up on stop

## Comparison: Pseudo Shell vs Real Shell

| Feature | Pseudo Shell (mvp-main.go) | Real Shell (real-shell-main.go) |
|---------|---------------------------|----------------------------------|
| Shell Process | New process per command | Persistent process per session |
| Environment | Maintained in Go memory | Maintained by shell process |
| Job Control | ❌ No | ✅ Yes (`&`, `fg`, `bg`) |
| Aliases | ❌ No | ✅ Yes |
| Functions | ❌ No | ✅ Yes |
| History | Basic (Go memory) | Full (shell history) |
| Pipes | Basic | Full process groups |
| Complexity | Simple | More complex |
| Resource Usage | Low (no idle processes) | Higher (idle shells) |

## Security Considerations

⚠️ **Important**: This gives users real shell access in containers!

- Containers are isolated from host
- Content directory mounted read-only
- Containers auto-remove on disconnect
- Consider adding:
  - Resource limits (CPU/memory)
  - Network restrictions
  - Command filtering
  - Timeout/expiration

## Troubleshooting

### "Docker client not available"
- Make sure Docker is running: `docker ps`
- Check Docker socket permissions: `ls -l /var/run/docker.sock`
- Add user to docker group: `sudo usermod -aG docker $USER` (then logout/login)

### "Failed to create container"
- Check Docker daemon is accessible
- Verify content directory exists
- Check disk space: `df -h`

### WebSocket connection fails
- Check firewall settings
- Verify port 8080 is accessible
- Check browser console for errors

## Files

- `real-shell-main.go` - Main server code with Docker integration
- `docker-compose.yml` - Docker Compose configuration
- `Dockerfile.real-shell` - Docker image for the backend
- `go.mod.real-shell` - Go dependencies

## Next Steps

To make this production-ready:
1. Add resource limits to containers
2. Implement session timeouts
3. Add command filtering/whitelisting
4. Set up proper logging
5. Add monitoring/metrics
6. Implement rate limiting


