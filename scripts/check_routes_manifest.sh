#!/bin/sh
set -eu
MANIFEST=generated/routes-manifest.json
BASE=generated/routes-manifest.baseline.json
if [ ! -f "$MANIFEST" ]; then
  echo "routes-manifest.json not found (run make routes-generate)" >&2
  exit 1
fi
if [ ! -f "$BASE" ]; then
  cp "$MANIFEST" "$BASE"
  echo "Baseline created at $BASE" >&2
  exit 0
fi

mkdir -p /cache/tmp
NEW_JSON=/cache/tmp/routes-manifest.new.json
BASE_JSON=/cache/tmp/routes-manifest.base.json
cp "$MANIFEST" "$NEW_JSON"
cp "$BASE" "$BASE_JSON"

# Build key maps: method|path -> compact json of interesting fields
to_kv() {
  jq -r '.routes[] | ( (.method + "|" + .path) + "\t" + ( {handler:(.handler//""),redirectTo:(.redirectTo//""),status:(.status//0),middleware:(.middleware//[]),websocket:(.websocket//false)} | @json) )' "$1" | sort
}

to_kv "$NEW_JSON" > /cache/tmp/new.kv
to_kv "$BASE_JSON" > /cache/tmp/base.kv

cut -f1 /cache/tmp/base.kv > /cache/tmp/base.keys
cut -f1 /cache/tmp/new.kv > /cache/tmp/new.keys

ADDED_KEYS=$(comm -13 /cache/tmp/base.keys /cache/tmp/new.keys || true)
REMOVED_KEYS=$(comm -23 /cache/tmp/base.keys /cache/tmp/new.keys || true)

# Build maps for common keys
COMMON_KEYS=$(comm -12 /cache/tmp/base.keys /cache/tmp/new.keys || true)
CHANGED_SUMMARY=""
if [ -n "$COMMON_KEYS" ]; then
  echo "$COMMON_KEYS" | while IFS= read -r k; do
    [ -z "$k" ] && continue
    BASE_VAL=$(grep "^$k\t" /cache/tmp/base.kv | cut -f2-)
    NEW_VAL=$(grep "^$k\t" /cache/tmp/new.kv | cut -f2-)
    [ "$BASE_VAL" = "$NEW_VAL" ] && continue
    # Compare individual fields
    CHANGES=""
    for field in handler redirectTo status websocket; do
      BV=$(echo "$BASE_VAL" | jq -r ".$field")
      NV=$(echo "$NEW_VAL" | jq -r ".$field")
      if [ "$BV" != "$NV" ]; then
        CHANGES="$CHANGES $field:$BV=>$NV"
      fi
    done
    # Middleware diff (set comparison)
    B_MW=$(echo "$BASE_VAL" | jq -r '.middleware | sort | join(",")')
    N_MW=$(echo "$NEW_VAL" | jq -r '.middleware | sort | join(",")')
    if [ "$B_MW" != "$N_MW" ]; then
      CHANGES="$CHANGES middleware:[$B_MW]=>[$N_MW]"
    fi
    [ -n "$CHANGES" ] && CHANGED_SUMMARY="$CHANGED_SUMMARY\n$k $CHANGES"
  done > /cache/tmp/changed.tmp
  CHANGED_SUMMARY=$(cat /cache/tmp/changed.tmp || true)
fi

if [ -z "$ADDED_KEYS$REMOVED_KEYS$CHANGED_SUMMARY" ]; then
  echo "Route manifest matches baseline." >&2
  exit 0
fi

echo "Route manifest drift detected:" >&2
if [ -n "$ADDED_KEYS" ]; then
  echo "\nAdded:" >&2
  echo "$ADDED_KEYS" | sed 's/^/  /' >&2
fi
if [ -n "$REMOVED_KEYS" ]; then
  echo "\nRemoved:" >&2
  echo "$REMOVED_KEYS" | sed 's/^/  /' >&2
fi
if [ -n "$CHANGED_SUMMARY" ]; then
  echo "\nChanged:" >&2
  echo "$CHANGED_SUMMARY" | sed 's/^/  /' >&2
fi
echo "\nIf intentional, update baseline: cp $MANIFEST $BASE" >&2
exit 1