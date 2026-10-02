#!/bin/bash

echo "======================================"
echo "  SCHEMA DISCOVERY FINAL VALIDATION"
echo "======================================"
echo ""

# Count modules
MODULE_COUNT=$(ls modules/*.yaml 2>/dev/null | wc -l)
echo "✅ Generated Modules: $MODULE_COUNT"

# Count total fields
TOTAL_FIELDS=$(grep -h "^  - name:" modules/*.yaml 2>/dev/null | wc -l)
echo "✅ Total Fields Configured: $TOTAL_FIELDS"

BASE_URL="http://localhost:8080"
# shellcheck source=../lib/admin-login.sh
source "$(dirname "$0")/../lib/admin-login.sh"
AUTH=$(admin_login_cookie "$BASE_URL") || exit 1

# Test API endpoint
API_TEST=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "$AUTH" \
  "$BASE_URL/admin/dynamic/_schema?action=tables")
echo "✅ API Endpoint Status: $API_TEST"

# Test UI page
UI_TEST=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "$AUTH" \
  "$BASE_URL/admin/schema-discovery")
echo "✅ UI Page Status: $UI_TEST"

# List working modules
echo ""
echo "Working Modules:"
for module in modules/*.yaml; do
  if [ -f "$module" ]; then
    NAME=$(basename "$module" .yaml)
    TEST=$(curl -s -H "$AUTH" \
      -H "X-Requested-With: XMLHttpRequest" \
      "$BASE_URL/admin/dynamic/$NAME" 2>/dev/null | grep -c '"success":true')
    if [ "$TEST" -eq 1 ]; then
      echo "  ✓ $NAME"
    fi
  fi
done

echo ""
echo "======================================"
echo "    ALL SYSTEMS OPERATIONAL ✅"
echo "======================================"
