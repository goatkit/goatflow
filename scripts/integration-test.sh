#!/bin/bash
#
# INTEGRATION TEST SUITE
# Tests HTTP API functionality with positive and negative cases
# IMPORTANT: Only runs against the dedicated TEST backend (port 18081)
#
# NOTE: Test backend runs with GOATFLOW_DISABLE_TEST_AUTH_BYPASS=0
# This auto-authenticates requests as user_id=1 (Admin), so login
# tests verify endpoint behavior but auth isn't required for API access.
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
PASSED=0
FAILED=0
SKIPPED=0
ERRORS=()

# Configuration - ONLY test backend, never dev/prod
TEST_BACKEND_PORT="${TEST_BACKEND_PORT:-18081}"
BASE_URL="http://localhost:${TEST_BACKEND_PORT}"
ADMIN_EMAIL="${DEMO_ADMIN_EMAIL:-root@localhost}"
ADMIN_PASSWORD="${DEMO_ADMIN_PASSWORD:-}"

if [ -z "$ADMIN_PASSWORD" ]; then
    echo -e "${RED}ERROR: DEMO_ADMIN_PASSWORD must be set in .env${NC}"
    exit 1
fi

LOG_FILE="/tmp/integration_test_$(date +%Y%m%d_%H%M%S).log"
COOKIES="/tmp/integration_test_cookies.txt"

# Safety check - refuse to run against port 8080 (dev/prod)
if [ "$TEST_BACKEND_PORT" = "8080" ]; then
    echo -e "${RED}ERROR: Refusing to run tests against port 8080 (dev/prod instance)${NC}"
    echo "Tests must run against the dedicated test backend (default: 18081)"
    echo "Start test stack with: make test-stack-up"
    exit 1
fi

# Logging functions
log() {
    echo -e "${BLUE}[$(date +%H:%M:%S)]${NC} $1" | tee -a "$LOG_FILE"
}

step() {
    echo -e "\n${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${CYAN}  $1${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

success() {
    echo -e "  ${GREEN}✓${NC} $1" | tee -a "$LOG_FILE"
    ((PASSED++))
}

fail() {
    echo -e "  ${RED}✗${NC} $1" | tee -a "$LOG_FILE"
    ((FAILED++))
    ERRORS+=("$1")
}

skip() {
    echo -e "  ${YELLOW}○${NC} $1 (skipped)" | tee -a "$LOG_FILE"
    ((SKIPPED++))
}

warning() {
    echo -e "  ${YELLOW}⚠${NC} $1" | tee -a "$LOG_FILE"
}

# Test HTTP response
test_http() {
    local test_name=$1
    local method=$2
    local url=$3
    local expected_code=$4
    local data=$5
    local check_content=$6
    
    local curl_opts="-s -w \n%{http_code}"
    
    case "$method" in
        GET)
            RESPONSE=$(curl $curl_opts -b "$COOKIES" "$BASE_URL$url" 2>/dev/null)
            ;;
        POST)
            RESPONSE=$(curl $curl_opts -b "$COOKIES" -c "$COOKIES" -X POST -d "$data" "$BASE_URL$url" 2>/dev/null)
            ;;
        PUT)
            RESPONSE=$(curl $curl_opts -b "$COOKIES" -X PUT -d "$data" "$BASE_URL$url" 2>/dev/null)
            ;;
        DELETE)
            RESPONSE=$(curl $curl_opts -b "$COOKIES" -X DELETE "$BASE_URL$url" 2>/dev/null)
            ;;
        POST_JSON)
            RESPONSE=$(curl $curl_opts -b "$COOKIES" -c "$COOKIES" -X POST -H "Content-Type: application/json" -d "$data" "$BASE_URL$url" 2>/dev/null)
            ;;
    esac
    
    HTTP_CODE=$(echo "$RESPONSE" | tail -1)
    BODY=$(echo "$RESPONSE" | head -n -1)
    
    # Check status code
    if [ "$HTTP_CODE" = "$expected_code" ]; then
        if [ -n "$check_content" ]; then
            if echo "$BODY" | grep -q "$check_content"; then
                success "$test_name"
            else
                fail "$test_name - content check failed (looking for: $check_content)"
                echo "Response: $BODY" >> "$LOG_FILE"
                return 1
            fi
        else
            success "$test_name"
        fi
    else
        fail "$test_name - expected $expected_code, got $HTTP_CODE"
        echo "Response: $BODY" >> "$LOG_FILE"
        return 1
    fi
    
    echo "$BODY"
}

# Check if server is running
check_server() {
    if ! curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/health" | grep -q "200"; then
        echo -e "${RED}Error: Test backend not responding at $BASE_URL${NC}"
        echo "Start the test stack with: make test-stack-up"
        exit 1
    fi
}

