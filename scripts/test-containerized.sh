#!/bin/bash

# Test script to verify Makefile targets work without host dependencies
# This ensures "containers first" principle is maintained

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "🧪 Testing Makefile containerization compliance..."
echo "================================================"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Track failures
FAILED_TESTS=()
PASSED_TESTS=()

# Function to test a command
test_command() {
    local name="$1"
    local cmd="$2"
    local expected_deps="$3"
    
    echo -n "Testing: $name... "
    
    # Check if command uses only allowed dependencies
    if echo "$cmd" | grep -qE "^\s*(go |npm |yarn |pnpm |node |python |pip |cargo |rustc |gcc |g\+\+|clang|make |cmake |gradle |mvn |ant |ruby |gem |perl |php |composer |dotnet |nuget |swift |kotlin |java |javac |scala |sbt |lua |R |julia |crystal |nim |zig |deno |bun )"; then
        # Check if it's wrapped in container execution
        if echo "$cmd" | grep -qE "CONTAINER_CMD|COMPOSE_CMD|docker run|podman run|docker exec|podman exec|docker compose|podman-compose"; then
            echo -e "${GREEN}✓${NC} (containerized)"
            PASSED_TESTS+=("$name")
        else
            echo -e "${RED}✗${NC} (requires $expected_deps but not containerized)"
            FAILED_TESTS+=("$name: requires $expected_deps")
        fi
    else
        echo -e "${GREEN}✓${NC} (no special deps)"
        PASSED_TESTS+=("$name")
    fi
}

# Function to check makefile targets
check_makefile() {
    cd "$PROJECT_ROOT"
    
    echo ""
    echo "Analyzing Makefile targets..."
    echo "-----------------------------"
    
    # Extract all targets and their commands
    while IFS= read -r line; do
        if [[ "$line" =~ ^([a-zA-Z0-9_-]+): ]]; then
            target="${BASH_REMATCH[1]}"
            
            # Skip phony and special targets
            if [[ "$target" == ".PHONY" ]] || [[ "$target" == "help" ]]; then
                continue
            fi
            
            # Get the command for this target
            cmd=$(make -n "$target" 2>/dev/null | head -5 || true)
            
            # Check specific problematic commands
            if echo "$cmd" | grep -q "go run\|go build\|go test" && ! echo "$cmd" | grep -qE "docker|podman|CONTAINER_CMD|COMPOSE_CMD"; then
                test_command "$target" "$cmd" "Go"
            elif echo "$cmd" | grep -q "npm\|yarn\|pnpm\|node" && ! echo "$cmd" | grep -qE "docker|podman|CONTAINER_CMD|COMPOSE_CMD"; then
                test_command "$target" "$cmd" "Node.js"
            elif echo "$cmd" | grep -q "python\|pip" && ! echo "$cmd" | grep -qE "docker|podman|CONTAINER_CMD|COMPOSE_CMD"; then
                test_command "$target" "$cmd" "Python"
            elif echo "$cmd" | grep -q "curl\|wget" && ! echo "$cmd" | grep -qE "docker|podman|CONTAINER_CMD|COMPOSE_CMD|command -v"; then
                test_command "$target" "$cmd" "curl/wget"
            elif echo "$cmd" | grep -q "psql" && ! echo "$cmd" | grep -qE "docker|podman|CONTAINER_CMD|COMPOSE_CMD"; then
                test_command "$target" "$cmd" "PostgreSQL client"
            fi
        fi
    done < Makefile
}

# Function to check for direct tool usage
check_direct_usage() {
    echo ""
    echo "Checking for direct tool usage in Makefile..."
    echo "---------------------------------------------"
    
    # Tools that should always be containerized
    TOOLS=("go" "npm" "yarn" "pnpm" "node" "python" "pip" "cargo" "rustc" "psql" "mysql" "mongo" "redis-cli" "curl" "wget")
    
    for tool in "${TOOLS[@]}"; do
        # Check if tool is used without container wrapper
        if grep -nE "^\s+[^#]*\b$tool\b" Makefile | grep -vE "CONTAINER_CMD|COMPOSE_CMD|docker|podman|command -v|which|echo|#" > /dev/null 2>&1; then
            echo -e "${YELLOW}⚠${NC}  Found potential direct usage of '$tool':"
            grep -nE "^\s+[^#]*\b$tool\b" Makefile | grep -vE "CONTAINER_CMD|COMPOSE_CMD|docker|podman|command -v|which|echo|#" | head -3
        fi
    done
}

# Function to verify container commands are properly formed
check_container_commands() {
    echo ""
    echo "Verifying container commands..."
    echo "-------------------------------"
    
    # Check synthesize commands use containers
    if grep -q "go run cmd/goatflow/main.go" Makefile; then
        # Check if these go commands are inside sh -c (which means they're in a container)
        non_containerized=$(grep "go run cmd/goatflow/main.go" Makefile | grep -v "sh -c" | grep -vE "CONTAINER_CMD|golang:|docker|podman" || true)
        if [ -n "$non_containerized" ]; then
            echo -e "${RED}✗${NC} Found non-containerized Go commands"
            FAILED_TESTS+=("Non-containerized Go execution")
        else
            echo -e "${GREEN}✓${NC} All Go commands are containerized"
            PASSED_TESTS+=("Go containerization")
        fi
    fi
    
    # Check test commands use containers
    if grep -q "go test" Makefile; then
        if grep "go test" Makefile | grep -v "COMPOSE_CMD.*exec.*backend" > /dev/null 2>&1; then
            echo -e "${RED}✗${NC} Found non-containerized test commands"
            FAILED_TESTS+=("Non-containerized tests")
        else
            echo -e "${GREEN}✓${NC} All test commands use containers"
            PASSED_TESTS+=("Test containerization")
        fi
    fi
}

# Run all checks
check_makefile
check_direct_usage
check_container_commands

# Summary
echo ""
echo "======================================="
echo "         TEST SUMMARY"
echo "======================================="
echo -e "${GREEN}Passed:${NC} ${#PASSED_TESTS[@]} tests"
echo -e "${RED}Failed:${NC} ${#FAILED_TESTS[@]} tests"

if [ ${#FAILED_TESTS[@]} -gt 0 ]; then
    echo ""
    echo "Failed tests:"
    for failure in "${FAILED_TESTS[@]}"; do
        echo "  - $failure"
    done
    echo ""
    echo "❌ Containerization compliance check FAILED"
    echo "   Some commands require tools not guaranteed on host"
    echo "   Please wrap these in container execution"
    exit 1
else
    echo ""
    echo "✅ Containerization compliance check PASSED"
    echo "   All commands properly containerized or use only basic tools"
fi

# Additional recommendations
echo ""
echo "Recommendations:"
echo "---------------"
echo "1. Always use \$(CONTAINER_CMD) or \$(COMPOSE_CMD) for tool execution"
echo "2. Prefer 'exec' into running containers over 'run' for performance"
echo "3. Use Alpine-based images for lightweight tool containers"
echo "4. Check for tool availability with 'command -v' before direct use"
echo "5. Provide containerized fallbacks for all operations"