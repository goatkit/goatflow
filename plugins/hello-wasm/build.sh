#!/bin/bash
# Build the hello-wasm plugin
# Requires TinyGo: https://tinygo.org/getting-started/install/

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Check for TinyGo
if ! command -v tinygo &> /dev/null; then
    echo "❌ TinyGo not found. Install from: https://tinygo.org/getting-started/install/"
    echo ""
    echo "Quick install (Linux):"
    echo "  wget https://github.com/tinygo-org/tinygo/releases/download/v0.32.0/tinygo_0.32.0_amd64.deb"
    echo "  sudo dpkg -i tinygo_0.32.0_amd64.deb"
    echo ""
    echo "Or via Docker:"
    echo "  docker run --rm -v \$(pwd):/src tinygo/tinygo:0.32.0 tinygo build -o /src/hello.wasm -target wasi -no-debug /src/main.go"
    exit 1
fi

echo "🔨 Building hello-wasm plugin..."
tinygo build -o hello.wasm -target wasi -no-debug -scheduler=none main.go

echo "✅ Built: hello.wasm ($(wc -c < hello.wasm | tr -d ' ') bytes)"
echo ""
echo "To install, copy to your plugins directory:"
echo "  cp hello.wasm /app/config/plugins/"
