#!/bin/bash

# Script to check for untranslated keys in the UI
# These show up as "admin.something" or "app.something" instead of actual text

set -e

echo "🔍 Checking for untranslated keys in GoatFlow UI..."
echo "================================================"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Login and get session
echo "📝 Logging in as admin..."
LOGIN_RESPONSE=$(curl -s -c /tmp/goatflow_cookies.txt -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "email=admin@demo.com&password=demo123" \
  -L -w "\n%{http_code}")

HTTP_CODE=$(echo "$LOGIN_RESPONSE" | tail -1)
if [ "$HTTP_CODE" != "200" ]; then
  echo -e "${RED}❌ Login failed with code: $HTTP_CODE${NC}"
  exit 1
fi

echo -e "${GREEN}✅ Logged in successfully${NC}"

# Function to check a page for translation keys
check_page() {
  local PAGE_NAME=$1
  local PAGE_URL=$2
  
  echo -e "\n📄 Checking $PAGE_NAME..."
  
  # Fetch the page
  PAGE_CONTENT=$(curl -s -b /tmp/goatflow_cookies.txt -L "http://localhost:8080$PAGE_URL")
  
  # Check if we got redirected to login
  if echo "$PAGE_CONTENT" | grep -q "Login to GoatFlow"; then
    echo -e "${RED}❌ Redirected to login page - session might have expired${NC}"
    return 1
  fi
  
  # Look for common translation key patterns in visible text
  # This regex looks for patterns like "admin.something" or "app.something"
  FOUND_KEYS=$(echo "$PAGE_CONTENT" | \
    grep -oE '>(admin\.|app\.|common\.|user\.|dashboard\.|tickets?\.|queue\.|error\.|success\.|warning\.|info\.|time\.|status\.|priority\.)[a-z_]+[a-z_]*<' | \
    sed 's/^>//' | sed 's/<$//' | \
    sort -u)
  
  # Also check for translation keys in button text, labels, etc
  BUTTON_KEYS=$(echo "$PAGE_CONTENT" | \
    grep -oE 'button[^>]*>[ ]*(admin\.|app\.|common\.)[a-z_]+[a-z_]*[ ]*<' | \
    grep -oE '(admin\.|app\.|common\.)[a-z_]+[a-z_]*' | \
    sort -u)
  
  # Check title attributes
  TITLE_KEYS=$(echo "$PAGE_CONTENT" | \
    grep -oE 'title="(admin\.|app\.|common\.)[a-z_]+[a-z_]*"' | \
    grep -oE '(admin\.|app\.|common\.)[a-z_]+[a-z_]*' | \
    sort -u)
  
  # Check placeholder attributes
  PLACEHOLDER_KEYS=$(echo "$PAGE_CONTENT" | \
    grep -oE 'placeholder="(admin\.|app\.|common\.)[a-z_]+[a-z_]*"' | \
    grep -oE '(admin\.|app\.|common\.)[a-z_]+[a-z_]*' | \
    sort -u)
  
  # Combine all found keys
  ALL_KEYS=$(echo -e "$FOUND_KEYS\n$BUTTON_KEYS\n$TITLE_KEYS\n$PLACEHOLDER_KEYS" | sort -u | grep -v '^$')
  
  if [ -n "$ALL_KEYS" ]; then
    echo -e "${RED}❌ Found untranslated keys on $PAGE_NAME:${NC}"
    echo "$ALL_KEYS" | while read key; do
      echo -e "   ${YELLOW}• $key${NC}"
    done
    return 1
  else
    echo -e "${GREEN}✅ No untranslated keys found on $PAGE_NAME${NC}"
    return 0
  fi
}

# Check various pages
FAILED=0

# Admin Dashboard
if ! check_page "Admin Dashboard" "/admin"; then
  FAILED=$((FAILED + 1))
fi

# Groups Page
if ! check_page "Groups Management" "/admin/groups"; then
  FAILED=$((FAILED + 1))
fi

# Users Page
if ! check_page "Users Management" "/admin/users"; then
  FAILED=$((FAILED + 1))
fi

# Clean up
rm -f /tmp/goatflow_cookies.txt

echo -e "\n================================================"
if [ $FAILED -eq 0 ]; then
  echo -e "${GREEN}✅ All pages checked successfully - no untranslated keys found!${NC}"
  exit 0
else
  echo -e "${RED}❌ Found untranslated keys on $FAILED page(s)${NC}"
  echo -e "${YELLOW}Please add the missing translations to the language files in:${NC}"
  echo "   internal/i18n/translations/en.json"
  echo "   internal/i18n/translations/de.json"
  exit 1
fi