# Clean up
cleanup() {
    rm -f "$COOKIES"
}

trap cleanup EXIT

#########################################
# MAIN TEST EXECUTION
#########################################

echo ""
echo -e "${CYAN}╔══════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║           GoatFlow INTEGRATION TEST SUITE                   ║${NC}"
echo -e "${CYAN}║              (Test Backend Only)                         ║${NC}"
echo -e "${CYAN}╚══════════════════════════════════════════════════════════╝${NC}"
echo ""
echo "  Test URL:  $BASE_URL"
echo "  Log file:  $LOG_FILE"
echo ""

# Verify server is up
check_server
success "Server responding at $BASE_URL"

#########################################
# 1. HEALTH & STATUS
#########################################
step "1/8  Health & Status Endpoints"

test_http "Health endpoint returns 200" \
    "GET" "/health" "200" "" "healthy" > /dev/null

test_http "API status endpoint" \
    "GET" "/api/v1/status" "200" "" "" > /dev/null

#########################################
# 2. AUTHENTICATION
#########################################
step "2/8  Authentication Tests"

# Positive: Valid login
test_http "Valid login with correct credentials" \
    "POST" "/api/auth/login" "200" \
    "email=$ADMIN_EMAIL&password=$ADMIN_PASSWORD" \
    "" > /dev/null

# Negative: Invalid password
rm -f "$COOKIES"
test_http "Reject invalid password" \
    "POST" "/api/auth/login" "401" \
    "email=$ADMIN_EMAIL&password=wrongpassword" \
    "" > /dev/null

# Negative: Missing credentials
test_http "Reject missing email" \
    "POST" "/api/auth/login" "400" \
    "password=test123" \
    "" > /dev/null

# Edge case: SQL injection attempt
test_http "Block SQL injection in login" \
    "POST" "/api/auth/login" "401" \
    "email=admin'--&password=x" \
    "" > /dev/null

# Re-login for subsequent tests
curl -s -X POST "$BASE_URL/api/auth/login" \
    -d "email=$ADMIN_EMAIL&password=$ADMIN_PASSWORD" \
    -c "$COOKIES" > /dev/null 2>&1

#########################################
# 3. GROUPS CRUD
#########################################
step "3/8  Groups CRUD Operations"

# Create group
GROUP_NAME="IntTest_$(date +%s)"
CREATE_RESPONSE=$(test_http "Create new group" \
    "POST" "/admin/groups" "200" \
    "name=$GROUP_NAME&comments=Integration+test&valid_id=1" \
    "success")

GROUP_ID=$(echo "$CREATE_RESPONSE" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)

if [ -n "$GROUP_ID" ]; then
    success "Extracted group ID: $GROUP_ID"
    
    # Read group
    test_http "Read created group" \
        "GET" "/admin/groups/$GROUP_ID" "200" "" "$GROUP_NAME" > /dev/null
    
    # Update group
    test_http "Update group" \
        "PUT" "/admin/groups/$GROUP_ID" "200" \
        "name=$GROUP_NAME&comments=Updated+comment&valid_id=1" \
        "success" > /dev/null
    
    # Delete group
    test_http "Delete group" \
        "DELETE" "/admin/groups/$GROUP_ID" "200" "" "success" > /dev/null
    
    # Verify deletion
    test_http "Verify group deleted (404)" \
        "GET" "/admin/groups/$GROUP_ID" "404" "" "" > /dev/null
else
    skip "Group CRUD tests (could not create group)"
fi

# Negative: Create duplicate
test_http "Reject duplicate group name" \
    "POST" "/admin/groups" "400" \
    "name=admin&comments=Duplicate&valid_id=1" \
    "" > /dev/null

# Negative: Create without name
test_http "Reject group without name" \
    "POST" "/admin/groups" "400" \
    "comments=NoName&valid_id=1" \
    "" > /dev/null

#########################################
# 4. SECURITY TESTS
#########################################
step "4/8  Security Tests"

# Test unauthenticated access
rm -f "$COOKIES"
UNAUTH_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/admin/groups")
if [ "$UNAUTH_CODE" = "303" ] || [ "$UNAUTH_CODE" = "302" ] || [ "$UNAUTH_CODE" = "401" ]; then
    success "Protected routes require authentication (got $UNAUTH_CODE)"
else
    fail "Protected routes accessible without auth (got $UNAUTH_CODE)"
fi

# Re-login
curl -s -X POST "$BASE_URL/api/auth/login" \
    -d "email=$ADMIN_EMAIL&password=$ADMIN_PASSWORD" \
    -c "$COOKIES" > /dev/null 2>&1

# XSS prevention
XSS_NAME="XSSTest_$(date +%s)"
XSS_RESPONSE=$(curl -s -b "$COOKIES" -X POST "$BASE_URL/admin/groups" \
    -d "name=$XSS_NAME&comments=<script>alert(1)</script>&valid_id=1")

