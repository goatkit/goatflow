#!/bin/bash
# Build the stats WASM plugin
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

if ! command -v tinygo &> /dev/null; then
    echo "🐳 Building with Docker..."
    docker run --rm -v "$(pwd)":/src tinygo/tinygo:0.32.0 \
        tinygo build -o /src/stats.wasm -target wasi -no-debug -scheduler=none /src/main.go
else
    echo "🔨 Building with TinyGo..."
    tinygo build -o stats.wasm -target wasi -no-debug -scheduler=none main.go
fi

echo "✅ Built: stats.wasm ($(wc -c < stats.wasm | tr -d ' ') bytes)"
