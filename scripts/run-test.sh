#!/bin/bash
# Run tests using the appropriate container runtime (docker or podman)

# Detect container runtime (same logic as Makefile)
CONTAINER_CMD=$(command -v podman 2> /dev/null || command -v docker 2> /dev/null || echo docker)

# Get current directory
WORKSPACE_DIR=$(pwd)

# Run the test
exec $CONTAINER_CMD run --rm \
    -v "$WORKSPACE_DIR:/workspace" \
    -w /workspace \
    gotrs-toolbox:latest \
    go test -v ./internal/api/ "$@"