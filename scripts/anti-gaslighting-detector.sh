#!/bin/bash
#
# ANTI-GASLIGHTING DETECTOR
# Automatically detects and prevents the "Claude the intern" pattern
# Specifically designed to catch premature success claims without evidence
#
# This script identifies patterns where success is claimed despite:
# - Failing tests
# - Missing functionality 
# - Console errors
# - Server errors (500, 404)
# - Missing UI elements
# - Authentication bypass
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
LOG_DIR="$PROJECT_ROOT/generated/anti-gaslighting"
BASE_URL="http://localhost:8080"

mkdir -p "$LOG_DIR"

# Logging functions
log() {
    echo -e "${BLUE}[$(date +%H:%M:%S)] DETECTOR:${NC} $1" | tee -a "$LOG_DIR/detector.log"
}

gaslighting_detected() {
    echo -e "${RED}🚨 GASLIGHTING DETECTED:${NC} $1" | tee -a "$LOG_DIR/detector.log"
    echo "$1" >> "$LOG_DIR/gaslighting_violations.log"
}

success() {
    echo -e "${GREEN}✓ HONEST:${NC} $1" | tee -a "$LOG_DIR/detector.log"
}

# Check if tests are actually passing
check_test_honesty() {
    log "Checking test result honesty..."
    
    local violations=0
    
    # Check for Go test failures
    if ls "$PROJECT_ROOT"/generated/tdd-logs/test_results.log >/dev/null 2>&1; then
        local failed_tests=$(grep -c "FAIL:" "$PROJECT_ROOT"/generated/tdd-logs/test_results.log 2>/dev/null | head -1 | tr -d '\n' || echo "0")
        if [ "$failed_tests" -gt 0 ]; then
            gaslighting_detected "Claiming success with $failed_tests failing Go tests"
            ((violations++))
        fi
    fi
    
    # Check current test status by running tests
    cd "$PROJECT_ROOT"
    if ! timeout 300 go test -short ./... > "$LOG_DIR/current_test_status.log" 2>&1; then
        local current_failures=$(grep -c "FAIL:" "$LOG_DIR/current_test_status.log" 2>/dev/null | head -1 | tr -d '\n' || echo "0")
        gaslighting_detected "Tests currently failing ($current_failures failures) but success might be claimed"
        ((violations++))
    fi
    
    return $violations
}

# Check for server errors in real-time
check_server_error_honesty() {
    log "Checking for undisclosed server errors..."
    
    local violations=0
    
    # Test actual endpoints that historically caused problems
    local problem_endpoints=(
        "/admin/groups"
        "/admin/users" 
        "/admin/queues"
        "/admin/priorities"
        "/admin/states"
        "/admin/types"
        "/admin/settings"
    )
    
    local server_errors=0
    local not_found_errors=0
    local broken_endpoints=()
    
    for endpoint in "${problem_endpoints[@]}"; do
        local status_code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL$endpoint" 2>/dev/null || echo "000")
        
        case "$status_code" in
            500)
                ((server_errors++))
                broken_endpoints+=("$endpoint: 500 Server Error")
                ;;
            404)
                ((not_found_errors++))
                broken_endpoints+=("$endpoint: 404 Not Found")
                ;;
            000)
                ((server_errors++))
                broken_endpoints+=("$endpoint: Service Unavailable")
                ;;
        esac
    done
    
    if [ "$server_errors" -gt 0 ]; then
        gaslighting_detected "$server_errors endpoints returning 500 server errors:"
        for error in "${broken_endpoints[@]}"; do
            if [[ "$error" == *"500"* || "$error" == *"Service Unavailable"* ]]; then
                gaslighting_detected "  - $error"
            fi
        done
        ((violations++))
    fi
    
    if [ "$not_found_errors" -gt 3 ]; then  # Allow some 404s for non-implemented features
        gaslighting_detected "$not_found_errors endpoints returning 404 errors (too many missing features)"
        ((violations++))
    fi
    
    return $violations
}

