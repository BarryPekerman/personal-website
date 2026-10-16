#!/bin/bash
# Run the portfolio terminal locally. APP_MODE defaults to the safe virtual
# shell; APP_MODE=local gives a real shell on this machine (never expose it).
set -euo pipefail
cd "$(dirname "$0")"

export APP_MODE="${APP_MODE:-virtual}"
echo "Starting in ${APP_MODE} mode on http://localhost:${PORT:-8080}"
go run ./cmd/server
