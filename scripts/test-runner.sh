#!/bin/bash
#
# COMPREHENSIVE TEST RUNNER
# Orchestrates all test suites with formatted output and logging
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Test counters
UNIT_PACKAGES_PASSED=0
UNIT_PACKAGES_FAILED=0
UNIT_TESTS_RUN=0
UNIT_TESTS_PASSED=0
UNIT_TESTS_FAILED=0
E2E_PASSED=0
E2E_FAILED=0
E2E_SKIPPED=0

# Configuration
LOG_DIR="/tmp/gotrs-test-$(date +%Y%m%d_%H%M%S)"
MAIN_LOG="$LOG_DIR/test-comprehensive.log"
UNIT_LOG="$LOG_DIR/unit-tests.log"
E2E_LOG="$LOG_DIR/e2e-tests.log"
CONTAINER_LOG="$LOG_DIR/container-backend.log"

# Create log directory
mkdir -p "$LOG_DIR"

# Logging functions
log() {
    echo -e "${BLUE}[$(date +%H:%M:%S)]${NC} $1" | tee -a "$MAIN_LOG"
}

step() {
    echo "" | tee -a "$MAIN_LOG"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}" | tee -a "$MAIN_LOG"
    echo -e "${CYAN}  $1${NC}" | tee -a "$MAIN_LOG"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}" | tee -a "$MAIN_LOG"
}

success() {
    echo -e "  ${GREEN}✓${NC} $1" | tee -a "$MAIN_LOG"
}

fail() {
    echo -e "  ${RED}✗${NC} $1" | tee -a "$MAIN_LOG"
}

skip() {
    echo -e "  ${YELLOW}○${NC} $1 (skipped)" | tee -a "$MAIN_LOG"
}

warning() {
    echo -e "  ${YELLOW}⚠${NC} $1" | tee -a "$MAIN_LOG"
}

#########################################
# BANNER
#########################################
echo ""
echo -e "${CYAN}╔══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║          GOTRS COMPREHENSIVE TEST SUITE                      ║${NC}"
echo -e "${CYAN}╚══════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo "  Started:   $(date '+%Y-%m-%d %H:%M:%S')"
echo "  Log dir:   $LOG_DIR"
echo ""

#########################################
# 0. STATIC VALIDATION (FAIL FAST)
#########################################
step "1/6  Static Validation"

# Validate no hardcoded routes in htmx_routes.go
if sh scripts/validate_routes.sh >> "$MAIN_LOG" 2>&1; then
    success "No hardcoded routes (all routes in YAML)"
else
    fail "Hardcoded routes detected - run ./scripts/validate_routes.sh for details"
    exit 1
fi

#########################################
# 1. ENVIRONMENT CHECK
#########################################
step "2/6  Environment Setup"

# Check compose command
if command -v docker &> /dev/null; then
    COMPOSE_CMD="docker compose"
    success "Docker available"
elif command -v podman &> /dev/null; then
    COMPOSE_CMD="podman-compose"
    success "Podman available"
else
    fail "No container runtime found"
    exit 1
fi

# Check test stack
if curl -s -o /dev/null -w "%{http_code}" "http://localhost:${TEST_BACKEND_PORT:-8082}/health" 2>/dev/null | grep -q "200"; then
    success "Test backend responding on port ${TEST_BACKEND_PORT:-8082}"
else
    warning "Test backend not running - will be started"
fi

#########################################
# 2. START TEST STACK
#########################################
step "3/6  Starting Test Stack"

log "Starting test containers..."
make test-stack-up 2>&1 | tee -a "$MAIN_LOG"
success "Test stack started"

# Capture container logs in background
$COMPOSE_CMD -f docker-compose.yml -f docker-compose.testdb.yml -f docker-compose.test.yaml logs -f backend-test > "$CONTAINER_LOG" 2>&1 &
CONTAINER_LOG_PID=$!
log "Container log capture started (PID: $CONTAINER_LOG_PID)"

# Give it a moment to stabilize
sleep 2

#########################################
# 3. UNIT TESTS
#########################################
step "4/6  Unit Tests"

log "Running Go unit tests..."

