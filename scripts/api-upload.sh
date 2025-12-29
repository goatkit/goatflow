#!/bin/bash
# Simple file upload using JWT auth
set -euo pipefail

if [[ -f ".env" ]]; then
  while IFS='=' read -r key value; do
    [[ -z "$key" || "$key" =~ ^[[:space:]]*# ]] && continue
    value=$(echo "$value" | sed 's/^"\(.*\)"$/\1/' | sed "s/^'\(.*\)'$/\1/")
    export "$key=$value"
  done < .env
fi

TEST_USERNAME="${ADMIN_USER:-${TEST_USERNAME:-root@localhost}}"
TEST_PASSWORD="${ADMIN_PASSWORD:-${TEST_PASSWORD:-}}"

if [ -z "$TEST_PASSWORD" ]; then
  echo "ERROR: ADMIN_PASSWORD or TEST_PASSWORD must be set in .env" >&2
  exit 1
fi

BACKEND_URL="${BACKEND_URL:-http://localhost:8080}"
METHOD="${METHOD:-POST}"
ENDPOINT="$1"
FILEPATH="$2"

payload=$(jq -nc --arg l "$TEST_USERNAME" --arg p "$TEST_PASSWORD" '{login:$l,password:$p}')
response=$(curl -k -s -w '\n%{http_code}' -X POST "$BACKEND_URL/api/auth/login" -H 'Content-Type: application/json' -H 'Accept: application/json' -d "$payload" || true)
body="${response%$'\n'*}"; http_code="${response##*$'\n'}"
if [[ "$http_code" != "200" ]]; then echo "auth failed"; echo "$body"; exit 1; fi
TOKEN=$(echo "$body" | jq -r '.access_token // .token')

curl -k -s -X POST "$BACKEND_URL$ENDPOINT" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Accept: application/json" \
  -F "file=@$FILEPATH" | jq .
