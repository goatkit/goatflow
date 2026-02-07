#!/bin/bash
# GoatFlow Database Fix Script
# Resolves the "database goatflow_user does not exist" error

echo "======================================"
echo "    GoatFlow Database Fix Script        "
echo "======================================"
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to detect compose command
detect_compose_command() {
    if command -v podman-compose > /dev/null 2>&1; then
        echo "podman-compose"
    elif command -v podman > /dev/null 2>&1 && podman compose version > /dev/null 2>&1; then
        echo "podman compose"
    elif command -v docker > /dev/null 2>&1 && docker compose version > /dev/null 2>&1; then
        echo "docker compose"
    elif command -v docker-compose > /dev/null 2>&1; then
        echo "docker-compose"
    else
        echo ""
    fi
}

# Function to detect container command
detect_container_command() {
    if command -v podman > /dev/null 2>&1; then
        echo "podman"
    elif command -v docker > /dev/null 2>&1; then
        echo "docker"
    else
        echo ""
    fi
}

# Detect commands
COMPOSE_CMD=$(detect_compose_command)
CONTAINER_CMD=$(detect_container_command)

if [ -z "$COMPOSE_CMD" ]; then
    echo -e "${RED}Error: No Docker or Podman compose command found!${NC}"
    echo "Please install Docker or Podman first."
    exit 1
fi

echo -e "${BLUE}Using compose command: ${GREEN}$COMPOSE_CMD${NC}"
echo -e "${BLUE}Using container command: ${GREEN}$CONTAINER_CMD${NC}"
echo ""

# Check current .env file
echo "Checking .env configuration..."
if [ ! -f .env ]; then
    echo -e "${YELLOW}Warning: .env file not found!${NC}"
    echo "Creating .env from .env.example..."
    cp .env.example .env
    echo -e "${GREEN}✓ Created .env file${NC}"
fi

# Check DB_USER value
current_db_user=$(grep "^DB_USER=" .env | cut -d'=' -f2)
echo "Current DB_USER in .env: $current_db_user"

if [ "$current_db_user" = "goatflow_user" ]; then
    echo -e "${YELLOW}Found incorrect DB_USER value: goatflow_user${NC}"
    echo "Fixing DB_USER to 'goatflow'..."
    
    # Backup .env
    cp .env .env.backup
    echo "Created backup: .env.backup"
    
    # Fix DB_USER
    sed -i 's/^DB_USER=goatflow_user/DB_USER=goatflow/' .env
    
    # Also fix DATABASE_URL if present
    sed -i 's|postgres://goatflow_user:|postgres://goatflow:|' .env
    
    echo -e "${GREEN}✓ Fixed DB_USER in .env${NC}"
elif [ "$current_db_user" = "goatflow" ]; then
    echo -e "${GREEN}✓ DB_USER is already correct${NC}"
else
    echo -e "${YELLOW}Warning: DB_USER has custom value: $current_db_user${NC}"
fi

echo ""
echo "Cleaning up old containers and volumes..."

# Stop all containers
echo "Stopping containers..."
$COMPOSE_CMD down 2>/dev/null || true

# Remove specific volumes
echo "Removing old database volumes..."
$CONTAINER_CMD volume rm goatflow_postgres_data 2>/dev/null || true
$CONTAINER_CMD volume rm goatflow_postgres_data 2>/dev/null || true
$CONTAINER_CMD volume rm postgres_data 2>/dev/null || true

# List and remove any goatflow-related volumes
echo "Cleaning up any remaining goatflow volumes..."
$CONTAINER_CMD volume ls | grep -i goatflow | awk '{print $2}' | while read vol; do
    echo "  Removing volume: $vol"
    $CONTAINER_CMD volume rm "$vol" 2>/dev/null || true
done

# Remove any stopped containers
echo "Removing stopped containers..."
$CONTAINER_CMD ps -a | grep -i goatflow | awk '{print $1}' | while read container; do
    echo "  Removing container: $container"
    $CONTAINER_CMD rm -f "$container" 2>/dev/null || true
done

echo ""
echo -e "${GREEN}✓ Cleanup complete!${NC}"
echo ""
echo "======================================"
echo "    Ready to start fresh!             "
echo "======================================"
echo ""
echo "Your database configuration is now:"
echo "  DB_NAME: goatflow"
echo "  DB_USER: goatflow"
echo "  DB_PASSWORD: goatflow_password"
echo ""
echo "To start the services, run:"
echo -e "  ${GREEN}make up${NC}"
echo "    OR"
echo -e "  ${GREEN}$COMPOSE_CMD up${NC}"
echo ""
echo "Then run tests with:"
echo -e "  ${GREEN}make test${NC}"
echo ""
echo "If you still see errors, check:"
echo "1. Ensure Docker/Podman daemon is running"
echo "2. Check logs with: make logs"
echo "3. Verify .env file has correct values"