#!/bin/sh
# Toolbox container entrypoint - validates cache permissions before running commands
# This script runs as the container user (1000:1000), NOT as root

set -e

CACHE_DIR="/cache"
CACHE_DIRS="$CACHE_DIR $CACHE_DIR/go-build $CACHE_DIR/go-mod $CACHE_DIR/xdg $CACHE_DIR/xdg/helm $CACHE_DIR/bun $CACHE_DIR/golangci-lint $CACHE_DIR/tmp"

# Check if we can write to cache directories
check_cache_permissions() {
    for dir in $CACHE_DIRS; do
        if [ -d "$dir" ]; then
            # Try to create a test file
            if ! touch "$dir/.write-test" 2>/dev/null; then
                return 1
            fi
            rm -f "$dir/.write-test" 2>/dev/null || true
        fi
    done
    return 0
}

# Create cache directories if they don't exist (as current user)
ensure_cache_dirs() {
    for dir in $CACHE_DIRS; do
        mkdir -p "$dir" 2>/dev/null || true
    done
    # Helm-specific subdirs
    mkdir -p "$CACHE_DIR/xdg/helm/repository" "$CACHE_DIR/xdg/helm/cache" "$CACHE_DIR/xdg/helm/config" 2>/dev/null || true
}

# Main logic
if [ -d "$CACHE_DIR" ]; then
    if ! check_cache_permissions; then
        echo "============================================================" >&2
        echo "ERROR: Cache directory has incorrect permissions" >&2
        echo "============================================================" >&2
        echo "" >&2
        echo "The cache volume contains root-owned files that prevent" >&2
        echo "the toolbox from operating correctly." >&2
        echo "" >&2
        echo "To fix this, run:" >&2
        echo "  make cache-fix" >&2
        echo "" >&2
        echo "Or recreate the volume:" >&2
        echo "  docker volume rm gotrs-ce_gotrs_cache" >&2
        echo "============================================================" >&2
        exit 1
    fi
fi

# Ensure all cache directories exist
ensure_cache_dirs

# Execute the command passed to the container
exec "$@"
