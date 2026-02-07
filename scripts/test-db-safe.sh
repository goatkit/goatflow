#!/bin/bash
# GoatFlow Safe Test Database Script
# This script ensures tests ONLY run against test databases
# It includes multiple safety checks to prevent production data loss
# Works both inside containers and on the host (using docker compose exec)

set -e
shopt -s extglob

load_env_file() {
    while IFS= read -r line || [ -n "$line" ]; do
        line="${line%$'\r'}"
        case "$line" in
            ''|\#*) continue ;;
        esac
        if [[ "$line" == *=* ]]; then
            local key="${line%%=*}"
            local value="${line#*=}"

            # trim leading/trailing whitespace
            key="${key##+([[:space:]])}"
            key="${key%%+([[:space:]])}"
            value="${value##+([[:space:]])}"
            value="${value%%+([[:space:]])}"

            if [[ "$key" == export\ * ]]; then
                key="${key#export }"
            fi

            if [[ "$value" == \"*\" && "$value" == *\" ]]; then
                value="${value:1:-1}"
            elif [[ "$value" == \'*\' && "$value" == *\' ]]; then
                value="${value:1:-1}"
            fi

            if [[ "$key" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]]; then
                # Respect pre-existing environment overrides (e.g., caller-supplied values)
                if [[ -z "${!key+x}" ]]; then
                    export "$key=$value"
                fi
            fi
        fi
    done < "$1"
}

# Load environment variables from .env (or custom ENV_FILE) before we override
ENV_FILE="${ENV_FILE:-.env}"
if [ -f "$ENV_FILE" ]; then
    echo "Loading environment from $ENV_FILE"
    load_env_file "$ENV_FILE"
else
    echo "No environment file found at $ENV_FILE; using in-script defaults"
fi

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

normalize_test_db_env() {
    local requested_driver="${TEST_DB_DRIVER:-postgres}"
    local driver_lower
    driver_lower=$(printf '%s' "$requested_driver" | tr '[:upper:]' '[:lower:]')

    local driver_family driver_value
    case "$driver_lower" in
        postgres|pgsql|pg)
            driver_family="postgres"
            driver_value="postgres"
            ;;
        mysql|mariadb)
            driver_family="mysql"
            driver_value="mysql"
            ;;
        *)
            echo -e "${RED}Unsupported TEST_DB_DRIVER '$requested_driver'. Use 'postgres' or 'mysql'.${NC}" >&2
            exit 1
            ;;
    esac

    local default_name default_user default_password default_host default_port
    if [ "$driver_family" = "postgres" ]; then
        default_name="${TEST_DB_POSTGRES_NAME:-goatflow_test}"
        default_user="${TEST_DB_POSTGRES_USER:-goatflow_user}"
        default_password="${TEST_DB_POSTGRES_PASSWORD:-}"
        default_host="${TEST_DB_POSTGRES_HOST:-postgres-test}"
        if [ "$IN_CONTAINER" = true ]; then
            default_port="${TEST_DB_POSTGRES_INTERNAL_PORT:-5432}"
        else
            default_port="${TEST_DB_POSTGRES_PORT:-5432}"
        fi
    else
        default_name="${TEST_DB_MYSQL_NAME:-otrs_test}"
        default_user="${TEST_DB_MYSQL_USER:-otrs}"
        default_password="${TEST_DB_MYSQL_PASSWORD:-}"
        default_host="${TEST_DB_MYSQL_HOST:-mariadb-test}"
        if [ "$IN_CONTAINER" = true ]; then
            default_port="${TEST_DB_MYSQL_INTERNAL_PORT:-3306}"
        else
            default_port="${TEST_DB_MYSQL_PORT:-3306}"
        fi
    fi

    local name="${TEST_DB_NAME:-$default_name}"
    if [[ "$name" != *_test ]]; then
        echo -e "${YELLOW}TEST_DB_NAME '$name' does not end with '_test'; enforcing suffix${NC}"
        name="${name}_test"
    fi

    local host="${TEST_DB_HOST:-$default_host}"
    local port="${TEST_DB_PORT:-$default_port}"
    local user="${TEST_DB_USER:-$default_user}"
    local password="${TEST_DB_PASSWORD:-$default_password}"

    if [ -z "$password" ]; then
        echo -e "${RED}ERROR: TEST_DB_PASSWORD (or TEST_DB_MYSQL_PASSWORD/TEST_DB_POSTGRES_PASSWORD) must be set in .env${NC}" >&2
        exit 1
    fi

    export TEST_DB_DRIVER="$driver_value"
    export TEST_DB_NAME="$name"
    export TEST_DB_HOST="$host"
    export TEST_DB_PORT="$port"
    export TEST_DB_USER="$user"
    export TEST_DB_PASSWORD="$password"
    export APP_ENV="${APP_ENV:-test}"

    export DB_DRIVER="$TEST_DB_DRIVER"
    export DB_NAME="$TEST_DB_NAME"
    export DB_HOST="$TEST_DB_HOST"
    export DB_PORT="$TEST_DB_PORT"
    export DB_USER="$TEST_DB_USER"
    export DB_PASSWORD="$TEST_DB_PASSWORD"
}

verify_db_env_alignment() {
    local key
    for key in DRIVER HOST PORT NAME USER PASSWORD; do
        local test_var="TEST_DB_${key}"
        local prod_var="DB_${key}"
        if [ "${!test_var}" != "${!prod_var}" ]; then
            echo -e "${RED}Environment mismatch: ${prod_var}='${!prod_var}' differs from ${test_var}='${!test_var}'${NC}" >&2
            echo -e "${RED}Tests must use the TEST_DB_* environment exclusively.${NC}" >&2
            exit 1
        fi
    done
}

resolve_container_db_host() {
    local driver="$1"
    local host="$2"

    if [[ "$driver" == "postgres" ]]; then
        case "$host" in
            localhost|127.0.0.1|::1|postgres)
                echo "postgres-test"
                ;;
            postgres-test)
                echo "postgres-test"
                ;;
            *)
                echo "$host"
                ;;
        esac
    else
        case "$host" in
            localhost|127.0.0.1|::1|mariadb)
                echo "mariadb-test"
                ;;
            mariadb-test)
                echo "mariadb-test"
                ;;
            *)
                echo "$host"
                ;;
        esac
    fi
}

