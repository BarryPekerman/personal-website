# Quick Start: Real Shell with Docker Compose

## Prerequisites

1. **Docker** installed and running
   ```bash
   docker ps  # Should work without errors
   ```

2. **Go 1.21+** (for building)

## Quick Start

### Option 1: Docker Compose (Recommended)

```bash
# 1. Copy go.mod
cp go.mod.real-shell go.mod

# 2. Start everything
docker-compose up --build

# 3. Open browser
# http://localhost:8080
```

### Option 2: Manual Build

```bash
# 1. Install dependencies
cp go.mod.real-shell go.mod
go mod download

# 2. Build
go build -o real-shell-server real-shell-main.go

# 3. Run (needs Docker socket access)
sudo ./real-shell-server
# Or if in docker group:
./real-shell-server

# 4. Open browser
# http://localhost:8080
```

## What You Get

✅ **Real Persistent Shell** - Each session has a `/bin/sh` process that stays alive
✅ **Full Shell Features** - Everything works: aliases, functions, job control, etc.
✅ **Isolated Containers** - Each user gets their own Docker container
✅ **Real-time Terminal** - xterm.js frontend with WebSocket streaming

## Test It

Try these commands to verify it's a real shell:

```bash
# Environment variables persist
export TEST=123
echo $TEST  # Should show: 123

# Aliases work
alias ll='ls -l'
ll  # Should work

# Job control works
sleep 10 &  # Background process
jobs  # Should show the job
fg    # Bring to foreground

# History works
history  # Shows command history
```

## Troubleshooting

**"Docker client not available"**
- Make sure Docker is running: `docker ps`
- Check permissions: `ls -l /var/run/docker.sock`
- Add to docker group: `sudo usermod -aG docker $USER` (logout/login)

**Port 8080 already in use**
- Change port in `docker-compose.yml` or `real-shell-main.go`

**Container creation fails**
- Check Docker daemon: `docker info`
- Check disk space: `df -h`

## Files

- `real-shell-main.go` - Real shell server
- `docker-compose.yml` - Docker Compose config
- `Dockerfile.real-shell` - Backend Docker image
- `go.mod.real-shell` - Go dependencies


