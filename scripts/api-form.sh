#!/bin/bash
# GoatFlow API Testing Script (form-urlencoded)
# Authenticates with JSON login, then sends application/x-www-form-urlencoded body.

set -euo pipefail

# Load environment variables from .env if it exists
if [[ -f ".env" ]]; then
    while IFS='=' read -r key value; do
        [[ -z "$key" || "$key" =~ ^[[:space:]]*# ]] && continue
        value=$(echo "$value" | sed 's/^"\(.*\)"$/\1/' | sed "s/^'\(.*\)'$/\1/")
        export "$key=$value"
    done < .env
fi

# Prefer ADMIN_USER/ADMIN_PASSWORD; fall back to TEST_USERNAME/TEST_PASSWORD
TEST_USERNAME="${ADMIN_USER:-${TEST_USERNAME:-root@localhost}}"
TEST_PASSWORD="${ADMIN_PASSWORD:-${TEST_PASSWORD:-}}"

if [ -z "$TEST_PASSWORD" ]; then
    echo "ERROR: ADMIN_PASSWORD or TEST_PASSWORD must be set in .env" >&2
    exit 1
fi

# Infer BACKEND_URL if not set
if [[ -z "${BACKEND_URL:-}" ]]; then
    if getent hosts goatflow-backend >/dev/null 2>&1; then
        BACKEND_URL="https://goatflow-backend:8080"
    else
        BACKEND_URL="http://localhost:8080"
    fi
fi

auth_json() {
    local payload
    payload=$(jq -nc --arg l "$TEST_USERNAME" --arg p "$TEST_PASSWORD" '{login:$l,password:$p}')
    local endpoints=("/api/auth/login" "/api/v1/auth/login")
    local e response body http_code token
    for e in "${endpoints[@]}"; do
        response=$(curl -k -s -w '\n%{http_code}' -X POST "$BACKEND_URL$e" -H 'Content-Type: application/json' -H 'Accept: application/json' -d "$payload" || true)
        body="${response%$'\n'*}"; http_code="${response##*$'\n'}"
        if [[ "${GOATFLOW_DEBUG:-}" == "1" || "${VERBOSE:-}" == "1" ]]; then
            echo "DEBUG: (POST $e) HTTP $http_code body: $body" >&2
        fi
        if [[ "$http_code" != "200" ]]; then continue; fi
        token=$(echo "$body" | jq -r '.access_token // .token // empty')
        if [[ -n "$token" && "$token" != "null" ]]; then
            echo "$token"
            return 0
        fi
    done
    return 1
}

if [[ $# -lt 2 ]]; then
    echo "Usage: $0 <METHOD> <ENDPOINT> [BODY]" >&2
    exit 1
fi

METHOD="$1"; ENDPOINT="$2"; BODY="${3:-}"

token=$(auth_json)
if [[ -z "$token" ]]; then
    echo "ERROR: auth failed" >&2
    exit 1
fi

args=(-k -s -w '\n%{http_code}' -X "$METHOD" "$BACKEND_URL$ENDPOINT" \
      -H "Authorization: Bearer $token" -H 'Accept: application/json' \
      -H 'Content-Type: application/x-www-form-urlencoded')
if [[ -n "$BODY" ]]; then args+=( -d "$BODY" ); fi

resp=$(curl "${args[@]}") || true
body="${resp%$'\n'*}"; code="${resp##*$'\n'}"
if [[ "${GOATFLOW_DEBUG:-}" == "1" || "${VERBOSE:-}" == "1" ]]; then
    echo "DEBUG: ($METHOD $ENDPOINT) HTTP $code body: $body" >&2
fi
if echo "$body" | jq . >/dev/null 2>&1; then
  echo "$body" | jq .
else
  echo "$body"
fi
exit 0
