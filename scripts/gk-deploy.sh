#!/bin/bash
# Deploys an installed plugin to the running GoatFlow backend via the upload API.
# Runs on the host: builds ZIP in toolbox, then uploads from inside the backend container.
# Usage: make gk-deploy NAME=<plugin-name>
set -euo pipefail

NAME="${1:-}"
if [ -z "$NAME" ]; then echo "Usage: gk-deploy.sh <plugin-name>"; exit 1; fi

PLUGIN_DIR="plugins/$NAME"
if [ ! -d "$PLUGIN_DIR" ]; then
    echo "ERROR: $PLUGIN_DIR not found. Run 'gk install $NAME' first."
    exit 1
fi

# Step 1: Package the installed plugin into a ZIP (inside toolbox)
echo "📦 Packaging $NAME..."
make --no-print-directory toolbox-exec ARGS="go run ./cmd/gk build $PLUGIN_DIR"

ZIP=$(ls dist/${NAME}-*.zip 2>/dev/null | head -1)
if [ -z "$ZIP" ]; then echo "ERROR: No ZIP produced for $NAME"; exit 1; fi

# Step 2: Copy ZIP into the backend container and upload via localhost API
BACKEND_ID=$(docker compose -f docker-compose.yml ps -q backend 2>/dev/null | head -1)
if [ -z "$BACKEND_ID" ]; then
    echo "ERROR: Backend container not running. Run 'make up' first."
    exit 1
fi

echo "📤 Uploading $ZIP to backend..."
docker cp "$ZIP" "$BACKEND_ID:/tmp/plugin-upload.zip"

# Run the upload from inside the backend container (localhost:8080 always works)
docker compose -f docker-compose.yml exec -T backend sh -c '
    TOKEN=$(curl -k -s -X POST http://localhost:8080/api/auth/login \
        -H "Content-Type: application/json" \
        -d "{\"login\":\"'"${ADMIN_USER:-root@localhost}"'\",\"password\":\"'${ADMIN_PASSWORD}'\"}" \
        | python3 -c "import json,sys;d=json.load(sys.stdin);print(d.get(\"access_token\") or d.get(\"token\") or \"\")");
    if [ -z "$TOKEN" ]; then echo "ERROR: Auth failed"; exit 1; fi;
    curl -k -s http://localhost:8080/api/v1/plugins/upload \
        -H "Authorization: Bearer $TOKEN" \
        -F "plugin=@/tmp/plugin-upload.zip" \
        | python3 -c "import json,sys;d=json.load(sys.stdin);print(json.dumps(d,indent=2))" 2>/dev/null || cat;
    rm -f /tmp/plugin-upload.zip
'