resolve_container_db_port() {
    local driver="$1"
    if [[ "$driver" == "postgres" ]]; then
        echo "5432"
    else
        echo "3306"
    fi
}

echo "======================================"
echo "    GoatFlow Safe Test Runner            "
echo "======================================"

# Detect Docker/Podman compose command
detect_compose_cmd() {
    if command -v podman-compose > /dev/null 2>&1 && command -v podman > /dev/null 2>&1; then
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
COMPOSE_TEST_FILES="-f docker-compose.yml -f docker-compose.testdb.yml"

# Check if running in container or local
if [ -f /.dockerenv ] || [ -f /run/.containerenv ]; then
    IN_CONTAINER=true
    echo "Running in container environment..."
else
    IN_CONTAINER=false
    echo "Running on host via container..."
    echo "Using compose command: $COMPOSE_CMD"
    
    if [ "${SKIP_BACKEND_CHECK:-0}" != "1" ]; then
        # Check if backend service is running
        if ! $COMPOSE_CMD ps --format '{{.Name}} {{.State}}' 2>/dev/null | awk '/backend/{print $2}' | grep -q "running"; then
            echo -e "${RED}Error: Backend service is not running.${NC}"
            echo "Please run 'make up' first to start the services."
            exit 1
        fi
    else
        echo "Skipping backend service check as requested."
    fi
fi

normalize_test_db_env
verify_db_env_alignment

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
    local db_name="$TEST_DB_NAME"

    if [[ "$db_name" != *_test ]]; then
        echo -e "${RED}ERROR: TEST_DB_NAME '$db_name' is invalid. It must end with '_test'.${NC}" >&2
        exit 1
    fi

    echo -e "${GREEN}✓ Using test database: $db_name${NC}"
}

# Safety Check 3: Hostname Check (prevent remote DB access during tests)
check_hostname() {
    local db_host="$TEST_DB_HOST"
    local db_driver="$TEST_DB_DRIVER"

    # Only allow localhost/postgres container for tests
    if [[ "$db_driver" == "postgres" ]]; then
        if [[ "$db_host" != "localhost" ]] && \
           [[ "$db_host" != "127.0.0.1" ]] && \
           [[ "$db_host" != "postgres" ]] && \
           [[ "$db_host" != "postgres-test" ]] && \
           [[ "$db_host" != "::1" ]]; then
            echo -e "${RED}ERROR: Tests can only run against local PostgreSQL databases!${NC}"
            echo -e "${RED}Current DB_HOST: $db_host${NC}"
            echo ""
            echo "For safety, tests are restricted to:"
            echo "- localhost"
            echo "- 127.0.0.1"
            echo "- postgres (container name)"
            echo "- postgres-test (test container name)"
            echo "- ::1 (IPv6 localhost)"
            exit 1
        fi
    else
        if [[ "$db_host" != "localhost" ]] && \
           [[ "$db_host" != "127.0.0.1" ]] && \
           [[ "$db_host" != "mariadb" ]] && \
           [[ "$db_host" != "mariadb-test" ]] && \
           [[ "$db_host" != "::1" ]]; then
            echo -e "${RED}ERROR: Tests can only run against local databases!${NC}"
            echo -e "${RED}Current DB_HOST: $db_host${NC}"
            echo ""
            echo "For safety, tests are restricted to:"
            echo "- localhost"
            echo "- 127.0.0.1"
            echo "- mariadb (container name)"
            echo "- mariadb-test (test container name)"
            echo "- ::1 (IPv6 localhost)"
            exit 1
        fi
    fi
    
    echo -e "${GREEN}✓ Database host check passed (TEST_DB_HOST=$db_host)${NC}"
}

