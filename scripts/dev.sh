#!/bin/bash
set -e

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_ROOT"

export GOCACHE="$PROJECT_ROOT/.gocache"
export GOMODCACHE="$PROJECT_ROOT/.gomodcache"
export GOFLAGS="-buildvcs=false"
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"

mkdir -p "$GOCACHE" "$GOMODCACHE"

HTTP_ADDR="${HTTP_ADDR:-:8081}"
HTTP_PORT="${HTTP_ADDR##*:}"

if lsof -i :"$HTTP_PORT" -sTCP:LISTEN -t >/dev/null 2>&1; then
    echo "Error: Port $HTTP_PORT is already in use."
    exit 1
fi

echo "Using go: $(which go)"
echo "GOCACHE: $GOCACHE"
echo "GOMODCACHE: $GOMODCACHE"
echo "HTTP port: $HTTP_PORT"

if [ "$1" != "--no-worker" ]; then
    echo "Starting worker..."
    go run ./cmd/worker &
    WORKER_PID=$!
    trap "kill $WORKER_PID 2>/dev/null || true" EXIT
fi

go run ./cmd/api
