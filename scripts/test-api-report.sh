#!/bin/bash

# Generate a comprehensive test report for all API implementations

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "========================================"
echo "API Test Report - $(date)"
echo "========================================"
echo ""

# Create a temp file for results
RESULTS_FILE="/tmp/api_test_results.txt"
> "$RESULTS_FILE"

# Function to test an API
test_api() {
    local name="$1"
    local pattern="$2"
    
    echo -e "${YELLOW}Testing $name...${NC}"
    
    # Run test and capture output
    if docker run --rm \
        -v "/home/nigel/git/goatkit/goatflow:/workspace" \
        -w /workspace \
        --network goatflow_goatflow-network \
        "${GO_IMAGE:-golang:1.24.11-alpine}" \
        sh -c "go test -v ./internal/api -run '$pattern' -count=1 2>&1" > /tmp/test_output.txt 2>&1; then
        
        # Check if tests actually passed
        if grep -q "PASS" /tmp/test_output.txt && ! grep -q "FAIL" /tmp/test_output.txt; then
            echo -e "${GREEN}✓ $name: PASSED${NC}"
            echo "$name: PASSED" >> "$RESULTS_FILE"
        else
            echo -e "${RED}✗ $name: FAILED${NC}"
            echo "$name: FAILED" >> "$RESULTS_FILE"
            # Show first error
            grep -E "Error|FAIL|panic" /tmp/test_output.txt | head -3
        fi
    else
        echo -e "${RED}✗ $name: COMPILATION FAILED${NC}"
        echo "$name: COMPILATION FAILED" >> "$RESULTS_FILE"
        # Show compilation error
        tail -5 /tmp/test_output.txt
    fi
    echo ""
}

# Test each API
test_api "Queue API" "TestQueueAPI"
test_api "Priority API" "TestPriorityAPI"
test_api "Article API" "TestArticleAPI"
test_api "Search API" "TestSearchAPI"
test_api "Ticket State API" "TestTicketStateAPI"
test_api "SLA API" "TestSLAAPI"
test_api "Statistics API" "TestStatisticsAPI"
test_api "Webhook API" "TestWebhookAPI"

# Generate summary
echo "========================================"
echo "SUMMARY"
echo "========================================"
PASSED=$(grep -c "PASSED" "$RESULTS_FILE")
FAILED=$(grep -c "FAILED" "$RESULTS_FILE")
TOTAL=$((PASSED + FAILED))

echo -e "Total Tests: $TOTAL"
echo -e "${GREEN}Passed: $PASSED${NC}"
echo -e "${RED}Failed: $FAILED${NC}"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All tests passed!${NC}"
else
    echo -e "${RED}✗ Some tests failed${NC}"
    echo ""
    echo "Failed tests:"
    grep "FAILED" "$RESULTS_FILE" | sed 's/: FAILED//'
fi
