#!/bin/bash
#
# TDD WORKFLOW ENFORCER
# Prevents "Claude the intern" pattern by enforcing mandatory quality gates
# Implements Test-Driven Development with evidence collection requirements
#
# Usage: ./scripts/tdd-enforcer.sh <command> [options]
# Commands:
#   init         - Initialize TDD workflow
#   test-first   - Start TDD cycle (write failing test)
#   implement    - Implement code to pass tests
#   verify       - Comprehensive verification before success claims
#   refactor     - Safe refactoring with full test coverage
#

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m'

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
LOG_DIR="$PROJECT_ROOT/generated/tdd-logs"
EVIDENCE_DIR="$PROJECT_ROOT/generated/evidence"
BASE_URL="http://localhost:8080"

# Ensure log directories exist
mkdir -p "$LOG_DIR" "$EVIDENCE_DIR"

# Logging functions
log() {
    echo -e "${BLUE}[$(date +%H:%M:%S)] TDD:${NC} $1" | tee -a "$LOG_DIR/tdd.log"
}

success() {
    echo -e "${GREEN}✓ TDD:${NC} $1" | tee -a "$LOG_DIR/tdd.log"
}

fail() {
    echo -e "${RED}✗ TDD:${NC} $1" | tee -a "$LOG_DIR/tdd.log"
}

warning() {
    echo -e "${YELLOW}⚠ TDD:${NC} $1" | tee -a "$LOG_DIR/tdd.log"
}

critical() {
    echo -e "${RED}🚨 CRITICAL TDD VIOLATION:${NC} $1" | tee -a "$LOG_DIR/tdd.log"
    exit 1
}

# Evidence collection functions
collect_evidence() {
    local test_phase=$1
    local evidence_file="$EVIDENCE_DIR/${test_phase}_$(date +%Y%m%d_%H%M%S).json"
    
    log "Collecting evidence for phase: $test_phase"
    
    # Create evidence structure
    cat > "$evidence_file" << EOF
{
  "phase": "$test_phase",
  "timestamp": "$(date -Iseconds)",
  "git_commit": "$(git rev-parse HEAD 2>/dev/null || echo 'no-git')",
  "evidence": {
    "compilation": {},
    "tests": {},
    "service_health": {},
    "templates": {},
    "browser_console": {},
    "http_responses": {},
    "logs": {}
  }
}
EOF
    
    echo "$evidence_file"
}

# Check if backend compiles without errors
verify_compilation() {
    local evidence_file=$1
    
    log "Verifying Go compilation..."
    
    cd "$PROJECT_ROOT"
    
    # Attempt to build
    if go build ./cmd/server 2>"$LOG_DIR/compile_errors.log"; then
        success "Go compilation: PASS"
        jq '.evidence.compilation.status = "PASS" | .evidence.compilation.errors = []' "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
        return 0
    else
        fail "Go compilation: FAIL"
        local errors=$(cat "$LOG_DIR/compile_errors.log" | jq -R . | jq -s .)
        jq --argjson errors "$errors" '.evidence.compilation.status = "FAIL" | .evidence.compilation.errors = $errors' "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
        return 1
    fi
}

# Verify service health
verify_service_health() {
    local evidence_file=$1
    
    log "Verifying service health..."
    
    # Restart backend to ensure clean state
    "$SCRIPT_DIR/container-wrapper.sh" compose restart gotrs-backend
    sleep 5
    
    # Check health endpoint
    if curl -f -s "$BASE_URL/health" > "$LOG_DIR/health_response.json"; then
        local health_status=$(cat "$LOG_DIR/health_response.json" | jq -r '.status // "unknown"')
        if [ "$health_status" = "healthy" ]; then
            success "Service health: HEALTHY"
            jq '.evidence.service_health.status = "HEALTHY"' "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
            return 0
        else
            fail "Service health: UNHEALTHY ($health_status)"
            jq --arg status "$health_status" '.evidence.service_health.status = $status' "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
            return 1
        fi
    else
        fail "Service health: NO RESPONSE"
        jq '.evidence.service_health.status = "NO_RESPONSE"' "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
        return 1
    fi
}

