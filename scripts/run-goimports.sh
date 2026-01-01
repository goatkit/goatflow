#!/usr/bin/env bash
set -euo pipefail

# Run goimports on git-tracked .go files, skipping files with build ignore tags
shopt -s nullglob
IFS=$'\n'
for f in $(git ls-files '*.go'); do
  # Skip files with //go:build ignore or // +build ignore in the first 3 lines
  if head -n 3 "$f" | grep -E -q '^[[:space:]]*//\s*go:build\s+ignore|^[[:space:]]*//\s*\+build\s+ignore'; then
    echo "skip $f"
    continue
  fi
  echo "fmt $f"
  goimports -w "$f" || true
done

echo "gofmt pass"
gofmt -w . || true

echo "done"