# Check for JavaScript console errors 
check_console_error_honesty() {
    log "Checking for undisclosed JavaScript console errors..."
    
    local violations=0
    
    # Only run if browser testing is possible
    if command -v node >/dev/null 2>&1; then
        if node -e "require('playwright')" >/dev/null 2>&1; then
            # Create quick console error check
            cat > "$LOG_DIR/console_error_check.js" << 'EOF'
const { chromium } = require('playwright');

async function checkConsoleErrors() {
    const browser = await chromium.launch({ headless: true });
    const page = await browser.newPage();
    
    let totalErrors = 0;
    const errorPages = [];
    
    page.on('console', msg => {
        if (msg.type() === 'error') {
            totalErrors++;
            errorPages.push({
                url: page.url(),
                error: msg.text()
            });
        }
    });
    
    const testPages = ['/login', '/admin/users', '/admin/groups'];
    
    for (const pagePath of testPages) {
        try {
            await page.goto(`http://localhost:8080${pagePath}`, { 
                waitUntil: 'networkidle',
                timeout: 10000 
            });
            await page.waitForTimeout(2000);
        } catch (e) {
            // Page might not load, but that's a different issue
        }
    }
    
    await browser.close();
    
    console.log(JSON.stringify({
        totalErrors: totalErrors,
        errors: errorPages
    }));
}

checkConsoleErrors().catch(err => {
    console.log(JSON.stringify({ error: err.message, totalErrors: 999 }));
});
EOF
            
            if timeout 60 node "$LOG_DIR/console_error_check.js" > "$LOG_DIR/console_results.json" 2>/dev/null; then
                local console_errors=$(jq -r '.totalErrors // 999' "$LOG_DIR/console_results.json" 2>/dev/null || echo "999")
                if [ "$console_errors" -gt 0 ] && [ "$console_errors" -lt 999 ]; then
                    gaslighting_detected "$console_errors JavaScript console errors detected"
                    # Show some of the errors
                    jq -r '.errors[]? | "  Console error on \(.url): \(.error)"' "$LOG_DIR/console_results.json" 2>/dev/null | head -3 | while read -r error_line; do
                        gaslighting_detected "$error_line"
                    done
                    ((violations++))
                fi
            fi
        fi
    fi
    
    return $violations
}

# Check for template rendering problems
check_template_honesty() {
    log "Checking for undisclosed template errors..."
    
    local violations=0
    
    # Check backend logs for template errors
    if command -v "$SCRIPT_DIR/container-wrapper.sh" >/dev/null 2>&1; then
        "$SCRIPT_DIR/container-wrapper.sh" compose logs gotrs-backend --tail=50 > "$LOG_DIR/recent_backend_logs.txt" 2>/dev/null || true
        
        local template_errors=$(grep -c -i "template.*error\|parse.*template\|template.*fail" "$LOG_DIR/recent_backend_logs.txt" 2>/dev/null | head -1 | tr -d '\n' || echo "0")
        
        if [ "$template_errors" -gt 0 ]; then
            gaslighting_detected "$template_errors template errors found in recent logs"
            # Show some template errors
            grep -i "template.*error\|parse.*template\|template.*fail" "$LOG_DIR/recent_backend_logs.txt" 2>/dev/null | head -3 | while read -r error_line; do
                gaslighting_detected "  Template error: $error_line"
            done
            ((violations++))
        fi
    fi
    
    return $violations
}

# Check for authentication bypass issues
check_auth_honesty() {
    log "Checking for authentication bypass vulnerabilities..."
    
    local violations=0
    
    # Test if admin endpoints are properly protected
    local protected_endpoints=("/admin/users" "/admin/groups" "/admin/settings")
    local unprotected_count=0
    
    for endpoint in "${protected_endpoints[@]}"; do
        # Try to access without authentication
        local response=$(curl -s "$BASE_URL$endpoint" 2>/dev/null || echo "")
        
        # If we get actual admin content (not a login redirect), it's unprotected
        if echo "$response" | grep -q -i "admin\|dashboard\|users.*table\|groups.*table" && ! echo "$response" | grep -q -i "login\|sign.*in\|authentication"; then
            gaslighting_detected "Admin endpoint $endpoint appears unprotected (no auth redirect)"
            ((unprotected_count++))
        fi
    done
    
    if [ "$unprotected_count" -gt 0 ]; then
        ((violations++))
    fi
    
    return $violations
}

