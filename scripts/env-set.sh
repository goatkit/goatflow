#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -lt 2 ]; then
  echo "usage: $0 KEY VALUE [FILE]" >&2
  exit 1
fi

key="$1"
value="$2"
file="${3:-.env}"

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

if [ ! -f "$file" ]; then
  echo "error: $file not found" >&2
  exit 1
fi

awk -v key="$key" -v value="$value" '
BEGIN {updated=0; lastline=""}
{
  if ($0 ~ "^" key "=") {
    print key "=" value
    updated=1
  } else {
    print $0
  }
  lastline=$0
}
END {
  if (!updated) {
    if (NR > 0 && length(lastline) != 0) {
      print ""
    }
    print key "=" value
  }
}
' "$file" >"$tmp"

mv "$tmp" "$file"