# Safety Check 4: Port Check (warn if using non-standard port)
check_port() {
    local db_driver="$TEST_DB_DRIVER"
    local db_port="$TEST_DB_PORT"
    local allowed_ports=""

    if [[ "$db_driver" == "postgres" ]]; then
        allowed_ports="5432 ${TEST_DB_POSTGRES_PORT:-}"
    else
        allowed_ports="3306 ${TEST_DB_MYSQL_PORT:-}"
    fi

    local port_ok=false
    for candidate in $allowed_ports; do
        if [[ -n "$candidate" && "$db_port" == "$candidate" ]]; then
            port_ok=true
            break
        fi
    done

    if [[ "$port_ok" != true ]]; then
        echo -e "${YELLOW}Notice: Using non-standard ${db_driver} port: $db_port${NC}"
        if [[ "${DB_SAFE_ASSUME_YES:-0}" =~ ^(1|y|Y|true|TRUE)$ ]]; then
            echo "Auto-confirm enabled (DB_SAFE_ASSUME_YES). Continuing."
        else
            read -p "Continue with tests? (y/N) " -n 1 -r
            echo
            if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                echo "Tests cancelled."
                exit 1
            fi
        fi
    else
        echo -e "${GREEN}✓ Using approved ${db_driver} port: $db_port${NC}"
    fi
}

# Safety Check 5: Create test database if needed
ensure_test_database() {
    local db_driver="$TEST_DB_DRIVER"
    local db_name="$TEST_DB_NAME"
    local db_user="$TEST_DB_USER"
    local db_pass="$TEST_DB_PASSWORD"
    local db_host="$TEST_DB_HOST"
    local connect_host
    local connect_port
    
    echo ""
    echo "Ensuring test database exists..."

    if [[ "$db_driver" != "postgres" ]]; then
        echo -e "${GREEN}✓ MariaDB test database handled by container init (DB_NAME=$db_name)${NC}"
        return
    fi

    connect_host=$(resolve_container_db_host "$db_driver" "$db_host")
    connect_port=$(resolve_container_db_port "$db_driver")
    
    if [ "$IN_CONTAINER" = true ]; then
        # Running in container - can use psql directly
        PGPASSWORD="$db_pass" psql -h "$connect_host" -p "$connect_port" -U "$db_user" -d postgres -tc \
            "SELECT 1 FROM pg_database WHERE datname = '$db_name'" | grep -q 1 || \
        PGPASSWORD="$db_pass" psql -h "$connect_host" -p "$connect_port" -U "$db_user" -d postgres -c \
            "CREATE DATABASE \"$db_name\"" && \
        echo -e "${GREEN}✓ Created test database: $db_name${NC}" || \
        echo -e "${GREEN}✓ Test database already exists: $db_name${NC}"
    else
        # Running on host - use docker compose exec to run psql in postgres container
        ensure_postgres_service
        $COMPOSE_CMD $COMPOSE_TEST_FILES exec -e PGPASSWORD="$db_pass" postgres-test psql -p "$connect_port" -U "$db_user" -d postgres -tc \
            "SELECT 1 FROM pg_database WHERE datname = '$db_name'" | grep -q 1 || \
        $COMPOSE_CMD $COMPOSE_TEST_FILES exec -e PGPASSWORD="$db_pass" postgres-test psql -p "$connect_port" -U "$db_user" -d postgres -c \
            "CREATE DATABASE \"$db_name\"" && \
        echo -e "${GREEN}✓ Created test database: $db_name${NC}" || \
        echo -e "${GREEN}✓ Test database already exists: $db_name${NC}"
    fi
}