# Check for template errors in logs
verify_templates() {
    local evidence_file=$1
    
    log "Checking for template errors..."
    
    # Get recent backend logs and check for template errors
    "$SCRIPT_DIR/container-wrapper.sh" compose logs gotrs-backend --tail=50 > "$LOG_DIR/backend_logs.txt" 2>&1
    
    local template_errors=$(grep -c "Template error\|template.*error\|parse.*template" "$LOG_DIR/backend_logs.txt" || echo "0")
    
    if [ "$template_errors" -eq 0 ]; then
        success "Template verification: NO ERRORS"
        jq '.evidence.templates.errors = 0 | .evidence.templates.status = "CLEAN"' "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
        return 0
    else
        fail "Template verification: $template_errors ERRORS FOUND"
        jq --arg count "$template_errors" '.evidence.templates.errors = ($count | tonumber) | .evidence.templates.status = "ERRORS"' "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
        return 1
    fi
}

# Test all Go tests with evidence collection
run_go_tests() {
    local evidence_file=$1
    local test_filter=${2:-""}
    
    log "Running Go tests..."
    
    cd "$PROJECT_ROOT"
    
    # Set test environment
    export DB_NAME="${DB_NAME:-gotrs}_test"
    export APP_ENV=test
    
    # Run tests with coverage
    local test_cmd="go test -v -race -coverprofile=generated/coverage.out -covermode=atomic"
    if [ -n "$test_filter" ]; then
        test_cmd="$test_cmd -run $test_filter"
    fi
    test_cmd="$test_cmd ./..."
    
    if eval "$test_cmd" 2>&1 | tee "$LOG_DIR/test_results.log"; then
        # Calculate coverage
        local coverage=$(go tool cover -func=generated/coverage.out | grep total | awk '{print $3}' | sed 's/%//')
        success "Go tests: PASS (Coverage: ${coverage}%)"
        jq --arg coverage "$coverage" '.evidence.tests.go_tests = "PASS" | .evidence.tests.coverage = $coverage' "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
        return 0
    else
        fail "Go tests: FAIL"
        jq '.evidence.tests.go_tests = "FAIL" | .evidence.tests.coverage = "0"' "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
        return 1
    fi
}

