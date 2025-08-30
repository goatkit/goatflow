#!/bin/bash

# Debug script to check live server state for sysconfig module

echo "=== LIVE SERVER SYSCONFIG DEBUG ==="
echo ""

echo "1. Checking if server is running..."
if curl -s http://localhost:8080/health > /dev/null; then
    echo "✓ Server is responding"
else
    echo "✗ Server not responding - start it first"
    echo "Run: make restart"
    exit 1
fi

echo ""
echo "2. Checking server logs for module loading..."
echo "Looking for 'Loaded module: sysconfig':"
make logs | grep -i "sysconfig" | head -10

echo ""
echo "3. Checking server logs for 'Dynamic Module System loaded':"
make logs | grep "Dynamic Module System loaded" | tail -1

echo ""
echo "4. Testing sysconfig endpoint directly..."
echo "GET /admin/sysconfig (should redirect to login):"
RESPONSE=$(curl -s -w "HTTP_STATUS:%{http_code}" http://localhost:8080/admin/sysconfig)
HTTP_STATUS=$(echo $RESPONSE | grep -o "HTTP_STATUS:[0-9]*" | cut -d: -f2)
echo "Status: $HTTP_STATUS"

if [ "$HTTP_STATUS" == "303" ] || [ "$HTTP_STATUS" == "302" ]; then
    echo "✓ sysconfig endpoint responds with redirect (expected for unauthenticated)"
elif [ "$HTTP_STATUS" == "404" ]; then
    echo "✗ sysconfig endpoint returns 404 - route not registered"
elif [ "$HTTP_STATUS" == "500" ]; then
    echo "✗ sysconfig endpoint returns 500 - server error"
else
    echo "? sysconfig endpoint returns $HTTP_STATUS"
fi

echo ""
echo "5. Testing dynamic module list endpoint..."
echo "GET /admin/dynamic/ (should show module list):"
MODULES_RESPONSE=$(curl -s -H "X-Requested-With: XMLHttpRequest" http://localhost:8080/admin/dynamic/)
echo "Response: $MODULES_RESPONSE"

if echo "$MODULES_RESPONSE" | grep -q "sysconfig"; then
    echo "✓ sysconfig appears in dynamic modules list"
else
    echo "✗ sysconfig NOT found in dynamic modules list"
    echo "This is the core issue!"
fi

echo ""
echo "6. Full modules list from API:"
echo "$MODULES_RESPONSE" | python3 -m json.tool 2>/dev/null || echo "Response is not valid JSON: $MODULES_RESPONSE"

echo ""
echo "=== DIAGNOSIS ==="
echo ""
echo "Expected results:"
echo "- Server should be running (✓ if checked above)"
echo "- Logs should show 'Loaded module: sysconfig'"
echo "- /admin/sysconfig should return 302/303 redirect"
echo "- Dynamic modules API should include sysconfig in the list"
echo ""
echo "If sysconfig is missing from the API list but loads correctly,"
echo "then the issue is in the GetAvailableModules() function or"
echo "something is modifying the configs map after loading."
echo ""
echo "Next steps if issue persists:"
echo "1. Apply debug modifications: ./apply_sysconfig_debug.sh prepare"
echo "2. Edit the handler file with debug versions"
echo "3. Restart server and check debug logs"
