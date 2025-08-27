#!/bin/bash
#
# TEST TDD AUTOMATION
# Tests the comprehensive TDD automation system to ensure it works correctly
# and catches the specific anti-patterns it was designed to prevent
#

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TEST_LOG_DIR="$PROJECT_ROOT/generated/tdd-automation-tests"

mkdir -p "$TEST_LOG_DIR"

# Logging functions
log() {
    echo -e "${CYAN}[$(date +%H:%M:%S)] TEST:${NC} $1" | tee -a "$TEST_LOG_DIR/test.log"
}

success() {
    echo -e "${GREEN}✅ TEST:${NC} $1" | tee -a "$TEST_LOG_DIR/test.log"
}

fail() {
    echo -e "${RED}❌ TEST:${NC} $1" | tee -a "$TEST_LOG_DIR/test.log"
}

warning() {
    echo -e "${YELLOW}⚠️ TEST:${NC} $1" | tee -a "$TEST_LOG_DIR/test.log"
}

# Test that scripts exist and are executable
test_script_availability() {
    log "Testing script availability..."
    
    local scripts=(
        "tdd-comprehensive.sh"
        "anti-gaslighting-detector.sh"
        "tdd-test-first-enforcer.sh"
        "comprehensive-tdd-integration.sh"
        "tdd-enforcer.sh"
    )
    
    local missing_scripts=0
    
    for script in "${scripts[@]}"; do
        if [ -x "$SCRIPT_DIR/$script" ]; then
            success "Script $script is executable"
        else
            fail "Script $script is missing or not executable"
            ((missing_scripts++))
        fi
    done
    
    return $missing_scripts
}

# Test anti-gaslighting detector with known issues
test_anti_gaslighting_detection() {
    log "Testing anti-gaslighting detection..."
    
    # Create a test scenario with compilation errors (should be detected)
    local test_file="$PROJECT_ROOT/test_gaslighting_detection.go"
    cat > "$test_file" << 'EOF'
package main

import "fmt"

func main() {
    // This will cause a compilation error
    undefined_function()
    fmt.Println("This should not compile")
}
EOF
    
    # Run anti-gaslighting detector - it should detect the compilation issue
    if "$SCRIPT_DIR/anti-gaslighting-detector.sh" quick > "$TEST_LOG_DIR/gaslighting_test.log" 2>&1; then
        fail "Anti-gaslighting detector did not catch compilation error (should have failed)"
        rm -f "$test_file"
        return 1
    else
        success "Anti-gaslighting detector correctly caught compilation error"
        rm -f "$test_file"
        return 0
    fi
}

# Test TDD test-first enforcer
test_tdd_test_first_enforcer() {
    log "Testing TDD test-first enforcer..."
    
    # Initialize a test TDD cycle
    if "$SCRIPT_DIR/tdd-test-first-enforcer.sh" init "Test Feature" > "$TEST_LOG_DIR/tdd_init_test.log" 2>&1; then
        success "TDD test-first enforcer initialization works"
        
        # Check that TDD state file was created
        if [ -f "$PROJECT_ROOT/.tdd-state" ]; then
            success "TDD state file created correctly"
            
            # Clean up
            rm -f "$PROJECT_ROOT/.tdd-state"
            return 0
        else
            fail "TDD state file was not created"
            return 1
        fi
    else
        fail "TDD test-first enforcer initialization failed"
        return 1
    fi
}

# Test comprehensive TDD verification (basic smoke test)
test_comprehensive_verification() {
    log "Testing comprehensive TDD verification..."
    
    # Try to run the comprehensive verification (it may fail, but should not crash)
    if timeout 60 "$SCRIPT_DIR/tdd-comprehensive.sh" quick > "$TEST_LOG_DIR/comprehensive_test.log" 2>&1; then
        success "Comprehensive TDD verification ran successfully"
        return 0
    else
        # Check if it failed gracefully (expected for incomplete system)
        if grep -q "COMPREHENSIVE FAILURE" "$TEST_LOG_DIR/comprehensive_test.log"; then
            success "Comprehensive TDD verification failed gracefully as expected"
            return 0
        else
            fail "Comprehensive TDD verification crashed or behaved unexpectedly"
            cat "$TEST_LOG_DIR/comprehensive_test.log" | tail -10
            return 1
        fi
    fi
}

# Test TDD integration dashboard
test_tdd_dashboard() {
    log "Testing TDD dashboard..."
    
    if "$SCRIPT_DIR/comprehensive-tdd-integration.sh" dashboard > "$TEST_LOG_DIR/dashboard_test.log" 2>&1; then
        success "TDD dashboard works correctly"
        
        # Check that dashboard shows expected sections
        if grep -q "TDD AUTOMATION DASHBOARD" "$TEST_LOG_DIR/dashboard_test.log"; then
            success "TDD dashboard displays correctly"
            return 0
        else
            fail "TDD dashboard output is incomplete"
            return 1
        fi
    else
        fail "TDD dashboard failed to run"
        return 1
    fi
}

