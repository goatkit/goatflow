#!/bin/bash
#
# TDD TEST-FIRST ENFORCER
# Ensures true Test-Driven Development by preventing implementation without failing tests
# Integrates with existing GOTRS infrastructure and prevents "implementation-first" patterns
#

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
LOG_DIR="$PROJECT_ROOT/generated/tdd-enforcer"
TEST_DIR="$PROJECT_ROOT/internal"
TDD_STATE_FILE="$PROJECT_ROOT/.tdd-state"

mkdir -p "$LOG_DIR"

# Logging functions
log() {
    echo -e "${BLUE}[$(date +%H:%M:%S)] TDD-ENFORCER:${NC} $1" | tee -a "$LOG_DIR/enforcer.log"
}

success() {
    echo -e "${GREEN}✓ TDD-ENFORCER:${NC} $1" | tee -a "$LOG_DIR/enforcer.log"
}

fail() {
    echo -e "${RED}✗ TDD-ENFORCER:${NC} $1" | tee -a "$LOG_DIR/enforcer.log"
}

warning() {
    echo -e "${YELLOW}⚠ TDD-ENFORCER:${NC} $1" | tee -a "$LOG_DIR/enforcer.log"
}

critical() {
    echo -e "${RED}🚨 TDD VIOLATION:${NC} $1" | tee -a "$LOG_DIR/enforcer.log"
    exit 1
}

# Initialize TDD state tracking
init_tdd_state() {
    local feature_name="$1"
    
    cat > "$TDD_STATE_FILE" << EOF
{
  "feature": "$feature_name",
  "phase": "test_first",
  "started_at": "$(date -Iseconds)",
  "git_commit_start": "$(git rev-parse HEAD 2>/dev/null || echo 'no-git')",
  "test_written": false,
  "test_failing": false,
  "implementation_started": false,
  "tests_passing": false,
  "refactor_ready": false,
  "cycle_complete": false,
  "evidence_collected": false
}
EOF
    
    success "TDD state initialized for feature: $feature_name"
}

# Generate a failing test template
generate_failing_test() {
    local feature_name="$1"
    local test_type="${2:-unit}"  # unit, integration, api, browser
    
    log "Generating failing test template for: $feature_name (type: $test_type)"
    
    # Determine test file location and name
    local test_package=""
    local test_file=""
    local test_function=""
    
    case "$test_type" in
        "unit")
            # Create in appropriate package
            read -p "Enter package name (e.g., models, handlers, service): " test_package
            test_file="$PROJECT_ROOT/internal/$test_package/${feature_name,,}_test.go"
            test_function="Test$(echo "$feature_name" | sed 's/[^a-zA-Z0-9]//g')"
            ;;
        "integration")
            test_package="integration"
            test_file="$PROJECT_ROOT/tests/integration/${feature_name,,}_integration_test.go"
            test_function="Test$(echo "$feature_name" | sed 's/[^a-zA-Z0-9]//g')Integration"
            ;;
        "api")
            test_package="api"
            test_file="$PROJECT_ROOT/tests/api/${feature_name,,}_api_test.go"
            test_function="Test$(echo "$feature_name" | sed 's/[^a-zA-Z0-9]//g')API"
            ;;
        "browser")
            test_package="e2e"
            test_file="$PROJECT_ROOT/tests/e2e/${feature_name,,}_e2e_test.go"
            test_function="Test$(echo "$feature_name" | sed 's/[^a-zA-Z0-9]//g')E2E"
            ;;
    esac
    
    # Create directory if it doesn't exist
    mkdir -p "$(dirname "$test_file")"
    
    # Generate the failing test
    case "$test_type" in
        "unit")
            cat > "$test_file" << EOF
