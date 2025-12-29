#!/bin/sh
set -eu

BASE_TPL=templates
BASE_JS=static/js
OUT_DIR=generated/api-map
TMP_DIR=tmp

mkdir -p "$OUT_DIR" "$TMP_DIR"
chmod 777 "$TMP_DIR" || true
chmod 775 "$OUT_DIR" || true

RAW_JSON="$OUT_DIR/api-map.json"
RAW_DOT="$OUT_DIR/api-map.dot"
RAW_MMD="$OUT_DIR/api-map.mmd"

# Collect page -> endpoint references
collect_refs() {
  find "$BASE_TPL" -type f \( -name '*.html' -o -name '*.pongo2' \) 2>/dev/null | while read -r f; do
    rel=${f#$BASE_TPL/}
    grep -Eho '/api(/[a-zA-Z0-9_./-]+)?' "$f" 2>/dev/null | sort -u | while read -r ep; do
      echo "{\"page\":\"$rel\",\"endpoint\":\"$ep\"}";
    done
  done
  find "$BASE_JS" -type f -name '*.js' 2>/dev/null | while read -r f; do
    rel=${f#$BASE_JS/}
    grep -Eho '/api(/[a-zA-Z0-9_./-]+)?' "$f" 2>/dev/null | sort -u | while read -r ep; do
      echo "{\"js\":\"$rel\",\"endpoint\":\"$ep\"}";
    done
  done
}

# Build JSON array
tmp_json="$TMP_DIR/api-map-$$.json"
tmp_refs="$TMP_DIR/api-refs-$$.txt"
collect_refs > "$tmp_refs"

# Convert to JSON array
echo '[' > "$tmp_json"
first=1
while read -r line; do
  if [ $first -eq 0 ]; then echo ',' >> "$tmp_json"; fi
  printf '%s' "$line" >> "$tmp_json"
  first=0
done < "$tmp_refs"
echo ']' >> "$tmp_json"
mv "$tmp_json" "$RAW_JSON"
rm -f "$tmp_refs"

# Generate DOT graph
{
  echo 'digraph APICoverage {'
  echo '  rankdir=LR;'
  echo '  node [shape=box,fontsize=10];'
  # Pages
  jq -r '.[] | select(has("page")) | "  \"PAGE: " + .page + "\" [shape=oval,style=filled,fillcolor=lightyellow];"' "$RAW_JSON" | sort -u
  # JS modules
  jq -r '.[] | select(has("js")) | "  \"JS: " + .js + "\" [shape=oval,style=filled,fillcolor=lightblue];"' "$RAW_JSON" | sort -u
  # Endpoints
  jq -r '.[] | "  \"EP: " + .endpoint + "\" [shape=box,style=filled,fillcolor=white];"' "$RAW_JSON" | sort -u
  # Edges
  jq -r '.[] | select(has("page")) | "  \"PAGE: " + .page + "\" -> \"EP: " + .endpoint + "\";"' "$RAW_JSON"
  jq -r '.[] | select(has("js")) | "  \"JS: " + .js + "\" -> \"EP: " + .endpoint + "\" [color=blue];"' "$RAW_JSON"
  echo '}'
} > "$RAW_DOT"

# Mermaid graph (simpler)
{
  echo 'graph LR'
  jq -r '.[] | select(has("page")) | "  p_" + ( .page | gsub("[^a-zA-Z0-9]";"_")) + "[\"P: " + .page + "\"]:::page --> e_" + ( .endpoint | gsub("[^a-zA-Z0-9]";"_"))' "$RAW_JSON"
  jq -r '.[] | select(has("js")) | "  j_" + ( .js | gsub("[^a-zA-Z0-9]";"_")) + "[\"J: " + .js + "\"]:::js --> e_" + ( .endpoint | gsub("[^a-zA-Z0-9]";"_"))' "$RAW_JSON"
  jq -r '.[] | "  e_" + ( .endpoint | gsub("[^a-zA-Z0-9]";"_")) + "[\"" + .endpoint + "\"]:::ep"' "$RAW_JSON" | sort -u
  echo 'classDef page fill:#FFF9C4,stroke:#333,stroke-width:1px;'
  echo 'classDef js fill:#BBDEFB,stroke:#333,stroke-width:1px;'
  echo 'classDef ep fill:#ECEFF1,stroke:#333,stroke-width:1px;'
} > "$RAW_MMD"

# Optional SVG render if graphviz available
if command -v dot >/dev/null 2>&1; then
  dot -Tsvg "$RAW_DOT" -o "$OUT_DIR/api-map.svg" || true
fi

echo "Generated: $RAW_JSON $RAW_DOT $RAW_MMD"
exit 0
