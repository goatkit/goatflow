#!/bin/bash
#
# COMPREHENSIVE TDD INTEGRATION
# Integrates all TDD automation tools with the existing GOTRS infrastructure
# Provides unified interface for evidence-based test-driven development
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
LOG_DIR="$PROJECT_ROOT/generated/tdd-integration"

# Ensure log directory exists
mkdir -p "$LOG_DIR"

# Logging functions
log() {
    echo -e "${CYAN}[$(date +%H:%M:%S)] TDD-INTEGRATION:${NC} $1" | tee -a "$LOG_DIR/integration.log"
}

success() {
    echo -e "${GREEN}✅ TDD-INTEGRATION:${NC} $1" | tee -a "$LOG_DIR/integration.log"
}

fail() {
    echo -e "${RED}❌ TDD-INTEGRATION:${NC} $1" | tee -a "$LOG_DIR/integration.log"
}

warning() {
    echo -e "${YELLOW}⚠️ TDD-INTEGRATION:${NC} $1" | tee -a "$LOG_DIR/integration.log"
}

# Initialize comprehensive TDD environment
init_comprehensive_tdd() {
    log "🚀 Initializing Comprehensive TDD Environment"
    
    # Create all necessary directories
    mkdir -p "$PROJECT_ROOT/generated/tdd-logs"
    mkdir -p "$PROJECT_ROOT/generated/evidence"
    mkdir -p "$PROJECT_ROOT/generated/test-results" 
    mkdir -p "$PROJECT_ROOT/generated/anti-gaslighting"
    mkdir -p "$PROJECT_ROOT/generated/tdd-enforcer"
    mkdir -p "$PROJECT_ROOT/tests/integration"
    mkdir -p "$PROJECT_ROOT/tests/api"
    mkdir -p "$PROJECT_ROOT/tests/e2e"
    
    # Create .gitignore for generated files
    if [ ! -f "$PROJECT_ROOT/generated/.gitignore" ]; then
        cat > "$PROJECT_ROOT/generated/.gitignore" << EOF
# Generated test automation files
*.log
*.html
*.json
*.out
*.tmp
test_credentials.csv
evidence/
tdd-logs/
test-results/
anti-gaslighting/
tdd-enforcer/
tdd-integration/
EOF
    fi
    
    # Initialize TDD configuration
    cat > "$PROJECT_ROOT/generated/tdd-config.json" << EOF
{
  "version": "1.0.0",
  "initialized_at": "$(date -Iseconds)",
  "tools": {
    "tdd_enforcer": "$SCRIPT_DIR/tdd-enforcer.sh",
    "comprehensive_tdd": "$SCRIPT_DIR/tdd-comprehensive.sh", 
    "anti_gaslighting": "$SCRIPT_DIR/anti-gaslighting-detector.sh",
    "test_first_enforcer": "$SCRIPT_DIR/tdd-test-first-enforcer.sh"
  },
  "quality_gates": {
    "compilation": {"required": true, "timeout": 120},
    "unit_tests": {"required": true, "min_coverage": 70, "timeout": 1800},
    "integration_tests": {"required": true, "timeout": 2700},
    "security_tests": {"required": true, "timeout": 600},
    "service_health": {"required": true, "timeout": 300},
    "database_tests": {"required": true, "timeout": 600},
    "template_tests": {"required": true, "timeout": 300},
    "api_tests": {"required": true, "min_success_rate": 80, "timeout": 900},
    "browser_tests": {"required": false, "max_console_errors": 0, "timeout": 1800},
    "performance_tests": {"required": false, "max_response_time": 3000, "timeout": 600},
    "regression_tests": {"required": true, "timeout": 900}
  },
  "anti_gaslighting": {
    "enabled": true,
    "check_before_success_claims": true,
    "historical_failure_detection": true,
    "zero_tolerance_violations": [
      "server_500_errors",
      "compilation_failures", 
      "authentication_bypasses",
      "password_echoing"
    ]
  }
}
EOF
    
    success "TDD environment initialized with comprehensive automation"
    success "Configuration saved to: generated/tdd-config.json"
    
    # Show available commands
    echo ""
    echo "🧪 Comprehensive TDD Commands Available:"
    echo "========================================"
    echo "make tdd-init FEATURE='Feature Name'     - Start new TDD cycle"
    echo "make tdd-test-first                      - Generate failing test"
    echo "make tdd-implement                       - Implement to pass tests"
    echo "make tdd-verify                          - Run all quality gates"
    echo "make tdd-comprehensive                   - Full comprehensive verification"
    echo "make anti-gaslighting                   - Detect false success claims"
    echo ""
}

