#!/bin/bash

# Test all API implementations from the last 2 days
# This script runs each test file and reports results

set -e

echo "========================================"
echo "Testing All API Implementations"
echo "========================================"
echo ""

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Track results
PASSED=0
FAILED=0
SKIPPED=0

# Function to run a test
run_test() {
    local test_name="$1"
    local test_pattern="$2"
    
    echo -e "${YELLOW}Testing: $test_name${NC}"
    
    # Run test in container
    if docker run --rm \
        -v "$(pwd):/workspace" \
        -w /workspace \
        --network goatflow_goatflow-network \
        "${GO_IMAGE:-golang:1.24.11-alpine}" \
        sh -c "go test -v ./internal/api -run '$test_pattern' -count=1 2>&1" | grep -q "PASS"; then
        echo -e "${GREEN}✓ $test_name: PASSED${NC}"
        ((PASSED++))
    else
        echo -e "${RED}✗ $test_name: FAILED${NC}"
        ((FAILED++))
    fi
    echo ""
}

# Test each API component
echo "1. Queue Management API"
run_test "Queue API Tests" "TestQueueAPI"

echo "2. Priority Management API"
run_test "Priority API Tests" "TestPriorityAPI"

echo "3. Article/Comment API"
run_test "Article API Tests" "TestArticleAPI"

echo "4. Search API"
run_test "Search API Tests" "TestSearchAPI"

echo "5. Ticket State API"
run_test "Ticket State API Tests" "TestTicketStateAPI"

echo "6. SLA Management API"
run_test "SLA API Tests" "TestSLAAPI"

echo "7. Statistics API"
run_test "Statistics API Tests" "TestStatisticsAPI"

echo "8. Webhook API"
run_test "Webhook API Tests" "TestWebhookAPI"

# Summary
echo "========================================"
echo "Test Summary"
echo "========================================"
echo -e "${GREEN}Passed: $PASSED${NC}"
echo -e "${RED}Failed: $FAILED${NC}"
echo -e "${YELLOW}Skipped: $SKIPPED${NC}"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed!${NC}"
    exit 1
fi
