#!/bin/bash
set -euo pipefail

USERNAME="${1:-}"
PASSWORD="${2:-}"

if [ -z "$USERNAME" ] || [ -z "$PASSWORD" ]; then
    echo "Usage: $0 <username> <password>"
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

DB_HOST="${DB_CONN_HOST:-${DB_HOST:-postgres}}"
DB_PORT="${DB_CONN_PORT:-${DB_PORT:-5432}}"
DB_NAME="${DB_CONN_NAME:-${DB_NAME:-goatflow}}"
DB_USER="${DB_CONN_USER:-${DB_USER:-goatflow_user}}"
DB_PASSWORD="${DB_CONN_PASSWORD:-${DB_PASSWORD:-}}"

if [ -z "$DB_PASSWORD" ]; then
  echo "ERROR: DB_PASSWORD must be set in .env" >&2
  exit 1
fi

TOOLBOX_IMAGE="${TOOLBOX_IMAGE:-ghcr.io/goatkit/goatflow/toolbox:latest}"
NETWORK="${DB_CONTAINER_NETWORK:-goatflow_goatflow-network}"

uid=$(id -u)
gid=$(id -g)

"$REPO_ROOT/scripts/container-wrapper.sh" run --rm \
    -v "$REPO_ROOT:/workspace" \
    -w /workspace \
    -u "$uid:$gid" \
    --network "$NETWORK" \
    -e DB_DRIVER=postgres \
    -e DB_HOST="$DB_HOST" \
    -e DB_PORT="$DB_PORT" \
    -e DB_NAME="$DB_NAME" \
    -e DB_USER="$DB_USER" \
    -e DB_PASSWORD="$DB_PASSWORD" \
    "$TOOLBOX_IMAGE" \
    goatflow reset-user --username="$USERNAME" --password="$PASSWORD" --enable
