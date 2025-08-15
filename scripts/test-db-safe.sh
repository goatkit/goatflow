#!/bin/bash
# GOTRS Safe Test Database Script
# This script ensures tests ONLY run against test databases
# It includes multiple safety checks to prevent production data loss
# Works both inside containers and on the host (using docker compose exec)

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo "======================================"
echo "    GOTRS Safe Test Runner            "
echo "======================================"

# Detect Docker/Podman compose command
detect_compose_cmd() {
    if command -v podman-compose > /dev/null 2>&1; then
        echo "podman-compose"
    elif command -v podman > /dev/null 2>&1 && podman compose version > /dev/null 2>&1; then
        echo "podman compose"
    elif command -v docker > /dev/null 2>&1 && docker compose version > /dev/null 2>&1; then
        echo "docker compose"
    elif command -v docker-compose > /dev/null 2>&1; then
        echo "docker-compose"
    else
        echo "docker compose"
    fi
}

COMPOSE_CMD=$(detect_compose_cmd)

# Check if running in container or local
if [ -f /.dockerenv ] || [ -f /run/.containerenv ]; then
    IN_CONTAINER=true
    echo "Running in container environment..."
else
    IN_CONTAINER=false
    echo "Running on host via container..."
    echo "Using compose command: $COMPOSE_CMD"
    
    # Check if backend service is running
    if ! $COMPOSE_CMD ps --services --filter "status=running" | grep -q "backend"; then
        echo -e "${RED}Error: Backend service is not running.${NC}"
        echo "Please run 'make up' first to start the services."
        exit 1
    fi
fi

# Safety Check 1: Environment Detection
check_environment() {
    local env="${APP_ENV:-development}"
    
    if [[ "$env" == "production" ]]; then
        echo -e "${RED}ERROR: Tests cannot run in production environment!${NC}"
        echo -e "${RED}APP_ENV is set to: $env${NC}"
        echo ""
        echo "To run tests, please:"
        echo "1. Use a development or test environment"
        echo "2. Set APP_ENV=test or APP_ENV=development"
        exit 1
    fi
    
    echo -e "${GREEN}✓ Environment check passed (APP_ENV=$env)${NC}"
}

# Safety Check 2: Database Name Verification
check_database_name() {
    local db_name="${DB_NAME:-gotrs}"
    
    # Ensure we're using a test database
    if [[ ! "$db_name" =~ _test$ ]] && [[ "$db_name" != "gotrs_test" ]]; then
        echo -e "${YELLOW}Warning: Database name doesn't end with '_test'${NC}"
        echo "Current DB_NAME: $db_name"
        echo ""
        echo "Switching to test database..."
        export DB_NAME="${db_name}_test"
        echo -e "${GREEN}✓ Now using DB_NAME=${DB_NAME}${NC}"
    else
        echo -e "${GREEN}✓ Using test database: $db_name${NC}"
    fi
}

# Safety Check 3: Hostname Check (prevent remote DB access during tests)
check_hostname() {
    local db_host="${DB_HOST:-localhost}"
    
    # Only allow localhost/postgres container for tests
    if [[ "$db_host" != "localhost" ]] && \
       [[ "$db_host" != "127.0.0.1" ]] && \
       [[ "$db_host" != "postgres" ]] && \
       [[ "$db_host" != "::1" ]]; then
        echo -e "${RED}ERROR: Tests can only run against local databases!${NC}"
        echo -e "${RED}Current DB_HOST: $db_host${NC}"
        echo ""
        echo "For safety, tests are restricted to:"
        echo "- localhost"
        echo "- 127.0.0.1"
        echo "- postgres (container name)"
        echo "- ::1 (IPv6 localhost)"
        exit 1
    fi
    
    echo -e "${GREEN}✓ Database host check passed (DB_HOST=$db_host)${NC}"
}

# Safety Check 4: Port Check (warn if using non-standard port)
check_port() {
    local db_port="${DB_PORT:-5432}"
    
    if [[ "$db_port" != "5432" ]]; then
        echo -e "${YELLOW}Notice: Using non-standard PostgreSQL port: $db_port${NC}"
        read -p "Continue with tests? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            echo "Tests cancelled."
            exit 1
        fi
    else
        echo -e "${GREEN}✓ Using standard PostgreSQL port: $db_port${NC}"
    fi
}

