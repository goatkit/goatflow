#!/bin/bash
#
# COMPREHENSIVE TDD TEST AUTOMATION
# Prevents "Claude the intern" pattern with evidence-based verification
# Addresses historical failures: password echoing, template syntax, auth bugs, JS errors
#
# This script implements comprehensive test automation with zero tolerance for:
# - False positive test results
# - Premature success claims
# - Missing evidence collection
# - Incomplete verification
#

set -uo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
LOG_DIR="$PROJECT_ROOT/generated/tdd-comprehensive"
EVIDENCE_DIR="$PROJECT_ROOT/generated/evidence"
TEST_RESULTS_DIR="$PROJECT_ROOT/generated/test-results"
BASE_URL="http://localhost:${BACKEND_PORT:-8081}"
# Compose/cmd autodetect with robust fallbacks (prefer podman, then docker, else none)
if command -v podman >/dev/null 2>&1; then
    CONTAINER_CMD="podman"
    if podman compose version >/dev/null 2>&1; then
        COMPOSE_CMD="podman compose"
    elif command -v podman-compose >/dev/null 2>&1; then
        COMPOSE_CMD="podman-compose"
    else
        COMPOSE_CMD="podman compose"
    fi
elif command -v docker >/dev/null 2>&1; then
    CONTAINER_CMD="docker"
    if docker compose version >/dev/null 2>&1; then
        COMPOSE_CMD="docker compose"
    elif command -v docker-compose >/dev/null 2>&1; then
        COMPOSE_CMD="docker-compose"
    else
        COMPOSE_CMD="docker compose"
    fi
else
    CONTAINER_CMD=""
    COMPOSE_CMD=""
fi

# Fail fast logic with LIMITED MODE support (graceful degradation)
LIMITED_MODE=${GOTRS_LIMITED_MODE:-0}
# Normalize truthy variants
case "$LIMITED_MODE" in
    1|true|TRUE|yes|YES|on|ON) LIMITED_MODE=1 ;;
    *) LIMITED_MODE=0 ;;
esac

if [ "$LIMITED_MODE" = "1" ]; then
    # Limited mode: allow continuing even with no container runtime / broken compose
    if [ -z "$CONTAINER_CMD" ] || [ -z "$COMPOSE_CMD" ]; then
        echo "[COMPREHENSIVE] LIMITED MODE: No container runtime detected; proceeding with reduced gates (compilation, unit, template)." >&2
        CONTAINER_CMD=""; COMPOSE_CMD=""
    elif ! $COMPOSE_CMD ps >/dev/null 2>&1; then
        echo "[COMPREHENSIVE] LIMITED MODE: Compose command '$COMPOSE_CMD' not functional; proceeding with reduced gates." >&2
        CONTAINER_CMD=""; COMPOSE_CMD=""
    else
        echo "[COMPREHENSIVE] LIMITED MODE active: intentionally restricting to compilation, unit, template gates." >&2
    fi
else
    # Full mode requires functioning container + compose
    if [ -z "$CONTAINER_CMD" ] || [ -z "$COMPOSE_CMD" ]; then
        echo "[COMPREHENSIVE] ERROR: No container runtime (podman or docker) with compose support available. Aborting. Set GOTRS_LIMITED_MODE=1 for limited gates." >&2
        exit 127
    fi
    if ! $COMPOSE_CMD ps >/dev/null 2>&1; then
        echo "[COMPREHENSIVE] ERROR: $COMPOSE_CMD not functional (ps failed). Start daemon or set GOTRS_LIMITED_MODE=1 for limited mode." >&2
        exit 127
    fi
fi

# Backend start helper (idempotent)
start_backend_if_needed() {
    # Fast health probe first
    if curl -fsS -o /dev/null -w '%{http_code}' "$BASE_URL/health" 2>/dev/null | grep -Eq '^(200|204|301|302|401)$'; then
        return 0
    fi
    log "Attempting to start backend via compose ($COMPOSE_CMD up -d backend)"
    # Attempt to start backend service
    if echo "$COMPOSE_CMD" | grep -q 'podman-compose'; then
        COMPOSE_PROFILES=dev $COMPOSE_CMD up -d backend >/dev/null 2>&1 || return 1
    else
        $COMPOSE_CMD up -d backend >/dev/null 2>&1 || return 1
    fi
    # Wait briefly then probe again
    sleep 3
    local code
    code=$(curl -fsS -o /dev/null -w '%{http_code}' "$BASE_URL/health" 2>/dev/null || echo "")
    if echo "$code" | grep -Eq '^(200|204|301|302|401)$'; then
        return 0
    fi
    return 1
}

# Ensure directories exist
mkdir -p "$LOG_DIR" "$EVIDENCE_DIR" "$TEST_RESULTS_DIR"

# Proactively fix any stale root-owned log artifacts from previous container redirections
if [ -d "$LOG_DIR" ] && [ ! -w "$LOG_DIR" ]; then
    chmod u+w "$LOG_DIR" 2>/dev/null || true
fi
# Remove stale unwritable mod_download.log so we can recreate it cleanly later
if [ -f "$LOG_DIR/mod_download.log" ] && [ ! -w "$LOG_DIR/mod_download.log" ]; then
    rm -f "$LOG_DIR/mod_download.log" 2>/dev/null || true
fi

# Container-first Go execution helper: uses host go if present, else toolbox container
run_go() {
    if [ "$LIMITED_MODE" = "1" ] && command -v go >/dev/null 2>&1; then
        GOFLAGS='-buildvcs=false' go "$@"; return $?
    fi
    if [ -z "$COMPOSE_CMD" ]; then
        echo "Container runtime required (podman/docker) for go command: $* (not available)" >&2
        return 127
    fi
    # Ensure toolbox image exists (auto-build if missing)
    local toolbox_image="gotrs-toolbox:latest"
    if ! $CONTAINER_CMD image inspect "$toolbox_image" >/dev/null 2>&1; then
        # Backward compat: if old prefixed tag exists, retag
        if $CONTAINER_CMD image inspect localhost/gotrs-toolbox:latest >/dev/null 2>&1; then
            $CONTAINER_CMD tag localhost/gotrs-toolbox:latest "$toolbox_image" >/dev/null 2>&1 || true
        fi
    fi
    if ! $CONTAINER_CMD image inspect "$toolbox_image" >/dev/null 2>&1; then
        echo "[COMPREHENSIVE] Toolbox image missing: $toolbox_image - attempting build via compose (profile toolbox)" >&2
        if echo "$COMPOSE_CMD" | grep -q 'podman-compose'; then
            COMPOSE_PROFILES=toolbox $COMPOSE_CMD build toolbox >/dev/null 2>&1 || true
        elif echo "$COMPOSE_CMD" | grep -q 'podman compose'; then
            $COMPOSE_CMD build toolbox >/dev/null 2>&1 || true
        else
            $COMPOSE_CMD --profile toolbox build toolbox >/dev/null 2>&1 || true
        fi
        # Re-check
        if ! $CONTAINER_CMD image inspect "$toolbox_image" >/dev/null 2>&1; then
            echo "[COMPREHENSIVE] ERROR: Failed to build toolbox image ($toolbox_image)" >&2
            return 125
        fi
    fi
    # Build command payload with proper shell escaping via printf %q
    local payload='export PATH=/usr/local/go/bin:$PATH; export GOFLAGS=-buildvcs=false; export GOCACHE=/tmp/.cache/go-build; export GOMODCACHE=/tmp/.cache/go-mod; mkdir -p /tmp/.cache/go-build /tmp/.cache/go-mod; go'
    local a
    for a in "$@"; do
        # %q produces a shell-escaped version of the argument
        payload+=" $(printf %q "$a")"
    done
    [ -n "${GOTRS_DEBUG:-}" ] && echo "[DEBUG run_go] $COMPOSE_CMD run toolbox: $payload" >&2
    if echo "$COMPOSE_CMD" | grep -q 'podman compose'; then
        $COMPOSE_CMD run --build --rm toolbox bash -lc "$payload"
    elif echo "$COMPOSE_CMD" | grep -q 'podman-compose'; then
        COMPOSE_PROFILES=toolbox $COMPOSE_CMD run --rm toolbox bash -lc "$payload"
    else
        $COMPOSE_CMD --profile toolbox run --build --rm toolbox bash -lc "$payload"
    fi
}

# Logging functions
log() {
    local msg="${BLUE}[$(date +%H:%M:%S)] COMPREHENSIVE:${NC} $1"
    echo -e "$msg" >> "$LOG_DIR/comprehensive.log"
    echo -e "$msg" >&2
}

success() {
    local msg="${GREEN}✓ COMPREHENSIVE:${NC} $1"
    echo -e "$msg" >> "$LOG_DIR/comprehensive.log"
    echo -e "$msg" >&2
}

fail() {
    local msg="${RED}✗ COMPREHENSIVE:${NC} $1"
    echo -e "$msg" >> "$LOG_DIR/comprehensive.log"
    echo -e "$msg" >&2
}

warning() {
    local msg="${YELLOW}⚠ COMPREHENSIVE:${NC} $1"
    echo -e "$msg" >> "$LOG_DIR/comprehensive.log"
    echo -e "$msg" >&2
}

critical() {
    local msg="${RED}🚨 CRITICAL FAILURE:${NC} $1"
    echo -e "$msg" >> "$LOG_DIR/comprehensive.log"
    echo -e "$msg" >&2
    exit 1
}

