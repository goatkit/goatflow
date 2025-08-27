# GOTRS TDD Quality Gates

## Overview

This document defines the mandatory quality gates that must pass before any success claims are made. These gates prevent the "Claude the intern" pattern where premature success claims are made without proper verification.

## Quality Gates

### 1. Compilation Gate
- **Purpose**: Ensure all Go code compiles without errors
- **Command**: `go build ./cmd/server`
- **Pass Criteria**: Exit code 0, no compilation errors
- **Evidence**: Compilation log output

### 2. Service Health Gate
- **Purpose**: Ensure the backend service starts and responds to health checks
- **Command**: `curl -f http://localhost:8080/health`
- **Pass Criteria**: HTTP 200 response with `{"status":"healthy"}`
- **Evidence**: Health endpoint JSON response

### 3. Template Gate
- **Purpose**: Ensure all templates render without errors
- **Command**: Check backend logs for template errors
- **Pass Criteria**: Zero template errors in recent logs
- **Evidence**: Backend log analysis showing no template errors

### 4. Go Test Gate
- **Purpose**: Ensure all tests pass with adequate coverage
- **Command**: `go test -v -race -coverprofile=coverage.out ./...`
- **Pass Criteria**: All tests pass, coverage > 70%
- **Evidence**: Test output and coverage report

### 5. HTTP Endpoint Gate
- **Purpose**: Ensure critical endpoints respond correctly
- **Endpoints Tested**: 
  - `/health` (200)
  - `/login` (200 or 303 redirect)
  - `/admin/groups` (200 or 303 redirect)
  - `/admin/users` (200 or 303 redirect)
  - `/admin/queues` (200 or 303 redirect)
  - `/admin/priorities` (200 or 303 redirect)
  - `/admin/states` (200 or 303 redirect)
  - `/admin/types` (200 or 303 redirect)
- **Pass Criteria**: 80% or more endpoints return 2xx or 3xx status
- **Evidence**: HTTP response status codes for each endpoint

### 6. Browser Console Gate
- **Purpose**: Ensure UI loads without JavaScript errors
- **Method**: Playwright browser automation checking console errors
- **Pass Criteria**: Zero console errors on tested pages
- **Evidence**: Browser console error log

### 7. Log Analysis Gate
- **Purpose**: Ensure backend runs without errors
- **Command**: Analyze recent backend logs for ERROR/PANIC messages
- **Pass Criteria**: Zero ERROR or PANIC messages in recent logs
- **Evidence**: Backend log analysis

## TDD Workflow States

### 1. Init Phase
- Initialize TDD workflow tracking
- Create necessary directories and state files
- **Next**: test-first phase

### 2. Test-First Phase
- Write failing test for new feature
- Run test to confirm it fails
- **Requirement**: Must have a failing test before proceeding
- **Next**: implement phase

### 3. Implementation Phase
- Write minimal code to make the test pass
- **Requirement**: Must have entered from test-first phase with failing test
- **Next**: verify phase

### 4. Verification Phase
- Run ALL quality gates
- Collect evidence for each gate
- Generate evidence report
- **Pass**: All 7 gates must pass (100% success rate)
- **Fail**: Any gate failure prevents success claim
- **Next**: refactor phase (if passed) or back to implementation

### 5. Refactor Phase
- Safe code refactoring with regression protection
- **Requirement**: Must have successful verification first
- **Process**: Run full verification after refactoring to ensure no regressions

## Evidence Collection

Every verification run produces:
1. **JSON Evidence File**: Structured data about each gate's results
2. **HTML Evidence Report**: Human-readable report with full details
3. **Log Files**: Detailed logs from each gate execution

Evidence files are stored in `generated/evidence/` with timestamps.

## Usage Examples

### Starting a New Feature

```bash
# Initialize TDD workflow
make tdd-init

# Start with failing test
make tdd-test-first FEATURE="User Password Reset"
# Write your failing test in appropriate *_test.go file

# Verify test fails (this should show failing test)
make tdd-verify --test-failing

# Implement minimal code to pass test
make tdd-implement
# Write your implementation code

# Run full verification before claiming success
make tdd-verify
# This runs ALL 7 quality gates and generates evidence

# If all gates pass, you can refactor safely
make tdd-refactor
# Make refactoring changes

# Verify no regressions after refactoring
make tdd-verify --refactor
```

### Checking Current Status

```bash
# Check current TDD workflow state
make tdd-status

# View recent evidence reports
make evidence-report

# Run quality gates independently for debugging
make quality-gates
```

## Anti-Patterns (DO NOT DO)

❌ **Claiming Success Without Evidence**
```bash
# WRONG - No evidence collected
echo "Feature complete!" # WITHOUT running tdd-verify
```

❌ **Skipping Quality Gates**
```bash
# WRONG - Only running some tests
go test ./internal/specific/package
echo "All tests pass!" # WITHOUT running all gates
```

❌ **Implementing Without Test-First**
```bash
# WRONG - Implementation before failing test
# Write code first, then test
make tdd-implement # This will fail - no test-first phase
```

❌ **Ignoring Gate Failures**
```bash
# WRONG - Claiming success with failing gates
make tdd-verify # Shows 5/7 gates pass
echo "Mostly working, good enough!" # 5/7 is NOT success
```

## Success Criteria

✅ **Proper TDD Success**
- ALL 7 quality gates pass (100% success rate)
- Evidence report generated and reviewed
- No exceptions or "minor issues" handwaving
- Actual browser console verification (0 JavaScript errors)
- Real HTTP endpoint testing (not just compilation)

## Integration with Existing Tools

### Makefile Integration
- `make test` automatically uses TDD verification if `.tdd-state` exists
- All TDD commands integrated into main Makefile help
- Container-first approach maintained

### Container Integration  
- All verification runs in containers using existing Dockerfile.toolbox
- Uses existing container-wrapper.sh for runtime detection
- Respects existing database and service configuration

### CI/CD Integration
- Evidence files can be archived in CI/CD pipelines
- Quality gate results can block deployments
- HTML reports can be published as artifacts

## Troubleshooting

### Gate Failures

**Compilation Gate Fails**
- Check `generated/tdd-logs/compile_errors.log` for specific errors
- Fix compilation issues before proceeding

**Service Health Gate Fails**
- Check backend container logs: `make backend-logs`
- Ensure services are running: `make up`
- Check port conflicts or container issues

**Template Gate Fails**
- Check backend logs for template-specific errors
- Look for missing template files or syntax errors
- Fix template issues in `internal/templates/`

**Go Test Gate Fails**
- Run tests individually to isolate failures
- Check test database configuration
- Use `make toolbox-test-run TEST=TestName` for specific test debugging

**HTTP Endpoint Gate Fails**
- Check which specific endpoints are failing
- Review backend logs for HTTP 500 errors
- Ensure authentication/routing is properly configured

**Browser Console Gate Fails**
- Requires Node.js and Playwright installation
- Check JavaScript errors in browser developer tools
- Fix frontend script errors

**Log Analysis Gate Fails**
- Review backend logs for ERROR/PANIC messages
- Fix underlying issues causing log errors
- Check database connectivity and configuration

### Performance Issues
- TDD verification typically takes 2-5 minutes
- Use `make quality-gates` to run gates independently for debugging
- Evidence collection adds minimal overhead
- Browser automation is the slowest gate (skipped if Playwright unavailable)

This system ensures no premature success claims and provides concrete evidence for all quality assertions.