# Ensure the postgres-test container is running when tests execute on the host
ensure_postgres_service() {
    if [ "$IN_CONTAINER" = true ]; then
        echo -e "${BLUE}Running inside container; assuming postgres-test is reachable...${NC}"
        return
    fi

    echo -e "${YELLOW}Ensuring postgres-test container is running...${NC}"
    APP_ENV=test TEST_DB_DRIVER=postgres TEST_DB_NAME="$TEST_DB_NAME" TEST_DB_USER="$TEST_DB_USER" TEST_DB_PASSWORD="$TEST_DB_PASSWORD" TEST_DB_HOST="$TEST_DB_HOST" TEST_DB_PORT="$TEST_DB_PORT" \
        $COMPOSE_CMD $COMPOSE_TEST_FILES up -d postgres-test >/dev/null

    echo -n "Waiting for postgres-test to accept connections"
    for _ in {1..40}; do
        if $COMPOSE_CMD $COMPOSE_TEST_FILES exec -T postgres-test pg_isready -U "$TEST_DB_USER" -d "$TEST_DB_NAME" >/dev/null 2>&1; then
            echo -e "\r${GREEN}✓ postgres-test is ready${NC}          "
            return
        fi
        printf '.'
        sleep 1
    done

    echo -e "\n${RED}postgres-test did not become ready in time${NC}"
    exit 1
}