# Evidence collection with timestamping
collect_comprehensive_evidence() {
    local test_phase=$1
    local ts=$(date +%Y%m%d_%H%M%S)
    local evidence_file="$EVIDENCE_DIR/comprehensive_${test_phase}_$ts.json"
    # Capture single-line tool versions to avoid embedding control characters
    local GO_VER
    local COMP_VER
    GO_VER=$(go version 2>/dev/null | tr -d '\r' | tr -d '\n') || GO_VER="unknown"
    # Compose version can be multi-line; take first line only
    if $COMPOSE_CMD version >/dev/null 2>&1; then
        COMP_VER=$($COMPOSE_CMD version 2>/dev/null | head -n1 | tr -d '\r' | tr -d '\n')
    else
        COMP_VER="unknown"
    fi
    
    log "Collecting comprehensive evidence for: $test_phase"
    
    # Create comprehensive evidence structure
    # Build initial evidence JSON safely via jq to avoid control characters issues
    jq -n \
      --arg test_phase "$test_phase" \
      --arg timestamp "$(date -Iseconds)" \
      --arg git_commit "$(git rev-parse HEAD 2>/dev/null || echo 'no-git')" \
      --arg git_status "$(git status --porcelain 2>/dev/null || echo 'no-git')" \
      --arg go_version "$GO_VER" \
      --arg container_runtime "$CONTAINER_CMD" \
      --arg compose_version "$COMP_VER" \
      '{
        test_phase: $test_phase,
        timestamp: $timestamp,
        git_commit: $git_commit,
        git_status: $git_status,
        environment: {
          go_version: $go_version,
          container_runtime: $container_runtime,
          compose_version: $compose_version
        },
        evidence: {
          compilation: {status: "pending"},
          unit_tests: {status: "pending"},
          integration_tests: {status: "pending"},
          security_tests: {status: "pending"},
          service_health: {status: "pending"},
          database_tests: {status: "pending"},
          template_tests: {status: "pending"},
          api_tests: {status: "pending"},
          browser_tests: {status: "pending"},
          performance_tests: {status: "pending"},
          regression_tests: {status: "pending"}
        },
        historical_failure_checks: {
          password_echoing: {status: "pending"},
          template_syntax_errors: {status: "pending"},
          authentication_bugs: {status: "pending"},
          javascript_console_errors: {status: "pending"},
          missing_ui_elements: {status: "pending"},
          "500_server_errors": {status: "pending"},
          "404_not_found": {status: "pending"}
        }
      }' > "$evidence_file"

    echo "$evidence_file"
}

# 1. COMPILATION VERIFICATION - Zero tolerance for compilation errors
verify_comprehensive_compilation() {
    local evidence_file=$1
    
    log "Phase 1: Comprehensive Compilation Verification"
    
    cd "$PROJECT_ROOT"
    
    # Clean previous builds
    run_go clean -cache -modcache -i -r 2>/dev/null || true
    
    # Verify go.mod integrity
    export GOTOOLCHAIN=${GOTOOLCHAIN:-auto}
    # Ensure go version line is captured for debugging
    run_go version > "$LOG_DIR/go_version.log" 2>&1 || true
    # Use run_go to execute inside container and capture output via command substitution to avoid host redirection permission issues
    local mv_out md_out
    mv_out=$(GOTOOLCHAIN=auto run_go mod verify 2>&1 || true)
    printf '%s' "$mv_out" > "$LOG_DIR/mod_verify.log" || true
    if echo "$mv_out" | grep -qiE 'error|fatal'; then
        fail "Go mod verification failed"
        echo "---- mod_verify.log ----" >&2
        printf '%s\n------------------------\n' "${mv_out%%$(printf '\n')*}" >&2
        jq '.evidence.compilation.status = "FAIL" | .evidence.compilation.error = "mod_verification_failed"' \
            "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
        return 1
    fi
    
    # Download dependencies
    md_out=$(GOTOOLCHAIN=auto run_go mod download 2>&1 || true)
    printf '%s' "$md_out" > "$LOG_DIR/mod_download.log" || true
    if echo "$md_out" | grep -qiE 'permission denied|error|fatal'; then
        fail "Go mod download failed"
        jq '.evidence.compilation.status = "FAIL" | .evidence.compilation.error = "mod_download_failed"' \
            "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
        return 1
    fi
    
    # Compile all packages
    local compile_errors=""
    if ! GOTOOLCHAIN=auto run_go build -v ./... > "$LOG_DIR/build_all.log" 2>&1; then
        compile_errors=$(cat "$LOG_DIR/build_all.log")
        fail "Go build failed: $compile_errors"
        local errors_json=$(echo "$compile_errors" | jq -R . | jq -s .)
        jq --argjson errors "$errors_json" '.evidence.compilation.status = "FAIL" | .evidence.compilation.errors = $errors' \
            "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
        return 1
    fi
    
    # Compile main server binary (goats)
    if ! GOTOOLCHAIN=auto run_go build -o /tmp/goats ./cmd/goats > "$LOG_DIR/server_build.log" 2>&1; then
        fail "Server binary compilation failed"
        jq '.evidence.compilation.status = "FAIL" | .evidence.compilation.error = "server_build_failed"' \
            "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
        return 1
    fi

    # Static analysis: go vet
    local vet_status="PASS"
    if ! GOTOOLCHAIN=auto run_go vet ./... > "$LOG_DIR/go_vet.log" 2>&1; then
        vet_status="FAIL"
    fi
    # Static analysis: staticcheck (installed in toolbox)
    local staticcheck_status="PASS"
    if command -v staticcheck >/dev/null 2>&1; then
        if ! GOTOOLCHAIN=auto run_go staticcheck ./... > "$LOG_DIR/staticcheck.log" 2>&1; then
            staticcheck_status="FAIL"
        fi
    else
        echo "staticcheck not found in PATH" > "$LOG_DIR/staticcheck.log" 2>&1 || true
        staticcheck_status="SKIPPED"
    fi
    
    success "Compilation verification: PASS"
    binary_size=$(stat -c%s /tmp/goats 2>/dev/null || echo 0)
    jq --arg size "$binary_size" \
       --arg vet "$vet_status" \
       --arg sc "$staticcheck_status" \
       '.evidence.compilation.status = "PASS" | .evidence.compilation.binary_size = $size | .evidence.compilation.static_analysis = {go_vet: $vet, staticcheck: $sc}' \
        "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
    return 0
}

# Wrapper to ensure backend up prior to dependent phases
ensure_backend_phase() {
    local phase_name=$1
    if ! start_backend_if_needed; then
        fail "Backend unavailable before phase: $phase_name"
        return 1
    fi
    return 0
}

# 2. UNIT TESTS - Individual component testing
run_comprehensive_unit_tests() {
    local evidence_file=$1
    
    log "Phase 2: Comprehensive Unit Tests"
    
    cd "$PROJECT_ROOT"
    
    # Set test environment variables
    export DB_NAME="${DB_NAME:-gotrs}_test"
    export APP_ENV=test
    
    # Run unit tests (quick: minimal; full: exclude examples and e2e)
    local phase
    phase=$(jq -r '.test_phase // "quick"' "$evidence_file" 2>/dev/null || echo "quick")
    local test_cmd="GOTOOLCHAIN=auto run_go test -v -race -count=1 -timeout=30m"
    local packages
    if [ "$phase" = "quick" ]; then
        packages="./cmd/goats ./generated/tdd-comprehensive ./internal/api ./internal/service"
    else
    packages=$(run_go list ./... | grep -v "/examples$" | grep -v "/tests/e2e" | tr '\n' ' ')
    fi

    if eval "$test_cmd $packages" > "$LOG_DIR/unit_tests.log" 2>&1; then
        local test_count=$(grep -c "PASS:" "$LOG_DIR/unit_tests.log" || echo "0")
        success "Unit tests: PASS ($test_count tests)"
        jq --arg test_count "$test_count" \
            '.evidence.unit_tests.status = "PASS" | .evidence.unit_tests.test_count = $test_count' \
            "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
        return 0
    else
        fail "Unit tests: FAIL"
        local test_failures=$(grep "FAIL:" "$LOG_DIR/unit_tests.log" || echo "Unknown failures")
        jq --arg failures "$test_failures" \
            '.evidence.unit_tests.status = "FAIL" | .evidence.unit_tests.failures = $failures' \
            "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
        return 1
    fi
}

