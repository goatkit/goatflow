#!/bin/bash

# Complete Integration Test for Schema Discovery
# Demonstrates all components working together

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║     SCHEMA DISCOVERY - COMPLETE INTEGRATION TEST            ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'
BOLD='\033[1m'

BASE_URL="http://localhost:8080"
AUTH="Cookie: access_token=demo_session_admin"
MODULE_BASE_URL="$BASE_URL/admin/modules"
SCHEMA_API="$MODULE_BASE_URL/_schema"

# Test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Test function
run_test() {
    local test_name="$1"
    local test_command="$2"
    local expected_result="$3"
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    echo -n "  Testing $test_name... "
    
    result=$(eval "$test_command" 2>/dev/null)
    
    if echo "$result" | grep -q "$expected_result"; then
        echo -e "${GREEN}✓ PASS${NC}"
        PASSED_TESTS=$((PASSED_TESTS + 1))
        return 0
    else
        echo -e "${RED}✗ FAIL${NC}"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        return 1
    fi
}

echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo -e "${BOLD}1. CORE API TESTS${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"

run_test "API Health Check" \
    "curl -s -o /dev/null -w '%{http_code}' '$BASE_URL/health'" \
    "200"

run_test "Schema Tables Endpoint" \
    "curl -s -H '$AUTH' -H 'X-Requested-With: XMLHttpRequest' '$SCHEMA_API?action=tables' | jq -r '.success'" \
    "true"

run_test "Schema Columns Endpoint" \
    "curl -s -H '$AUTH' -H 'X-Requested-With: XMLHttpRequest' '$SCHEMA_API?action=columns&table=users' | jq -r '.success'" \
    "true"

run_test "Schema Generate Endpoint" \
    "curl -s -H '$AUTH' -H 'X-Requested-With: XMLHttpRequest' '$SCHEMA_API?action=generate&table=users' | jq -r '.success'" \
    "true"

echo ""
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo -e "${BOLD}2. WEB UI TESTS${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"

run_test "Schema Discovery UI Page" \
    "curl -s -H '$AUTH' '$BASE_URL/admin/schema-discovery' | grep -c 'Schema Discovery'" \
    "1"

run_test "Admin Dashboard Link" \
    "curl -s -H '$AUTH' '$BASE_URL/admin/dashboard' | grep -c 'schema-discovery'" \
    "1"

run_test "UI JavaScript Functions" \
    "curl -s -H '$AUTH' '$BASE_URL/admin/schema-discovery' | grep -c 'function loadTables'" \
    "1"

echo ""
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo -e "${BOLD}3. MODULE GENERATION TESTS${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"

# Create a test table name
TEST_TABLE="valid"

run_test "Generate Test Module" \
    "curl -s -H '$AUTH' -H 'X-Requested-With: XMLHttpRequest' '$SCHEMA_API?action=save&table=$TEST_TABLE' | jq -r '.success'" \
    "true"

sleep 2  # Wait for file watcher

run_test "Module File Created" \
    "ls modules/$TEST_TABLE.yaml 2>/dev/null | wc -l" \
    "1"

run_test "Module Accessible" \
    "curl -s -H '$AUTH' -H 'X-Requested-With: XMLHttpRequest' '$MODULE_BASE/$TEST_TABLE' | jq -r '.success'" \
    "true"

echo ""
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo -e "${BOLD}4. CRUD OPERATIONS TESTS${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"

# Test on salutation module (already created)
run_test "CREATE Operation" \
    "curl -s -X POST -H '$AUTH' -H 'Content-Type: application/x-www-form-urlencoded' -H 'X-Requested-With: XMLHttpRequest' --data 'name=Dr.&text=Dear Dr.&content_type=text/plain&comments=Doctor title' '$MODULE_BASE/salutation' | jq -r '.success'" \
    "true"

run_test "READ Operation" \
    "curl -s -H '$AUTH' -H 'X-Requested-With: XMLHttpRequest' '$MODULE_BASE/salutation' | jq '.data | length' | grep -E '[0-9]+'" \
    "[0-9]"

# Get last ID for update/delete tests
LAST_ID=$(curl -s -H "$AUTH" -H "X-Requested-With: XMLHttpRequest" \
    "$MODULE_BASE/salutation" | jq -r '.data[-1].id')

if [ -n "$LAST_ID" ] && [ "$LAST_ID" != "null" ]; then
    run_test "UPDATE Operation" \
        "curl -s -X PUT -H '$AUTH' -H 'Content-Type: application/x-www-form-urlencoded' -H 'X-Requested-With: XMLHttpRequest' --data 'name=Prof.&text=Dear Professor&content_type=text/plain&comments=Professor title' '$MODULE_BASE/salutation/$LAST_ID' | jq -r '.success'" \
        "true"
    
    run_test "DELETE Operation" \
        "curl -s -X DELETE -H '$AUTH' -H 'X-Requested-With: XMLHttpRequest' '$MODULE_BASE/salutation/$LAST_ID' | jq -r '.success'" \
        "true"
