# Cleaning Up Old Docker Containers

If you see leftover Docker containers from previous runs, you can clean them up:

## Check for Old Containers

```bash
docker ps -a | grep alpine
```

## Stop and Remove (if you have permissions)

```bash
# Stop all running alpine containers
docker ps -q --filter ancestor=alpine:latest | xargs -r docker stop

# Remove all stopped alpine containers
docker ps -aq --filter ancestor=alpine:latest | xargs -r docker rm
```

## Note

The **current implementation does NOT use Docker**. It uses local `/bin/sh` directly via PTY. Any Docker containers you see are from old code versions and can be safely ignored or removed.

If you don't have Docker permissions, you can:
1. Ask your system admin to add you to the `docker` group: `sudo usermod -aG docker $USER` (then log out/in)
2. Or just ignore the old containers - they won't affect the new code



