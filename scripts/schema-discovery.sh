#!/bin/bash
# Container-first schema discovery tool

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Detect container runtime (same as container-wrapper.sh)
if command -v podman &> /dev/null; then
    CONTAINER_CMD="podman"
elif command -v docker &> /dev/null; then
    CONTAINER_CMD="docker"
else
    echo "Error: Neither docker nor podman found" >&2
    exit 1
fi

# Default values
DB_HOST="goatflow-postgres"
DB_PORT="5432"
DB_USER="goatflow_user"
DB_PASSWORD="${DB_PASSWORD:-}"
DB_NAME="goatflow"
OUTPUT_DIR="/app/modules/generated"
VERBOSE=""
TABLE=""

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --host)
            DB_HOST="$2"
            shift 2
            ;;
        --port)
            DB_PORT="$2"
            shift 2
            ;;
        --user)
            DB_USER="$2"
            shift 2
            ;;
        --password)
            DB_PASSWORD="$2"
            shift 2
            ;;
        --database)
            DB_NAME="$2"
            shift 2
            ;;
        --output)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        --table)
            TABLE="$2"
            shift 2
            ;;
        --verbose)
            VERBOSE="-verbose"
            shift
            ;;
        --help)
            echo "Usage: $0 [options]"
            echo "Options:"
            echo "  --host HOST        Database host (default: goatflow-postgres)"
            echo "  --port PORT        Database port (default: 5432)"
            echo "  --user USER        Database user (default: goatflow_user)"
            echo "  --password PASS    Database password"
            echo "  --database DB      Database name (default: goatflow)"
            echo "  --output DIR       Output directory (default: /app/modules/generated)"
            echo "  --table TABLE      Specific table to generate (empty for all)"
            echo "  --verbose          Verbose output"
            echo "  --help             Show this help message"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Build the Go binary in container
echo "Building schema discovery tool in container..."
$CONTAINER_CMD run --rm \
    -v "$PROJECT_ROOT:/app" \
    -w /app \
    -e CGO_ENABLED=0 \
    -e GOOS=linux \
    -e GOARCH=amd64 \
    -e GOCACHE=/tmp/.cache/go-build \
    -e GOMODCACHE=/tmp/.cache/go-mod \
    "${GO_IMAGE:-golang:1.24.11-alpine}" \
    go build -o /app/schema-discovery ./cmd/schema-discovery

if [ $? -ne 0 ]; then
    echo "Failed to build schema discovery tool"
    exit 1
fi

echo "Running schema discovery in container..."

# Prepare table argument if specified
TABLE_ARG=""
if [ -n "$TABLE" ]; then
    TABLE_ARG="-table $TABLE"
fi

# Run the schema discovery tool in container with network access to database
$CONTAINER_CMD run --rm \
    --network goatflow_goatflow-network \
    -v "$PROJECT_ROOT:/app" \
    -w /app \
    alpine:latest \
    /app/schema-discovery \
        -host "$DB_HOST" \
        -port "$DB_PORT" \
        -user "$DB_USER" \
        -password "$DB_PASSWORD" \
        -database "$DB_NAME" \
        -output "$OUTPUT_DIR" \
        $TABLE_ARG \
        $VERBOSE

if [ $? -eq 0 ]; then
    echo ""
    echo "Schema discovery complete! YAML files generated in: $PROJECT_ROOT/modules/generated/"
else
    echo "Schema discovery failed"
    exit 1
fi