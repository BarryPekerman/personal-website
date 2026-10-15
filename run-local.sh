#!/bin/bash
# Run the portfolio terminal server in local mode

export APP_MODE=local
export CONTENT_PATH=./content

echo "Starting portfolio terminal server in LOCAL mode..."
echo "Content path: $CONTENT_PATH"
echo "Open http://localhost:8080 in your browser"
echo ""

go run cmd/server/main.go



