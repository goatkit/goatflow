#!/bin/sh
set -eu

SRC_FILE=internal/api/htmx_routes.go
BASELINE_FILE=routing/static_routes_baseline.txt
TMP_DIR=tmp
mkdir -p "$TMP_DIR" routing
chmod 777 "$TMP_DIR" || true

CURRENT="$TMP_DIR/static_routes_current.txt"

# Extract code-defined routes (protected/protectedAPI groups) ignoring obvious infra
grep -E 'protected(API)?\.(GET|POST|PUT|DELETE|PATCH)\("/[^" ]*' "$SRC_FILE" 2>/dev/null \
  | sed -E 's/.*\("(\/[^"]*)".*/\1/' \
  | grep -v -E '^(/health|/static/|/favicon\.ico)$' \
  | sort -u > "$CURRENT" || true

if [ ! -s "$CURRENT" ]; then
  echo "No static business routes detected (good)."
  # If baseline exists and had entries previously, we keep it until migration done
  exit 0
fi

if [ ! -f "$BASELINE_FILE" ]; then
  cp "$CURRENT" "$BASELINE_FILE"
  echo "# Baseline created: $BASELINE_FILE" >&2
  echo "# Commit this file. Future static additions will fail." >&2
  exit 0
fi

NEW=$(comm -13 "$BASELINE_FILE" "$CURRENT" || true)

if [ -n "$NEW" ]; then
  echo "ERROR: New hard-coded routes detected (should move to YAML):" >&2
  echo "$NEW" >&2
  echo "---" >&2
  echo "To accept temporarily (not recommended), append to baseline: $BASELINE_FILE" >&2
  exit 1
fi

echo "Static route audit passed."
exit 0