if echo "$XSS_RESPONSE" | grep -q "<script>alert"; then
    fail "XSS: Script tags not escaped in response"
else
    success "XSS: Script tags properly handled"
fi

#########################################
# 5. LOOKUPS API
#########################################
step "5/8  Lookups API"

test_http "Get queues" \
    "GET" "/api/lookups/queues" "200" "" "" > /dev/null

test_http "Get priorities" \
    "GET" "/api/lookups/priorities" "200" "" "" > /dev/null

test_http "Get types" \
    "GET" "/api/lookups/types" "200" "" "" > /dev/null

test_http "Get statuses" \
    "GET" "/api/lookups/statuses" "200" "" "" > /dev/null

test_http "Get form data (combined)" \
    "GET" "/api/lookups/form-data" "200" "" "queues" > /dev/null

#########################################
# 6. PERFORMANCE
#########################################
step "6/8  Performance Checks"

# Test response times
measure_response() {
    local url=$1
    local name=$2
    local threshold=$3
    
    START_TIME=$(date +%s%N)
    curl -s -b "$COOKIES" "$BASE_URL$url" > /dev/null
    END_TIME=$(date +%s%N)
    RESPONSE_TIME=$(( ($END_TIME - $START_TIME) / 1000000 ))
    
    if [ $RESPONSE_TIME -lt $threshold ]; then
        success "$name: ${RESPONSE_TIME}ms (< ${threshold}ms)"
    else
        warning "$name: ${RESPONSE_TIME}ms (target: < ${threshold}ms)"
    fi
}

measure_response "/health" "Health endpoint" 100
measure_response "/api/lookups/form-data" "Lookups API" 300
measure_response "/admin/groups" "Admin groups page" 500

#########################################
# 7. CONCURRENT ACCESS
#########################################
step "7/8  Concurrent Access"

log "Creating 5 groups concurrently..."
CONCURRENT_SUCCESS=0
for i in {1..5}; do
    (
        curl -s -b "$COOKIES" -X POST "$BASE_URL/admin/groups" \
            -d "name=Concurrent_${i}_$(date +%s%N)&comments=Concurrent+test&valid_id=1" \
            > "/tmp/concurrent_$i.log" 2>&1
    ) &
done
wait

for i in {1..5}; do
    if grep -q "success" "/tmp/concurrent_$i.log" 2>/dev/null; then
        ((CONCURRENT_SUCCESS++))
    fi
    rm -f "/tmp/concurrent_$i.log"
done

if [ $CONCURRENT_SUCCESS -ge 4 ]; then
    success "Concurrent operations: $CONCURRENT_SUCCESS/5 succeeded"
else
    fail "Concurrent operations: only $CONCURRENT_SUCCESS/5 succeeded"
fi

#########################################
# 8. LOG ANALYSIS
#########################################
step "8/8  Log Analysis"

# Check if we can access logs via make target
if command -v make >/dev/null 2>&1 && [ -f Makefile ]; then
    # Use make logs if available
    RECENT_LOGS=$(timeout 5 make logs 2>&1 | tail -100 || echo "")
    
    if [ -n "$RECENT_LOGS" ]; then
        ERROR_COUNT=$(echo "$RECENT_LOGS" | grep -c "ERROR\|PANIC" || true)
        HTTP_500_COUNT=$(echo "$RECENT_LOGS" | grep -c " 500 " || true)
        
        if [ $ERROR_COUNT -eq 0 ]; then
            success "No ERROR/PANIC in recent logs"
        else
            warning "Found $ERROR_COUNT ERROR/PANIC messages in logs"
        fi
        
        if [ $HTTP_500_COUNT -eq 0 ]; then
            success "No HTTP 500 errors in recent logs"
        else
            warning "Found $HTTP_500_COUNT HTTP 500 errors in logs"
        fi
    else
        skip "Log analysis (could not retrieve logs)"
    fi
else
    skip "Log analysis (make not available)"
fi

#########################################
# SUMMARY
#########################################
echo ""
echo -e "${CYAN}╔══════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║                    TEST SUMMARY                          ║${NC}"
echo -e "${CYAN}╚══════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "  ${GREEN}Passed:${NC}  $PASSED"
echo -e "  ${RED}Failed:${NC}  $FAILED"
echo -e "  ${YELLOW}Skipped:${NC} $SKIPPED"
echo ""

if [ $FAILED -gt 0 ]; then
    echo -e "${RED}Failed tests:${NC}"
    for error in "${ERRORS[@]}"; do
        echo "  • $error"
    done
    echo ""
    echo -e "Full log: $LOG_FILE"
    exit 1
else
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
fi