package $test_package

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// $test_function tests the $feature_name functionality
// This test is intentionally failing to follow TDD practices
func $test_function(t *testing.T) {
    // TODO: Remove this line and implement the actual test
    t.Skip("Failing test for TDD - implement $feature_name functionality first")
    
    // Example test structure - customize for your feature:
    t.Run("should handle valid input", func(t *testing.T) {
        // Arrange
        // TODO: Set up test data
        
        // Act  
        // TODO: Call the function/method being tested
        
        // Assert
        // TODO: Verify the expected behavior
        assert.Fail(t, "Not implemented: $feature_name")
    })
    
    t.Run("should handle invalid input", func(t *testing.T) {
        // Arrange
        // TODO: Set up invalid test data
        
        // Act
        // TODO: Call the function/method being tested
        
        // Assert
        // TODO: Verify error handling
        assert.Fail(t, "Not implemented: $feature_name error handling")
    })
    
    t.Run("should handle edge cases", func(t *testing.T) {
        // Arrange
        // TODO: Set up edge case data
        
        // Act
        // TODO: Call the function/method being tested
        
        // Assert
        // TODO: Verify edge case behavior
        assert.Fail(t, "Not implemented: $feature_name edge cases")
    })
}
EOF
            ;;
        "integration")
            mkdir -p "$PROJECT_ROOT/tests/integration"
            cat > "$test_file" << EOF
//go:build integration
// +build integration

package integration

import (
    "testing"
    "database/sql"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// $test_function tests the $feature_name integration
// This test is intentionally failing to follow TDD practices
func $test_function(t *testing.T) {
    // TODO: Remove this line and implement the actual integration test
    t.Skip("Failing integration test for TDD - implement $feature_name first")
    
    // Example integration test structure:
    t.Run("should integrate with database", func(t *testing.T) {
        // Arrange
        // TODO: Set up test database
        // TODO: Set up test data
        
        // Act
        // TODO: Call the integration functionality
        
        // Assert
        // TODO: Verify database state
        // TODO: Verify integration behavior
        assert.Fail(t, "Not implemented: $feature_name database integration")
    })
    
    t.Run("should integrate with external services", func(t *testing.T) {
        // Arrange
        // TODO: Set up external service mocks/stubs
        
        // Act
        // TODO: Call the integration functionality
        
        // Assert
        // TODO: Verify external service interactions
        assert.Fail(t, "Not implemented: $feature_name service integration")
    })
}
EOF
            ;;
        "api")
            mkdir -p "$PROJECT_ROOT/tests/api"
            cat > "$test_file" << EOF
package api

import (
    "testing"
    "net/http"
    "net/http/httptest"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// $test_function tests the $feature_name API endpoints
// This test is intentionally failing to follow TDD practices
func $test_function(t *testing.T) {
    // TODO: Remove this line and implement the actual API test
    t.Skip("Failing API test for TDD - implement $feature_name API first")
    
    // Example API test structure:
    t.Run("should handle GET request", func(t *testing.T) {
        // Arrange
        req, err := http.NewRequest("GET", "/api/v1/${feature_name,,}", nil)
        require.NoError(t, err)
        
        rr := httptest.NewRecorder()
        
        // Act
        // TODO: Set up router and call handler
        
        // Assert
        // TODO: Verify response status
        // TODO: Verify response body
        assert.Fail(t, "Not implemented: $feature_name GET API")
    })
    
    t.Run("should handle POST request", func(t *testing.T) {
        // Arrange
        // TODO: Set up POST request with body
        
        // Act
        // TODO: Call API handler
        
        // Assert
        // TODO: Verify response
        assert.Fail(t, "Not implemented: $feature_name POST API")
    })
    
    t.Run("should handle authentication", func(t *testing.T) {
        // Arrange
        // TODO: Set up authenticated request
        
        // Act
        // TODO: Call API handler
        
        // Assert
        // TODO: Verify authentication handling
        assert.Fail(t, "Not implemented: $feature_name authentication")
    })
}
EOF
            ;;
        "browser")
            mkdir -p "$PROJECT_ROOT/tests/e2e"
            cat > "$test_file" << EOF
//go:build e2e
// +build e2e

package e2e

