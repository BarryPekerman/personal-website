#!/bin/bash
# Run the portfolio terminal locally
set -euo pipefail
cd "$(dirname "$0")"

echo "Starting on http://localhost:${PORT:-8080}"
go run ./cmd/server