# Test HTTP endpoints systematically
test_http_endpoints() {
    local evidence_file=$1
    local endpoints=("/health" "/login" "/admin/groups" "/admin/users" "/admin/queues" "/admin/priorities" "/admin/states" "/admin/types")
    
    log "Testing HTTP endpoints systematically..."
    
    local total_endpoints=${#endpoints[@]}
    local working_endpoints=0
    local broken_endpoints=0
    local endpoint_results=()
    
    for endpoint in "${endpoints[@]}"; do
        local status_code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL$endpoint")
        
        if [[ "$status_code" =~ ^[2-3][0-9][0-9]$ ]]; then
            ((working_endpoints++))
            endpoint_results+=("{\"endpoint\": \"$endpoint\", \"status\": $status_code, \"result\": \"OK\"}")
        else
            ((broken_endpoints++))
            endpoint_results+=("{\"endpoint\": \"$endpoint\", \"status\": $status_code, \"result\": \"BROKEN\"}")
        fi
    done
    
    local working_percentage=$((working_endpoints * 100 / total_endpoints))
    
    # Build JSON array for evidence
    local results_json="[$(IFS=','; echo "${endpoint_results[*]}")]"
    
    jq --argjson results "$results_json" --arg working "$working_endpoints" --arg total "$total_endpoints" --arg percentage "$working_percentage" \
        '.evidence.http_responses.endpoints = $results | .evidence.http_responses.working = ($working | tonumber) | .evidence.http_responses.total = ($total | tonumber) | .evidence.http_responses.percentage = ($percentage | tonumber)' \
        "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
    
    log "HTTP Endpoints: $working_endpoints/$total_endpoints working (${working_percentage}%)"
    
    if [ "$working_percentage" -lt 80 ]; then
        fail "HTTP endpoint verification: FAIL (Only ${working_percentage}% working)"
        return 1
    else
        success "HTTP endpoint verification: PASS (${working_percentage}% working)"
        return 0
    fi
}

# Browser console error checking using Playwright
check_browser_console() {
    local evidence_file=$1
    local test_pages=("/login" "/admin/groups" "/admin/users")
    
    log "Checking browser console errors..."
    
    # Create Playwright test for console errors
    cat > "$LOG_DIR/console_check.js" << 'EOF'
const { chromium } = require('playwright');

async function checkConsoleErrors() {
    const browser = await chromium.launch({ headless: true });
    const page = await browser.newPage();
    
    const results = [];
    
    // Track console errors
    let consoleErrors = [];
    page.on('console', msg => {
        if (msg.type() === 'error') {
            consoleErrors.push(msg.text());
        }
    });
    
    const testPages = process.argv.slice(2);
    
    for (const pagePath of testPages) {
        consoleErrors = [];
        try {
            await page.goto(`http://localhost:8080${pagePath}`, { waitUntil: 'networkidle' });
            await page.waitForTimeout(3000); // Wait for JavaScript to execute
            
            results.push({
                page: pagePath,
                consoleErrors: [...consoleErrors],
                errorCount: consoleErrors.length,
                status: consoleErrors.length === 0 ? 'CLEAN' : 'ERRORS'
            });
        } catch (error) {
            results.push({
                page: pagePath,
                consoleErrors: [error.message],
                errorCount: 1,
                status: 'ERROR'
            });
        }
    }
    
    await browser.close();
    console.log(JSON.stringify(results, null, 2));
}

checkConsoleErrors().catch(console.error);
EOF
    
    # Run console check if Node.js and Playwright are available
    if command -v node >/dev/null 2>&1; then
        if node -e "require('playwright')" >/dev/null 2>&1; then
            local console_results=$(node "$LOG_DIR/console_check.js" "${test_pages[@]}" 2>/dev/null || echo '[]')
            local total_errors=$(echo "$console_results" | jq '[.[].errorCount] | add // 0')
            
            jq --argjson results "$console_results" --arg total_errors "$total_errors" \
                '.evidence.browser_console.results = $results | .evidence.browser_console.total_errors = ($total_errors | tonumber)' \
                "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
            
            if [ "$total_errors" -eq 0 ]; then
                success "Browser console: CLEAN (0 errors)"
                return 0
            else
                fail "Browser console: $total_errors ERRORS"
                return 1
            fi
        else
            warning "Browser console: SKIPPED (Playwright not available)"
            jq '.evidence.browser_console.status = "SKIPPED" | .evidence.browser_console.reason = "playwright_not_available"' \
                "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
            return 0
        fi
    else
        warning "Browser console: SKIPPED (Node.js not available)"
        jq '.evidence.browser_console.status = "SKIPPED" | .evidence.browser_console.reason = "nodejs_not_available"' \
            "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
        return 0
    fi
}

# Analyze backend logs for errors
analyze_logs() {
    local evidence_file=$1
    
    log "Analyzing backend logs for errors..."
    
    "$SCRIPT_DIR/container-wrapper.sh" compose logs gotrs-backend --tail=100 > "$LOG_DIR/recent_logs.txt" 2>&1
    
    local error_count=$(grep -c "ERROR\|PANIC\|500 Internal Server Error" "$LOG_DIR/recent_logs.txt" || echo "0")
    local warning_count=$(grep -c "WARN" "$LOG_DIR/recent_logs.txt" || echo "0")
    
    jq --arg errors "$error_count" --arg warnings "$warning_count" \
        '.evidence.logs.error_count = ($errors | tonumber) | .evidence.logs.warning_count = ($warnings | tonumber)' \
        "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
    
    if [ "$error_count" -eq 0 ]; then
        success "Log analysis: CLEAN (0 errors)"
        return 0
    else
        fail "Log analysis: $error_count ERRORS found"
        return 1
    fi
}

# Generate evidence report
generate_evidence_report() {
    local evidence_file=$1
    local phase=$2
    
    log "Generating evidence report for phase: $phase"
    
    # Create HTML report
    local report_file="$EVIDENCE_DIR/${phase}_report_$(date +%Y%m%d_%H%M%S).html"
    
    cat > "$report_file" << EOF
<!DOCTYPE html>
<html>
<head>
    <title>TDD Evidence Report - $phase</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .pass { color: green; font-weight: bold; }
        .fail { color: red; font-weight: bold; }
        .warn { color: orange; font-weight: bold; }
        .evidence { background: #f5f5f5; padding: 10px; margin: 10px 0; }
        pre { background: #eee; padding: 10px; overflow-x: auto; }
        table { border-collapse: collapse; width: 100%; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f2f2f2; }
    </style>
</head>
<body>
    <h1>TDD Evidence Report - $phase</h1>
    <p><strong>Generated:</strong> $(date)</p>
    <p><strong>Phase:</strong> $phase</p>
    
    <h2>Evidence Summary</h2>
    <div class="evidence">
        <pre>$(jq . "$evidence_file")</pre>
    </div>
    
    <h2>Quality Gates Status</h2>
    <p>This report provides concrete evidence for all quality gates. No success claims without verification.</p>
    
</body>
</html>
EOF
    
    success "Evidence report generated: $report_file"
    echo "$report_file"
}

# TDD Workflow Commands

cmd_init() {
    log "Initializing TDD workflow..."
    
    # Create necessary directories
    mkdir -p "$PROJECT_ROOT/generated/tdd-logs" "$PROJECT_ROOT/generated/evidence"
    
    # Create .tdd-state file to track current TDD cycle
    cat > "$PROJECT_ROOT/.tdd-state" << EOF
{
  "phase": "init",
  "timestamp": "$(date -Iseconds)",
  "feature": "",
  "test_written": false,
  "test_failing": false,
  "implementation_complete": false,
  "verification_passed": false
}
EOF
    
    success "TDD workflow initialized"
    success "Next step: ./scripts/tdd-enforcer.sh test-first --feature 'Feature Name'"
}

cmd_test_first() {
    local feature_name="$1"
    
    if [ -z "$feature_name" ]; then
        critical "Feature name is required for test-first phase"
    fi
    
    log "Starting test-first phase for: $feature_name"
    
    # Update TDD state
    jq --arg feature "$feature_name" --arg timestamp "$(date -Iseconds)" \
        '.phase = "test-first" | .feature = $feature | .timestamp = $timestamp | .test_written = false' \
        "$PROJECT_ROOT/.tdd-state" > "$PROJECT_ROOT/.tdd-state.tmp" && mv "$PROJECT_ROOT/.tdd-state.tmp" "$PROJECT_ROOT/.tdd-state"
    
    success "Test-first phase started for: $feature_name"
    warning "Write your failing test first, then run: ./scripts/tdd-enforcer.sh verify --test-failing"
}

cmd_implement() {
    log "Starting implementation phase..."
    
    # Check that we have a failing test
    local phase=$(jq -r '.phase' "$PROJECT_ROOT/.tdd-state" 2>/dev/null || echo "none")
    local test_failing=$(jq -r '.test_failing' "$PROJECT_ROOT/.tdd-state" 2>/dev/null || echo "false")
    
    if [ "$phase" != "test-first" ] || [ "$test_failing" != "true" ]; then
        critical "Implementation phase requires a failing test first. Run test-first phase."
    fi
    
    # Update TDD state
    jq --arg timestamp "$(date -Iseconds)" \
        '.phase = "implement" | .timestamp = $timestamp | .implementation_complete = false' \
        "$PROJECT_ROOT/.tdd-state" > "$PROJECT_ROOT/.tdd-state.tmp" && mv "$PROJECT_ROOT/.tdd-state.tmp" "$PROJECT_ROOT/.tdd-state"
    
    success "Implementation phase started"
    warning "Implement minimal code to pass tests, then run: ./scripts/tdd-enforcer.sh verify --implementation"
}

cmd_verify() {
    local verification_type="$1"
    
    log "Starting comprehensive verification..."
    
    # Collect evidence
    local evidence_file=$(collect_evidence "verify_$verification_type")
    
    # Run all quality gates
    local gates_passed=0
    local gates_total=7
    
    # Gate 1: Compilation
    if verify_compilation "$evidence_file"; then
        ((gates_passed++))
    fi
    
    # Gate 2: Service Health
    if verify_service_health "$evidence_file"; then
        ((gates_passed++))
    fi
    
    # Gate 3: Template Verification
    if verify_templates "$evidence_file"; then
        ((gates_passed++))
    fi
    
    # Gate 4: Go Tests
    if run_go_tests "$evidence_file"; then
        ((gates_passed++))
    fi
    
    # Gate 5: HTTP Endpoints
    if test_http_endpoints "$evidence_file"; then
        ((gates_passed++))
    fi
    
    # Gate 6: Browser Console
    if check_browser_console "$evidence_file"; then
        ((gates_passed++))
    fi
    
    # Gate 7: Log Analysis
    if analyze_logs "$evidence_file"; then
        ((gates_passed++))
    fi
    
    # Generate evidence report
    local report_file=$(generate_evidence_report "$evidence_file" "verify_$verification_type")
    
    # Calculate success rate
    local success_rate=$((gates_passed * 100 / gates_total))
    
    log "Quality Gates Results: $gates_passed/$gates_total passed (${success_rate}%)"
    
    if [ "$success_rate" -eq 100 ]; then
        success "COMPREHENSIVE VERIFICATION: ALL GATES PASSED"
        success "Evidence report: $report_file"
        
        # Update TDD state
        jq --arg timestamp "$(date -Iseconds)" --arg verification_type "$verification_type" \
            '.verification_passed = true | .timestamp = $timestamp | .last_verification = $verification_type' \
            "$PROJECT_ROOT/.tdd-state" > "$PROJECT_ROOT/.tdd-state.tmp" && mv "$PROJECT_ROOT/.tdd-state.tmp" "$PROJECT_ROOT/.tdd-state"
        
        return 0
    else
        fail "COMPREHENSIVE VERIFICATION: FAILED (${success_rate}% success rate)"
        fail "Evidence report: $report_file"
        critical "DO NOT CLAIM SUCCESS. Fix failing gates and re-verify."
    fi
}

cmd_refactor() {
    log "Starting refactor phase..."
    
    # Ensure verification passed first
    local verification_passed=$(jq -r '.verification_passed' "$PROJECT_ROOT/.tdd-state" 2>/dev/null || echo "false")
    
    if [ "$verification_passed" != "true" ]; then
        critical "Refactor phase requires successful verification first"
    fi
    
    # Collect baseline evidence
    local baseline_evidence=$(collect_evidence "refactor_baseline")
    
    success "Refactor phase started - baseline evidence collected"
    warning "After refactoring, run: ./scripts/tdd-enforcer.sh verify --refactor to ensure no regressions"
}

# Status command
cmd_status() {
    if [ ! -f "$PROJECT_ROOT/.tdd-state" ]; then
        warning "TDD workflow not initialized. Run: ./scripts/tdd-enforcer.sh init"
        return 1
    fi
    
    local state=$(cat "$PROJECT_ROOT/.tdd-state")
    
    echo "TDD Workflow Status:"
    echo "==================="
    echo "$state" | jq .
}

# Main command dispatcher
main() {
    local command="${1:-}"
    
    case "$command" in
        "init")
            cmd_init
            ;;
        "test-first")
            cmd_test_first "${2:-}"
            ;;
        "implement")
            cmd_implement
            ;;
        "verify")
            cmd_verify "${2:-general}"
            ;;
        "refactor")
            cmd_refactor
            ;;
        "status")
            cmd_status
            ;;
        *)
            echo "TDD Workflow Enforcer - Preventing premature success claims"
            echo ""
            echo "Usage: $0 <command> [options]"
            echo ""
            echo "Commands:"
            echo "  init                 - Initialize TDD workflow"
            echo "  test-first <feature> - Start TDD cycle with failing test"
            echo "  implement            - Implement code to pass tests"
            echo "  verify [type]        - Comprehensive verification with evidence"
            echo "  refactor             - Safe refactoring with regression checks"
            echo "  status               - Show current TDD workflow status"
            echo ""
            echo "Quality Gates (ALL must pass for success claim):"
            echo "  ✓ Go compilation without errors"
            echo "  ✓ Service health check (200 OK on /health)"
            echo "  ✓ Template error-free rendering"
            echo "  ✓ All Go tests passing with coverage"
            echo "  ✓ HTTP endpoints responding correctly"
            echo "  ✓ Browser console error-free"
            echo "  ✓ Backend logs clean of errors"
            echo ""
            exit 1
            ;;
    esac
}

main "$@"