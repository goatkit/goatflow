#!/usr/bin/env bash
set -euo pipefail

# Detect raw 'go ' or 'golangci-lint' usage in Makefile outside approved contexts.
# Approved patterns (regex allowlist):
#  - toolbox-exec ARGS="go ..."
#  - ghcr.io/goatkit/goatflow/toolbox:latest (container run) followed by go
#  - go build/test inside bash -lc within a single explicit container run block
# For simplicity we flag any standalone lines starting with a tab + go / golangci-lint.

FILE="Makefile"
[ -f "$FILE" ] || { echo "ERROR: Makefile not found"; exit 1; }

violations=$(grep -nE '\t(go|golangci-lint) (build|test|run|vet|mod|list)|\tgo$' "$FILE" || true)

if [ -n "$violations" ]; then
  echo "❌ Found potential raw host Go invocations (enforce container-first):" >&2
  echo "$violations" >&2
  exit 1
fi

echo "✅ No raw host Go commands detected"