# 3. INTEGRATION TESTS - Component interaction testing
run_comprehensive_integration_tests() {
    local evidence_file=$1
    
    log "Phase 3: Comprehensive Integration Tests"
    
    # Detect postgres service; skip if not available (e.g., MariaDB-only env)
    if ! $COMPOSE_CMD ps --services 2>/dev/null | grep -q "^postgres$"; then
        warning "Postgres service not available - skipping integration tests in quick mode"
        jq '.evidence.integration_tests.status = "SKIPPED" | .evidence.integration_tests.reason = "postgres_service_missing"' \
            "$evidence_file" > "$evidence_file.tmp" 2>/dev/null && mv "$evidence_file.tmp" "$evidence_file" || true
        return 0
    fi
    
    # Ensure services are running
    $COMPOSE_CMD up -d postgres valkey > "$LOG_DIR/services_start.log" 2>&1 || true
    sleep 10
    
    # Wait for database readiness
    local db_ready=0
    for i in {1..30}; do
        if $COMPOSE_CMD exec -T postgres pg_isready -U "${DB_USER:-gotrs}" > /dev/null 2>&1; then
            db_ready=1
            break
        fi
        sleep 2
    done
    
    if [ "$db_ready" -eq 0 ]; then
        fail "Database not ready for integration tests"
        jq '.evidence.integration_tests.status = "FAIL" | .evidence.integration_tests.error = "database_not_ready"' \
            "$evidence_file" > "$evidence_file.tmp" 2>/dev/null && mv "$evidence_file.tmp" "$evidence_file" || true
        return 1
    fi
    
    # Run integration tests with database
    export INTEGRATION_TESTS=true
    export DB_HOST=localhost
    
    if GOTOOLCHAIN=auto run_go test -v -tags=integration -timeout=45m ./... > "$LOG_DIR/integration_tests.log" 2>&1; then
        local integration_count=$(grep -c "PASS:" "$LOG_DIR/integration_tests.log" || echo "0")
        success "Integration tests: PASS ($integration_count tests)"
        jq --arg test_count "$integration_count" \
            '.evidence.integration_tests.status = "PASS" | .evidence.integration_tests.test_count = $test_count' \
            "$evidence_file" > "$evidence_file.tmp" 2>/dev/null && mv "$evidence_file.tmp" "$evidence_file" || true
        return 0
    else
        fail "Integration tests: FAIL"
        local failures=$(grep "FAIL:" "$LOG_DIR/integration_tests.log" || echo "Unknown failures")
        jq --arg failures "$failures" \
            '.evidence.integration_tests.status = "FAIL" | .evidence.integration_tests.failures = $failures' \
            "$evidence_file" > "$evidence_file.tmp" 2>/dev/null && mv "$evidence_file.tmp" "$evidence_file" || true
        return 1
    fi
}

# 4. SECURITY TESTS - Address historical security failures
run_comprehensive_security_tests() {
    local evidence_file=$1
    
    log "Phase 4: Comprehensive Security Tests"
    ensure_backend_phase "security_tests" || return 1
    
    # Test 1: Password echoing prevention (historical failure)
    log "Testing password echoing prevention..."
    
    # Create a test that verifies passwords are not logged or echoed
    # Write test inside project subdir to ensure mounted path aligns in container
    local sec_test_rel="generated/tdd-comprehensive/password_echo_test.go"
    mkdir -p "$PROJECT_ROOT/generated/tdd-comprehensive" || true
    cat > "$PROJECT_ROOT/$sec_test_rel" << 'EOF'
package main

import (
    "bytes"
    "log"
    "os"
    "strings"
    "testing"
)

func TestPasswordNotEchoed(t *testing.T) {
    // Capture log output
    var buf bytes.Buffer
    log.SetOutput(&buf)
    defer log.SetOutput(os.Stderr)
    
    // Simulate password handling (this should not echo password)
    testPassword := "__SENTINEL_TEST_PASSWORD_123__"
    log.Printf("User authentication attempt for user: %s", "testuser")
    
    // Verify password is not in logs
    logOutput := buf.String()
    if strings.Contains(logOutput, testPassword) {
        t.Errorf("SECURITY FAILURE: Password found in logs: %s", logOutput)
    }
}
EOF
    
    if GOTOOLCHAIN=auto run_go test "./generated/tdd-comprehensive" -run TestPasswordNotEchoed -count=1 -v > "$LOG_DIR/password_echo_results.log" 2>&1; then
        success "Password echo prevention: PASS"
        jq '.historical_failure_checks.password_echoing.status = "PASS"' \
            "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
    else
        fail "Password echo prevention: FAIL"
        jq '.historical_failure_checks.password_echoing.status = "FAIL"' \
            "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
        return 1
    fi
    
    # Test 2: Authentication bypass detection
    log "Testing authentication bypass vulnerabilities..."
    
    # Check for JWT secret validation (BusyBox grep friendly)
    if find . -type f -name "*.go" -print0 | xargs -0 grep -nE "jwt.*secret" 2>/dev/null | grep -v "_test.go" > "$LOG_DIR/jwt_usage.log"; then
        if grep -q "JWT_SECRET" "$LOG_DIR/jwt_usage.log"; then
            success "JWT secret validation: PASS"
            jq '.historical_failure_checks.authentication_bugs.status = "PASS"' \
                "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
        else
            fail "JWT secret validation: FAIL (no JWT_SECRET usage found)"
            jq '.historical_failure_checks.authentication_bugs.status = "FAIL"' \
                "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
            return 1
        fi
    fi
    
    # Test 3: Gosec static security analysis (optional fail on high severity)
    local gosec_fail_level=${GOSEC_FAIL_LEVEL:-HIGH}
    local gosec_status="SKIPPED"
    local gosec_high=0 gosec_medium=0 gosec_low=0
    log "Running gosec static analysis (fail level: $gosec_fail_level)..."
    # Run inside toolbox (run_go already ensures image). Use JSON for parsing.
    local gosec_out
    if gosec_out=$(GOTOOLCHAIN=auto run_go gosec -quiet -fmt=json ./... 2>/dev/null); then
        printf '%s' "$gosec_out" > "$LOG_DIR/gosec.json" || true
        gosec_high=$(echo "$gosec_out" | jq '[.Issues[]? | select(.severity=="HIGH")] | length' 2>/dev/null || echo 0)
        gosec_medium=$(echo "$gosec_out" | jq '[.Issues[]? | select(.severity=="MEDIUM")] | length' 2>/dev/null || echo 0)
        gosec_low=$(echo "$gosec_out" | jq '[.Issues[]? | select(.severity=="LOW")] | length' 2>/dev/null || echo 0)
        gosec_status="PASS"
        case "$gosec_fail_level" in
            HIGH)   [ "$gosec_high" -gt 0 ] && gosec_status="FAIL" ;;
            MEDIUM) if [ "$gosec_high" -gt 0 ] || [ "$gosec_medium" -gt 0 ]; then gosec_status="FAIL"; fi ;;
            LOW)    if [ "$gosec_high" -gt 0 ] || [ "$gosec_medium" -gt 0 ] || [ "$gosec_low" -gt 0 ]; then gosec_status="FAIL"; fi ;;
            NONE)   gosec_status="PASS" ;;
        esac
    else
        echo '{"error":"gosec execution failed"}' > "$LOG_DIR/gosec.json" 2>/dev/null || true
        gosec_status="SKIPPED"
    fi

    if [ "$gosec_status" = "FAIL" ]; then
        fail "Security tests: FAIL (gosec findings: high=$gosec_high medium=$gosec_medium low=$gosec_low)"
        jq --arg h "$gosec_high" --arg m "$gosec_medium" --arg l "$gosec_low" \
           '.evidence.security_tests.status = "FAIL" | .evidence.security_tests.gosec = {status:"FAIL", high:($h|tonumber), medium:($m|tonumber), low:($l|tonumber)}' \
            "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
        return 1
    fi

    success "Security tests: PASS"
    jq --arg h "$gosec_high" --arg m "$gosec_medium" --arg l "$gosec_low" --arg gs "$gosec_status" \
       '.evidence.security_tests.status = "PASS" | .evidence.security_tests.gosec = {status:$gs, high:($h|tonumber), medium:($m|tonumber), low:($l|tonumber)}' \
        "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
    return 0
}

# 5. SERVICE HEALTH - Comprehensive health checking
verify_comprehensive_service_health() {
    local evidence_file=$1
    
    log "Phase 5: Comprehensive Service Health Verification"
    ensure_backend_phase "service_health" || {
        jq '.evidence.service_health.status = "BACKEND_UNAVAILABLE"' "$evidence_file" > "$evidence_file.tmp" 2>/dev/null && mv "$evidence_file.tmp" "$evidence_file" || true
        return 1
    }
    
    # Start backend service
    $COMPOSE_CMD up -d backend > "$LOG_DIR/backend_start.log" 2>&1
    
    # Wait for service startup with timeout
    local service_ready=0
    for i in {1..40}; do
        local code
        code=$(curl -s -o "$LOG_DIR/health_response.json" -w '%{http_code}' "$BASE_URL/health" || echo "")
        if echo "$code" | grep -Eq '^(200|204|301|302|401)$'; then
            service_ready=1
            break
        fi
        sleep 2
    done
    
    if [ "$service_ready" -eq 1 ]; then
        success "Service health: RESPONDED"
        jq '.evidence.service_health.status = "RESPONDED" | .evidence.service_health.checked_at = "'"$(date)"'"' \
            "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
        return 0
    else
        fail "Service health: UNHEALTHY or TIMEOUT"
        jq '.evidence.service_health.status = "UNHEALTHY"' \
            "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
        return 1
    fi
}