# Helper to parse unit test results from log file
parse_unit_test_results() {
    local log_file="$1"

    # Parse package results from output
    UNIT_PACKAGES_PASSED=$(grep -c "^ok" "$log_file" 2>/dev/null || true)
    UNIT_PACKAGES_PASSED=${UNIT_PACKAGES_PASSED:-0}
    UNIT_PACKAGES_FAILED=$(grep -c "^FAIL" "$log_file" 2>/dev/null || true)
    UNIT_PACKAGES_FAILED=${UNIT_PACKAGES_FAILED:-0}

    # Count ALL tests including subtests using "=== RUN" lines
    # This gives the true total of all test cases executed
    UNIT_TESTS_RUN=$(grep -c "^=== RUN" "$log_file" 2>/dev/null || true)
    UNIT_TESTS_RUN=${UNIT_TESTS_RUN:-0}

    # Count failures (--- FAIL: lines indicate actual failures)
    UNIT_TESTS_FAILED=$(grep -c "^--- FAIL:" "$log_file" 2>/dev/null || true)
    UNIT_TESTS_FAILED=${UNIT_TESTS_FAILED:-0}

    # Passed = total run minus failures
    UNIT_TESTS_PASSED=$((UNIT_TESTS_RUN - UNIT_TESTS_FAILED))
}

if make test-unit > "$UNIT_LOG" 2>&1; then
    parse_unit_test_results "$UNIT_LOG"

    if [ "$UNIT_PACKAGES_FAILED" = "0" ] || [ -z "$UNIT_PACKAGES_FAILED" ]; then
        UNIT_PACKAGES_FAILED=0
        if [ "$UNIT_TESTS_FAILED" = "0" ]; then
            success "Unit tests: $UNIT_TESTS_PASSED tests passed ($UNIT_PACKAGES_PASSED packages)"
        else
            fail "Unit tests: $UNIT_TESTS_PASSED passed, $UNIT_TESTS_FAILED failed ($UNIT_PACKAGES_PASSED packages)"
        fi
    else
        fail "Unit tests: $UNIT_TESTS_PASSED passed, $UNIT_TESTS_FAILED failed ($UNIT_PACKAGES_FAILED packages failed)"
    fi
else
    parse_unit_test_results "$UNIT_LOG"
    fail "Unit tests failed: $UNIT_TESTS_PASSED passed, $UNIT_TESTS_FAILED failed - see $UNIT_LOG"
fi

#########################################
# 4. E2E PLAYWRIGHT TESTS
#########################################
step "5/6  E2E Playwright Tests"

log "Running Playwright browser tests..."

# Helper to parse E2E test results from log file
parse_e2e_test_results() {
    local log_file="$1"

    # Check for build failure first
    E2E_BUILD_FAILED=0
    if grep -q "\[build failed\]" "$log_file" 2>/dev/null; then
        E2E_BUILD_FAILED=1
    fi

    # Count ALL tests including subtests using "=== RUN" lines
    E2E_RUN=$(grep -c "^=== RUN" "$log_file" 2>/dev/null || true)
    E2E_RUN=${E2E_RUN:-0}

    # Count failures and skips
    E2E_FAILED=$(grep -c "^--- FAIL:" "$log_file" 2>/dev/null || true)
    E2E_FAILED=${E2E_FAILED:-0}
    E2E_SKIPPED=$(grep -c "^--- SKIP:" "$log_file" 2>/dev/null || true)
    E2E_SKIPPED=${E2E_SKIPPED:-0}

    # Passed = total run minus failures minus skipped
    E2E_PASSED=$((E2E_RUN - E2E_FAILED - E2E_SKIPPED))
    # Guard against negative (shouldn't happen but be safe)
    if [ "$E2E_PASSED" -lt 0 ]; then
        E2E_PASSED=0
    fi
}

if make test-e2e-playwright-go \
    BASE_URL=http://backend-test:8080 \
    PLAYWRIGHT_NETWORK=gotrs-ce_gotrs-network \
    TEST_USERNAME="${TEST_USERNAME:-root@localhost}" \
    TEST_PASSWORD="${TEST_PASSWORD}" > "$E2E_LOG" 2>&1; then

    parse_e2e_test_results "$E2E_LOG"

    if [ "$E2E_FAILED" = "0" ]; then
        success "E2E tests: $E2E_PASSED passed, $E2E_SKIPPED skipped"
    else
        fail "E2E tests: $E2E_PASSED passed, $E2E_FAILED failed, $E2E_SKIPPED skipped"
    fi