# Run full TDD cycle with comprehensive verification
run_full_tdd_cycle() {
    local feature_name="${1:-}"
    
    if [ -z "$feature_name" ]; then
        fail "Feature name required for TDD cycle"
        return 1
    fi
    
    log "🔄 Starting Full TDD Cycle for: $feature_name"
    
    local cycle_start_time=$(date +%s)
    local phase_results=()
    local overall_success=true
    
    # Phase 1: Initialize TDD
    log "Phase 1/6: TDD Initialization"
    if "$SCRIPT_DIR/tdd-test-first-enforcer.sh" init "$feature_name"; then
        phase_results+=("1. TDD Init: ✅ SUCCESS")
    else
        phase_results+=("1. TDD Init: ❌ FAILED")
        overall_success=false
    fi
    
    # Phase 2: Generate failing test
    log "Phase 2/6: Test Generation"
    echo "Select test type:"
    echo "1) Unit test"
    echo "2) Integration test"
    echo "3) API test"
    echo "4) Browser E2E test"
    read -p "Enter choice (1-4): " test_choice
    
    local test_type="unit"
    case "$test_choice" in
        1) test_type="unit" ;;
        2) test_type="integration" ;;
        3) test_type="api" ;;
        4) test_type="browser" ;;
    esac
    
    if test_file=$("$SCRIPT_DIR/tdd-test-first-enforcer.sh" generate-test "$test_type"); then
        phase_results+=("2. Test Gen: ✅ SUCCESS")
        log "Generated test file: $test_file"
        
        # Pause for user to customize test
        echo ""
        warning "📝 IMPORTANT: Customize your test before continuing!"
        warning "Edit the generated test file: $test_file"
        warning "Remove t.Skip() statements and add real test logic"
        read -p "Press Enter when test is ready and should fail..."
        
        # Verify test fails
        if "$SCRIPT_DIR/tdd-test-first-enforcer.sh" verify-failing "$test_file"; then
            phase_results+=("3. Test Fails: ✅ SUCCESS")
        else
            phase_results+=("3. Test Fails: ❌ FAILED")
            overall_success=false
        fi
    else
        phase_results+=("2. Test Gen: ❌ FAILED")
        overall_success=false
    fi
    
    # Phase 3: Implementation pause
    log "Phase 4/6: Implementation Phase"
    echo ""
    warning "🔧 IMPLEMENTATION TIME:"
    warning "Now implement the minimal code needed to make your test pass"
    warning "Focus only on making the test pass, don't over-engineer"
    read -p "Press Enter when implementation is complete..."
    
    # Verify tests pass
    if [ -n "${test_file:-}" ]; then
        if "$SCRIPT_DIR/tdd-test-first-enforcer.sh" verify-passing "$test_file"; then
            phase_results+=("4. Tests Pass: ✅ SUCCESS")
        else
            phase_results+=("4. Tests Pass: ❌ FAILED")
            overall_success=false
        fi
    fi
    
    # Phase 4: Anti-gaslighting check
    log "Phase 5/6: Anti-Gaslighting Verification"
    if "$SCRIPT_DIR/anti-gaslighting-detector.sh" detect; then
        phase_results+=("5. Anti-Gaslighting: ✅ SUCCESS")
    else
        phase_results+=("5. Anti-Gaslighting: ❌ FAILED")
        overall_success=false
    fi
    
    # Phase 5: Comprehensive verification
    log "Phase 6/6: Comprehensive Quality Gates"
    if "$SCRIPT_DIR/tdd-comprehensive.sh" comprehensive; then
        phase_results+=("6. Comprehensive: ✅ SUCCESS")
    else
        phase_results+=("6. Comprehensive: ❌ FAILED")
        overall_success=false
    fi
    
    # Calculate cycle time
    local cycle_end_time=$(date +%s)
    local cycle_duration=$((cycle_end_time - cycle_start_time))
    local cycle_minutes=$((cycle_duration / 60))
    local cycle_seconds=$((cycle_duration % 60))
    
    # Generate comprehensive cycle report
    local report_file="$LOG_DIR/full_tdd_cycle_$(date +%Y%m%d_%H%M%S).html"
    
    cat > "$report_file" << EOF
