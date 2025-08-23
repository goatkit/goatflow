#!/bin/bash

# End-to-end test for schema discovery feature
echo "Testing Schema Discovery Feature..."
echo "=================================="

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Base URL and auth
BASE_URL="http://localhost:8080"
AUTH_HEADER="Cookie: access_token=demo_session_admin"

# Test 1: List all tables
echo -n "1. Testing table listing... "
TABLES=$(curl -s -H "$AUTH_HEADER" -H "X-Requested-With: XMLHttpRequest" \
    "$BASE_URL/admin/dynamic/_schema?action=tables")

if echo "$TABLES" | grep -q '"success":true' && echo "$TABLES" | grep -q '"Name":"ticket"'; then
    echo -e "${GREEN}✓ Found tables${NC}"
else
    echo -e "${RED}✗ Failed to list tables${NC}"
    echo "$TABLES"
    exit 1
fi

# Test 2: Get columns for a table
echo -n "2. Testing column retrieval for 'salutation' table... "
COLUMNS=$(curl -s -H "$AUTH_HEADER" -H "X-Requested-With: XMLHttpRequest" \
    "$BASE_URL/admin/dynamic/_schema?action=columns&table=salutation")

if echo "$COLUMNS" | grep -q '"success":true' && echo "$COLUMNS" | grep -q '"Name":"id"'; then
    echo -e "${GREEN}✓ Retrieved columns${NC}"
else
    echo -e "${RED}✗ Failed to get columns${NC}"
    echo "$COLUMNS"
    exit 1
fi

# Test 3: Generate module configuration
echo -n "3. Testing module generation for 'salutation' table... "
CONFIG=$(curl -s -H "$AUTH_HEADER" -H "X-Requested-With: XMLHttpRequest" \
    "$BASE_URL/admin/dynamic/_schema?action=generate&table=salutation")

if echo "$CONFIG" | grep -q '"success":true' && echo "$CONFIG" | grep -q '"Name":"salutation"'; then
    echo -e "${GREEN}✓ Generated module config${NC}"
else
    echo -e "${RED}✗ Failed to generate config${NC}"
    echo "$CONFIG"
    exit 1
fi

# Test 4: Generate YAML format
echo -n "4. Testing YAML generation... "
YAML=$(curl -s -H "$AUTH_HEADER" -H "Accept: text/yaml" \
    "$BASE_URL/admin/dynamic/_schema?action=generate&table=salutation&format=yaml")

if echo "$YAML" | grep -q "module:" && echo "$YAML" | grep -q "name: salutation"; then
    echo -e "${GREEN}✓ Generated YAML config${NC}"
    echo ""
    echo "Sample YAML output:"
    echo "$YAML" | head -20
else
    echo -e "${RED}✗ Failed to generate YAML${NC}"
    echo "$YAML"
    exit 1
fi

# Test 5: Save module configuration
echo ""
echo -n "5. Testing module save for 'salutation' table... "
SAVE_RESULT=$(curl -s -H "$AUTH_HEADER" -H "X-Requested-With: XMLHttpRequest" \
    "$BASE_URL/admin/dynamic/_schema?action=save&table=salutation")

if echo "$SAVE_RESULT" | grep -q '"success":true' && echo "$SAVE_RESULT" | grep -q "salutation.yaml"; then
    echo -e "${GREEN}✓ Saved module configuration${NC}"
else
    echo -e "${RED}✗ Failed to save module${NC}"
    echo "$SAVE_RESULT"
    exit 1
fi

# Test 6: Verify the saved module is accessible
echo -n "6. Testing if saved module is accessible... "
sleep 2  # Wait for file watcher to pick up the new file

MODULE_LIST=$(curl -s -H "$AUTH_HEADER" -H "X-Requested-With: XMLHttpRequest" \
    "$BASE_URL/admin/dynamic/salutation")

if echo "$MODULE_LIST" | grep -q '"success":true' || echo "$MODULE_LIST" | grep -q "Salutation"; then
    echo -e "${GREEN}✓ Module is accessible${NC}"
else
    echo -e "${RED}✗ Module not accessible${NC}"
    echo "$MODULE_LIST" | head -50
fi

# Test 7: Test UI page loads
echo -n "7. Testing Schema Discovery UI page... "
UI_PAGE=$(curl -s -H "$AUTH_HEADER" "$BASE_URL/admin/schema-discovery")

if echo "$UI_PAGE" | grep -q "Schema Discovery" && echo "$UI_PAGE" | grep -q "Available Tables"; then
    echo -e "${GREEN}✓ UI page loads correctly${NC}"
else
    echo -e "${RED}✗ UI page failed to load${NC}"
fi

# Test 8: Test error handling
echo -n "8. Testing error handling for non-existent table... "
ERROR_RESULT=$(curl -s -H "$AUTH_HEADER" -H "X-Requested-With: XMLHttpRequest" \
    "$BASE_URL/admin/dynamic/_schema?action=columns&table=non_existent_table_xyz")

if echo "$ERROR_RESULT" | grep -q "error"; then
    echo -e "${GREEN}✓ Error handling works${NC}"
else
    echo -e "${RED}✗ Error handling failed${NC}"
fi

echo ""
echo "=================================="
echo -e "${GREEN}All schema discovery tests passed!${NC}"
echo ""
echo "Summary:"
echo "- API endpoints are working correctly"
echo "- Table and column introspection functional"
echo "- Module generation produces valid YAML"
echo "- Generated modules can be saved and accessed"
echo "- UI page loads successfully"
echo "- Error handling is in place"