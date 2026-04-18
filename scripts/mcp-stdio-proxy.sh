#!/bin/bash
# MCP stdio-to-HTTP proxy for Claude Code.
# Reads JSON-RPC from stdin, POSTs to GoatFlow MCP endpoint, writes response to stdout.
#
# Usage in .mcp.json:
#   { "type": "stdio", "command": "./scripts/mcp-stdio-proxy.sh" }
#
# Requires: GOATFLOW_MCP_URL and GOATFLOW_MCP_TOKEN environment variables.

URL="${GOATFLOW_MCP_URL:-http://localhost:8080/api/mcp}"
TOKEN="${GOATFLOW_MCP_TOKEN}"

if [ -z "$TOKEN" ]; then
  echo '{"jsonrpc":"2.0","error":{"code":-32600,"message":"GOATFLOW_MCP_TOKEN not set"},"id":null}' >&2
  exit 1
fi

while IFS= read -r line; do
  # Skip empty lines
  [ -z "$line" ] && continue

  # POST to GoatFlow MCP endpoint
  response=$(curl -s -X POST "$URL" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "$line" 2>/dev/null)

  echo "$response"
done
