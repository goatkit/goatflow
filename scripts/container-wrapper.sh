#!/bin/bash
# Container runtime wrapper - auto-detects docker or podman

# Detect container runtime
if command -v podman &> /dev/null; then
    CONTAINER_CMD="podman"
    if command -v podman-compose &> /dev/null; then
        COMPOSE_CMD="podman-compose"
    elif podman compose version &> /dev/null 2>&1; then
        COMPOSE_CMD="podman compose"
    else
        echo "Error: podman found but no compose command available" >&2
        exit 1
    fi
elif command -v docker &> /dev/null; then
    CONTAINER_CMD="docker"
    if docker compose version &> /dev/null 2>&1; then
        COMPOSE_CMD="docker compose"
    elif command -v docker-compose &> /dev/null; then
        COMPOSE_CMD="docker-compose"
    else
        echo "Error: docker found but no compose command available" >&2
        exit 1
    fi
else
    echo "Error: Neither docker nor podman found" >&2
    exit 1
fi

# Export for use by other scripts
export CONTAINER_CMD
export COMPOSE_CMD

# Only execute if this script is run directly, not sourced
if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
    # If called with arguments, execute them
    if [ $# -gt 0 ]; then
        if [ "$1" = "compose" ]; then
            shift
            exec $COMPOSE_CMD "$@"
        else
            exec $CONTAINER_CMD "$@"
        fi
    fi
    
    # Otherwise just print the detected commands
    echo "Container: $CONTAINER_CMD"
    echo "Compose: $COMPOSE_CMD"
fi