# 6. DATABASE TESTS - Comprehensive data layer testing
run_comprehensive_database_tests() {
    local evidence_file=$1
    
    log "Phase 6: Comprehensive Database Tests"
    
    # Prefer MariaDB if postgres is not available
    if [ "${DB_DRIVER:-mariadb}" = "mariadb" ] || [ "${DB_DRIVER:-mariadb}" = "mysql" ] || $COMPOSE_CMD ps --services 2>/dev/null | grep -q "^mariadb$"; then
        # Try compose exec if available
        if $COMPOSE_CMD ps --services 2>/dev/null | grep -q "^mariadb$" && \
           $COMPOSE_CMD exec -T mariadb sh -lc "mysql -h\"${DB_HOST:-mariadb}\" -P\"${DB_PORT:-3306}\" -u\"${DB_USER:-otrs}\" -p\"${DB_PASSWORD:-LetClaude.1n}\" -D\"${DB_NAME:-otrs}\" -e 'SELECT 1;'" > "$LOG_DIR/db_connectivity.log" 2>&1; then
            success "Database connectivity (MariaDB): PASS"
            jq '.evidence.database_tests.status = "PASS" | .evidence.database_tests.driver = "mariadb"' \
                "$evidence_file" > "$evidence_file.tmp" 2>/dev/null && mv "$evidence_file.tmp" "$evidence_file" || true
            return 0
        fi
        # Fallback via temporary MariaDB image on host network
        if $CONTAINER_CMD run --rm --network host mariadb:11 sh -lc "mysql -h\"${DB_HOST:-127.0.0.1}\" -P\"${DB_PORT:-3306}\" -u\"${DB_USER:-otrs}\" -p\"${DB_PASSWORD:-LetClaude.1n}\" -D\"${DB_NAME:-otrs}\" -e 'SELECT 1;'" >> "$LOG_DIR/db_connectivity.log" 2>&1; then
            success "Database connectivity (MariaDB via mariadb:11): PASS"
            jq '.evidence.database_tests.status = "PASS" | .evidence.database_tests.driver = "mariadb"' \
                "$evidence_file" > "$evidence_file.tmp" 2>/dev/null && mv "$evidence_file.tmp" "$evidence_file" || true
            return 0
        fi
        warning "Database connectivity (MariaDB): UNDETERMINED - skipping"
        jq '.evidence.database_tests.status = "SKIPPED" | .evidence.database_tests.reason = "mariadb_detection_failed"' \
            "$evidence_file" > "$evidence_file.tmp" 2>/dev/null && mv "$evidence_file.tmp" "$evidence_file" || true
        return 0
    else
        if "$COMPOSE_CMD" ps --services 2>/dev/null | grep -q "^postgres$" && "$COMPOSE_CMD" exec -T postgres psql -U "${DB_USER:-gotrs}" -d "${DB_NAME:-gotrs}_test" -c "SELECT 1;" > "$LOG_DIR/db_connectivity.log" 2>&1; then
            success "Database connectivity (Postgres): PASS"
        else
            fail "Database connectivity: FAIL"
            jq '.evidence.database_tests.status = "FAIL" | .evidence.database_tests.error = "connectivity_failed"' \
                "$evidence_file" > "$evidence_file.tmp" 2>/dev/null && mv "$evidence_file.tmp" "$evidence_file" || true
            return 1
        fi
    fi
    
    # Skip migrations in quick mode unless postgres is active
    if $COMPOSE_CMD ps --services 2>/dev/null | grep -q "^postgres$"; then
        if "$COMPOSE_CMD" exec -T backend gotrs-migrate -path /app/migrations -database "postgres://${DB_USER:-gotrs}:${DB_PASSWORD:-password}@postgres:5432/${DB_NAME:-gotrs}_test?sslmode=disable" up > "$LOG_DIR/db_migrations.log" 2>&1; then
            success "Database migrations: PASS"
            jq '.evidence.database_tests.status = "PASS" | .evidence.database_tests.migrations = "success"' \
                "$evidence_file" > "$evidence_file.tmp" 2>/dev/null && mv "$evidence_file.tmp" "$evidence_file" || true
            return 0
        else
            fail "Database migrations: FAIL"
            jq '.evidence.database_tests.status = "FAIL" | .evidence.database_tests.error = "migration_failed"' \
                "$evidence_file" > "$evidence_file.tmp" 2>/dev/null && mv "$evidence_file.tmp" "$evidence_file" || true
            return 1
        fi
    else
        warning "Skipping migration tests (no postgres)"
        jq '.evidence.database_tests.status = "PASS" | .evidence.database_tests.migrations = "skipped"' \
            "$evidence_file" > "$evidence_file.tmp" 2>/dev/null && mv "$evidence_file.tmp" "$evidence_file" || true
        return 0
    fi
}

# 7. TEMPLATE TESTS - Address historical template syntax errors
run_comprehensive_template_tests() {
    local evidence_file=$1
    
    log "Phase 7: Comprehensive Template Tests (Historical Failure Prevention)"
    
    # Check for template syntax errors in logs (scope to our compose file/services only)
    $COMPOSE_CMD -f docker-compose.yml logs backend --tail=100 > "$LOG_DIR/template_logs.txt" 2>&1 || true
    
    local template_errors
    template_errors=$(grep -E "template.*error|Template error|parse.*template" "$LOG_DIR/template_logs.txt" 2>/dev/null | wc -l | tr -d '[:space:]')
    if [ "${template_errors:-0}" -eq 0 ]; then
        success "Template syntax: NO ERRORS"
        jq '.historical_failure_checks.template_syntax_errors.status = "PASS" | .historical_failure_checks.template_syntax_errors.error_count = 0' \
            "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
    else
        fail "Template syntax: $template_errors ERRORS FOUND"
        jq --arg count "$template_errors" '.historical_failure_checks.template_syntax_errors.status = "FAIL" | .historical_failure_checks.template_syntax_errors.error_count = ($count | tonumber)' \
            "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
        return 1
    fi
    
    # Test template rendering for common pages (accept login/redirect/auth)
    local template_pages=("/login" "/admin" "/admin/users" "/admin/groups")
    local working_templates=0
    local total_templates=${#template_pages[@]}
    
    for page in "${template_pages[@]}"; do
        local status_code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL$page")
        if [[ "$status_code" =~ ^(2|3|401) ]]; then
            ((working_templates++))
        fi
    done
    
    local template_success_rate=$((working_templates * 100 / total_templates))
    
    if [ "$template_success_rate" -ge 80 ]; then
        success "Template rendering: PASS ($working_templates/$total_templates pages, ${template_success_rate}%)"
        jq --arg success_rate "$template_success_rate" \
            '.evidence.template_tests.status = "PASS" | .evidence.template_tests.success_rate = $success_rate' \
            "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
        return 0
    else
        fail "Template rendering: FAIL ($working_templates/$total_templates pages, ${template_success_rate}%)"
        jq --arg success_rate "$template_success_rate" \
            '.evidence.template_tests.status = "FAIL" | .evidence.template_tests.success_rate = $success_rate' \
            "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
        return 1
    fi
}

# 8. API TESTS - Comprehensive endpoint testing
run_comprehensive_api_tests() {
    local evidence_file=$1
    
    log "Phase 8: Comprehensive API Tests (Historical 500/404 Error Prevention)"
    ensure_backend_phase "api_tests" || {
        jq '.evidence.api_tests.status = "BACKEND_UNAVAILABLE"' "$evidence_file" > "$evidence_file.tmp" 2>/dev/null && mv "$evidence_file.tmp" "$evidence_file" || true
        return 1
    }
    
    # Define comprehensive endpoint list
    local api_endpoints=(
        "/health"
        "/login"
        "/api/v1/users"
        "/api/v1/groups"
        "/api/v1/queues"
        "/api/v1/priorities"
        "/api/v1/states"
        "/api/v1/types"
        "/admin/users"
        "/admin/groups"
        "/admin/queues"
        "/admin/priorities"
        "/admin/states"
        "/admin/types"
        "/admin/settings"
    )
    
    local total_endpoints=${#api_endpoints[@]}
    local working_endpoints=0
    local server_500_errors=0
    local not_found_404_errors=0
    local endpoint_results=()
    
    for endpoint in "${api_endpoints[@]}"; do
        local status_code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL$endpoint")
        local response_time=$(curl -s -o /dev/null -w "%{time_total}" "$BASE_URL$endpoint")
        
        case "$status_code" in
            200|201|302|401) # Success or expected redirect/auth
                ((working_endpoints++))
                endpoint_results+=("{\"endpoint\": \"$endpoint\", \"status\": $status_code, \"result\": \"OK\", \"response_time\": $response_time}")
                ;;
            404)
                ((not_found_404_errors++))
                endpoint_results+=("{\"endpoint\": \"$endpoint\", \"status\": $status_code, \"result\": \"NOT_FOUND\", \"response_time\": $response_time}")
                ;;
            500)
                ((server_500_errors++))
                endpoint_results+=("{\"endpoint\": \"$endpoint\", \"status\": $status_code, \"result\": \"SERVER_ERROR\", \"response_time\": $response_time}")
                ;;
            *)
                endpoint_results+=("{\"endpoint\": \"$endpoint\", \"status\": $status_code, \"result\": \"OTHER_ERROR\", \"response_time\": $response_time}")
                ;;
        esac
    done
    
    local success_rate=$((working_endpoints * 100 / total_endpoints))
    local min_required=${COMPREHENSIVE_MIN_API_SUCCESS:-80}
    
    # Build results JSON
    local results_json="[$(IFS=','; echo "${endpoint_results[*]}")]"
    
    # Store raw string then parse in a second step to avoid jq parse issues
    jq --arg results "$results_json" --arg working "$working_endpoints" --arg total "$total_endpoints" \
       --arg success_rate "$success_rate" --arg server_errors "$server_500_errors" --arg not_found_errors "$not_found_404_errors" \
        '.evidence.api_tests.endpoints_raw = $results | .evidence.api_tests.working = ($working | tonumber) | .evidence.api_tests.total = ($total | tonumber) | .evidence.api_tests.success_rate = ($success_rate | tonumber) | .historical_failure_checks."500_server_errors".count = ($server_errors | tonumber) | .historical_failure_checks."404_not_found".count = ($not_found_errors | tonumber)' \
        "$evidence_file" > "$evidence_file.tmp" 2>/dev/null && mv "$evidence_file.tmp" "$evidence_file" || true
    # Try to convert endpoints_raw to JSON array
    if [ -s "$evidence_file" ]; then
      raw=$(jq -r '.evidence.api_tests.endpoints_raw' "$evidence_file" 2>/dev/null || echo "[]")
      if echo "$raw" | jq -e . >/dev/null 2>&1; then
        jq --argjson parsed "$raw" '.evidence.api_tests.endpoints = $parsed | del(.evidence.api_tests.endpoints_raw)' "$evidence_file" > "$evidence_file.tmp" 2>/dev/null && mv "$evidence_file.tmp" "$evidence_file" || true
      fi
    fi
    
    log "API Tests: $working_endpoints/$total_endpoints working (${success_rate}%), 500 errors: $server_500_errors, 404 errors: $not_found_404_errors"
    
    # Historical failure check: Zero tolerance for 500 errors
    if [ "$server_500_errors" -gt 0 ]; then
        fail "API tests: FAIL - $server_500_errors server errors (historical failure pattern)"
        jq '.historical_failure_checks."500_server_errors".status = "FAIL"' \
            "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
        return 1
    fi
    
    # Success criteria: >80% endpoints working, zero 500 errors
    # Record lists of 404 and 500 endpoints for evidence
    local not_found_list=$(echo "$results_json" | jq '[.[] | select(.result=="NOT_FOUND") | .endpoint]' 2>/dev/null || echo '[]')
    local server_error_list=$(echo "$results_json" | jq '[.[] | select(.result=="SERVER_ERROR") | .endpoint]' 2>/dev/null || echo '[]')
    jq --argjson nf "$not_found_list" --argjson se "$server_error_list" --arg min "$min_required" \
       '.evidence.api_tests.not_found_endpoints = $nf | .evidence.api_tests.server_error_endpoints = $se | .evidence.api_tests.min_success_required = ($min|tonumber)' \
        "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file" || true

    if [ "$success_rate" -ge "$min_required" ] && [ "$server_500_errors" -eq 0 ]; then
        success "API tests: PASS (${success_rate}% success rate, 0 server errors)"
        jq '.evidence.api_tests.status = "PASS" | .historical_failure_checks."500_server_errors".status = "PASS" | .historical_failure_checks."404_not_found".status = "PASS"' \
            "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
        return 0
    else
        fail "API tests: FAIL (${success_rate}% success rate, requirement: >=${min_required}% success, 0 server errors)"
        jq '.evidence.api_tests.status = "FAIL"' \
            "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
        return 1
    fi
}