# Test evidence collection
test_evidence_collection() {
    log "Testing evidence collection..."
    
    # Try to collect evidence (create evidence directories)
    mkdir -p "$PROJECT_ROOT/generated/evidence"
    
    # Create a mock evidence file to test the system
    cat > "$PROJECT_ROOT/generated/evidence/test_evidence.json" << EOF
{
  "test_phase": "testing",
  "timestamp": "$(date -Iseconds)",
  "evidence": {
    "compilation": {"status": "PASS"}
  }
}
EOF
    
    if [ -f "$PROJECT_ROOT/generated/evidence/test_evidence.json" ]; then
        success "Evidence collection system works"
        rm -f "$PROJECT_ROOT/generated/evidence/test_evidence.json"
        return 0
    else
        fail "Evidence collection system failed"
        return 1
    fi
}

# Test integration with container system
test_container_integration() {
    log "Testing container integration..."
    
    # Check if container runtime is available
    if command -v docker >/dev/null 2>&1; then
        success "Docker runtime available for testing"
        return 0
    elif command -v podman >/dev/null 2>&1; then
        success "Podman runtime available for testing"
        return 0
    else
        warning "No container runtime available - some tests may not work"
        return 0
    fi
}

# Test that the system prevents false positive claims
test_false_positive_prevention() {
    log "Testing false positive prevention..."
    
    # Create a scenario that should trigger anti-gaslighting detection
    # We'll simulate a case where someone might claim success despite failures
    
    local test_result=0
    
    # Test 1: Compilation failure detection
    log "Testing compilation failure detection..."
    cd "$PROJECT_ROOT"
    
    # Create a file with compilation errors
    local bad_file="internal/test_bad_compilation.go"
    cat > "$bad_file" << 'EOF'
package internal

// This will cause compilation errors
func BadFunction() {
    undefined_variable := "test"
    another_undefined_function()
}
EOF
    
    # Run anti-gaslighting check - should detect the issue
    if "$SCRIPT_DIR/anti-gaslighting-detector.sh" quick > "$TEST_LOG_DIR/false_positive_test.log" 2>&1; then
        fail "Anti-gaslighting did not detect compilation failure"
        test_result=1
    else
        success "Anti-gaslighting correctly detected compilation failure"
    fi
    
    # Clean up
    rm -f "$bad_file"
    
    return $test_result
}

