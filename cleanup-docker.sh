#!/bin/bash
# Restart Docker daemon and clean up old containers

echo "Restarting Docker daemon..."
sudo systemctl restart docker

echo "Waiting for Docker to start..."
sleep 3

echo "Checking Docker status..."
sudo systemctl status docker --no-pager | head -5

echo ""
echo "Cleaning up old Alpine containers..."
docker ps -aq --filter ancestor=alpine:latest | xargs -r docker rm -f 2>/dev/null || echo "No containers to remove or permission denied"

echo ""
echo "Verifying cleanup..."
docker ps -a | grep alpine || echo "No Alpine containers found"

echo ""
echo "Done! Docker daemon restarted and containers cleaned."



