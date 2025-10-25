#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

if [ -n "${CONTAINER_CMD:-}" ]; then
    read -r -a CONTAINER_BIN <<<"$CONTAINER_CMD"
else
    if command -v podman >/dev/null 2>&1; then
        CONTAINER_BIN=(podman)
    elif command -v docker >/dev/null 2>&1; then
        CONTAINER_BIN=(docker)
    else
        echo "podman or docker is required" >&2
        exit 1
    fi
fi

if [ -n "${COMPOSE_CMD:-}" ]; then
    read -r -a COMPOSE_BIN <<<"$COMPOSE_CMD"
else
    if command -v docker-compose >/dev/null 2>&1; then
        COMPOSE_BIN=($(command -v docker-compose))
    elif command -v podman-compose >/dev/null 2>&1; then
        COMPOSE_BIN=($(command -v podman-compose))
    else
        COMPOSE_BIN=(docker compose)
    fi
fi

case "${1:-}" in
    exec)
        shift
        "${CONTAINER_BIN[@]}" exec "$@"
        ;;
    run)
        shift
        "${CONTAINER_BIN[@]}" run "$@"
        ;;
    compose)
        shift
        "${COMPOSE_BIN[@]}" "$@"
        ;;
    logs)
        shift
        "${COMPOSE_BIN[@]}" logs "$@"
        ;;
    *)
        "${CONTAINER_BIN[@]}" "$@"
        ;;
esac
