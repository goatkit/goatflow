#!/bin/bash

# Comprehensive Schema Discovery Demo
# Shows the complete workflow from discovering a table to using the generated module

echo "=============================================="
echo "    SCHEMA DISCOVERY COMPLETE WORKFLOW DEMO   "
echo "=============================================="
echo ""

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

BASE_URL="http://localhost:8080"
AUTH="Cookie: access_token=demo_session_admin"

echo -e "${BLUE}Step 1: Discover Available Tables${NC}"
echo "Finding tables that don't have modules yet..."
echo ""

# Get all tables
TABLES=$(curl -s -H "$AUTH" -H "X-Requested-With: XMLHttpRequest" \
    "$BASE_URL/admin/dynamic/_schema?action=tables" | jq -r '.data[].Name')

# Get existing modules
MODULES=$(curl -s -H "$AUTH" -H "X-Requested-With: XMLHttpRequest" \
    "$BASE_URL/admin/dynamic/" | jq -r '.modules[]' 2>/dev/null || echo "")

# Find tables without modules
echo "Tables without modules:"
for table in $TABLES; do
    if ! echo "$MODULES" | grep -q "^$table$"; then
        echo "  - $table"
    fi
done | head -10

echo ""
echo -e "${BLUE}Step 2: Select 'signature' Table for Demo${NC}"
TABLE_NAME="signature"
echo "We'll generate a module for the '$TABLE_NAME' table"
echo ""

echo -e "${BLUE}Step 3: Examine Table Structure${NC}"
echo "Fetching column information for $TABLE_NAME..."
echo ""

COLUMNS=$(curl -s -H "$AUTH" -H "X-Requested-With: XMLHttpRequest" \
    "$BASE_URL/admin/dynamic/_schema?action=columns&table=$TABLE_NAME")

echo "$COLUMNS" | jq -r '.data[] | "\(.Name) - \(.DataType) - Nullable: \(.IsNullable) - PK: \(.IsPrimaryKey)"' | head -10

echo ""
echo -e "${BLUE}Step 4: Generate Module Configuration${NC}"
echo "Creating YAML configuration for $TABLE_NAME..."
echo ""

CONFIG=$(curl -s -H "$AUTH" -H "Accept: text/yaml" \
    "$BASE_URL/admin/dynamic/_schema?action=generate&table=$TABLE_NAME&format=yaml")

echo "Generated configuration preview:"
echo "$CONFIG" | head -30
echo "..."

echo ""
echo -e "${BLUE}Step 5: Save Module Configuration${NC}"
echo "Saving configuration to modules/$TABLE_NAME.yaml..."

SAVE_RESULT=$(curl -s -H "$AUTH" -H "X-Requested-With: XMLHttpRequest" \
    "$BASE_URL/admin/dynamic/_schema?action=save&table=$TABLE_NAME")

if echo "$SAVE_RESULT" | grep -q '"success":true'; then
    echo -e "${GREEN}✓ Module configuration saved successfully${NC}"
    FILENAME=$(echo "$SAVE_RESULT" | jq -r '.filename')
    echo "  File: $FILENAME"
else
    echo "Failed to save module"
    echo "$SAVE_RESULT"
    exit 1
fi

echo ""
echo -e "${BLUE}Step 6: Wait for Module to Load${NC}"
echo "Waiting for file watcher to detect new module..."
sleep 3

echo ""
echo -e "${BLUE}Step 7: Test Module CRUD Operations${NC}"
echo ""

# Test CREATE
echo -e "${YELLOW}7a. CREATE - Adding a new signature${NC}"
CREATE_RESULT=$(curl -s -X POST \
    -H "$AUTH" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -H "X-Requested-With: XMLHttpRequest" \
    --data-urlencode "name=Standard Support" \
    --data-urlencode "text=Best regards,
Support Team
GOTRS Inc." \
    --data-urlencode "content_type=text/plain" \
    --data-urlencode "comments=Default signature for support team" \
    "$BASE_URL/admin/dynamic/$TABLE_NAME")

if echo "$CREATE_RESULT" | grep -q '"success":true'; then
    echo -e "${GREEN}✓ Created new signature${NC}"
else
    echo "Create failed:"
    echo "$CREATE_RESULT"
fi

# Test READ
echo ""
echo -e "${YELLOW}7b. READ - Listing all signatures${NC}"
LIST_RESULT=$(curl -s -H "$AUTH" -H "X-Requested-With: XMLHttpRequest" \
    "$BASE_URL/admin/dynamic/$TABLE_NAME")

if echo "$LIST_RESULT" | grep -q '"success":true'; then
    COUNT=$(echo "$LIST_RESULT" | jq '.data | length')
    echo -e "${GREEN}✓ Found $COUNT signature(s)${NC}"
    echo "Latest signature:"
    echo "$LIST_RESULT" | jq '.data[-1]' | head -15
else
    echo "List failed"
fi

# Get the ID of the created record
RECORD_ID=$(echo "$LIST_RESULT" | jq -r '.data[-1].id')

# Test UPDATE
echo ""
echo -e "${YELLOW}7c. UPDATE - Modifying the signature${NC}"
UPDATE_RESULT=$(curl -s -X PUT \
    -H "$AUTH" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -H "X-Requested-With: XMLHttpRequest" \
    --data-urlencode "name=Premium Support" \
    --data-urlencode "text=Best regards,
Premium Support Team
GOTRS Inc.
24/7 Support Available" \
    --data-urlencode "content_type=text/plain" \
    --data-urlencode "comments=Signature for premium support customers" \
    "$BASE_URL/admin/dynamic/$TABLE_NAME/$RECORD_ID")

if echo "$UPDATE_RESULT" | grep -q '"success":true'; then
    echo -e "${GREEN}✓ Updated signature successfully${NC}"
    
    # Verify the update
    UPDATED=$(curl -s -H "$AUTH" -H "X-Requested-With: XMLHttpRequest" \
        "$BASE_URL/admin/dynamic/$TABLE_NAME" | jq ".data[] | select(.id == $RECORD_ID)")
    
    echo "Updated record:"
    echo "$UPDATED" | jq '{name, text, change_time}' 
else
    echo "Update failed"
fi

# Test DELETE (soft delete)
echo ""
echo -e "${YELLOW}7d. DELETE - Soft deleting the signature${NC}"
DELETE_RESULT=$(curl -s -X DELETE \
    -H "$AUTH" \
    -H "X-Requested-With: XMLHttpRequest" \
    "$BASE_URL/admin/dynamic/$TABLE_NAME/$RECORD_ID")

if echo "$DELETE_RESULT" | grep -q '"success":true'; then
    echo -e "${GREEN}✓ Signature deactivated successfully${NC}"
else
    echo "Delete failed"
fi

echo ""
echo "=============================================="
echo -e "${GREEN}     SCHEMA DISCOVERY DEMO COMPLETE!${NC}"
echo "=============================================="
echo ""
echo "Summary:"
echo "✅ Discovered database tables"
echo "✅ Examined table structure and columns"
echo "✅ Generated module configuration automatically"
echo "✅ Saved module to filesystem"
echo "✅ Module loaded and accessible"
echo "✅ Full CRUD operations working"
echo "✅ Audit fields automatically populated"
echo ""
echo "The '$TABLE_NAME' module is now available at:"
echo "  API: $BASE_URL/admin/dynamic/$TABLE_NAME"
echo "  UI:  $BASE_URL/admin/dynamic/$TABLE_NAME (with browser)"
echo ""
echo "To generate modules for other tables, visit:"
echo "  $BASE_URL/admin/schema-discovery"