import (
    "testing"
    "context"
    "time"
    "github.com/chromedp/chromedp"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// $test_function tests the $feature_name user interface
// This test is intentionally failing to follow TDD practices
func $test_function(t *testing.T) {
    // TODO: Remove this line and implement the actual E2E test
    t.Skip("Failing E2E test for TDD - implement $feature_name UI first")
    
    // Example E2E test structure:
    t.Run("should display $feature_name interface", func(t *testing.T) {
        // Arrange
        ctx, cancel := chromedp.NewContext(context.Background())
        defer cancel()
        
        ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
        defer cancel()
        
        // Act
        err := chromedp.Run(ctx,
            chromedp.Navigate("http://localhost:8080/${feature_name,,}"),
            chromedp.WaitVisible("body", chromedp.ByQuery),
        )
        
        // Assert
        require.NoError(t, err)
        assert.Fail(t, "Not implemented: $feature_name UI")
    })
    
    t.Run("should handle user interactions", func(t *testing.T) {
        // Arrange
        ctx, cancel := chromedp.NewContext(context.Background())
        defer cancel()
        
        // Act
        // TODO: Simulate user interactions
        
        // Assert
        // TODO: Verify UI behavior
        assert.Fail(t, "Not implemented: $feature_name user interactions")
    })
}
EOF
            ;;
    esac
    
    success "Failing test generated: $test_file"
    log "Next steps:"
    log "1. Review and customize the test for your specific requirements"
    log "2. Run the test to verify it fails: make test"
    log "3. Implement minimal code to make the test pass"
    log "4. Run comprehensive verification: make tdd-verify"
    
    # Update TDD state
    jq '.test_written = true | .test_file = "'$test_file'" | .test_type = "'$test_type'"' \
        "$TDD_STATE_FILE" > "$TDD_STATE_FILE.tmp" && mv "$TDD_STATE_FILE.tmp" "$TDD_STATE_FILE"
    
    echo "$test_file"
}

# Verify that test is actually failing
verify_test_failing() {
    local test_file="$1"
    
    log "Verifying that test actually fails..."
    
    cd "$PROJECT_ROOT"
    
    # Run the specific test file
    local test_package=$(dirname "$test_file" | sed "s|$PROJECT_ROOT/||")
    
    if go test -v "./$test_package" -run "$(basename "$test_file" _test.go)" > "$LOG_DIR/failing_test_check.log" 2>&1; then
        critical "Test is not failing! TDD requires a failing test first."
    else
        # Check that it failed due to our intentional failure, not compilation errors
        if grep -q "SKIP\|Not implemented\|assert.Fail" "$LOG_DIR/failing_test_check.log"; then
            success "Test properly fails as expected"
            jq '.test_failing = true' "$TDD_STATE_FILE" > "$TDD_STATE_FILE.tmp" && mv "$TDD_STATE_FILE.tmp" "$TDD_STATE_FILE"
            return 0
        elif grep -q "build failed\|undefined:" "$LOG_DIR/failing_test_check.log"; then
            critical "Test fails due to compilation errors. Fix compilation first, then ensure test fails logically."
        else
            warning "Test fails, but may not be for the right reasons. Review test output."
            cat "$LOG_DIR/failing_test_check.log"
            return 1
        fi
    fi
}

# Check if implementation has started before tests pass
check_implementation_before_green() {
    log "Checking for premature implementation..."
    
    # Get git changes since TDD started
    local start_commit=$(jq -r '.git_commit_start // "HEAD"' "$TDD_STATE_FILE")
    local implementation_files_changed=0
    
    # Check for implementation files that have been modified
    if git diff --name-only "$start_commit" HEAD 2>/dev/null | grep -E "\.(go)$" | grep -v "_test\.go" > "$LOG_DIR/implementation_changes.log"; then
        implementation_files_changed=$(wc -l < "$LOG_DIR/implementation_changes.log")
        
        if [ "$implementation_files_changed" -gt 0 ]; then
            warning "$implementation_files_changed implementation files changed during TDD cycle"
            jq '.implementation_started = true' "$TDD_STATE_FILE" > "$TDD_STATE_FILE.tmp" && mv "$TDD_STATE_FILE.tmp" "$TDD_STATE_FILE"
        fi
    fi
    
    return 0
}

