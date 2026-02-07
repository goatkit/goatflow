#!/bin/bash
# GoatFlow Compose Wrapper Script
# Automatically detects and uses the right compose command
# Supports: docker compose, docker-compose, podman compose, podman-compose

# Function to detect the best compose command
detect_compose_command() {
    # Check for podman-compose first (podman users typically prefer it)
    if command -v podman-compose > /dev/null 2>&1; then
        echo "podman-compose"
        return 0
    fi
    
    # Check for podman compose plugin
    if command -v podman > /dev/null 2>&1 && podman compose version > /dev/null 2>&1; then
        echo "podman compose"
        return 0
    fi
    
    # Check for docker compose plugin (modern Docker)
    if command -v docker > /dev/null 2>&1 && docker compose version > /dev/null 2>&1; then
        echo "docker compose"
        return 0
    fi
    
    # Check for docker-compose standalone (legacy)
    if command -v docker-compose > /dev/null 2>&1; then
        echo "docker-compose"
        return 0
    fi
    
    # If nothing found, return error
    echo >&2 "Error: No Docker/Podman compose command found!"
    echo >&2 "Please install one of:"
    echo >&2 "  - Docker with 'docker compose' plugin (recommended)"
    echo >&2 "  - docker-compose standalone"
    echo >&2 "  - Podman with 'podman compose' plugin"
    echo >&2 "  - podman-compose"
    exit 1
}

# Main execution
COMPOSE_CMD=$(detect_compose_command)

# Show which command is being used (only if verbose or no args)
if [ "$1" = "--verbose" ] || [ "$1" = "-v" ] || [ $# -eq 0 ]; then
    echo "Using: $COMPOSE_CMD" >&2
    if [ "$1" = "--verbose" ] || [ "$1" = "-v" ]; then
        shift  # Remove the verbose flag
    fi
fi

# If no arguments, show help
if [ $# -eq 0 ]; then
    echo "GoatFlow Compose Wrapper"
    echo ""
    echo "Usage: ./compose.sh [compose-command] [args...]"
    echo ""
    echo "Examples:"
    echo "  ./compose.sh up           # Start all services"
    echo "  ./compose.sh down         # Stop all services"
    echo "  ./compose.sh logs -f      # Follow logs"
    echo "  ./compose.sh exec backend sh  # Shell into backend"
    echo ""
    echo "This wrapper automatically uses the best available compose command."
    exit 0
fi

# Execute the compose command with all passed arguments (support multi-word command)
if echo "$COMPOSE_CMD" | grep -q ' '; then
    # Split into array so multi-word command executes properly
    IFS=' ' read -r -a _cmd_parts <<< "$COMPOSE_CMD"
    exec "${_cmd_parts[@]}" "$@"
else
    exec "$COMPOSE_CMD" "$@"
fi