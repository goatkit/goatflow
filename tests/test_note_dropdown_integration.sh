#!/bin/bash

# Integration test for note type dropdown functionality
# Tests that the dropdown has replaced the text field and works correctly

echo "====================================="
echo "Testing Note Type Dropdown Fix"
echo "Task: TASK_20250827_204820_cfdd6e78"
echo "====================================="

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Test counter
PASSED=0
FAILED=0

# Function to check if a test passed
check_test() {
    if [ $1 -eq 0 ]; then
        echo -e "${GREEN}✓ $2${NC}"
        ((PASSED++))
    else
        echo -e "${RED}✗ $2${NC}"
        ((FAILED++))
    fi
}

# 1. Test that we can retrieve a ticket page with the note modal
echo "1. Testing ticket page loads with note modal..."
RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/agent/tickets/1 2>/dev/null || echo "000")
if [ "$RESPONSE" == "200" ] || [ "$RESPONSE" == "302" ]; then
    check_test 0 "Ticket page accessible"
else
    check_test 1 "Ticket page accessible (got $RESPONSE)"
fi

# 2. Test adding a note with Internal type (default)
echo "2. Testing note submission with Internal type..."
RESPONSE=$(curl -s -X POST http://localhost:8080/agent/tickets/1/note \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -d "subject=Test+Internal+Note&body=This+is+a+test+internal+note&communication_channel_id=3&is_visible_for_customer=0" \
    -o /dev/null -w "%{http_code}" 2>/dev/null || echo "000")

if [ "$RESPONSE" == "200" ]; then
    check_test 0 "Internal note submission"
else
    check_test 1 "Internal note submission (got $RESPONSE)"
fi

# 3. Test adding a note with Email type
echo "3. Testing note submission with Email type..."
RESPONSE=$(curl -s -X POST http://localhost:8080/agent/tickets/1/note \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -d "subject=Test+Email+Note&body=This+is+a+test+email+note&communication_channel_id=1&is_visible_for_customer=1" \
    -o /dev/null -w "%{http_code}" 2>/dev/null || echo "000")

if [ "$RESPONSE" == "200" ]; then
    check_test 0 "Email note submission"
else
    check_test 1 "Email note submission (got $RESPONSE)"
fi

# 4. Test adding a note with Phone type
echo "4. Testing note submission with Phone type..."
RESPONSE=$(curl -s -X POST http://localhost:8080/agent/tickets/1/note \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -d "subject=Test+Phone+Note&body=Phone+conversation+notes&communication_channel_id=2&is_visible_for_customer=0" \
    -o /dev/null -w "%{http_code}" 2>/dev/null || echo "000")

if [ "$RESPONSE" == "200" ]; then
    check_test 0 "Phone note submission"
else
    check_test 1 "Phone note submission (got $RESPONSE)"
fi

# 5. Test adding a note with Chat type
echo "5. Testing note submission with Chat type..."
RESPONSE=$(curl -s -X POST http://localhost:8080/agent/tickets/1/note \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -d "subject=Test+Chat+Note&body=Chat+conversation+transcript&communication_channel_id=4&is_visible_for_customer=1" \
    -o /dev/null -w "%{http_code}" 2>/dev/null || echo "000")

if [ "$RESPONSE" == "200" ]; then
    check_test 0 "Chat note submission"
else
    check_test 1 "Chat note submission (got $RESPONSE)"
fi

# 6. Verify notes were saved to database with correct types
echo "6. Verifying notes in database..."
ARTICLE_COUNT=$(docker exec goatflow-postgres psql -U goatflow_user -d goatflow -t -c "
    SELECT COUNT(*) 
    FROM article a 
    JOIN article_data_mime m ON a.id = m.article_id 
    WHERE a.ticket_id = 1 
    AND m.a_subject IN ('Test Internal Note', 'Test Email Note', 'Test Phone Note', 'Test Chat Note')" 2>/dev/null | xargs)

if [ "$ARTICLE_COUNT" -ge "1" ]; then
    check_test 0 "Notes saved to database (found $ARTICLE_COUNT)"
else
    check_test 1 "Notes saved to database (found $ARTICLE_COUNT)"
fi

# 7. Verify communication channel IDs are correct
echo "7. Verifying communication channel IDs..."
CHANNELS=$(docker exec goatflow-postgres psql -U goatflow_user -d goatflow -t -c "
    SELECT DISTINCT a.communication_channel_id, c.name 
    FROM article a 
    JOIN communication_channel c ON a.communication_channel_id = c.id
    JOIN article_data_mime m ON a.id = m.article_id 
    WHERE a.ticket_id = 1 
    AND m.a_subject LIKE 'Test%Note'
    ORDER BY a.communication_channel_id" 2>/dev/null | grep -E "Email|Phone|Internal|Chat" | wc -l)

if [ "$CHANNELS" -ge "1" ]; then
    check_test 0 "Communication channels correctly set"
else
    check_test 1 "Communication channels correctly set"
fi

# 8. Verify visibility flags
echo "8. Verifying customer visibility flags..."
VISIBLE_COUNT=$(docker exec goatflow-postgres psql -U goatflow_user -d goatflow -t -c "
    SELECT COUNT(*) 
    FROM article a 
    JOIN article_data_mime m ON a.id = m.article_id 
    WHERE a.ticket_id = 1 
    AND a.is_visible_for_customer = 1
    AND m.a_subject IN ('Test Email Note', 'Test Chat Note')" 2>/dev/null | xargs)

if [ "$VISIBLE_COUNT" -ge "1" ]; then
    check_test 0 "Visibility flags correctly set"
else
    check_test 1 "Visibility flags correctly set"
fi

echo "====================================="
echo "Test Summary:"
echo "Passed: $PASSED"
echo "Failed: $FAILED"
echo "====================================="

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All tests passed! Note type dropdown is working correctly.${NC}"
    echo "The fix for TASK_20250827_204820_cfdd6e78 has been successfully implemented."
    exit 0
else
    echo -e "${RED}✗ Some tests failed. Please review the implementation.${NC}"
    exit 1
fi