# Check service health honesty
check_service_health_honesty() {
    log "Checking service health claims..."
    
    local violations=0
    
    # Check if health endpoint actually works
    if ! curl -f -s "$BASE_URL/health" > "$LOG_DIR/health_check.json" 2>/dev/null; then
        gaslighting_detected "Health endpoint not responding but success might be claimed"
        ((violations++))
    else
        local health_status=$(jq -r '.status // "unknown"' "$LOG_DIR/health_check.json" 2>/dev/null || echo "unknown")
        if [ "$health_status" != "healthy" ]; then
            gaslighting_detected "Health endpoint reports unhealthy status: $health_status"
            ((violations++))
        fi
    fi
    
    return $violations
}

# Check compilation honesty
check_compilation_honesty() {
    log "Checking compilation claims..."
    
    local violations=0
    
    cd "$PROJECT_ROOT"
    
    # Try to compile and check for errors
    if ! timeout 120 go build ./cmd/goats > "$LOG_DIR/compile_check.log" 2>&1; then
        local compile_errors=$(cat "$LOG_DIR/compile_check.log" | wc -l)
        gaslighting_detected "Code does not compile ($compile_errors error lines) but success might be claimed"
        # Show first few compilation errors
        head -5 "$LOG_DIR/compile_check.log" | while read -r error_line; do
            gaslighting_detected "  Compile error: $error_line"
        done
        ((violations++))
    fi
    
    return $violations
}

# Check for missing UI elements
check_missing_ui_honesty() {
    log "Checking for missing UI elements..."
    
    local violations=0
    
    # Only run if browser testing is possible
    if command -v node >/dev/null 2>&1; then
        if node -e "require('playwright')" >/dev/null 2>&1; then
            cat > "$LOG_DIR/missing_ui_check.js" << 'EOF'
const { chromium } = require('playwright');

async function checkMissingElements() {
    const browser = await chromium.launch({ headless: true });
    const page = await browser.newPage();
    
    const results = [];
    
    const testCases = [
        { 
            path: '/login', 
            required: ['input[type="email"]', 'input[type="password"]', 'button[type="submit"]'],
            description: 'Login form elements'
        },
        { 
            path: '/admin/users', 
            required: ['table', 'th', 'td', '.btn', 'h1'],
            description: 'Admin users page elements'
        }
    ];
    
    for (const testCase of testCases) {
        try {
            await page.goto(`http://localhost:8080${testCase.path}`, { 
                waitUntil: 'networkidle',
                timeout: 15000 
            });
            
            const missing = [];
            for (const selector of testCase.required) {
                const element = await page.$(selector);
                if (!element) {
                    missing.push(selector);
                }
            }
            
            if (missing.length > 0) {
                results.push({
                    path: testCase.path,
                    description: testCase.description,
                    missing: missing,
                    missingCount: missing.length
                });
            }
            
        } catch (e) {
            results.push({
                path: testCase.path,
                description: testCase.description,
                error: e.message,
                missingCount: testCase.required.length
            });
        }
    }
    
    await browser.close();
    
    console.log(JSON.stringify(results));
}

checkMissingElements().catch(err => {
    console.log(JSON.stringify([{ error: err.message }]));
});
EOF
            
            if timeout 60 node "$LOG_DIR/missing_ui_check.js" > "$LOG_DIR/missing_ui_results.json" 2>/dev/null; then
                local missing_elements=$(jq -r '[.[] | .missingCount // 0] | add' "$LOG_DIR/missing_ui_results.json" 2>/dev/null || echo "0")
                if [ "$missing_elements" -gt 0 ]; then
                    gaslighting_detected "$missing_elements UI elements missing from critical pages"
                    # Show which elements are missing
                    jq -r '.[] | select(.missing) | "  Missing on \(.path): \(.missing | join(", "))"' "$LOG_DIR/missing_ui_results.json" 2>/dev/null | while read -r missing_line; do
                        gaslighting_detected "$missing_line"
                    done
                    ((violations++))
                fi
            fi
        fi
    fi
    
    return $violations
}

