#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

DB_HOST="${DB_CONN_HOST:-${DB_HOST:-postgres}}"
DB_PORT="${DB_CONN_PORT:-${DB_PORT:-5432}}"
DB_NAME="${DB_CONN_NAME:-${DB_NAME:-gotrs}}"
DB_USER="${DB_CONN_USER:-${DB_USER:-gotrs_user}}"
DB_PASSWORD="${DB_CONN_PASSWORD:-${DB_PASSWORD:-gotrs_password}}"

compose() {
    "$REPO_ROOT/scripts/container-wrapper.sh" compose "$@"
}

run_sql() {
    local sql="$1"
    local db="${2:-$DB_NAME}"
    compose exec -T postgres psql -U "$DB_USER" -d "$db" -c "$sql"
}

run_sql_file() {
    local file="$1"
    local db="${2:-$DB_NAME}"
    compose exec -T postgres psql -U "$DB_USER" -d "$db" < "$file"
}

run_migrations() {
    local direction="${1:-up}"
    local count="${2:-}"
    local db="${3:-$DB_NAME}"
    echo "Running migrations $direction on database: $db"
    if [ -n "$count" ]; then
        compose exec backend migrate \
            -path /app/migrations \
            -database "postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${db}?sslmode=disable" \
            "$direction" "$count"
    else
        compose exec backend migrate \
            -path /app/migrations \
            -database "postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${db}?sslmode=disable" \
            "$direction"
    fi
}

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
        compose exec postgres psql -U "$DB_USER" -d "${2:-$DB_NAME}"
        ;;
    status)
        compose exec backend migrate \
            -path /app/migrations \
            -database "postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=disable" \
            version
        ;;
    reset)
        db="${2:-$DB_NAME}"
        echo "Resetting database: $db"
        run_migrations "down" "-all" "$db"
        run_migrations "up" "" "$db"
        echo "Database reset complete"
        ;;
    seed-dev)
        echo "Seeding development database"
        run_migrations "up" "" "$DB_NAME"
        echo "Development database seeded"
        ;;
    test)
        echo "Testing database connection"
        if run_sql "SELECT version();" > /dev/null 2>&1; then
            echo "Database connection successful"
            run_sql "SELECT version();"
        else
            echo "Database connection failed"
            exit 1
        fi
        ;;
    help|*)
        cat <<EOF
Database Operations Wrapper

Usage: $0 <command> [options]

Commands:
  sql <query>       Run SQL query
  file <path>       Run SQL file
  migrate [up|down] [count] [db]  Run migrations
  shell [db]        Open psql shell
  status            Show migration status
  reset [db]        Reset database (down all, up all)
  seed-dev          Seed development database
  test              Test database connection
  help              Show this help

Environment:
  DB_HOST:     $DB_HOST
  DB_PORT:     $DB_PORT
  DB_NAME:     $DB_NAME
  DB_USER:     $DB_USER
EOF
        ;;
esac
