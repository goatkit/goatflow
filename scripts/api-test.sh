#!/bin/bash
# GOTRS API Testing Script
# Handles authentication and makes API calls with proper TLS

set -euo pipefail

# Load environment variables from .env if it exists
if [[ -f ".env" ]]; then
    # Use a safer method to load .env that handles special characters
    while IFS='=' read -r key value; do
        # Skip empty lines and comments
        [[ -z "$key" || "$key" =~ ^[[:space:]]*# ]] && continue
        # Remove quotes if present
        value=$(echo "$value" | sed 's/^"\(.*\)"$/\1/' | sed "s/^'\(.*\)'$/\1/")
        export "$key=$value"
    done < .env
fi

# Also try to load ADMIN_USER and ADMIN_PASSWORD directly if the above failed
if [[ -f ".env" ]]; then
    ADMIN_USER_FROM_ENV=$(grep '^ADMIN_USER=' .env | cut -d'=' -f2- | sed 's/^"\(.*\)"$/\1/' | sed "s/^'\(.*\)'$/\1/")
    ADMIN_PASSWORD_FROM_ENV=$(grep '^ADMIN_PASSWORD=' .env | cut -d'=' -f2- | sed 's/^"\(.*\)"$/\1/' | sed "s/^'\(.*\)'$/\1/")
    if [[ -n "$ADMIN_USER_FROM_ENV" ]]; then
        export ADMIN_USER="$ADMIN_USER_FROM_ENV"
    fi
    if [[ -n "$ADMIN_PASSWORD_FROM_ENV" ]]; then
        export ADMIN_PASSWORD="$ADMIN_PASSWORD_FROM_ENV"
    fi
fi

if [[ "${GOTRS_DEBUG:-}" == "1" || "${VERBOSE:-}" == "1" ]]; then
    echo "DEBUG: BACKEND_URL=${BACKEND_URL:-<not set>}"
    echo "DEBUG: ADMIN_USER=${ADMIN_USER:-<not set>}"
    echo "DEBUG: ADMIN_PASSWORD=${ADMIN_PASSWORD:-<not set>}"
    echo "DEBUG: TEST_USERNAME=${TEST_USERNAME:-<not set>}"
    echo "DEBUG: TEST_PASSWORD=${TEST_PASSWORD:-<not set>}"
fi

# Test credentials - MUST be set in .env
# Prefer ADMIN_USER/ADMIN_PASSWORD, fall back to TEST_USERNAME/TEST_PASSWORD
TEST_USERNAME="${ADMIN_USER:-${TEST_USERNAME:-root@localhost}}"
TEST_PASSWORD="${ADMIN_PASSWORD:-${TEST_PASSWORD:-}}"

if [ -z "$TEST_PASSWORD" ]; then
    echo "ERROR: ADMIN_PASSWORD or TEST_PASSWORD must be set in .env" >&2
    exit 1
fi

# Infer BACKEND_URL if not set: prefer https container name else http localhost
if [[ -z "${BACKEND_URL:-}" ]]; then
    if getent hosts gotrs-backend >/dev/null 2>&1; then
        BACKEND_URL="https://gotrs-backend:8080"
    else
        BACKEND_URL="http://localhost:8080"
    fi
fi

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}ℹ️  $1${NC}"
}

log_warn() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

auth_json() {
    local payload
    payload=$(jq -nc --arg l "$TEST_USERNAME" --arg p "$TEST_PASSWORD" '{login:$l,password:$p}')
    local endpoints=("/api/auth/login" "/api/v1/auth/login")
    local e response body http_code token
    for e in "${endpoints[@]}"; do
        response=$(curl -k -s -w '\n%{http_code}' -X POST "$BACKEND_URL$e" -H 'Content-Type: application/json' -H 'Accept: application/json' -d "$payload" || true)
        body="${response%$'\n'*}"; http_code="${response##*$'\n'}"
                if [[ "${GOTRS_DEBUG:-}" == "1" || "${VERBOSE:-}" == "1" ]]; then
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

# Function to make API call
make_api_call() {
    local method="$1" endpoint="$2" body="${3:-}" attempts=0 max_attempts=2 token csrf header_args response status_line resp_body http_code
    while (( attempts < max_attempts )); do
        attempts=$((attempts+1))
                if [[ "${GOTRS_DEBUG:-}" == "1" || "${VERBOSE:-}" == "1" ]]; then
                    echo "DEBUG: Auth attempt $attempts" >&2
                fi
        token=$(auth_json) || { echo "DEBUG: auth failed" >&2; return 1; }
    header_args=(-H "Authorization: Bearer $token" -H 'Content-Type: application/json' -H 'Accept: application/json')
        if [[ "${GOTRS_DEBUG:-}" == "1" || "${VERBOSE:-}" == "1" ]]; then
            echo "DEBUG: Using token=$token" >&2
        fi
        local curl_parts=(-k -s -w '\n%{http_code}' -X "$method" "$BACKEND_URL$endpoint" "${header_args[@]}")
        if [[ -n "$body" ]]; then curl_parts+=(-d "$body"); fi
        status_line=$(curl "${curl_parts[@]}") || true
        resp_body="${status_line%$'\n'*}"; http_code="${status_line##*$'\n'}"
                if [[ "${GOTRS_DEBUG:-}" == "1" || "${VERBOSE:-}" == "1" ]]; then
                    echo "DEBUG: ($method $endpoint) HTTP $http_code body: $resp_body" >&2
                fi
        if [[ "$http_code" == "401" && $attempts -lt $max_attempts ]]; then
                        if [[ "${GOTRS_DEBUG:-}" == "1" || "${VERBOSE:-}" == "1" ]]; then
                            echo "DEBUG: 401 received, retrying auth" >&2
                        fi
            continue
        fi
        if echo "$resp_body" | jq . >/dev/null 2>&1; then
            echo "$resp_body" | jq .
        else
            echo "$resp_body"
        fi
        return 0
    done
    echo "ERROR: Request failed after $attempts attempts" >&2
    return 1
}

# Main script
if [[ "${GOTRS_DEBUG:-}" == "1" || "${VERBOSE:-}" == "1" ]]; then
    echo "DEBUG: Starting main script"
fi
if [[ $# -lt 2 ]]; then
    log_error "Usage: $0 <METHOD> <ENDPOINT> [BODY]"
    log_error "Example: $0 GET /api/v1/tickets"
    log_error "Example: $0 POST /api/v1/tickets '{\"title\":\"Test\"}'"
    exit 1
fi

METHOD="$1"
ENDPOINT="$2"
if [[ -z "$METHOD" ]]; then METHOD="GET"; fi
BODY="${3:-}"

if [[ "${GOTRS_DEBUG:-}" == "1" || "${VERBOSE:-}" == "1" ]]; then
    echo "DEBUG: METHOD=$METHOD, ENDPOINT=$ENDPOINT, BODY=$BODY"
fi

make_api_call "$METHOD" "$ENDPOINT" "$BODY"