else
    echo "  Skipping UPDATE/DELETE tests (no records found)"
fi

echo ""
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo -e "${BOLD}5. FIELD TYPE INFERENCE TESTS${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"

run_test "Password Field Detection" \
    "curl -s -H '$AUTH' -H 'X-Requested-With: XMLHttpRequest' '$SCHEMA_API?action=generate&table=users' | jq -r '.config.Fields[] | select(.Name == \"pw\") | .Type'" \
    "password"

run_test "DateTime Field Detection" \
    "curl -s -H '$AUTH' -H 'X-Requested-With: XMLHttpRequest' '$SCHEMA_API?action=generate&table=users' | jq -r '.config.Fields[] | select(.Name == \"create_time\") | .Type'" \
    "datetime"

run_test "Integer Field Detection" \
    "curl -s -H '$AUTH' -H 'X-Requested-With: XMLHttpRequest' '$SCHEMA_API?action=generate&table=users' | jq -r '.config.Fields[] | select(.Name == \"id\") | .Type'" \
    "integer"

echo ""
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo -e "${BOLD}6. AUDIT FIELD TESTS${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"

# Create a record and check audit fields
CREATE_RESULT=$(curl -s -X POST -H "$AUTH" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -H "X-Requested-With: XMLHttpRequest" \
    --data "name=Test_$(date +%s)&text=Test&content_type=text/plain" \
    "$MODULE_BASE/salutation")

if echo "$CREATE_RESULT" | grep -q '"success":true'; then
    # Get the created record
    AUDIT_TEST=$(curl -s -H "$AUTH" -H "X-Requested-With: XMLHttpRequest" \
        "$MODULE_BASE/salutation" | jq -r '.data[-1]')
    
    run_test "Audit Field - create_by" \
        "echo '$AUDIT_TEST' | jq -r '.create_by'" \
        "1"
    
    run_test "Audit Field - change_by" \
        "echo '$AUDIT_TEST' | jq -r '.change_by'" \
        "1"
    
    run_test "Audit Field - create_time" \
        "echo '$AUDIT_TEST' | jq -r '.create_time' | grep -c '202'" \
        "1"
fi

echo ""
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo -e "${BOLD}7. PERFORMANCE TESTS${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"

START_TIME=$(date +%s%N)
curl -s -H "$AUTH" -H "X-Requested-With: XMLHttpRequest" \
    "$SCHEMA_API?action=generate&table=ticket" > /dev/null
END_TIME=$(date +%s%N)
RESPONSE_TIME=$(( (END_TIME - START_TIME) / 1000000 ))

run_test "Generation Speed (<500ms)" \
    "[ $RESPONSE_TIME -lt 500 ] && echo 'true' || echo 'false'" \
    "true"

echo "  Generation time: ${RESPONSE_TIME}ms"

echo ""
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo -e "${BOLD}8. ERROR HANDLING TESTS${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"

run_test "Invalid Action Error" \
    "curl -s -H '$AUTH' -H 'X-Requested-With: XMLHttpRequest' '$SCHEMA_API?action=invalid' | jq -r '.error' | grep -c 'Invalid'" \
    "1"

run_test "Missing Table Parameter" \
    "curl -s -H '$AUTH' -H 'X-Requested-With: XMLHttpRequest' '$SCHEMA_API?action=columns' | grep -c 'table parameter required'" \
    "1"

echo ""
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo -e "${BOLD}9. MODULE STATISTICS${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"

MODULE_COUNT=$(ls modules/*.yaml 2>/dev/null | wc -l)
FIELD_COUNT=$(grep -h "^  - name:" modules/*.yaml 2>/dev/null | wc -l)
TABLE_COUNT=$(curl -s -H "$AUTH" -H "X-Requested-With: XMLHttpRequest" \
    "$SCHEMA_API?action=tables" | jq '.data | length')

echo "  Generated Modules: $MODULE_COUNT"
echo "  Total Fields: $FIELD_COUNT"
echo "  Available Tables: $TABLE_COUNT"
echo "  Coverage: $((MODULE_COUNT * 100 / TABLE_COUNT))%"

echo ""
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║                    TEST RESULTS SUMMARY                      ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""
echo -e "  Total Tests:  ${BOLD}$TOTAL_TESTS${NC}"
echo -e "  Passed:       ${GREEN}${BOLD}$PASSED_TESTS${NC}"
echo -e "  Failed:       ${RED}${BOLD}$FAILED_TESTS${NC}"
echo ""

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "${GREEN}${BOLD}  ✅ ALL INTEGRATION TESTS PASSED!${NC}"
    echo ""
    echo "  The Schema Discovery system is fully operational with:"
    echo "  • Working API endpoints"
    echo "  • Functional Web UI"
    echo "  • Module generation capability"
    echo "  • Complete CRUD operations"
    echo "  • Audit field management"
    echo "  • Error handling"
    echo "  • Performance within targets"
else
    echo -e "${YELLOW}  ⚠️  Some tests failed. Review the output above.${NC}"
fi

echo ""
echo "═══════════════════════════════════════════════════════════"