# Safety Check 5: Create test database if needed
ensure_test_database() {
    local db_name="${DB_NAME}"
    local db_user="${DB_USER:-gotrs}"
    local db_pass="${DB_PASSWORD:-gotrs_password}"
    local db_host="${DB_HOST:-postgres}"
    
    echo ""
    echo "Ensuring test database exists..."
    
    if [ "$IN_CONTAINER" = true ]; then
        # Running in container - can use psql directly
        PGPASSWORD="$db_pass" psql -h "$db_host" -U "$db_user" -d postgres -tc \
            "SELECT 1 FROM pg_database WHERE datname = '$db_name'" | grep -q 1 || \
        PGPASSWORD="$db_pass" psql -h "$db_host" -U "$db_user" -d postgres -c \
            "CREATE DATABASE \"$db_name\"" && \
        echo -e "${GREEN}✓ Created test database: $db_name${NC}" || \
        echo -e "${GREEN}✓ Test database already exists: $db_name${NC}"
    else
        # Running on host - use docker compose exec to run psql in postgres container
        $COMPOSE_CMD exec -e PGPASSWORD="$db_pass" postgres psql -U "$db_user" -d postgres -tc \
            "SELECT 1 FROM pg_database WHERE datname = '$db_name'" | grep -q 1 || \
        $COMPOSE_CMD exec -e PGPASSWORD="$db_pass" postgres psql -U "$db_user" -d postgres -c \
            "CREATE DATABASE \"$db_name\"" && \
        echo -e "${GREEN}✓ Created test database: $db_name${NC}" || \
        echo -e "${GREEN}✓ Test database already exists: $db_name${NC}"
    fi
}

# Run all safety checks
run_safety_checks() {
    echo ""
    echo "Running safety checks..."
    echo "========================"
    
    check_environment
    check_database_name
    check_hostname
    check_port
    ensure_test_database
    
    echo ""
    echo -e "${GREEN}✅ All safety checks passed!${NC}"
    echo ""
}

# Clean test database (with confirmation)
clean_test_database() {
    local db_name="${DB_NAME}"
    local db_user="${DB_USER:-gotrs}"
    local db_pass="${DB_PASSWORD:-gotrs_password}"
    
    if [[ ! "$db_name" =~ _test$ ]]; then
        echo -e "${RED}ERROR: Will not clean non-test database: $db_name${NC}"
        return 1
    fi
    
    echo -e "${YELLOW}Cleaning test database: $db_name${NC}"
    echo "This will drop and recreate the test database."
    read -p "Continue? (y/N) " -n 1 -r
    echo
    
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        # Drop and recreate
        if [ "$IN_CONTAINER" = true ]; then
            PGPASSWORD="$db_pass" psql -h "${DB_HOST:-postgres}" \
                -U "$db_user" -d postgres -c \
                "DROP DATABASE IF EXISTS \"$db_name\"; CREATE DATABASE \"$db_name\";"
        else
            $COMPOSE_CMD exec -e PGPASSWORD="$db_pass" postgres psql \
                -U "$db_user" -d postgres -c \
                "DROP DATABASE IF EXISTS \"$db_name\"; CREATE DATABASE \"$db_name\";"
        fi
        echo -e "${GREEN}✓ Test database cleaned${NC}"
    else
        echo "Skipping database cleanup."
    fi
}

# Run tests function
run_tests() {
    echo "Running tests..."
    echo "================"
    
    if [ "$IN_CONTAINER" = true ]; then
        mkdir -p generated
        go test -v -race -coverprofile=generated/coverage.out -covermode=atomic ./...
    else
        mkdir -p generated
        $COMPOSE_CMD exec -e DB_NAME="${DB_NAME}" -e APP_ENV="${APP_ENV}" backend \
            sh -c "mkdir -p generated && go test -v -race -coverprofile=generated/coverage.out -covermode=atomic ./..."
    fi
    
    if [ $? -eq 0 ]; then
        echo ""
        echo -e "${GREEN}✓ Tests completed successfully${NC}"
    else
        echo ""
        echo -e "${RED}✗ Some tests failed${NC}"
        exit 1
    fi
}

# Main execution
main() {
    case "${1:-test}" in
        test)
            run_safety_checks
            run_tests
            ;;
            
        clean)
            run_safety_checks
            clean_test_database
            ;;
            
        check)
            run_safety_checks
            echo "Safety checks completed. Ready to run tests."
            ;;
            
        *)
            echo "Usage: $0 [test|clean|check]"
            echo "  test  - Run tests with safety checks (default)"
            echo "  clean - Clean test database"
            echo "  check - Run safety checks only"
            exit 1
            ;;
    esac
}

# Export test environment variables
export APP_ENV="${APP_ENV:-test}"
export DB_NAME="${DB_NAME:-gotrs}_test"

# Show current configuration
echo "Current test configuration:"
echo "  APP_ENV: $APP_ENV"
echo "  DB_NAME: $DB_NAME"
echo "  DB_HOST: ${DB_HOST:-postgres}"
echo "  DB_USER: ${DB_USER:-gotrs}"

# Run main function
main "$@"