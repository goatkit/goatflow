#!/bin/bash
# Shared helper: log in as the demo admin through the web login
# (POST /api/auth/login) and print a curl Cookie header carrying the
# resulting access token. Source this file, then:
#
#   AUTH=$(admin_login_cookie "$BASE_URL") || exit 1
#   curl -H "$AUTH" "$BASE_URL/admin/..."
#
# Credentials come from DEMO_ADMIN_EMAIL / DEMO_ADMIN_PASSWORD (see .env).
# There is no test auth bypass.

admin_login_cookie() {
    local base_url="${1:-http://localhost:8080}"
    local login="${DEMO_ADMIN_EMAIL:-root@localhost}"
    local password="${DEMO_ADMIN_PASSWORD:-}"
    if [ -z "$password" ]; then
        echo "DEMO_ADMIN_PASSWORD must be set (see .env)" >&2
        return 1
    fi
    local payload token
    payload=$(jq -nc --arg l "$login" --arg p "$password" '{login:$l,password:$p}')
    token=$(curl -s -X POST "$base_url/api/auth/login" \
        -H "Content-Type: application/json" -H "Accept: application/json" \
        -d "$payload" | jq -r '.access_token // empty')
    if [ -z "$token" ]; then
        echo "login failed for $login at $base_url/api/auth/login" >&2
        return 1
    fi
    echo "Cookie: access_token=$token"
}
