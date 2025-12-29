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
UNIT_PASSED=0
UNIT_FAILED=0
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
# 1. ENVIRONMENT CHECK
#########################################
step "1/5  Environment Setup"

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
step "2/5  Starting Test Stack"

log "Starting test containers..."
make test-stack-up >> "$MAIN_LOG" 2>&1
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
step "3/5  Unit Tests"

log "Running Go unit tests..."
if make toolbox-test > "$UNIT_LOG" 2>&1; then
    # Parse results from output
    UNIT_PASSED=$(grep -c "^ok" "$UNIT_LOG" 2>/dev/null || true)
    UNIT_PASSED=${UNIT_PASSED:-0}
    UNIT_FAILED=$(grep -c "^FAIL" "$UNIT_LOG" 2>/dev/null || true)
    UNIT_FAILED=${UNIT_FAILED:-0}
    
    if [ "$UNIT_FAILED" = "0" ] || [ -z "$UNIT_FAILED" ]; then
        UNIT_FAILED=0
        success "All unit tests passed ($UNIT_PASSED packages)"
    else
        fail "Unit tests: $UNIT_PASSED passed, $UNIT_FAILED failed"
    fi
else
    fail "Unit tests failed - see $UNIT_LOG"
    UNIT_FAILED=1
fi

#########################################
# 4. E2E PLAYWRIGHT TESTS
#########################################
step "4/5  E2E Playwright Tests"

log "Running Playwright browser tests..."
if make test-e2e-playwright-go \
    BASE_URL=http://backend-test:8080 \
    PLAYWRIGHT_NETWORK=gotrs-ce_gotrs-network \
    DEMO_ADMIN_EMAIL=testuser \
    DEMO_ADMIN_PASSWORD=admin > "$E2E_LOG" 2>&1; then
    
    # Parse results
    E2E_PASSED=$(grep -c "^--- PASS:" "$E2E_LOG" 2>/dev/null || true)
    E2E_PASSED=${E2E_PASSED:-0}
    E2E_FAILED=$(grep -c "^--- FAIL:" "$E2E_LOG" 2>/dev/null || true)
    E2E_FAILED=${E2E_FAILED:-0}
    E2E_SKIPPED=$(grep -c "^--- SKIP:" "$E2E_LOG" 2>/dev/null || true)
    E2E_SKIPPED=${E2E_SKIPPED:-0}
    
    success "E2E tests: $E2E_PASSED passed, $E2E_SKIPPED skipped"
else
    E2E_PASSED=$(grep -c "^--- PASS:" "$E2E_LOG" 2>/dev/null || true)
    E2E_PASSED=${E2E_PASSED:-0}
    E2E_FAILED=$(grep -c "^--- FAIL:" "$E2E_LOG" 2>/dev/null || true)
    E2E_FAILED=${E2E_FAILED:-0}
    E2E_SKIPPED=$(grep -c "^--- SKIP:" "$E2E_LOG" 2>/dev/null || true)
    E2E_SKIPPED=${E2E_SKIPPED:-0}
    
    fail "E2E tests: $E2E_PASSED passed, $E2E_FAILED failed, $E2E_SKIPPED skipped"
fi

#########################################
# 5. LOG ANALYSIS
#########################################
step "5/5  Log Analysis"

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

TOTAL_FAILED=$((UNIT_FAILED + E2E_FAILED))

# Checklist style summary
echo "  Test Results:"
if [ "$UNIT_FAILED" = "0" ]; then
    echo -e "    ${GREEN}[✓]${NC} Unit Tests          ($UNIT_PASSED packages passed)"
else
    echo -e "    ${RED}[✗]${NC} Unit Tests          ($UNIT_FAILED packages failed)"
fi

if [ "$E2E_FAILED" = "0" ]; then
    echo -e "    ${GREEN}[✓]${NC} E2E Playwright      ($E2E_PASSED passed, $E2E_SKIPPED skipped)"
else
    echo -e "    ${RED}[✗]${NC} E2E Playwright      ($E2E_FAILED failed, $E2E_PASSED passed)"
fi

if [ "${ERROR_COUNT:-0}" = "0" ] && [ "${HTTP_500_COUNT:-0}" = "0" ]; then
    echo -e "    ${GREEN}[✓]${NC} Container Health    (no errors detected)"
else
    echo -e "    ${YELLOW}[!]${NC} Container Health    ($ERROR_COUNT errors, $HTTP_500_COUNT 500s)"
fi

echo ""
echo "  Evidence Files:"
echo "    • Main log:      $MAIN_LOG"
echo "    • Unit tests:    $UNIT_LOG"
echo "    • E2E tests:     $E2E_LOG"
echo "    • Container log: $CONTAINER_LOG"
echo ""

if [ "$TOTAL_FAILED" = "0" ]; then
    echo -e "  ${GREEN}══════════════════════════════════════════════════════════${NC}"
    echo -e "  ${GREEN}                    ALL TESTS PASSED                      ${NC}"
    echo -e "  ${GREEN}══════════════════════════════════════════════════════════${NC}"
    echo ""
    exit 0
else
    echo -e "  ${RED}══════════════════════════════════════════════════════════${NC}"
    echo -e "  ${RED}                   TESTS FAILED ($TOTAL_FAILED)                     ${NC}"
    echo -e "  ${RED}══════════════════════════════════════════════════════════${NC}"
    echo ""
    
    # Show failed test details
    if [ "$E2E_FAILED" != "0" ]; then
        echo "  Failed E2E tests:"
        grep "^--- FAIL:" "$E2E_LOG" 2>/dev/null | sed 's/^/    /' || true
        echo ""
    fi
    
    exit 1
fi
