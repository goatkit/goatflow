#!/bin/sh
# Toolbox container entrypoint - validates cache writability before running commands
# This script runs as the container user (1000:1000), NOT as root

set -e

CACHE_DIR="/workspace"

# Cache directories matching Dockerfile.toolbox ENV vars (GOCACHE, GOMODCACHE, etc.)
CACHE_DIRS="$CACHE_DIR/.go-build $CACHE_DIR/.gomodcache $CACHE_DIR/.golangci-lint $CACHE_DIR/.bun $CACHE_DIR/.xdg-cache/helm/repository $CACHE_DIR/.xdg-cache/helm/cache $CACHE_DIR/.xdg-cache/helm/config"

# Ensure cache directories exist and are writable (ownership doesn't matter)
ensure_cache_writable() {
    mkdir -p "$CACHE_DIRS" 2>/dev/null || true
    
    for dir in $CACHE_DIRS; do
        if [ -d "$dir" ]; then
            # Just check we can write, don't care about ownership
            touch "$dir/.write-test" 2>/dev/null && \
                rm -f "$dir/.write-test" 2>/dev/null || true
        fi
    done
}

ensure_cache_writable

# Execute the command passed to the container
exec "$@"