else
    parse_e2e_test_results "$E2E_LOG"
    if [ "$E2E_BUILD_FAILED" = "1" ]; then
        fail "E2E tests: BUILD FAILED - see $E2E_LOG"
    else
        fail "E2E tests: $E2E_PASSED passed, $E2E_FAILED failed, $E2E_SKIPPED skipped"
    fi
fi

#########################################
# 5. LOG ANALYSIS
#########################################
step "6/6  Log Analysis"

# Stop container log capture
if [ -n "$CONTAINER_LOG_PID" ]; then
    kill $CONTAINER_LOG_PID 2>/dev/null || true
    wait $CONTAINER_LOG_PID 2>/dev/null || true
fi

# Analyze container logs
if [ -f "$CONTAINER_LOG" ]; then
    ERROR_COUNT=$(grep -c "ERROR\|PANIC\|level=error" "$CONTAINER_LOG" 2>/dev/null || true)
    ERROR_COUNT=${ERROR_COUNT:-0}
    HTTP_500_COUNT=$(grep -c " 500 \| status=500" "$CONTAINER_LOG" 2>/dev/null || true)
    HTTP_500_COUNT=${HTTP_500_COUNT:-0}
    WARNING_COUNT=$(grep -c "WARNING\|level=warn" "$CONTAINER_LOG" 2>/dev/null || true)
    WARNING_COUNT=${WARNING_COUNT:-0}

    if [ "$ERROR_COUNT" = "0" ] || [ -z "$ERROR_COUNT" ]; then
        ERROR_COUNT=0
        success "No ERROR/PANIC in container logs"
    else
        warning "Found $ERROR_COUNT ERROR/PANIC messages in logs"
    fi

    if [ "$HTTP_500_COUNT" = "0" ] || [ -z "$HTTP_500_COUNT" ]; then
        HTTP_500_COUNT=0
        success "No HTTP 500 errors in container logs"
    else
        fail "Found $HTTP_500_COUNT HTTP 500 errors"
    fi

    if [ "$WARNING_COUNT" != "0" ] && [ -n "$WARNING_COUNT" ]; then
        warning "Found $WARNING_COUNT warnings (review $CONTAINER_LOG)"
    fi
else
    warning "Container log file not found"
    ERROR_COUNT=0
    HTTP_500_COUNT=0
fi

#########################################
# SUMMARY
#########################################
echo ""
echo -e "${CYAN}══════════════════════════════════════════════════════════════${NC}"
echo -e "${CYAN}                      TEST SUMMARY                            ${NC}"
echo -e "${CYAN}══════════════════════════════════════════════════════════════${NC}"
echo ""

# Calculate totals
# E2E_RUN is set by parse_e2e_test_results, default to sum if not set
E2E_RUN=${E2E_RUN:-$((E2E_PASSED + E2E_FAILED + E2E_SKIPPED))}
TOTAL_TESTS_RUN=$((UNIT_TESTS_RUN + E2E_RUN))
TOTAL_TESTS_PASSED=$((UNIT_TESTS_PASSED + E2E_PASSED))
TOTAL_TESTS_FAILED=$((UNIT_TESTS_FAILED + E2E_FAILED))
TOTAL_TESTS_SKIPPED=$((E2E_SKIPPED))
TOTAL_PACKAGES_FAILED=$((UNIT_PACKAGES_FAILED))

# Checklist style summary
echo "  Test Results:"
if [ "$UNIT_TESTS_FAILED" = "0" ] && [ "$UNIT_PACKAGES_FAILED" = "0" ]; then
    echo -e "    ${GREEN}[✓]${NC} Unit Tests          ${UNIT_TESTS_PASSED} passed (${UNIT_PACKAGES_PASSED} packages)"
elif [ "$UNIT_PACKAGES_FAILED" = "0" ]; then
    echo -e "    ${RED}[✗]${NC} Unit Tests          ${UNIT_TESTS_PASSED} passed, ${UNIT_TESTS_FAILED} failed (${UNIT_PACKAGES_PASSED} packages)"
else
    echo -e "    ${RED}[✗]${NC} Unit Tests          ${UNIT_TESTS_PASSED} passed, ${UNIT_TESTS_FAILED} failed (${UNIT_PACKAGES_FAILED} packages failed)"
fi

if [ "${E2E_BUILD_FAILED:-0}" = "1" ]; then
    echo -e "    ${RED}[✗]${NC} E2E Playwright      BUILD FAILED"
    # Count build failure as a failure for total
    TOTAL_TESTS_FAILED=$((TOTAL_TESTS_FAILED + 1))