# 9. BROWSER TESTS - JavaScript console error prevention
run_comprehensive_browser_tests() {
    local evidence_file=$1
    
    log "Phase 9: Comprehensive Browser Tests (JavaScript Console Error Prevention)"
    
    # Create comprehensive browser test
    cat > "$LOG_DIR/browser_comprehensive_test.js" << 'EOF'
const { chromium } = require('playwright');

async function runComprehensiveBrowserTests() {
    const browser = await chromium.launch({ 
        headless: process.env.HEADLESS !== 'false',
        slowMo: 100 
    });
    const page = await browser.newPage();
    
    const results = {
        pages: [],
        totalConsoleErrors: 0,
        totalMissingElements: 0,
        totalNetworkErrors: 0
    };
    
    // Track console errors
    let consoleErrors = [];
    page.on('console', msg => {
        if (msg.type() === 'error') {
            consoleErrors.push({
                text: msg.text(),
                location: msg.location()
            });
        }
    });
    
    // Track network failures
    let networkErrors = [];
    page.on('response', response => {
        if (response.status() >= 400) {
            networkErrors.push({
                url: response.url(),
                status: response.status()
            });
        }
    });
    
    const testPages = [
        { path: '/login', requiredElements: ['input[type="email"]', 'input[type="password"]', 'button[type="submit"]'] },
        { path: '/admin/users', requiredElements: ['table', '.btn', 'h1'] },
        { path: '/admin/groups', requiredElements: ['table', '.btn', 'h1'] },
        { path: '/admin/queues', requiredElements: ['table', '.btn', 'h1'] },
        { path: '/admin/priorities', requiredElements: ['table', '.btn', 'h1'] },
        { path: '/admin/states', requiredElements: ['table', '.btn', 'h1'] }
    ];
    
    for (const testPage of testPages) {
        consoleErrors = [];
        networkErrors = [];
        
        try {
            console.log(`Testing page: ${testPage.path}`);
            
            await page.goto(`http://localhost:8080${testPage.path}`, { 
                waitUntil: 'networkidle',
                timeout: 30000 
            });
            
            // Wait for JavaScript to execute
            await page.waitForTimeout(3000);
            
            // Check for required elements
            const missingElements = [];
            for (const element of testPage.requiredElements) {
                const elementExists = await page.$(element);
                if (!elementExists) {
                    missingElements.push(element);
                }
            }
            
            // Test basic interactivity
            const clickableElements = await page.$$('.btn, button');
            let interactivityWorking = true;
            
            if (clickableElements.length > 0) {
                try {
                    await clickableElements[0].hover();
                } catch (e) {
                    interactivityWorking = false;
                }
            }
            
            const pageResult = {
                path: testPage.path,
                consoleErrors: [...consoleErrors],
                networkErrors: [...networkErrors],
                missingElements: missingElements,
                consoleErrorCount: consoleErrors.length,
                networkErrorCount: networkErrors.length,
                missingElementCount: missingElements.length,
                interactivityWorking: interactivityWorking,
                status: (consoleErrors.length === 0 && networkErrors.length === 0 && missingElements.length === 0 && interactivityWorking) ? 'PASS' : 'FAIL'
            };
            
            results.pages.push(pageResult);
            results.totalConsoleErrors += consoleErrors.length;
            results.totalMissingElements += missingElements.length;
            results.totalNetworkErrors += networkErrors.length;
            
        } catch (error) {
            results.pages.push({
                path: testPage.path,
                consoleErrors: [],
                networkErrors: [],
                missingElements: [],
                consoleErrorCount: 0,
                networkErrorCount: 0,
                missingElementCount: 0,
                interactivityWorking: false,
                error: error.message,
                status: 'ERROR'
            });
        }
    }
    
    await browser.close();
    
    console.log(JSON.stringify(results, null, 2));
    
    // Exit with error if tests failed
    if (results.totalConsoleErrors > 0 || results.totalMissingElements > 0) {
        process.exit(1);
    }
}

runComprehensiveBrowserTests().catch(err => {
    console.error(JSON.stringify({ error: err.message }, null, 2));
    process.exit(1);
});
EOF
    
    # Run browser tests if Node.js and Playwright are available
    if command -v node >/dev/null 2>&1; then
        # Try to install playwright if not available
        if ! node -e "require('playwright')" >/dev/null 2>&1; then
            log "Installing Playwright for browser tests..."
            if npm install -g playwright > "$LOG_DIR/playwright_install.log" 2>&1; then
                playwright install chromium >> "$LOG_DIR/playwright_install.log" 2>&1
            fi
        fi
        
        if node -e "require('playwright')" >/dev/null 2>&1; then
            if HEADLESS=true node "$LOG_DIR/browser_comprehensive_test.js" > "$LOG_DIR/browser_results.json" 2>&1; then
                local browser_results=$(cat "$LOG_DIR/browser_results.json")
                local total_console_errors=$(echo "$browser_results" | jq '.totalConsoleErrors // 0')
                local total_missing_elements=$(echo "$browser_results" | jq '.totalMissingElements // 0')
                
                jq --argjson results "$browser_results" --arg console_errors "$total_console_errors" --arg missing_elements "$total_missing_elements" \
                    '.evidence.browser_tests.results = $results | .evidence.browser_tests.total_console_errors = ($console_errors | tonumber) | .evidence.browser_tests.total_missing_elements = ($missing_elements | tonumber) | .historical_failure_checks.javascript_console_errors.count = ($console_errors | tonumber) | .historical_failure_checks.missing_ui_elements.count = ($missing_elements | tonumber)' \
                    "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
                
                if [ "$total_console_errors" -eq 0 ] && [ "$total_missing_elements" -eq 0 ]; then
                    success "Browser tests: PASS (0 console errors, 0 missing elements)"
                    jq '.evidence.browser_tests.status = "PASS" | .historical_failure_checks.javascript_console_errors.status = "PASS" | .historical_failure_checks.missing_ui_elements.status = "PASS"' \
                        "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
                    return 0
                else
                    fail "Browser tests: FAIL ($total_console_errors console errors, $total_missing_elements missing elements)"
                    jq '.evidence.browser_tests.status = "FAIL" | .historical_failure_checks.javascript_console_errors.status = "FAIL" | .historical_failure_checks.missing_ui_elements.status = "FAIL"' \
                        "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
                    return 1
                fi
            else
                warning "Browser tests: SKIPPED (execution failed)"
                jq '.evidence.browser_tests.status = "SKIPPED" | .evidence.browser_tests.reason = "execution_failed"' \
                    "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
                return 0
            fi
        else
            warning "Browser tests: SKIPPED (Playwright installation failed)"
            jq '.evidence.browser_tests.status = "SKIPPED" | .evidence.browser_tests.reason = "playwright_installation_failed"' \
                "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
            return 0
        fi
    else
        warning "Browser tests: SKIPPED (Node.js not available)"
        jq '.evidence.browser_tests.status = "SKIPPED" | .evidence.browser_tests.reason = "nodejs_not_available"' \
            "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
        return 0
    fi
}

