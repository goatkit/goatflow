#!/usr/bin/env bash
set -euo pipefail

# Detect raw host 'go' or 'golangci-lint' commands in Makefile recipes.
# A recipe command line is one tab, optional make prefixes (@ - +), then the
# command. Go commands inside a container run continue on lines indented with
# two or more tabs, so they are not flagged; host commands start a recipe line.

FILE="Makefile"
[ -f "$FILE" ] || { echo "ERROR: Makefile not found"; exit 1; }

violations=$(awk '/^\t[@+-]*(go|golangci-lint)([ \t]|$)/ { printf "%d:%s\n", NR, $0 }' "$FILE")

if [ -n "$violations" ]; then
  echo "❌ Found raw host Go invocations (enforce container-first):" >&2
  echo "$violations" >&2
  exit 1
fi

echo "✅ No raw host Go commands detected"