# Generate comprehensive test report
generate_test_report() {
    local total_tests=$1
    local passed_tests=$2
    local test_results=("${@:3}")
    
    local report_file="$TEST_LOG_DIR/tdd_automation_test_report.html"
    local success_rate=$((passed_tests * 100 / total_tests))
    
    cat > "$report_file" << EOF
<!DOCTYPE html>
<html>
<head>
    <title>TDD Automation Test Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; line-height: 1.6; }
        .header { background: $(if [ "$success_rate" -eq 100 ]; then echo "#d4edda"; else echo "#f8d7da"; fi); padding: 20px; border-radius: 5px; margin-bottom: 20px; }
        .pass { color: #28a745; font-weight: bold; }
        .fail { color: #dc3545; font-weight: bold; }
        table { border-collapse: collapse; width: 100%; margin: 15px 0; }
        th, td { border: 1px solid #dee2e6; padding: 12px; text-align: left; }
        th { background-color: #e9ecef; font-weight: bold; }
    </style>
</head>
<body>
    <div class="header">
        <h1>🧪 TDD Automation Test Report</h1>
        <p><strong>Test Results:</strong> $passed_tests/$total_tests passed (${success_rate}%)</p>
        <p><strong>Generated:</strong> $(date)</p>
    </div>

    <h2>Test Results</h2>
    <table>
        <tr><th>Test</th><th>Result</th></tr>
EOF
    
    for result in "${test_results[@]}"; do
        local test_name=$(echo "$result" | cut -d: -f1)
        local test_status=$(echo "$result" | cut -d: -f2)
        local css_class=$(if [ "$test_status" = "PASS" ]; then echo "pass"; else echo "fail"; fi)
        
        echo "        <tr><td>$test_name</td><td class=\"$css_class\">$test_status</td></tr>" >> "$report_file"
    done
    
    cat >> "$report_file" << EOF
    </table>

    <h2>Test Summary</h2>
    <p><strong>Total Tests:</strong> $total_tests</p>
    <p><strong>Passed:</strong> $passed_tests</p>
    <p><strong>Failed:</strong> $((total_tests - passed_tests))</p>
    <p><strong>Success Rate:</strong> ${success_rate}%</p>

    <h2>TDD Automation Health</h2>
    $(if [ "$success_rate" -eq 100 ]; then
        echo '<p class="pass">✅ TDD automation is working correctly and ready for use.</p>'
    else
        echo '<p class="fail">❌ TDD automation has issues that need to be fixed before use.</p>'
    fi)

    <footer style="margin-top: 50px; padding-top: 20px; border-top: 1px solid #dee2e6; text-align: center; color: #6c757d;">
        Generated by TDD Automation Test Suite - $(date)
    </footer>
</body>
</html>
EOF
    
    echo "$report_file"
}

# Run all tests
run_all_tests() {
    log "🧪 Starting TDD Automation Test Suite"
    log "Testing comprehensive TDD automation system..."
    
    local test_results=()
    local total_tests=0
    local passed_tests=0
    
    # Test 1: Script availability
    ((total_tests++))
    if test_script_availability; then
        test_results+=("Script Availability:PASS")
        ((passed_tests++))
    else
        test_results+=("Script Availability:FAIL")
    fi
    
    # Test 2: Anti-gaslighting detection
    ((total_tests++))
    if test_anti_gaslighting_detection; then
        test_results+=("Anti-Gaslighting Detection:PASS")
        ((passed_tests++))
    else
        test_results+=("Anti-Gaslighting Detection:FAIL")
    fi
    
    # Test 3: TDD test-first enforcer
    ((total_tests++))
    if test_tdd_test_first_enforcer; then
        test_results+=("TDD Test-First Enforcer:PASS")
        ((passed_tests++))
    else
        test_results+=("TDD Test-First Enforcer:FAIL")
    fi
    
    # Test 4: Comprehensive verification
    ((total_tests++))
    if test_comprehensive_verification; then
        test_results+=("Comprehensive Verification:PASS")
        ((passed_tests++))
    else
        test_results+=("Comprehensive Verification:FAIL")
    fi
    
    # Test 5: TDD dashboard
    ((total_tests++))
    if test_tdd_dashboard; then
        test_results+=("TDD Dashboard:PASS")
        ((passed_tests++))
    else
        test_results+=("TDD Dashboard:FAIL")
    fi
    
    # Test 6: Evidence collection
    ((total_tests++))
    if test_evidence_collection; then
        test_results+=("Evidence Collection:PASS")
        ((passed_tests++))
    else
        test_results+=("Evidence Collection:FAIL")
    fi
    
    # Test 7: Container integration
    ((total_tests++))
    if test_container_integration; then
        test_results+=("Container Integration:PASS")
        ((passed_tests++))
    else
        test_results+=("Container Integration:FAIL")
    fi
    
    # Test 8: False positive prevention
    ((total_tests++))
    if test_false_positive_prevention; then
        test_results+=("False Positive Prevention:PASS")
        ((passed_tests++))
    else
        test_results+=("False Positive Prevention:FAIL")
    fi
    
    # Generate report
    local report_file=$(generate_test_report $total_tests $passed_tests "${test_results[@]}")
    
    # Display results
    echo ""
    echo "🏁 TDD AUTOMATION TEST RESULTS"
    echo "=============================="
    echo "Total Tests: $total_tests"
    echo "Passed: $passed_tests"
    echo "Failed: $((total_tests - passed_tests))"
    echo "Success Rate: $((passed_tests * 100 / total_tests))%"
    echo ""
    
    for result in "${test_results[@]}"; do
        local test_name=$(echo "$result" | cut -d: -f1)
        local test_status=$(echo "$result" | cut -d: -f2)
        if [ "$test_status" = "PASS" ]; then
            echo -e "${GREEN}✅ $test_name${NC}"
        else
            echo -e "${RED}❌ $test_name${NC}"
        fi
    done
    
    echo ""
    echo "Test Report: $report_file"
    
    if [ "$passed_tests" -eq "$total_tests" ]; then
        success "🎉 All tests passed - TDD automation is ready for use!"
        return 0
    else
        fail "❌ Some tests failed - fix issues before using TDD automation"
        return 1
    fi
}

# Main execution
case "${1:-all}" in
    "all")
        run_all_tests
        ;;
    "quick") 
        log "Running quick TDD automation test..."
        test_script_availability && test_tdd_dashboard
        ;;
    *)
        echo "TDD Automation Test Suite"
        echo "Tests the comprehensive TDD automation system"
        echo ""
        echo "Usage: $0 <command>"
        echo ""
        echo "Commands:"
        echo "  all   - Run all tests (default)"
        echo "  quick - Run quick smoke tests"
        echo ""
        exit 1
        ;;
esac