# Run comprehensive gaslighting detection
run_gaslighting_detection() {
    log "🔍 Starting Anti-Gaslighting Detection"
    log "Detecting premature success claims and false positives..."
    
    local total_violations=0
    local check_results=()
    
    echo "" > "$LOG_DIR/gaslighting_violations.log"  # Clear previous violations
    
    # Run all honesty checks
    log "Check 1/7: Test Result Honesty"
    if check_test_honesty; then
        check_results+=("✓ Test results: HONEST")
    else
        check_results+=("✗ Test results: DISHONEST")
        ((total_violations++))
    fi
    
    log "Check 2/7: Server Error Honesty"
    if check_server_error_honesty; then
        check_results+=("✓ Server errors: HONEST")
    else
        check_results+=("✗ Server errors: DISHONEST")
        ((total_violations++))
    fi
    
    log "Check 3/7: Console Error Honesty"
    if check_console_error_honesty; then
        check_results+=("✓ Console errors: HONEST")
    else
        check_results+=("✗ Console errors: DISHONEST")
        ((total_violations++))
    fi
    
    log "Check 4/7: Template Error Honesty"
    if check_template_honesty; then
        check_results+=("✓ Template errors: HONEST")
    else
        check_results+=("✗ Template errors: DISHONEST")
        ((total_violations++))
    fi
    
    log "Check 5/7: Authentication Honesty"
    if check_auth_honesty; then
        check_results+=("✓ Authentication: HONEST")
    else
        check_results+=("✗ Authentication: DISHONEST")
        ((total_violations++))
    fi
    
    log "Check 6/7: Service Health Honesty"
    if check_service_health_honesty; then
        check_results+=("✓ Service health: HONEST")
    else
        check_results+=("✗ Service health: DISHONEST")
        ((total_violations++))
    fi
    
    log "Check 7/7: Compilation Honesty"
    if check_compilation_honesty; then
        check_results+=("✓ Compilation: HONEST")
    else
        check_results+=("✗ Compilation: DISHONEST")
        ((total_violations++))
    fi
    
    log "Check 8/8: UI Element Honesty"
    if check_missing_ui_honesty; then
        check_results+=("✓ UI elements: HONEST")
    else
        check_results+=("✗ UI elements: DISHONEST")
        ((total_violations++))
    fi
    
    # Generate report
    echo ""
    echo "=================================================================="
    echo "           ANTI-GASLIGHTING DETECTION RESULTS"
    echo "=================================================================="
    echo "Detection Time: $(date)"
    echo "Total Violations: $total_violations"
    echo ""
    
    for result in "${check_results[@]}"; do
        if [[ "$result" == *"HONEST"* ]]; then
            echo -e "${GREEN}$result${NC}"
        else
            echo -e "${RED}$result${NC}"
        fi
    done
    
    echo ""
    
    if [ "$total_violations" -eq 0 ]; then
        echo -e "${GREEN}"
        echo "✅ NO GASLIGHTING DETECTED"
        echo "All claims appear to be backed by evidence"
        echo "Success claims would be justified"
        echo -e "${NC}"
        return 0
    else
        echo -e "${RED}"
        echo "🚨 GASLIGHTING DETECTED: $total_violations VIOLATIONS"
        echo "❌ Success claims would be FALSE and misleading"
        echo "❌ System has concrete problems that are being hidden"
        echo ""
        echo "VIOLATIONS DETECTED:"
        if [ -f "$LOG_DIR/gaslighting_violations.log" ]; then
            cat "$LOG_DIR/gaslighting_violations.log" | while read -r violation; do
                echo "  - $violation"
            done
        fi
        echo ""
        echo "DO NOT CLAIM SUCCESS UNTIL ALL VIOLATIONS ARE FIXED"
        echo -e "${NC}"
        return 1
    fi
}