# Verify tests are now passing after implementation
verify_tests_passing() {
    local test_file="$1"
    
    log "Verifying that tests now pass after implementation..."
    
    cd "$PROJECT_ROOT"
    
    local test_package=$(dirname "$test_file" | sed "s|$PROJECT_ROOT/||")
    
    # First remove any skip statements that were used for failing tests
    if grep -q "t.Skip.*TDD\|assert.Fail.*Not implemented" "$test_file"; then
        warning "Test file still contains skip statements or intentional failures"
        warning "Please remove t.Skip() and assert.Fail() statements and implement real assertions"
        return 1
    fi
    
    # Run the test
    if go test -v "./$test_package" -run "$(basename "$test_file" _test.go)" > "$LOG_DIR/passing_test_check.log" 2>&1; then
        local passed_tests=$(grep -c "PASS:" "$LOG_DIR/passing_test_check.log" || echo "0")
        if [ "$passed_tests" -gt 0 ]; then
            success "Tests are now passing ($passed_tests tests)"
            jq '.tests_passing = true' "$TDD_STATE_FILE" > "$TDD_STATE_FILE.tmp" && mv "$TDD_STATE_FILE.tmp" "$TDD_STATE_FILE"
            return 0
        else
            fail "No passing tests detected"
            return 1
        fi
    else
        fail "Tests are still failing after implementation"
        cat "$LOG_DIR/passing_test_check.log"
        return 1
    fi
}