<!DOCTYPE html>
<html>
<head>
    <title>Full TDD Cycle Report - $feature_name</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; line-height: 1.6; }
        .header { background: $(if [ "$overall_success" = true ]; then echo "#d4edda"; else echo "#f8d7da"; fi); padding: 20px; border-radius: 5px; margin-bottom: 20px; border: 2px solid $(if [ "$overall_success" = true ]; then echo "#28a745"; else echo "#dc3545"; fi); }
        .success { color: #28a745; font-weight: bold; }
        .failure { color: #dc3545; font-weight: bold; }
        .phase { background: #f8f9fa; padding: 15px; margin: 15px 0; border-left: 4px solid #007bff; }
        table { border-collapse: collapse; width: 100%; margin: 15px 0; }
        th, td { border: 1px solid #dee2e6; padding: 12px; text-align: left; }
        th { background-color: #e9ecef; font-weight: bold; }
        .metric { text-align: center; font-size: 18px; font-weight: bold; }
    </style>
</head>
<body>
    <div class="header">
        <h1>🧪 Full TDD Cycle Report</h1>
        <p><strong>Feature:</strong> $feature_name</p>
        <p><strong>Overall Result:</strong> <span class="$(if [ "$overall_success" = true ]; then echo "success"; else echo "failure"; fi)">$(if [ "$overall_success" = true ]; then echo "✅ SUCCESS"; else echo "❌ FAILURE"; fi)</span></p>
        <p><strong>Cycle Duration:</strong> ${cycle_minutes}m ${cycle_seconds}s</p>
        <p><strong>Completed:</strong> $(date)</p>
    </div>

    <h2>TDD Cycle Phase Results</h2>
    <table>
        <tr><th>Phase</th><th>Result</th><th>Description</th></tr>
EOF
    
    local phase_descriptions=(
        "TDD initialization and state tracking"
        "Failing test generation with proper structure"
        "Verification that test actually fails before implementation"
        "Minimal implementation to make tests pass"
        "Detection of false success claims and hidden failures"
        "Comprehensive quality gate verification with evidence"
    )
    
    for i in "${!phase_results[@]}"; do
        local phase_result="${phase_results[$i]}"
        local phase_desc="${phase_descriptions[$i]}"
        echo "        <tr><td>$(echo "$phase_result" | cut -d: -f1)</td><td>$(echo "$phase_result" | cut -d: -f2)</td><td>$phase_desc</td></tr>" >> "$report_file"
    done
    
    cat >> "$report_file" << EOF
    </table>

    <h2>TDD Principles Enforced</h2>
    <div class="phase">
        <ul>
            <li>✅ Test-first development (no implementation without failing test)</li>
            <li>✅ Red-Green-Refactor cycle followed</li>
            <li>✅ Anti-gaslighting protection (no false success claims)</li>
            <li>✅ Evidence-based verification</li>
            <li>✅ Comprehensive quality gate coverage</li>
            <li>✅ Historical failure pattern prevention</li>
        </ul>
    </div>

    <h2>Quality Assurance Metrics</h2>
    <div class="metric">
        Total Cycle Time: ${cycle_minutes}m ${cycle_seconds}s<br>
        Success Rate: $(if [ "$overall_success" = true ]; then echo "100%"; else echo "< 100%"; fi)<br>
        Evidence Files Generated: $(find "$PROJECT_ROOT/generated" -name "*.json" -o -name "*.html" -newer "$PROJECT_ROOT/generated/tdd-config.json" | wc -l)
    </div>

    $(if [ "$overall_success" = true ]; then
        echo '<div class="phase"><h3 class="success">🎉 TDD CYCLE COMPLETED SUCCESSFULLY</h3><p>All phases completed with comprehensive verification. The feature has been developed using proper TDD practices with evidence-based success verification.</p></div>'
    else
        echo '<div class="phase"><h3 class="failure">❌ TDD CYCLE INCOMPLETE</h3><p>One or more phases failed. Review the failures above and complete the TDD cycle properly before claiming success.</p></div>'
    fi)

    <footer style="margin-top: 50px; padding-top: 20px; border-top: 1px solid #dee2e6; text-align: center; color: #6c757d;">
        Generated by Comprehensive TDD Integration - $(date)
    </footer>
</body>
</html>
EOF
    
    # Display results
    echo ""
    echo "🏁 TDD CYCLE COMPLETED"
    echo "======================"
    for result in "${phase_results[@]}"; do
        if [[ "$result" == *"SUCCESS"* ]]; then
            echo -e "${GREEN}$result${NC}"
        else
            echo -e "${RED}$result${NC}"
        fi
    done
    echo ""
    echo "Cycle Duration: ${cycle_minutes}m ${cycle_seconds}s"
    echo "Report Generated: $report_file"
    
    if [ "$overall_success" = true ]; then
        success "🎉 TDD CYCLE SUCCESSFUL - Feature development complete!"
        return 0
    else
        fail "❌ TDD CYCLE INCOMPLETE - Fix failures before claiming success"
        return 1
    fi
}

# Quick development verification (subset for fast feedback)
run_quick_verification() {
    log "⚡ Running Quick Development Verification"
    
    local quick_results=()
    local quick_success=true
    
    # Quick compilation check
    cd "$PROJECT_ROOT"
    if timeout 60 go build ./cmd/server > "$LOG_DIR/quick_compile.log" 2>&1; then
        quick_results+=("Compilation: ✅")
    else
        quick_results+=("Compilation: ❌")
        quick_success=false
    fi
    
    # Quick test run (short tests only)
    if timeout 180 go test -short ./... > "$LOG_DIR/quick_tests.log" 2>&1; then
        quick_results+=("Quick Tests: ✅")
    else
        quick_results+=("Quick Tests: ❌")
        quick_success=false
    fi
    
    # Quick service health (if running)
    if curl -f -s "http://localhost:8080/health" > /dev/null 2>&1; then
        quick_results+=("Service Health: ✅")
    else
        quick_results+=("Service Health: ⚠️ Not Running")
    fi
    
    # Quick gaslighting check
    if "$SCRIPT_DIR/anti-gaslighting-detector.sh" quick > /dev/null 2>&1; then
        quick_results+=("Anti-Gaslighting: ✅")
    else
        quick_results+=("Anti-Gaslighting: ❌")
        quick_success=false
    fi
    
    echo "Quick Verification Results:"
    for result in "${quick_results[@]}"; do
        if [[ "$result" == *"✅"* ]]; then
            echo -e "${GREEN}  $result${NC}"
        elif [[ "$result" == *"⚠️"* ]]; then
            echo -e "${YELLOW}  $result${NC}"
        else
            echo -e "${RED}  $result${NC}"
        fi
    done
    
    if [ "$quick_success" = true ]; then
        success "Quick verification passed - safe to continue development"
        return 0
    else
        fail "Quick verification failed - fix issues before continuing"
        return 1
    fi
}

# Show TDD dashboard with current status
show_tdd_dashboard() {
    log "📊 TDD Dashboard"
    
    echo ""
    echo "🧪 GOTRS TDD AUTOMATION DASHBOARD"
    echo "=================================="
    echo "Current Time: $(date)"
    echo ""
    
    # Show TDD state if exists
    if [ -f "$PROJECT_ROOT/.tdd-state" ]; then
        echo "📋 Current TDD Cycle:"
        echo "--------------------"
        local feature=$(jq -r '.feature // "Unknown"' "$PROJECT_ROOT/.tdd-state")
        local phase=$(jq -r '.phase // "Unknown"' "$PROJECT_ROOT/.tdd-state")
        local test_written=$(jq -r '.test_written // false' "$PROJECT_ROOT/.tdd-state")
        local test_failing=$(jq -r '.test_failing // false' "$PROJECT_ROOT/.tdd-state")
        local tests_passing=$(jq -r '.tests_passing // false' "$PROJECT_ROOT/.tdd-state")
        
        echo "Feature: $feature"
        echo "Phase: $phase"
        echo "Test Written: $(if [ "$test_written" = "true" ]; then echo "✅"; else echo "❌"; fi)"
        echo "Test Failing: $(if [ "$test_failing" = "true" ]; then echo "✅"; else echo "❌"; fi)"
        echo "Tests Passing: $(if [ "$tests_passing" = "true" ]; then echo "✅"; else echo "❌"; fi)"
        echo ""
    else
        echo "📋 No Active TDD Cycle"
        echo "--------------------"
        echo "Start a new cycle: make tdd-init FEATURE='Feature Name'"
        echo ""
    fi
    
    # Show recent evidence files
    echo "📁 Recent Evidence Files:"
    echo "------------------------"
    if find "$PROJECT_ROOT/generated/evidence" -name "*.html" -type f 2>/dev/null | head -3 | while read -r evidence_file; do
        echo "$(ls -la "$evidence_file" | awk '{print $6, $7, $8, $9}')"
    done; then
        echo ""
    else
        echo "No evidence files found"
        echo ""
    fi
    
    # Show test count
    echo "📊 Test Statistics:"
    echo "------------------"
    local total_test_files=$(find "$PROJECT_ROOT" -name "*_test.go" -type f | wc -l)
    local unit_tests=$(find "$PROJECT_ROOT/internal" -name "*_test.go" -type f | wc -l)
    local integration_tests=$(find "$PROJECT_ROOT/tests" -name "*_test.go" -type f 2>/dev/null | wc -l)
    
    echo "Total Test Files: $total_test_files"
    echo "Unit Tests: $unit_tests"
    echo "Integration/E2E Tests: $integration_tests"
    echo ""
    
    # Show available commands
    echo "🔧 Available TDD Commands:"
    echo "-------------------------"
    echo "make tdd-init FEATURE='Feature Name'  - Start new TDD cycle"
    echo "make tdd-comprehensive                - Full comprehensive verification"
    echo "make anti-gaslighting                - Check for false success claims"
    echo "make tdd-quick                       - Quick development verification"
    echo "make tdd-dashboard                   - Show this dashboard"
    echo ""
}

# Main command dispatcher
case "${1:-dashboard}" in
    "init")
        init_comprehensive_tdd
        ;;
    "full-cycle")
        feature_name="${2:-}"
        run_full_tdd_cycle "$feature_name"
        ;;
    "quick")
        run_quick_verification
        ;;
    "dashboard")
        show_tdd_dashboard
        ;;
    *)
        echo "Comprehensive TDD Integration"
        echo "Unified interface for evidence-based test-driven development"
        echo ""
        echo "Usage: $0 <command> [options]"
        echo ""
        echo "Commands:"
        echo "  init              - Initialize comprehensive TDD environment"
        echo "  full-cycle 'Name' - Run complete TDD cycle with verification"
        echo "  quick             - Quick verification for development"
        echo "  dashboard         - Show TDD status dashboard"
        echo ""
        echo "Integration with make commands:"
        echo "  make tdd-init FEATURE='Feature Name'"
        echo "  make tdd-comprehensive"
        echo "  make anti-gaslighting" 
        echo "  make tdd-quick"
        echo "  make tdd-dashboard"
        echo ""
        exit 1
        ;;
esac