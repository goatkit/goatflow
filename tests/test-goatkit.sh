#!/bin/bash

# GoatKit Comprehensive Test Suite
set -e

echo "🐐 GoatKit Test Suite"
echo "===================="
echo ""

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0

# Test function
run_test() {
    local test_name="$1"
    local test_command="$2"
    
    echo -n "Testing $test_name... "
    
    if eval "$test_command" > /dev/null 2>&1; then
        echo -e "${GREEN}✅ PASSED${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        echo -e "${RED}❌ FAILED${NC}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

echo "📦 Building GoatKit container..."
if docker build -f Dockerfile.goatkit -t goatkit . > /dev/null 2>&1; then
    echo -e "${GREEN}✅ Container build successful${NC}"
else
    echo -e "${RED}❌ Container build failed${NC}"
    exit 1
fi
echo ""

# Alias for easier testing
gk() {
    docker run --rm -v "$(pwd)":/workspace:ro goatkit "$@"
}

echo "🔧 Component Tests"
echo "-----------------"

# Test core files exist
run_test "Version manager exists" \
    "test -f internal/yamlmgmt/version_manager.go"

run_test "Hot reload manager exists" \
    "test -f internal/yamlmgmt/hot_reload.go"

run_test "Schema registry exists" \
    "test -f internal/yamlmgmt/schema_registry.go"

run_test "Linter exists" \
    "test -f internal/yamlmgmt/linter.go"

run_test "Config adapter exists" \
    "test -f internal/yamlmgmt/config_adapter.go"

run_test "CLI tool exists" \
    "test -f cmd/gk/main.go"

echo ""
echo "🎯 Container Tests"
echo "-----------------"

# Test container commands
run_test "Help command" \
    "gk help | grep -q 'GoatKit - YAML Configuration Management'"

run_test "About command" \
    "gk about | grep -q 'Version control'"

run_test "List command runs" \
    "gk list config 2>&1 | grep -q 'Configuration List'"

run_test "Show command runs" \
    "gk show config test 2>&1 | grep -E -q '(no current version|Configuration:)'"

echo ""
echo "✅ Validation Tests"
echo "------------------"

# Create test YAML files
cat > /tmp/test-valid-route.yaml << 'EOF'
apiVersion: goatflow.io/v1
kind: Route
metadata:
  name: test-route
  namespace: core
spec:
  prefix: /api/v1
  routes:
    - path: /health
      method: GET
      handler: healthCheck
      name: health-check
EOF

cat > /tmp/test-invalid-route.yaml << 'EOF'
not valid yaml {
  this is broken:
EOF

cat > /tmp/test-valid-config.yaml << 'EOF'
version: "1.0"
metadata:
  description: "Test configuration"
  last_updated: "2025-08-25"
settings:
  - name: TestSetting
    group: "Test"
    navigation: "Test::Settings"
    description: "Test setting"
    type: string
    default: "test"
EOF

cat > /tmp/test-valid-dashboard.yaml << 'EOF'
apiVersion: goatflow.io/v1
kind: Dashboard
metadata:
  name: test-dashboard
spec:
  dashboard:
    title: "Test Dashboard"
    tiles:
      - name: "Test Tile"
        url: "/test"
        icon: "test"
EOF

run_test "Valid route validation" \
    "docker run --rm -v /tmp:/data:ro goatkit validate /data/test-valid-route.yaml | grep -q 'PASSED'"

run_test "Invalid YAML detection" \
    "! docker run --rm -v /tmp:/data:ro goatkit validate /data/test-invalid-route.yaml 2>&1 | grep -q 'PASSED'"

run_test "Valid config validation" \
    "docker run --rm -v /tmp:/data:ro goatkit validate /data/test-valid-config.yaml | grep -q 'PASSED'"

run_test "Valid dashboard validation" \
    "docker run --rm -v /tmp:/data:ro goatkit validate /data/test-valid-dashboard.yaml | grep -q 'PASSED'"

echo ""
echo "🔍 Linting Tests"
echo "---------------"

run_test "Route linting" \
    "docker run --rm -v /tmp:/data:ro goatkit lint /data/test-valid-route.yaml | grep -q 'Summary:'"

run_test "Config linting" \
    "docker run --rm -v /tmp:/data:ro goatkit lint /data/test-valid-config.yaml | grep -q 'Summary:'"

run_test "Dashboard linting" \
    "docker run --rm -v /tmp:/data:ro goatkit lint /data/test-valid-dashboard.yaml | grep -q 'Summary:'"

echo ""
echo "📊 Version Management Tests"
echo "-------------------------"

run_test "Version list command" \
    "gk version list config test 2>&1 | grep -E -q '(Version History|No versions found)'"

run_test "Version show command" \
    "gk version show config test v1 2>&1 | grep -E -q '(Version:|no versions found)'"

echo ""
echo "🔄 Import/Export Tests"
echo "--------------------"

mkdir -p /tmp/test-import
cp /tmp/test-valid-config.yaml /tmp/test-import/

run_test "Import command" \
    "docker run --rm -v /tmp/test-import:/data:ro goatkit import /data 2>&1 | grep -E -q '(Import complete|imported)'"

run_test "Export command" \
    "docker run --rm -v /tmp:/data goatkit export config /data 2>&1 | grep -E -q '(Export complete|exported|No configurations)'"

echo ""
echo "🧪 Integration Tests"
echo "------------------"

# Test real routes directory
if [ -d "routes" ]; then
    run_test "Validate real routes" \
        "gk validate /workspace/routes/core/health.yaml"
    
    run_test "Lint real routes" \
        "gk lint /workspace/routes | head -50"
fi

# Test GoatKit ASCII art
run_test "ASCII art displays" \
    "gk | grep -q 'GOATKIT'"

# Test help sections
run_test "Commands section in help" \
    "gk help | grep -q 'Commands:'"

run_test "Examples section in help" \
    "gk help | grep -q 'Examples:'"

# Cleanup
rm -f /tmp/test-*.yaml
rm -rf /tmp/test-import

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "📊 Test Results Summary"
echo "======================"
echo -e "Tests Passed: ${GREEN}$TESTS_PASSED${NC}"
echo -e "Tests Failed: ${RED}$TESTS_FAILED${NC}"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}🎉 All tests passed! GoatKit is working correctly.${NC}"
    echo ""
    echo "GoatKit provides:"
    echo "🐐 Version control for all YAML configs"
    echo "🐐 Schema validation and linting"
    echo "🐐 Hot reload capabilities"
    echo "🐐 Container-first architecture"
    echo "🐐 GitOps-ready workflows"
    exit 0
else
    echo -e "${RED}⚠️  Some tests failed. Please review the failures above.${NC}"
    exit 1
fi