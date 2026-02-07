#!/bin/bash
# GoatFlow Test Runner Script
# This script runs all tests and generates a coverage report
# Works both inside containers and on the host (using docker compose exec)

echo "======================================"
echo "    GoatFlow Unit Test Coverage Report   "
echo "======================================"
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

shopt -s extglob

# Detect Docker/Podman compose command
detect_compose_cmd() {
    if command -v docker > /dev/null 2>&1 && docker compose version > /dev/null 2>&1; then
        echo "docker compose"
        return
    fi

    if command -v docker-compose > /dev/null 2>&1; then
        echo "docker-compose"
        return
    fi

    if command -v podman > /dev/null 2>&1 && podman compose version > /dev/null 2>&1; then
        echo "podman compose"
        return
    fi

    if command -v podman-compose > /dev/null 2>&1 && podman --version > /dev/null 2>&1; then
        echo "podman-compose"
        return
    fi

    echo "docker compose"
}

COMPOSE_CMD=$(detect_compose_cmd)
COMPOSE_TEST_FILES="-f docker-compose.yml -f docker-compose.testdb.yml"

# Check if running in container or local
if [ -f /.dockerenv ] || [ -f /run/.containerenv ]; then
    echo "Running tests in container environment..."
    IN_CONTAINER=true
else
    echo "Running tests on host via container..."
    echo "Using compose command: $COMPOSE_CMD"
    IN_CONTAINER=false
fi

echo ""

# Load environment variables from .env if present (respect existing exports)
load_env_file() {
    local env_file="${ENV_FILE:-.env}"
    if [ ! -f "$env_file" ]; then
        return
    fi

    while IFS= read -r line || [ -n "$line" ]; do
        line="${line%$'\r'}"
        case "$line" in
            ''|\#*) continue ;;
        esac

        if [[ "$line" == *=* ]]; then
            local key="${line%%=*}"
            local value="${line#*=}"

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

            if [[ "$key" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] && [[ -z "${!key+x}" ]]; then
                export "$key=$value"
            fi
        fi
    done < "$env_file"
}

load_env_file

# Resolve container-friendly hostnames for toolbox runs
resolve_container_db_host() {
    local driver="$1"
    local host="$2"

    case "$driver" in
        postgres)
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
            ;;
        mysql)
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
            ;;
        *)
            echo "$host"
            ;;
    esac
}