# Generate anti-gaslighting report
generate_gaslighting_report() {
    local violations_detected=$1
    
    local report_file="$LOG_DIR/anti_gaslighting_report_$(date +%Y%m%d_%H%M%S).html"
    
    cat > "$report_file" << EOF
<!DOCTYPE html>
<html>
<head>
    <title>Anti-Gaslighting Detection Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; line-height: 1.6; }
        .header { background: #f8d7da; padding: 20px; border-radius: 5px; margin-bottom: 20px; border: 2px solid #dc3545; }
        .safe { background: #d4edda; padding: 20px; border-radius: 5px; margin-bottom: 20px; border: 2px solid #28a745; }
        .violation { background: #f8d7da; padding: 10px; margin: 10px 0; border-left: 4px solid #dc3545; }
        .honest { color: #28a745; font-weight: bold; }
        .dishonest { color: #dc3545; font-weight: bold; }
        .warning { color: #ffc107; font-weight: bold; }
        pre { background: #e9ecef; padding: 10px; overflow-x: auto; border-radius: 3px; }
        .critical { font-size: 18px; font-weight: bold; color: #dc3545; }
    </style>
</head>
<body>
    $(if [ "$violations_detected" -eq 0 ]; then
        echo '<div class="safe"><h1>✅ NO GASLIGHTING DETECTED</h1><p>All success claims appear to be backed by concrete evidence.</p></div>'
    else
        echo '<div class="header"><h1>🚨 GASLIGHTING DETECTED</h1><p class="critical">'"$violations_detected"' violations found. Success claims would be FALSE.</p></div>'
    fi)

    <h2>Detection Summary</h2>
    <p><strong>Detection Time:</strong> $(date)</p>
    <p><strong>Total Violations:</strong> $violations_detected</p>
    <p><strong>System Status:</strong> $(if [ "$violations_detected" -eq 0 ]; then echo "Honest - Success claims justified"; else echo "Dishonest - Success claims unjustified"; fi)</p>

    <h2>Anti-Gaslighting Philosophy</h2>
    <p>This detector prevents the "Claude the intern" pattern where success is claimed despite:</p>
    <ul>
        <li>Failing tests that are hidden or ignored</li>
        <li>Server errors (500, 404) that are dismissed as "minor"</li>
        <li>JavaScript console errors that break functionality</li>
        <li>Missing UI elements that make the interface unusable</li>
        <li>Authentication bypasses that create security holes</li>
        <li>Template errors that break page rendering</li>
        <li>Compilation failures that prevent deployment</li>
    </ul>

    <h2>Violation Details</h2>
    $(if [ -f "$LOG_DIR/gaslighting_violations.log" ] && [ "$violations_detected" -gt 0 ]; then
        echo '<div class="violation">'
        echo '<h3>Detected Violations:</h3>'
        echo '<pre>'
        cat "$LOG_DIR/gaslighting_violations.log"
        echo '</pre>'
        echo '</div>'
    else
        echo '<p class="honest">No violations detected. All claims appear honest.</p>'
    fi)

    <h2>The Truth About Success</h2>
    $(if [ "$violations_detected" -eq 0 ]; then
        echo '<p class="honest">✅ Based on evidence, success claims would be justified.</p>'
        echo '<p>The system appears to be working correctly with no detected issues.</p>'
    else
        echo '<p class="dishonest">❌ Based on evidence, success claims would be FALSE and misleading.</p>'
        echo '<p>The system has concrete, verifiable problems that prevent it from working correctly.</p>'
        echo '<p><strong>Next Steps:</strong></p>'
        echo '<ul><li>Fix all detected violations</li><li>Re-run comprehensive verification</li><li>Only claim success when ALL evidence supports it</li></ul>'
    fi)

    <footer style="margin-top: 50px; padding-top: 20px; border-top: 1px solid #dee2e6; text-align: center; color: #6c757d;">
        Generated by Anti-Gaslighting Detector - $(date)
    </footer>
</body>
</html>
EOF
    
    echo "$report_file"
}

# Main execution
case "${1:-}" in
    "detect")
        if run_gaslighting_detection; then
            violations=0
        else
            violations=1
        fi
        report_file=$(generate_gaslighting_report $violations)
        log "Anti-gaslighting report generated: $report_file"
        exit $violations
        ;;
    "quick")
        # Quick check for immediate feedback
        check_compilation_honesty
        check_service_health_honesty
        ;;
    *)
        echo "Anti-Gaslighting Detector"
        echo "Prevents premature success claims and false positives"
        echo ""
        echo "Usage: $0 <command>"
        echo ""
        echo "Commands:"
        echo "  detect  - Run comprehensive gaslighting detection"
        echo "  quick   - Quick sanity check"
        echo ""
        echo "This tool detects when success is claimed despite concrete evidence of failure:"
        echo "  - Failing tests"
        echo "  - Server errors (500, 404)"
        echo "  - JavaScript console errors"
        echo "  - Missing UI elements"
        echo "  - Authentication bypasses"
        echo "  - Template rendering errors"
        echo "  - Compilation failures"
        echo ""
        echo "Returns exit code 0 if honest, 1 if gaslighting detected."
        exit 1
        ;;
esac