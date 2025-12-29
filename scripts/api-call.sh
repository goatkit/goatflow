#!/bin/bash
set -euo pipefail

# Load .env for BACKEND_URL and ADMIN creds if present
if [[ -f ".env" ]]; then
  while IFS='=' read -r key value; do
    [[ -z "$key" || "$key" =~ ^[[:space:]]*# ]] && continue
    value=$(echo "$value" | sed 's/^"\(.*\)"$/\1/' | sed "s/^'\(.*\)'$/\1/")
    export "$key=$value"
  done < .env
fi

BACKEND_URL="${BACKEND_URL:-http://localhost:8080}"
LOGIN="${LOGIN:-${ADMIN_USER:-root@localhost}}"
PASSWORD="${PASSWORD:-${ADMIN_PASSWORD:-}}"

if [ -z "$PASSWORD" ]; then
  echo "ERROR: PASSWORD or ADMIN_PASSWORD must be set in .env" >&2
  exit 1
fi

# Inputs are via env for consistency with http-call.sh
METHOD="${METHOD:-GET}"
ENDPOINT="${ENDPOINT:-/}" 
BODY="${BODY:-}"
CONTENT_TYPE="${CONTENT_TYPE:-application/json}"

# Authenticate to get token
payload=$(jq -nc --arg l "$LOGIN" --arg p "$PASSWORD" '{login:$l,password:$p}')
resp=$(curl -sk -w '\n%{http_code}' -X POST "$BACKEND_URL/api/auth/login" -H 'Content-Type: application/json' -H 'Accept: application/json' -d "$payload" || true)
body="${resp%$'\n'*}"; code="${resp##*$'\n'}"
if [[ "$code" != "200" ]]; then
  echo "auth failed ($code)" 1>&2
  echo "$body" 1>&2
  exit 1
fi
AUTH_TOKEN=$(echo "$body" | jq -r '.access_token // .token // empty')
if [[ -z "$AUTH_TOKEN" || "$AUTH_TOKEN" == "null" ]]; then
  echo "auth response missing token" 1>&2
  echo "$body" 1>&2
  exit 1
fi

# Delegate to http-call with token
METHOD="$METHOD" ENDPOINT="$ENDPOINT" BODY="$BODY" CONTENT_TYPE="$CONTENT_TYPE" BACKEND_URL="$BACKEND_URL" AUTH_TOKEN="$AUTH_TOKEN" \
  "$(dirname "$0")/http-call.sh"