elif [ "$E2E_FAILED" = "0" ]; then
    echo -e "    ${GREEN}[✓]${NC} E2E Playwright      ${E2E_PASSED} passed, ${E2E_SKIPPED} skipped"
else
    echo -e "    ${RED}[✗]${NC} E2E Playwright      ${E2E_PASSED} passed, ${E2E_FAILED} failed, ${E2E_SKIPPED} skipped"
fi

if [ "${ERROR_COUNT:-0}" = "0" ] && [ "${HTTP_500_COUNT:-0}" = "0" ]; then
    echo -e "    ${GREEN}[✓]${NC} Container Health    (no errors detected)"
else
    echo -e "    ${YELLOW}[!]${NC} Container Health    ($ERROR_COUNT errors, $HTTP_500_COUNT 500s)"
fi

echo ""
echo "  ─────────────────────────────────────────────────────────"
echo -e "  ${CYAN}TOTAL:${NC} ${TOTAL_TESTS_PASSED} passed, ${TOTAL_TESTS_FAILED} failed, ${TOTAL_TESTS_SKIPPED} skipped (${TOTAL_TESTS_RUN} tests)"
echo "  ─────────────────────────────────────────────────────────"

# Extract and display translation coverage table
if [ -f "$UNIT_LOG" ]; then
    I18N_TABLE=$(grep -E "(┌|│|└|├|Total:.*languages)" "$UNIT_LOG" 2>/dev/null | sed 's/.*validation_test.go:[0-9]*: //' || true)
    if [ -n "$I18N_TABLE" ]; then
        echo ""
        echo -e "  ${CYAN}Translation Coverage:${NC}"
        echo "$I18N_TABLE" | sed 's/^/    /'
        echo ""
    fi
fi

echo ""
echo "  Evidence Files:"
echo "    • Main log:      $MAIN_LOG"
echo "    • Unit tests:    $UNIT_LOG"
echo "    • E2E tests:     $E2E_LOG"
echo "    • Container log: $CONTAINER_LOG"
echo ""

if [ "$TOTAL_TESTS_FAILED" = "0" ] && [ "$TOTAL_PACKAGES_FAILED" = "0" ]; then
    echo -e "  ${GREEN}══════════════════════════════════════════════════════════${NC}"
    echo -e "  ${GREEN}       ALL ${TOTAL_TESTS_PASSED} TESTS PASSED                           ${NC}"
    echo -e "  ${GREEN}══════════════════════════════════════════════════════════${NC}"
    echo ""
    exit 0
else
    echo -e "  ${RED}══════════════════════════════════════════════════════════${NC}"
    if [ "$TOTAL_PACKAGES_FAILED" != "0" ] && [ "$TOTAL_TESTS_FAILED" = "0" ]; then
        echo -e "  ${RED}       ${TOTAL_PACKAGES_FAILED} PACKAGES FAILED TO BUILD                  ${NC}"
    elif [ "$TOTAL_PACKAGES_FAILED" != "0" ]; then
        echo -e "  ${RED}       ${TOTAL_TESTS_FAILED} TESTS FAILED, ${TOTAL_PACKAGES_FAILED} PACKAGES FAILED       ${NC}"
    else
        echo -e "  ${RED}       ${TOTAL_TESTS_FAILED} TESTS FAILED                                 ${NC}"
    fi
    echo -e "  ${RED}══════════════════════════════════════════════════════════${NC}"
    echo ""

    # Show failed package details (build errors)
    if [ "$TOTAL_PACKAGES_FAILED" != "0" ]; then
        echo "  Failed packages (build errors):"
        grep -E "^FAIL\s+github.com" "$UNIT_LOG" 2>/dev/null | head -10 | sed 's/^/    /' || true
        echo ""
    fi

    # Show failed test details
    if [ "$UNIT_TESTS_FAILED" != "0" ]; then
        echo "  Failed Unit tests:"
        grep "^--- FAIL:" "$UNIT_LOG" 2>/dev/null | head -10 | sed 's/^/    /' || true
        echo ""
    fi

    if [ "$E2E_FAILED" != "0" ]; then
        echo "  Failed E2E tests:"
        grep "^--- FAIL:" "$E2E_LOG" 2>/dev/null | head -10 | sed 's/^/    /' || true
        echo ""
    fi

    exit 1
fi