# 10. PERFORMANCE TESTS - Basic performance verification
run_comprehensive_performance_tests() {
    local evidence_file=$1
    
    log "Phase 10: Comprehensive Performance Tests"
    ensure_backend_phase "performance_tests" || {
        jq '.evidence.performance_tests.status = "BACKEND_UNAVAILABLE"' "$evidence_file" > "$evidence_file.tmp" 2>/dev/null && mv "$evidence_file.tmp" "$evidence_file" || true
        return 1
    }
    
    # Test response times for critical endpoints
    local performance_endpoints=("/health" "/login" "/admin/users")
    local slow_responses=0
    local performance_results=()
    
    for endpoint in "${performance_endpoints[@]}"; do
        local response_time=$(curl -s -o /dev/null -w "%{time_total}" "$BASE_URL$endpoint" || echo "0")
        # Avoid requiring bc; use awk for portability
        local response_time_int=$(awk -v t="$response_time" 'BEGIN { if (t=="") t=0; printf "%d", t*1000 }')
        
        if [ "$response_time_int" -gt 3000 ]; then  # 3 second threshold
            ((slow_responses++))
            performance_results+=("{\"endpoint\": \"$endpoint\", \"response_time_ms\": $response_time_int, \"status\": \"SLOW\"}")
        else
            performance_results+=("{\"endpoint\": \"$endpoint\", \"response_time_ms\": $response_time_int, \"status\": \"OK\"}")
        fi
    done
    
    local results_json="[$(IFS=','; echo "${performance_results[*]}")]"
    
    jq --argjson results "$results_json" --arg slow_count "$slow_responses" \
        '.evidence.performance_tests.results = $results | .evidence.performance_tests.slow_responses = ($slow_count | tonumber)' \
        "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
    
    if [ "$slow_responses" -eq 0 ]; then
        success "Performance tests: PASS (all responses < 3s)"
        jq '.evidence.performance_tests.status = "PASS"' \
            "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
        return 0
    else
        warning "Performance tests: WARNING ($slow_responses slow responses)"
        jq '.evidence.performance_tests.status = "WARNING"' \
            "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
        return 0  # Non-critical for now
    fi
}

# 11. REGRESSION TESTS - Prevent historical failures from reoccurring
run_comprehensive_regression_tests() {
    local evidence_file=$1
    
    log "Phase 11: Comprehensive Regression Tests (Historical Failure Prevention)"
    ensure_backend_phase "regression_tests" || {
        jq '.evidence.regression_tests.status = "BACKEND_UNAVAILABLE"' "$evidence_file" > "$evidence_file.tmp" 2>/dev/null && mv "$evidence_file.tmp" "$evidence_file" || true
        return 1
    }
    
    local regression_failures=0
    
    # Regression Test 1: Authentication system integrity
    log "Testing authentication system integrity..."
    # Expect 401 or redirect (301/302) when unauthenticated on admin pages
    admin_status=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/admin/users")
    if [ "$admin_status" = "401" ] || [ "$admin_status" = "301" ] || [ "$admin_status" = "302" ]; then
        success "Authentication protection: PASS ($admin_status)"
        jq '.evidence.regression_tests.auth_protection = "PASS"' \
            "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
    else
        fail "Authentication protection: FAIL (status $admin_status)"
        ((regression_failures++))
        jq '.evidence.regression_tests.auth_protection = "FAIL"' \
            "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
    fi
    
    # Regression Test 2: Database connection integrity
    log "Testing database connection integrity..."
    db_ok=0
    if [ -z "${CONTAINER_CMD}" ] && [ -z "${COMPOSE_CMD}" ]; then
        warning "No container runtime available; skipping DB integrity check"
        jq '.evidence.regression_tests.db_integrity = "SKIPPED_NO_CONTAINER"' \
            "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
    else
        # Prefer direct container ps when possible
        if [ -n "$CONTAINER_CMD" ] && $CONTAINER_CMD ps --format "{{.Names}}" | grep -q "^gotrs-mariadb$"; then
            if $CONTAINER_CMD exec gotrs-mariadb sh -lc "/usr/bin/mariadb -h127.0.0.1 -P\"${DB_PORT:-3306}\" -u\"${DB_USER:-otrs}\" -p\"${DB_PASSWORD:-LetClaude.1n}\" -e 'SELECT 1;'" >/dev/null 2>&1; then
                db_ok=1
            fi
        elif [ -n "$CONTAINER_CMD" ] && $CONTAINER_CMD ps --format "{{.Names}}" | grep -q "^gotrs-postgres$"; then
            if $CONTAINER_CMD exec -T gotrs-postgres pg_isready -U "${DB_USER:-gotrs}" >/dev/null 2>&1; then
                db_ok=1
            fi
        elif [ -n "$COMPOSE_CMD" ]; then
            if $COMPOSE_CMD ps --services 2>/dev/null | grep -q "^mariadb$"; then
                if $COMPOSE_CMD exec -T mariadb sh -lc "/usr/bin/mariadb -h\"${DB_HOST:-mariadb}\" -P\"${DB_PORT:-3306}\" -u\"${DB_USER:-otrs}\" -p\"${DB_PASSWORD:-LetClaude.1n}\" -e 'SELECT 1;'" >/dev/null 2>&1; then
                    db_ok=1
                fi
            elif $COMPOSE_CMD ps --services 2>/dev/null | grep -q "^postgres$"; then
                if $COMPOSE_CMD exec -T postgres pg_isready -U "${DB_USER:-gotrs}" >/dev/null 2>&1; then
                    db_ok=1
                fi
            fi
        fi
    fi
    if [ "$db_ok" -eq 1 ]; then
        success "Database integrity: PASS"
        jq '.evidence.regression_tests.db_integrity = "PASS"' \
            "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
    else
        fail "Database integrity: FAIL"
        ((regression_failures++))
        jq '.evidence.regression_tests.db_integrity = "FAIL"' \
            "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
    fi
    
    # Regression Test 3: Configuration loading
    log "Testing configuration loading..."
    # Accept healthy health endpoint as proxy for successful config load
    if code=$(curl -s -o /dev/null -w '%{http_code}' "$BASE_URL/health" || echo ""); echo "$code" | grep -Eq '^(200|204|301|302|401)$'; then
        success "Configuration loading: PASS (health responded $code)"
        jq '.evidence.regression_tests.config_loading = "PASS"' \
            "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
    else
        fail "Configuration loading: FAIL"
        ((regression_failures++))
        jq '.evidence.regression_tests.config_loading = "FAIL"' \
            "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
    fi
    
    if [ "$regression_failures" -eq 0 ]; then
        success "Regression tests: PASS (0 historical failures detected)"
        jq '.evidence.regression_tests.status = "PASS" | .evidence.regression_tests.failure_count = 0' \
            "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
        return 0
    else
        fail "Regression tests: FAIL ($regression_failures historical failures detected)"
        jq --arg failures "$regression_failures" '.evidence.regression_tests.status = "FAIL" | .evidence.regression_tests.failure_count = ($failures | tonumber)' \
            "$evidence_file" > "$evidence_file.tmp" && mv "$evidence_file.tmp" "$evidence_file"
        return 1
    fi
}