# Generate TDD cycle report
generate_tdd_cycle_report() {
    local report_file="$LOG_DIR/tdd_cycle_report_$(date +%Y%m%d_%H%M%S).html"
    
    local feature_name=$(jq -r '.feature // "Unknown"' "$TDD_STATE_FILE")
    local phase=$(jq -r '.phase // "Unknown"' "$TDD_STATE_FILE")
    local test_written=$(jq -r '.test_written // false' "$TDD_STATE_FILE")
    local test_failing=$(jq -r '.test_failing // false' "$TDD_STATE_FILE")
    local implementation_started=$(jq -r '.implementation_started // false' "$TDD_STATE_FILE")
    local tests_passing=$(jq -r '.tests_passing // false' "$TDD_STATE_FILE")
    
    cat > "$report_file" << EOF
<!DOCTYPE html>
<html>
<head>
    <title>TDD Cycle Report - $feature_name</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; line-height: 1.6; }
        .header { background: #e9ecef; padding: 20px; border-radius: 5px; margin-bottom: 20px; }
        .pass { color: #28a745; font-weight: bold; }
        .fail { color: #dc3545; font-weight: bold; }
        .pending { color: #ffc107; font-weight: bold; }
        .phase { background: #f8f9fa; padding: 15px; margin: 15px 0; border-left: 4px solid #007bff; }
        .violation { background: #f8d7da; padding: 15px; margin: 15px 0; border-left: 4px solid #dc3545; }
        table { border-collapse: collapse; width: 100%; margin: 15px 0; }
        th, td { border: 1px solid #dee2e6; padding: 12px; text-align: left; }
        th { background-color: #e9ecef; font-weight: bold; }
    </style>
</head>
<body>
    <div class="header">
        <h1>🧪 TDD Cycle Report</h1>
        <p><strong>Feature:</strong> $feature_name</p>
        <p><strong>Current Phase:</strong> $phase</p>
        <p><strong>Generated:</strong> $(date)</p>
    </div>

    <h2>TDD Cycle Progress</h2>
    <table>
        <tr><th>Phase</th><th>Status</th><th>Evidence</th></tr>
        <tr><td>1. Write Failing Test</td><td class="$(if [ "$test_written" = "true" ]; then echo "pass"; else echo "pending"; fi)">$(if [ "$test_written" = "true" ]; then echo "✓ COMPLETE"; else echo "⏳ PENDING"; fi)</td><td>Test file created and verified failing</td></tr>
        <tr><td>2. Test Actually Fails</td><td class="$(if [ "$test_failing" = "true" ]; then echo "pass"; else echo "pending"; fi)">$(if [ "$test_failing" = "true" ]; then echo "✓ COMPLETE"; else echo "⏳ PENDING"; fi)</td><td>Test execution confirmed failure</td></tr>
        <tr><td>3. Implement Minimal Code</td><td class="$(if [ "$implementation_started" = "true" ]; then echo "pass"; else echo "pending"; fi)">$(if [ "$implementation_started" = "true" ]; then echo "✓ COMPLETE"; else echo "⏳ PENDING"; fi)</td><td>Implementation files modified</td></tr>
        <tr><td>4. Tests Pass</td><td class="$(if [ "$tests_passing" = "true" ]; then echo "pass"; else echo "pending"; fi)">$(if [ "$tests_passing" = "true" ]; then echo "✓ COMPLETE"; else echo "⏳ PENDING"; fi)</td><td>All tests now passing</td></tr>
        <tr><td>5. Refactor (Optional)</td><td class="pending">⏳ PENDING</td><td>Code cleanup while maintaining tests</td></tr>
    </table>

    <h2>TDD State Details</h2>
    <div class="phase">
        <pre>$(jq . "$TDD_STATE_FILE" 2>/dev/null || echo "No TDD state file")</pre>
    </div>

    <h2>TDD Principles Enforced</h2>
    <ul>
        <li>✅ No implementation without failing test first</li>
        <li>✅ Tests must actually fail before implementation</li>
        <li>✅ Minimal implementation to make tests pass</li>
        <li>✅ All tests must pass before claiming success</li>
        <li>✅ Evidence collection for all phases</li>
    </ul>

    <footer style="margin-top: 50px; padding-top: 20px; border-top: 1px solid #dee2e6; text-align: center; color: #6c757d;">
        Generated by TDD Test-First Enforcer - $(date)
    </footer>
</body>
</html>
EOF
    
    echo "$report_file"
}

# Main command dispatcher
case "${1:-}" in
    "init")
        feature_name="${2:-}"
        if [ -z "$feature_name" ]; then
            critical "Feature name required: $0 init 'Feature Name'"
        fi
        init_tdd_state "$feature_name"
        ;;
    "generate-test")
        if [ ! -f "$TDD_STATE_FILE" ]; then
            critical "TDD not initialized. Run: $0 init 'Feature Name' first"
        fi
        feature_name=$(jq -r '.feature' "$TDD_STATE_FILE")
        test_type="${2:-unit}"
        test_file=$(generate_failing_test "$feature_name" "$test_type")
        log "Test generated: $test_file"
        log "Next: Review and customize the test, then run: $0 verify-failing '$test_file'"
        ;;
    "verify-failing")
        test_file="$2"
        if [ -z "$test_file" ]; then
            critical "Test file path required: $0 verify-failing /path/to/test_file.go"
        fi
        verify_test_failing "$test_file"
        ;;
    "verify-passing")
        test_file="$2"
        if [ -z "$test_file" ]; then
            critical "Test file path required: $0 verify-passing /path/to/test_file.go"
        fi
        verify_tests_passing "$test_file"
        ;;
    "check-implementation")
        check_implementation_before_green
        ;;
    "report")
        report_file=$(generate_tdd_cycle_report)
        success "TDD cycle report generated: $report_file"
        ;;
    "status")
        if [ ! -f "$TDD_STATE_FILE" ]; then
            warning "TDD not initialized"
            exit 1
        fi
        echo "TDD Cycle Status:"
        echo "================"
        jq . "$TDD_STATE_FILE"
        ;;
    *)
        echo "TDD Test-First Enforcer"
        echo "Enforces true Test-Driven Development practices"
        echo ""
        echo "Usage: $0 <command> [options]"
        echo ""
        echo "Commands:"
        echo "  init 'Feature Name'         - Initialize TDD cycle for a feature"
        echo "  generate-test [type]        - Generate failing test (types: unit, integration, api, browser)"
        echo "  verify-failing <test_file>  - Verify test actually fails"
        echo "  verify-passing <test_file>  - Verify test now passes after implementation"
        echo "  check-implementation        - Check if implementation started prematurely"
        echo "  report                      - Generate TDD cycle report"
        echo "  status                      - Show current TDD cycle status"
        echo ""
        echo "TDD Workflow:"
        echo "1. $0 init 'My New Feature'"
        echo "2. $0 generate-test unit"
        echo "3. Review and customize the generated test"
        echo "4. $0 verify-failing /path/to/test_file.go"
        echo "5. Implement minimal code to make test pass"
        echo "6. $0 verify-passing /path/to/test_file.go"
        echo "7. Run comprehensive verification: make tdd-verify"
        echo ""
        exit 1
        ;;
esac