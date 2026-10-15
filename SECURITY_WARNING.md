# ⚠️ SECURITY WARNING: Local Mode

## Important Security Notice

**Local Mode (`APP_MODE=local`) runs commands directly on your host system.**

This means:
- ❌ **NO ISOLATION**: Commands run with your user's permissions
- ❌ **NO SANDBOXING**: Can access your entire filesystem
- ❌ **NO CONTAINER**: Not running in Docker/Kubernetes isolation
- ⚠️ **DANGEROUS**: Commands like `sudo rm /` will attempt to run on your actual system

## What Local Mode Does

Local mode uses `os/exec` to spawn a shell (`/bin/bash` or `/bin/sh`) directly on your machine:
- Working directory: `./content` (or `CONTENT_PATH`)
- User: Your actual system user
- Permissions: Your actual user permissions
- Filesystem: Your actual filesystem

## When to Use Local Mode

✅ **Safe for:**
- Development and testing
- Frontend/UI work
- WebSocket debugging
- Terminal emulation testing

❌ **NOT safe for:**
- Production deployment
- Public-facing servers
- Running untrusted code
- Demonstrating security features

## Production Mode

For production, use **Kubernetes mode** (default):
- Each session runs in an isolated pod
- Network policies prevent access to your cluster
- Resource limits prevent abuse
- Pod Security Standards enforce restrictions

## Recommendation

**Only use Local Mode for development on your own machine. Never expose it publicly.**



