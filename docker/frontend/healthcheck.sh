#!/bin/sh
# Customer frontend healthcheck script
# Uses APP_PORT environment variable set by container (defaults to 8080)

PORT="${APP_PORT:-8080}"
wget --no-verbose --tries=1 --spider "http://localhost:${PORT}/health" || exit 1