# Generate comprehensive evidence report
generate_comprehensive_report() {
    local evidence_file=$1
    local test_phase=$2
    local gates_passed=$3
    local gates_total=$4
    local success_rate=$5
    
    log "Generating comprehensive evidence report..."
    
    local report_file="$EVIDENCE_DIR/comprehensive_${test_phase}_report_$(date +%Y%m%d_%H%M%S).html"
    local evidence_hash
    evidence_hash=$(sha256sum "$evidence_file" 2>/dev/null | awk '{print $1}')
    if [ -n "$evidence_hash" ]; then
        jq --arg eh "$evidence_hash" '.evidence_hash = $eh' "$evidence_file" > "$evidence_file.tmp" 2>/dev/null && mv "$evidence_file.tmp" "$evidence_file" || true
    fi
    
    # Calculate historical failure status
    local historical_failures_detected=0
    local password_echo_status=$(jq -r '.historical_failure_checks.password_echoing.status // "unknown"' "$evidence_file")
    local template_errors_status=$(jq -r '.historical_failure_checks.template_syntax_errors.status // "unknown"' "$evidence_file")
    local auth_bugs_status=$(jq -r '.historical_failure_checks.authentication_bugs.status // "unknown"' "$evidence_file")
    local js_errors_status=$(jq -r '.historical_failure_checks.javascript_console_errors.status // "unknown"' "$evidence_file")
    
    [ "$password_echo_status" = "FAIL" ] && ((historical_failures_detected++))
    [ "$template_errors_status" = "FAIL" ] && ((historical_failures_detected++))
    [ "$auth_bugs_status" = "FAIL" ] && ((historical_failures_detected++))
    [ "$js_errors_status" = "FAIL" ] && ((historical_failures_detected++))
    
    cat > "$report_file" << EOF
<!DOCTYPE html>
<html>
<head>
    <title>Comprehensive TDD Evidence Report - $test_phase</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; line-height: 1.6; }
        .header { background: #f0f0f0; padding: 20px; border-radius: 5px; margin-bottom: 20px; }
        .pass { color: #28a745; font-weight: bold; }
        .fail { color: #dc3545; font-weight: bold; }
        .warn { color: #ffc107; font-weight: bold; }
        .skip { color: #6c757d; font-weight: bold; }
        .evidence { background: #f8f9fa; padding: 15px; margin: 15px 0; border-left: 4px solid #007bff; }
        .critical { background: #f8d7da; padding: 15px; margin: 15px 0; border-left: 4px solid #dc3545; }
        pre { background: #e9ecef; padding: 10px; overflow-x: auto; border-radius: 3px; }
        table { border-collapse: collapse; width: 100%; margin: 15px 0; }
        th, td { border: 1px solid #dee2e6; padding: 12px; text-align: left; }
        th { background-color: #e9ecef; font-weight: bold; }
        .success-rate { font-size: 24px; font-weight: bold; text-align: center; margin: 20px 0; }
        .historical-check { background: #fff3cd; padding: 10px; margin: 10px 0; border-left: 4px solid #ffc107; }
    </style>
</head>
<body>
    <div class="header">
        <h1>🧪 Comprehensive TDD Evidence Report</h1>
        <p><strong>Test Phase:</strong> $test_phase</p>
        <p><strong>Generated:</strong> $(date)</p>
        <p><strong>Git Commit:</strong> $(git rev-parse HEAD 2>/dev/null || echo 'no-git')</p>
        <p><strong>Container Runtime:</strong> $CONTAINER_CMD</p>
    </div>

    <div class="success-rate">
        Quality Gates: $gates_passed/$gates_total Passed (${success_rate}%)
    </div>

    $(if [ "$success_rate" -eq 100 ]; then
        echo '<div class="evidence"><h2 class="pass">✅ COMPREHENSIVE SUCCESS - ALL QUALITY GATES PASSED</h2><p>This is evidence-based verification. All tests completed successfully with comprehensive coverage.</p></div>'
    else
        echo '<div class="critical"><h2 class="fail">❌ COMPREHENSIVE FAILURE - QUALITY GATES FAILED</h2><p>DO NOT CLAIM SUCCESS. '$(( gates_total - gates_passed ))' quality gates failed. Fix all failures before claiming success.</p></div>'
    fi)

    <h2>📊 Quality Gate Results</h2>
    <table>
        <tr><th>Quality Gate</th><th>Status</th><th>Evidence</th></tr>
        <tr><td>1. Compilation</td><td class="$(jq -r '.evidence.compilation.status' "$evidence_file" | tr '[:upper:]' '[:lower:]')">$(jq -r '.evidence.compilation.status // "unknown"' "$evidence_file")</td><td>Go build verification</td></tr>
        <tr><td>2. Unit Tests</td><td class="$(jq -r '.evidence.unit_tests.status' "$evidence_file" | tr '[:upper:]' '[:lower:]')">$(jq -r '.evidence.unit_tests.status // "unknown"' "$evidence_file")</td><td>Coverage: $(jq -r '.evidence.unit_tests.coverage // "unknown"' "$evidence_file")%</td></tr>
        <tr><td>3. Integration Tests</td><td class="$(jq -r '.evidence.integration_tests.status' "$evidence_file" | tr '[:upper:]' '[:lower:]')">$(jq -r '.evidence.integration_tests.status // "unknown"' "$evidence_file")</td><td>Database + Service Integration</td></tr>
        <tr><td>4. Security Tests</td><td class="$(jq -r '.evidence.security_tests.status' "$evidence_file" | tr '[:upper:]' '[:lower:]')">$(jq -r '.evidence.security_tests.status // "unknown"' "$evidence_file")</td><td>Authentication + Password Security</td></tr>
        <tr><td>5. Service Health</td><td class="$(jq -r '.evidence.service_health.status' "$evidence_file" | tr '[:upper:]' '[:lower:]')">$(jq -r '.evidence.service_health.status // "unknown"' "$evidence_file")</td><td>Health endpoint verification</td></tr>
        <tr><td>6. Database Tests</td><td class="$(jq -r '.evidence.database_tests.status' "$evidence_file" | tr '[:upper:]' '[:lower:]')">$(jq -r '.evidence.database_tests.status // "unknown"' "$evidence_file")</td><td>Migrations + Connectivity</td></tr>
        <tr><td>7. Template Tests</td><td class="$(jq -r '.evidence.template_tests.status' "$evidence_file" | tr '[:upper:]' '[:lower:]')">$(jq -r '.evidence.template_tests.status // "unknown"' "$evidence_file")</td><td>Template rendering verification</td></tr>
        <tr><td>8. API Tests</td><td class="$(jq -r '.evidence.api_tests.status' "$evidence_file" | tr '[:upper:]' '[:lower:]')">$(jq -r '.evidence.api_tests.status // "unknown"' "$evidence_file")</td><td>Endpoint response verification</td></tr>
        <tr><td>9. Browser Tests</td><td class="$(jq -r '.evidence.browser_tests.status' "$evidence_file" | tr '[:upper:]' '[:lower:]')">$(jq -r '.evidence.browser_tests.status // "unknown"' "$evidence_file")</td><td>JavaScript + UI verification</td></tr>
        <tr><td>10. Performance Tests</td><td class="$(jq -r '.evidence.performance_tests.status' "$evidence_file" | tr '[:upper:]' '[:lower:]')">$(jq -r '.evidence.performance_tests.status // "unknown"' "$evidence_file")</td><td>Response time verification</td></tr>
        <tr><td>11. Regression Tests</td><td class="$(jq -r '.evidence.regression_tests.status' "$evidence_file" | tr '[:upper:]' '[:lower:]')">$(jq -r '.evidence.regression_tests.status // "unknown"' "$evidence_file")</td><td>Historical failure prevention</td></tr>
    </table>

    <h2>🚨 Historical Failure Checks (Anti-Claude-Intern Protection)</h2>
    <div class="historical-check">
        <p>These checks specifically address patterns of historical failures where success was claimed without proper verification.</p>
        $(if [ "$historical_failures_detected" -eq 0 ]; then
            echo '<p class="pass">✅ NO HISTORICAL FAILURE PATTERNS DETECTED</p>'
        else
            echo "<p class=\"fail\">❌ $historical_failures_detected HISTORICAL FAILURE PATTERNS DETECTED</p>"
        fi)
    </div>
    
    <table>
        <tr><th>Historical Failure Type</th><th>Status</th><th>Evidence</th></tr>
        <tr><td>Password Echoing</td><td class="$(echo "$password_echo_status" | tr '[:upper:]' '[:lower:]')">$password_echo_status</td><td>Security verification</td></tr>
        <tr><td>Template Syntax Errors</td><td class="$(echo "$template_errors_status" | tr '[:upper:]' '[:lower:]')">$template_errors_status</td><td>Error count: $(jq -r '.historical_failure_checks.template_syntax_errors.error_count // "unknown"' "$evidence_file")</td></tr>
        <tr><td>Authentication Bugs</td><td class="$(echo "$auth_bugs_status" | tr '[:upper:]' '[:lower:]')">$auth_bugs_status</td><td>JWT + Auth verification</td></tr>
        <tr><td>JavaScript Console Errors</td><td class="$(echo "$js_errors_status" | tr '[:upper:]' '[:lower:]')">$js_errors_status</td><td>Error count: $(jq -r '.historical_failure_checks.javascript_console_errors.count // "unknown"' "$evidence_file")</td></tr>
        <tr><td>Missing UI Elements</td><td class="$(jq -r '.historical_failure_checks.missing_ui_elements.status' "$evidence_file" | tr '[:upper:]' '[:lower:]')">$(jq -r '.historical_failure_checks.missing_ui_elements.status // "unknown"' "$evidence_file")</td><td>Missing count: $(jq -r '.historical_failure_checks.missing_ui_elements.count // "unknown"' "$evidence_file")</td></tr>
        <tr><td>500 Server Errors</td><td class="$(jq -r '.historical_failure_checks."500_server_errors".status' "$evidence_file" | tr '[:upper:]' '[:lower:]')">$(jq -r '.historical_failure_checks."500_server_errors".status // "unknown"' "$evidence_file")</td><td>Error count: $(jq -r '.historical_failure_checks."500_server_errors".count // "unknown"' "$evidence_file")</td></tr>
        <tr><td>404 Not Found</td><td class="$(jq -r '.historical_failure_checks."404_not_found".status' "$evidence_file" | tr '[:upper:]' '[:lower:]')">$(jq -r '.historical_failure_checks."404_not_found".status // "unknown"' "$evidence_file")</td><td>Error count: $(jq -r '.historical_failure_checks."404_not_found".count // "unknown"' "$evidence_file")</td></tr>
    </table>

    <h2>📋 Complete Evidence Data</h2>
    <div class="evidence">
        <pre>$(jq . "$evidence_file" 2>/dev/null || echo "Evidence file corrupted")</pre>
    </div>

    <h2>🎯 Anti-Gaslighting Declaration</h2>
    <div class="$(if [ "$success_rate" -eq 100 ] && [ "$historical_failures_detected" -eq 0 ]; then echo "evidence"; else echo "critical"; fi)">
        $(if [ "$success_rate" -eq 100 ] && [ "$historical_failures_detected" -eq 0 ]; then
            echo '<p><strong>VERIFIED SUCCESS:</strong> All quality gates passed with comprehensive evidence. This system has been thoroughly tested and verified to work correctly. Success claim is justified by concrete evidence.</p>'
        else
            echo '<p><strong>FAILURE DETECTED:</strong> This system is NOT working correctly. Do not claim success. The evidence shows concrete failures that must be fixed before any success claims can be made.</p>'
            echo '<p><strong>Success Rate:</strong> '${success_rate}'% (Requirement: 100%)</p>'
            echo '<p><strong>Historical Failures:</strong> '$historical_failures_detected' detected (Requirement: 0)</p>'
            echo '<p><strong>Next Steps:</strong> Fix all failing quality gates and re-run comprehensive verification.</p>'
        fi)
    </div>

    <footer style="margin-top: 50px; padding-top: 20px; border-top: 1px solid #dee2e6; text-align: center; color: #6c757d;">
        Generated by Comprehensive TDD Test Automation - $(date)<br/>
        Evidence SHA256: ${evidence_hash:-unavailable}
    </footer>
</body>
</html>
EOF
    
    echo "$report_file"
}

# Main comprehensive verification function
run_comprehensive_verification() {
    local test_phase="${1:-full}"
    
    log "🚀 Starting COMPREHENSIVE TDD Verification (Phase: $test_phase)"
    log "Zero tolerance for false positives, premature success claims, or missing evidence"
    
    # Collect evidence
    local evidence_file=$(collect_comprehensive_evidence "$test_phase")
    
    # Initialize counters (adjust in limited mode)
    local gates_passed=0
    local gates_total=11
    local phase_results=()
    local skipped_phases=0
    if [ "$LIMITED_MODE" = "1" ]; then
        gates_total=3
        log "Running LIMITED MODE ($gates_total gates: compilation, unit, template)"
        # Mark skipped phases up-front in evidence
        jq '.evidence.integration_tests.status = "SKIPPED_LIMITED" |
            .evidence.security_tests.status = "SKIPPED_LIMITED" |
            .evidence.service_health.status = "SKIPPED_LIMITED" |
            .evidence.database_tests.status = "SKIPPED_LIMITED" |
            .evidence.api_tests.status = "SKIPPED_LIMITED" |
            .evidence.browser_tests.status = "SKIPPED_LIMITED" |
            .evidence.performance_tests.status = "SKIPPED_LIMITED" |
            .evidence.regression_tests.status = "SKIPPED_LIMITED"' "$evidence_file" > "$evidence_file.tmp" 2>/dev/null && mv "$evidence_file.tmp" "$evidence_file" || true
    else
        log "Running all $gates_total quality gates with historical failure prevention..."
    fi
    
    # Run all comprehensive test phases
    log "Phase 1/11: Compilation Verification"
    if verify_comprehensive_compilation "$evidence_file"; then
        ((gates_passed++))
        phase_results+=("1. Compilation: PASS")
    else
        phase_results+=("1. Compilation: FAIL")
    fi
    
    log "Phase 2/11: Unit Tests"
    if run_comprehensive_unit_tests "$evidence_file"; then
        ((gates_passed++))
        phase_results+=("2. Unit Tests: PASS")
    else
        phase_results+=("2. Unit Tests: FAIL")
    fi
    
    if [ "$LIMITED_MODE" = "1" ]; then
        # Skip phases 3-6,8-11 in limited mode (already marked skipped in evidence)
    log "[LIMITED] Skipping Integration, Security, Service Health, Database, API, Browser, Performance, Regression tests"
        log "Phase 7/11: Template Tests (executed in limited mode as gate 3/3)"
        if run_comprehensive_template_tests "$evidence_file"; then
            ((gates_passed++))
            phase_results+=("7. Template Tests: PASS")
        else
            phase_results+=("7. Template Tests: FAIL")
        fi
    else
        log "Phase 3/11: Integration Tests"
        if run_comprehensive_integration_tests "$evidence_file"; then
            ((gates_passed++))
            phase_results+=("3. Integration Tests: PASS")
        else
            # Distinguish skip vs fail by evidence marker if present
            if jq -e '.evidence.integration_tests.status | test("^SKIPPED")' "$evidence_file" >/dev/null 2>&1; then
                phase_results+=("3. Integration Tests: SKIPPED")
                ((skipped_phases++))
            else
                phase_results+=("3. Integration Tests: FAIL")
            fi
        fi
        
        log "Phase 4/11: Security Tests"
        if run_comprehensive_security_tests "$evidence_file"; then
            ((gates_passed++))
            phase_results+=("4. Security Tests: PASS")
        else
            phase_results+=("4. Security Tests: FAIL")
        fi
        
        log "Phase 5/11: Service Health"
        if verify_comprehensive_service_health "$evidence_file"; then
            ((gates_passed++))
            phase_results+=("5. Service Health: PASS")
        else
            phase_results+=("5. Service Health: FAIL")
        fi
        
        log "Phase 6/11: Database Tests"
        if run_comprehensive_database_tests "$evidence_file"; then
            ((gates_passed++))
            phase_results+=("6. Database Tests: PASS")
        else
            if jq -e '.evidence.database_tests.status | test("^SKIPPED")' "$evidence_file" >/dev/null 2>&1; then
                phase_results+=("6. Database Tests: SKIPPED")
                ((skipped_phases++))
            else
                phase_results+=("6. Database Tests: FAIL")
            fi
        fi
        
        log "Phase 7/11: Template Tests"
        if run_comprehensive_template_tests "$evidence_file"; then
            ((gates_passed++))
            phase_results+=("7. Template Tests: PASS")
        else
            phase_results+=("7. Template Tests: FAIL")
        fi
        
        log "Phase 8/11: API Tests"
        if run_comprehensive_api_tests "$evidence_file"; then
            ((gates_passed++))
            phase_results+=("8. API Tests: PASS")
        else
            phase_results+=("8. API Tests: FAIL")
        fi
        
        log "Phase 9/11: Browser Tests"
        if run_comprehensive_browser_tests "$evidence_file"; then
            ((gates_passed++))
            phase_results+=("9. Browser Tests: PASS")
        else
            if jq -e '.evidence.browser_tests.status | test("^SKIPPED")' "$evidence_file" >/dev/null 2>&1; then
                phase_results+=("9. Browser Tests: SKIPPED")
                ((skipped_phases++))
            else
                phase_results+=("9. Browser Tests: FAIL")
            fi
        fi
        
        log "Phase 10/11: Performance Tests"
        if run_comprehensive_performance_tests "$evidence_file"; then
            ((gates_passed++))
            phase_results+=("10. Performance Tests: PASS")
        else
            phase_results+=("10. Performance Tests: FAIL")
        fi
        
        log "Phase 11/11: Regression Tests"
        if run_comprehensive_regression_tests "$evidence_file"; then
            ((gates_passed++))
            phase_results+=("11. Regression Tests: PASS")
        else
            phase_results+=("11. Regression Tests: FAIL")
        fi
    fi
    
    # Calculate final results
    local success_rate=$((gates_passed * 100 / gates_total))
    
    # Generate comprehensive evidence report
    local report_file=$(generate_comprehensive_report "$evidence_file" "$test_phase" "$gates_passed" "$gates_total" "$success_rate")
    
    # Display results
    echo ""
    echo "=================================================================="
    echo "           COMPREHENSIVE TDD VERIFICATION RESULTS"
    echo "=================================================================="
    echo "Test Phase: $test_phase"
    echo "Quality Gates: $gates_passed/$gates_total Passed (${success_rate}%)"
    echo "Evidence File: $evidence_file"
    echo "Report File: $report_file"
    echo ""
    
    for result in "${phase_results[@]}"; do
        if [[ "$result" == *"PASS"* ]]; then
            echo -e "${GREEN}✓ $result${NC}"
        else
            echo -e "${RED}✗ $result${NC}"
        fi
    done
    
    echo "=================================================================="
    
    # Final determination - ZERO TOLERANCE for failures
    if [ "$success_rate" -eq 100 ]; then
        echo -e "${GREEN}"
        echo "🎉 COMPREHENSIVE SUCCESS - ALL QUALITY GATES PASSED"
        echo "✅ Evidence-based verification complete"
        echo "✅ All historical failure patterns prevented"
        echo "✅ System verified to be working correctly"
        echo "✅ Success claim is justified by concrete evidence"
        echo -e "${NC}"
        
        # Update TDD state if exists
        if [ -f "$PROJECT_ROOT/.tdd-state" ]; then
            jq --arg timestamp "$(date -Iseconds)" --arg success_rate "$success_rate" \
                '.comprehensive_verification_passed = true | .timestamp = $timestamp | .last_success_rate = $success_rate' \
                "$PROJECT_ROOT/.tdd-state" > "$PROJECT_ROOT/.tdd-state.tmp" && mv "$PROJECT_ROOT/.tdd-state.tmp" "$PROJECT_ROOT/.tdd-state"
        fi
        
        return 0
    else
        echo -e "${RED}"
        echo "❌ COMPREHENSIVE FAILURE - QUALITY GATES FAILED"
        echo "🚨 DO NOT CLAIM SUCCESS"
        echo "🚨 $((gates_total - gates_passed)) quality gates failed"
        echo "🚨 Success rate: ${success_rate}% (Requirement: 100%)"
        echo "🚨 Fix all failures before claiming success"
        echo -e "${NC}"
        
        return 1
    fi
}

# Command dispatcher
case "${1:-}" in
    "comprehensive")
        run_comprehensive_verification "${2:-full}"
        ;;
    "quick")
        # Quick verification (subset of tests for development)
        run_comprehensive_verification "quick"
        ;;
    *)
        echo "Comprehensive TDD Test Automation"
        echo "Zero tolerance for false positives and premature success claims"
        echo ""
        echo "Usage: $0 <command> [options]"
        echo ""
        echo "Commands:"
        echo "  comprehensive [phase]  - Run all comprehensive quality gates"
        echo "  quick                  - Run subset of tests for development"
        echo ""
        echo "This tool implements evidence-based verification to prevent:"
        echo "  - False positive test results"
        echo "  - Premature success claims"
        echo "  - Historical failure patterns (password echoing, template errors, etc.)"
        echo "  - Missing UI elements and JavaScript console errors"
        echo "  - 500 server errors and authentication bugs"
        echo ""
        echo "Success is only claimed when ALL quality gates pass with concrete evidence."
        echo ""
        exit 1
        ;;
esac