resolve_container_db_port() {
    local driver="$1"
    if [[ "$driver" == "postgres" ]]; then
        echo "5432"
    else
        echo "3306"
    fi
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

ensure_postgres_service() {
    if [ "$IN_CONTAINER" = true ]; then
        return
    fi

    echo "Ensuring postgres-test service is available..."
    APP_ENV=test TEST_DB_DRIVER=postgres \
        $COMPOSE_CMD $COMPOSE_TEST_FILES up -d postgres-test >/dev/null

    echo -n "Waiting for postgres-test to accept connections"
    for _ in {1..40}; do
        if $COMPOSE_CMD $COMPOSE_TEST_FILES exec -T postgres-test pg_isready -U "${TEST_DB_USER}" -d "${TEST_DB_NAME}" >/dev/null 2>&1; then
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
        return
    fi

    echo "Ensuring mariadb-test service is available..."
    APP_ENV=test TEST_DB_DRIVER=mysql \
        $COMPOSE_CMD $COMPOSE_TEST_FILES up -d mariadb-test >/dev/null

    echo -n "Waiting for mariadb-test to accept connections"
    for _ in {1..60}; do
        if $COMPOSE_CMD $COMPOSE_TEST_FILES exec -T mariadb-test mariadb-admin --ssl=0 ping -h 127.0.0.1 -P 3306 -u "${TEST_DB_USER}" -p"${TEST_DB_PASSWORD}" >/dev/null 2>&1; then
            echo -e "\r${GREEN}✓ mariadb-test is ready${NC}          "
            return
        fi
        printf '.'
        sleep 1
    done

    echo -e "\n${RED}mariadb-test did not become ready in time${NC}"
    exit 1
}

# Normalize test database environment (prefer TEST_* overrides)
initialize_test_db_env() {
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
            echo -e "${RED}Unsupported TEST_DB_DRIVER value: $requested_driver${NC}" >&2
            echo "Valid options: postgres, mysql" >&2
            exit 1
            ;;
    esac

    local default_name default_user default_password default_host default_port
    if [ "$driver_family" = "postgres" ]; then
        default_name="${TEST_DB_POSTGRES_NAME:-goatflow_test}"
        default_user="${TEST_DB_POSTGRES_USER:-goatflow_user}"
        default_password="${TEST_DB_POSTGRES_PASSWORD:-}"
        if [ "$IN_CONTAINER" = true ]; then
            default_host="${TEST_DB_POSTGRES_HOST:-postgres-test}"
            default_port="${TEST_DB_POSTGRES_INTERNAL_PORT:-5432}"
        else
            default_host="${TEST_DB_POSTGRES_HOST:-postgres-test}"
            default_port="${TEST_DB_POSTGRES_PORT:-5432}"
        fi
    else
        default_name="${TEST_DB_MYSQL_NAME:-otrs_test}"
        default_user="${TEST_DB_MYSQL_USER:-otrs}"
        default_password="${TEST_DB_MYSQL_PASSWORD:-}"
        if [ "$IN_CONTAINER" = true ]; then
            default_host="${TEST_DB_MYSQL_HOST:-mariadb-test}"
            default_port="${TEST_DB_MYSQL_INTERNAL_PORT:-3306}"
        else
            default_host="${TEST_DB_MYSQL_HOST:-mariadb-test}"
            default_port="${TEST_DB_MYSQL_PORT:-3306}"
        fi
    fi

    local name="${TEST_DB_NAME:-$default_name}"
    if [[ "$name" != *_test ]]; then
        echo -e "${YELLOW}TEST_DB_NAME '$name' does not end with '_test'; enforcing test suffix${NC}"
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

    export DB_DRIVER="$TEST_DB_DRIVER"
    export DB_NAME="$TEST_DB_NAME"
    export DB_HOST="$TEST_DB_HOST"
    export DB_PORT="$TEST_DB_PORT"
    export DB_USER="$TEST_DB_USER"
    export DB_PASSWORD="$TEST_DB_PASSWORD"
    export APP_ENV="${APP_ENV:-test}"
}

initialize_test_db_env
verify_db_env_alignment

CONTAINER_DB_HOST=$(resolve_container_db_host "$TEST_DB_DRIVER" "$TEST_DB_HOST")
CONTAINER_DB_PORT=$(resolve_container_db_port "$TEST_DB_DRIVER")
if [ "$IN_CONTAINER" = true ]; then
    export TEST_DB_HOST="$CONTAINER_DB_HOST"
    export DB_HOST="$CONTAINER_DB_HOST"
    export TEST_DB_PORT="$CONTAINER_DB_PORT"
    export DB_PORT="$CONTAINER_DB_PORT"
fi

# Function to run commands either directly or via docker compose exec
run_command() {
    local cmd="$*"
    if [ "$IN_CONTAINER" = true ]; then
        eval "$cmd"
    else
        $COMPOSE_CMD $COMPOSE_TEST_FILES --profile toolbox run --rm \
            -e APP_ENV=test \
            -e TEST_DB_DRIVER="$TEST_DB_DRIVER" \
            -e TEST_DB_HOST="$CONTAINER_DB_HOST" \
            -e TEST_DB_PORT="$CONTAINER_DB_PORT" \
            -e TEST_DB_NAME="$TEST_DB_NAME" \
            -e TEST_DB_USER="$TEST_DB_USER" \
            -e TEST_DB_PASSWORD="$TEST_DB_PASSWORD" \
            -e DB_DRIVER="$TEST_DB_DRIVER" \
            -e DB_HOST="$CONTAINER_DB_HOST" \
            -e DB_PORT="$CONTAINER_DB_PORT" \
            -e DB_NAME="$TEST_DB_NAME" \
            -e DB_USER="$TEST_DB_USER" \
            -e DB_PASSWORD="$TEST_DB_PASSWORD" \
            toolbox bash -lc "$cmd"
    fi
}

# Run tests with coverage
echo "Running unit tests with coverage..."
echo "======================================"

# Test all packages
mkdir -p generated
if [ "$IN_CONTAINER" = true ]; then
    run_command "mkdir -p generated && go test -v -race -coverprofile=generated/coverage.out -covermode=atomic ./..."
    TEST_RESULT=$?
else
    if [ "$TEST_DB_DRIVER" = "postgres" ]; then
        ensure_postgres_service
    else
        ensure_mariadb_service
    fi
    run_command "mkdir -p generated && go test -v -race -coverprofile=generated/coverage.out -covermode=atomic ./..."
    TEST_RESULT=$?
fi

# Check if tests passed
if [ $TEST_RESULT -eq 0 ]; then
    echo -e "${GREEN}✓ All tests passed!${NC}"
else
    echo -e "${RED}✗ Some tests failed!${NC}"
    exit 1
fi

echo ""
echo "Coverage Summary:"
echo "=================="

# Generate coverage report
run_command "go tool cover -func=generated/coverage.out | tail -n 1"

echo ""
echo "Detailed Package Coverage:"
echo "========================="

# Show coverage by package
run_command "go tool cover -func=generated/coverage.out | grep -E '^github.com/goatkit/goatflow' | sort"

echo ""
echo "======================================"
echo "To view HTML coverage report:"
if [ "$IN_CONTAINER" = true ]; then
    echo "  go tool cover -html=generated/coverage.out -o generated/coverage.html"
    echo "  Then open generated/coverage.html in a browser"
else
    echo "  make test-html"
    echo "  Then open generated/coverage.html in a browser"
fi
echo "======================================"

# Generate HTML report if requested
if [ "$1" = "--html" ]; then
    echo ""
    echo "Generating HTML coverage report..."
    mkdir -p generated
    run_command "mkdir -p generated && go tool cover -html=generated/coverage.out -o generated/coverage.html"
    echo -e "${GREEN}✓ HTML report generated: generated/coverage.html${NC}"
    if [ "$IN_CONTAINER" = false ]; then
        echo "Coverage HTML written via mounted workspace"
    fi
fi

# Calculate total coverage percentage
if ! COVERAGE_LINE=$(run_command "go tool cover -func=generated/coverage.out | tail -n 1"); then
    echo -e "${RED}Failed to calculate coverage percentage${NC}"
    exit 1
fi
COVERAGE=$(echo "$COVERAGE_LINE" | awk '{print $3}')

echo ""
echo -e "Total Coverage: ${GREEN}${COVERAGE}${NC}"

# Check if coverage meets threshold
THRESHOLD=70.0
COVERAGE_NUM=$(echo $COVERAGE | sed 's/%//')

# Use awk for comparison since bc might not be available
if awk -v cov="$COVERAGE_NUM" -v thresh="$THRESHOLD" 'BEGIN {exit !(cov >= thresh)}'; then
    echo -e "${GREEN}✓ Coverage meets threshold of ${THRESHOLD}%${NC}"
else
    echo -e "${YELLOW}⚠ Coverage ${COVERAGE} is below threshold of ${THRESHOLD}%${NC}"
fi

echo ""
echo "Test Files Created:"
echo "==================="
if [ "$IN_CONTAINER" = true ]; then
    find . -name "*_test.go" -type f | while read file; do
        echo "  ✓ $file"
    done
else
    $COMPOSE_CMD $COMPOSE_TEST_FILES --profile toolbox run --rm toolbox find . -name "*_test.go" -type f | while read file; do
        echo "  ✓ $file"
    done
fi

echo ""
echo "======================================"
echo "    Test Run Complete!                "
echo "======================================"