ensure_mariadb_service() {
    if [ "$IN_CONTAINER" = true ]; then
        echo -e "${BLUE}Running inside container; assuming mariadb-test is reachable...${NC}"
        return
    fi

    echo -e "${YELLOW}Ensuring mariadb-test container is running...${NC}"
    APP_ENV=test TEST_DB_DRIVER=mysql TEST_DB_NAME="$TEST_DB_NAME" TEST_DB_USER="$TEST_DB_USER" TEST_DB_PASSWORD="$TEST_DB_PASSWORD" TEST_DB_HOST="$TEST_DB_HOST" TEST_DB_PORT="$TEST_DB_PORT" \
        $COMPOSE_CMD $COMPOSE_TEST_FILES up -d mariadb-test >/dev/null

    echo -n "Waiting for mariadb-test to accept connections"
    for _ in {1..60}; do
    if $COMPOSE_CMD $COMPOSE_TEST_FILES exec -T mariadb-test mariadb-admin --ssl=0 ping -h 127.0.0.1 -P 3306 -u "$TEST_DB_USER" -p"$TEST_DB_PASSWORD" >/dev/null 2>&1; then
            echo -e "\r${GREEN}✓ mariadb-test is ready${NC}          "
            return
        fi
        printf '.'
        sleep 1
    done

    echo -e "\n${RED}mariadb-test did not become ready in time${NC}"
    exit 1
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
    local db_name="$TEST_DB_NAME"
    local db_user="$TEST_DB_USER"
    local db_pass="$TEST_DB_PASSWORD"
    local db_driver="$TEST_DB_DRIVER"
    local db_host="$TEST_DB_HOST"
    local db_port="$TEST_DB_PORT"
    local connect_host
    local connect_port
    
    if [[ ! "$db_name" =~ _test$ ]]; then
        echo -e "${RED}ERROR: Will not clean non-test database: $db_name${NC}"
        return 1
    fi

    if [[ "$db_driver" != "postgres" ]]; then
        echo -e "${YELLOW}MariaDB test database cleanup is handled by recreating the container.${NC}"
        echo "Use 'make test-db-down && make test-db-up' to reset the schema."
        return 0
    fi
    
    echo -e "${YELLOW}Cleaning test database: $db_name${NC}"
    echo "This will drop and recreate the test database."
    read -p "Continue? (y/N) " -n 1 -r
    echo
    
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        # Drop and recreate
        if [ "$IN_CONTAINER" = true ]; then
            connect_host=$(resolve_container_db_host "$db_driver" "$db_host")
            connect_port=$(resolve_container_db_port "$db_driver")
            PGPASSWORD="$db_pass" psql -h "$connect_host" -p "$connect_port" \
                -U "$db_user" -d postgres -c \
                "DROP DATABASE IF EXISTS \"$db_name\"; CREATE DATABASE \"$db_name\";"
        else
            connect_port=$(resolve_container_db_port "$db_driver")
            $COMPOSE_CMD $COMPOSE_TEST_FILES exec -e PGPASSWORD="$db_pass" postgres-test psql \
                -U "$db_user" -p "$connect_port" -d postgres -c \
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
        if [[ "$TEST_DB_DRIVER" == "postgres" ]]; then
            ensure_postgres_service
        else
            ensure_mariadb_service
        fi
        local toolbox_host
        local toolbox_port
        toolbox_host=$(resolve_container_db_host "$TEST_DB_DRIVER" "$TEST_DB_HOST")
        toolbox_port=$(resolve_container_db_port "$TEST_DB_DRIVER")
        local test_command='mkdir -p generated && go test -v -race -coverprofile=generated/coverage.out -covermode=atomic ./...'
        if echo "$COMPOSE_CMD" | grep -q "podman-compose"; then
            COMPOSE_PROFILES=toolbox $COMPOSE_CMD $COMPOSE_TEST_FILES run --rm \
                -e APP_ENV="$APP_ENV" \
                -e TEST_DB_NAME="$TEST_DB_NAME" \
                -e TEST_DB_HOST="$toolbox_host" \
                -e TEST_DB_DRIVER="$TEST_DB_DRIVER" \
                -e TEST_DB_PORT="$toolbox_port" \
                -e TEST_DB_USER="$TEST_DB_USER" \
                -e TEST_DB_PASSWORD="$TEST_DB_PASSWORD" \
                -e DB_NAME="$DB_NAME" \
                -e DB_HOST="$toolbox_host" \
                -e DB_DRIVER="$DB_DRIVER" \
                -e DB_PORT="$toolbox_port" \
                -e DB_USER="$DB_USER" \
                -e DB_PASSWORD="$DB_PASSWORD" \
                toolbox bash -c "$test_command"
        else
            $COMPOSE_CMD $COMPOSE_TEST_FILES --profile toolbox run --rm \
                -e APP_ENV="$APP_ENV" \
                -e TEST_DB_NAME="$TEST_DB_NAME" \
                -e TEST_DB_HOST="$toolbox_host" \
                -e TEST_DB_DRIVER="$TEST_DB_DRIVER" \
                -e TEST_DB_PORT="$toolbox_port" \
                -e TEST_DB_USER="$TEST_DB_USER" \
                -e TEST_DB_PASSWORD="$TEST_DB_PASSWORD" \
                -e DB_NAME="$DB_NAME" \
                -e DB_HOST="$toolbox_host" \
                -e DB_DRIVER="$DB_DRIVER" \
                -e DB_PORT="$toolbox_port" \
                -e DB_USER="$DB_USER" \
                -e DB_PASSWORD="$DB_PASSWORD" \
                toolbox bash -c "$test_command"
        fi
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

        up)
            run_safety_checks
            if [[ "$TEST_DB_DRIVER" == "postgres" ]]; then
                ensure_postgres_service
            else
                ensure_mariadb_service
            fi
            ;;

        down)
            if [ "$IN_CONTAINER" = true ]; then
                echo -e "${YELLOW}Skipping test database shutdown inside container${NC}"
            else
                if [[ "$TEST_DB_DRIVER" == "postgres" ]]; then
                    $COMPOSE_CMD $COMPOSE_TEST_FILES stop postgres-test >/dev/null 2>&1 || true
                    $COMPOSE_CMD $COMPOSE_TEST_FILES rm -f postgres-test >/dev/null 2>&1 || true
                    echo -e "${GREEN}✓ postgres-test container stopped${NC}"
                else
                    $COMPOSE_CMD $COMPOSE_TEST_FILES stop mariadb-test >/dev/null 2>&1 || true
                    $COMPOSE_CMD $COMPOSE_TEST_FILES rm -f mariadb-test >/dev/null 2>&1 || true
                    echo -e "${GREEN}✓ mariadb-test container stopped${NC}"
                fi
            fi
            ;;
            
        *)
            echo "Usage: $0 [test|clean|check|up|down]"
            echo "  test  - Run tests with safety checks (default)"
            echo "  clean - Clean test database"
            echo "  check - Run safety checks only"
            echo "  up    - Start the selected test database container after safety checks"
            echo "  down  - Stop the selected test database container"
            exit 1
            ;;
    esac
}

# Show current configuration
echo "Current test configuration:"
echo "  APP_ENV: $APP_ENV"
echo "  TEST_DB_DRIVER: $TEST_DB_DRIVER"
echo "  TEST_DB_NAME: $TEST_DB_NAME"
echo "  TEST_DB_HOST: $TEST_DB_HOST"
echo "  TEST_DB_PORT: $TEST_DB_PORT"
echo "  TEST_DB_USER: $TEST_DB_USER"

# Legacy variables retained for compatibility; ensure they mirror TEST_DB_*
echo "  DB_DRIVER: $DB_DRIVER"
echo "  DB_NAME: $DB_NAME"
echo "  DB_HOST: $DB_HOST"
echo "  DB_PORT: $DB_PORT"
echo "  DB_USER: $DB_USER"

# Run main function
main "$@"