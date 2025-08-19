#!/bin/bash
# Database operations wrapper - handles common database tasks

# Source container wrapper to get the right commands
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/container-wrapper.sh"

# Load environment variables from .env if it exists
if [ -f .env ]; then
    set -a
    source .env
    set +a
fi

# Default database settings
DB_HOST="${DB_HOST:-postgres}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-gotrs}"
DB_USER="${DB_USER:-gotrs_user}"
DB_PASSWORD="${DB_PASSWORD:-lOZgkt-.dI4ftN@uxTqOY4uc}"

# Function to run SQL commands
run_sql() {
    local sql="$1"
    local db="${2:-$DB_NAME}"
    
    $COMPOSE_CMD exec -T postgres psql -U "$DB_USER" -d "$db" -c "$sql"
}

# Function to run SQL file
run_sql_file() {
    local file="$1"
    local db="${2:-$DB_NAME}"
    
    $COMPOSE_CMD exec -T postgres psql -U "$DB_USER" -d "$db" < "$file"
}

# Function to run migrations
run_migrations() {
    local direction="${1:-up}"
    local count="${2:-}"
    local db="${3:-$DB_NAME}"
    
    echo "Running migrations $direction on database: $db"
    
    if [ -n "$count" ]; then
        $COMPOSE_CMD exec backend migrate \
            -path /app/migrations \
            -database "postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${db}?sslmode=disable" \
            "$direction" "$count"
    else
        $COMPOSE_CMD exec backend migrate \
            -path /app/migrations \
            -database "postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${db}?sslmode=disable" \
            "$direction"
    fi
}

# Main command handler
case "${1:-help}" in
    sql)
        shift
        run_sql "$@"
        ;;
    
    file)
        shift
        run_sql_file "$@"
        ;;
    
    migrate)
        shift
        run_migrations "$@"
        ;;
    
    shell)
        $COMPOSE_CMD exec postgres psql -U "$DB_USER" -d "${2:-$DB_NAME}"
        ;;
    
    status)
        $COMPOSE_CMD exec backend migrate \
            -path /app/migrations \
            -database "postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=disable" \
            version
        ;;
    
    reset)
        db="${2:-$DB_NAME}"
        echo "⚠️  Resetting database: $db"
        run_migrations "down" "-all" "$db"
        run_migrations "up" "" "$db"
        echo "✅ Database reset complete"
        ;;
    
    seed-dev)
        echo "Seeding development database with test data..."
        run_migrations "up" "" "$DB_NAME"
        echo "✅ Development database seeded"
        ;;
    
    test)
        echo "Testing database connection..."
        if run_sql "SELECT version();" > /dev/null 2>&1; then
            echo "✅ Database connection successful"
            run_sql "SELECT version();"
        else
            echo "❌ Database connection failed"
            exit 1
        fi
        ;;
    
    help|*)
        echo "Database Operations Wrapper"
        echo ""
        echo "Usage: $0 <command> [options]"
        echo ""
        echo "Commands:"
        echo "  sql <query>       - Run SQL query"
        echo "  file <path>       - Run SQL file"
        echo "  migrate [up|down] [count] [db] - Run migrations"
        echo "  shell [db]        - Open psql shell"
        echo "  status            - Show migration status"
        echo "  reset [db]        - Reset database (down all, up all)"
        echo "  seed-dev          - Seed development database"
        echo "  test              - Test database connection"
        echo "  help              - Show this help"
        echo ""
        echo "Environment:"
        echo "  DB_HOST:     $DB_HOST"
        echo "  DB_PORT:     $DB_PORT"
        echo "  DB_NAME:     $DB_NAME"
        echo "  DB_USER:     $DB_USER"
        echo "  Container:   $CONTAINER_CMD"
        echo "  Compose:     $COMPOSE_CMD"
        ;;
esac