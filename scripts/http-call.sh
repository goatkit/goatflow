#!/bin/bash
set -euo pipefail

# Load .env to infer BACKEND_URL if present
if [[ -f ".env" ]]; then
  while IFS='=' read -r key value; do
    [[ -z "$key" || "$key" =~ ^[[:space:]]*# ]] && continue
    value=$(echo "$value" | sed 's/^"\(.*\)"$/\1/' | sed "s/^'\(.*\)'$/\1/")
    export "$key=$value"
  done < .env
fi

# Prefer environment variables, fall back to positional args
METHOD="${METHOD:-${1:-GET}}"
ENDPOINT="${ENDPOINT:-${2:-/}}"
BODY="${BODY:-${3:-}}"
CONTENT_TYPE="${CONTENT_TYPE:-${4:-text/html}}"

if [[ -z "${BACKEND_URL:-}" ]]; then
  for host in gotrs-backend backend gotrs-ce-backend-1; do
    if getent hosts "$host" >/dev/null 2>&1; then
      BACKEND_URL="http://$host:8080"; break
    fi
  done
  BACKEND_URL="${BACKEND_URL:-http://localhost:8080}"
fi

# Cookie jar for session
COOKIE_JAR=$(mktemp)
trap "rm -f $COOKIE_JAR" EXIT

# If LOGIN and PASSWORD are set, authenticate first (use JSON API for token)
if [[ -n "${LOGIN:-}" && -n "${PASSWORD:-}" && -z "${AUTH_TOKEN:-}" ]]; then
  # Perform JSON login to get access_token
  login_output=$(curl -k -s \
    -X POST "$BACKEND_URL/api/auth/login" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json" \
    -d "{\"username\":\"${LOGIN}\",\"password\":\"${PASSWORD}\"}")

  # Extract access_token from JSON response
  AUTH_TOKEN=$(echo "$login_output" | grep -oE '"access_token"\s*:\s*"[^"]+"' | sed 's/.*"access_token"\s*:\s*"\([^"]*\)".*/\1/')

  if [[ -z "$AUTH_TOKEN" ]]; then
    echo "❌ Login failed: invalid credentials"
    echo "   Check ADMIN_USER and ADMIN_PASSWORD in your .env file"
    exit 1
  fi
fi

# Build curl args
args=(-k -i -s -X "$METHOD" "$BACKEND_URL$ENDPOINT" -H "Accept: $CONTENT_TYPE")

# Use cookie jar if we logged in, or Bearer token if provided
if [[ -n "${LOGIN:-}" && -n "${PASSWORD:-}" && -z "${AUTH_TOKEN:-}" ]]; then
  args+=(-b "$COOKIE_JAR" -c "$COOKIE_JAR")
elif [[ -n "${AUTH_TOKEN:-}" ]]; then
  args+=(-H "Authorization: Bearer $AUTH_TOKEN")
fi

if [[ -n "$BODY" ]]; then
  args+=(-H "Content-Type: $CONTENT_TYPE" -d "$BODY")
fi

curl "${args[@]}" | sed -n '1,200p'
