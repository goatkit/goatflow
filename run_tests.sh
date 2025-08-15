#!/bin/bash
# GOTRS Test Runner Script
# This script runs all tests and generates a coverage report
# Works both inside containers and on the host (using docker compose exec)

echo "======================================"
echo "    GOTRS Unit Test Coverage Report   "
echo "======================================"
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

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
    echo "Running tests in container environment..."
    IN_CONTAINER=true
else
    echo "Running tests on host via container..."
    echo "Using compose command: $COMPOSE_CMD"
    IN_CONTAINER=false
    
    # Check if backend service is running
    if ! $COMPOSE_CMD ps --services --filter "status=running" | grep -q "backend"; then
        echo -e "${RED}Error: Backend service is not running.${NC}"
        echo "Please run 'make up' first to start the services."
        exit 1
    fi
fi

echo ""

# Function to run commands either directly or via docker compose exec
run_command() {
    if [ "$IN_CONTAINER" = true ]; then
        # Run directly in container
        eval "$@"
    else
        # Run via docker compose exec
        $COMPOSE_CMD exec -e DB_NAME=${DB_NAME:-gotrs}_test -e APP_ENV=test backend sh -c "$@"
    fi
}

# Run tests with coverage
echo "Running unit tests with coverage..."
echo "======================================"

# Test all packages
if [ "$IN_CONTAINER" = true ]; then
    mkdir -p generated
    go test -v -race -coverprofile=generated/coverage.out -covermode=atomic ./...
    TEST_RESULT=$?
else
    mkdir -p generated
    $COMPOSE_CMD exec -e DB_NAME=${DB_NAME:-gotrs}_test -e APP_ENV=test backend \
        sh -c "mkdir -p generated && go test -v -race -coverprofile=generated/coverage.out -covermode=atomic ./..."
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
run_command "go tool cover -func=generated/coverage.out | grep -E '^github.com/gotrs-io/gotrs-ce' | sort"

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
    # Ensure generated directory exists
    mkdir -p generated
    
    if [ "$IN_CONTAINER" = true ]; then
        mkdir -p generated
        go tool cover -html=generated/coverage.out -o generated/coverage.html
        echo -e "${GREEN}✓ HTML report generated: generated/coverage.html${NC}"
    else
        # Generate in container, then copy to host
        $COMPOSE_CMD exec backend sh -c "mkdir -p generated && go tool cover -html=generated/coverage.out -o generated/coverage.html"
        $COMPOSE_CMD cp backend:/app/generated/coverage.html ./generated/coverage.html
        echo -e "${GREEN}✓ HTML report generated: generated/coverage.html${NC}"
        echo "Report copied to host: ./generated/coverage.html"
    fi
fi

# Calculate total coverage percentage
if [ "$IN_CONTAINER" = true ]; then
    COVERAGE=$(go tool cover -func=generated/coverage.out | tail -n 1 | awk '{print $3}')
else
    COVERAGE=$($COMPOSE_CMD exec backend sh -c "go tool cover -func=generated/coverage.out | tail -n 1 | awk '{print \$3}'")
fi

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
    $COMPOSE_CMD exec backend find . -name "*_test.go" -type f | while read file; do
        echo "  ✓ $file"
    done
fi

echo ""
echo "======================================"
echo "    Test Run Complete!                "
echo "======================================"