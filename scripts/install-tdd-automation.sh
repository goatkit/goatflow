#!/bin/bash
#
# INSTALL TDD AUTOMATION
# Installs comprehensive test automation that prevents "Claude the intern" pattern
# Integrates with existing GOTRS infrastructure while enforcing true TDD
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

# Logging functions
log() {
    echo -e "${CYAN}[$(date +%H:%M:%S)] INSTALL:${NC} $1"
}

success() {
    echo -e "${GREEN}✅ INSTALL:${NC} $1"
}

fail() {
    echo -e "${RED}❌ INSTALL:${NC} $1"
}

warning() {
    echo -e "${YELLOW}⚠️ INSTALL:${NC} $1"
}

# Backup existing Makefile
backup_makefile() {
    if [ -f "$PROJECT_ROOT/Makefile" ]; then
        local backup_file="$PROJECT_ROOT/Makefile.backup.$(date +%Y%m%d_%H%M%S)"
        cp "$PROJECT_ROOT/Makefile" "$backup_file"
        success "Makefile backed up to: $backup_file"
        echo "$backup_file"
    else
        fail "No existing Makefile found"
        return 1
    fi
}

# Update Makefile help section
update_makefile_help() {
    log "Updating Makefile help section..."
    
    # Add new TDD commands to help section after line 48 (after evidence-report line)
    local help_addition='
	@echo "Advanced TDD Commands (Zero Tolerance for False Claims):"
	@echo "  make tdd-comprehensive           - Run ALL quality gates with evidence"
	@echo "  make anti-gaslighting            - Detect false success claims" 
	@echo "  make tdd-test-first-init FEATURE=name - Initialize test-first TDD cycle"
	@echo "  make tdd-full-cycle FEATURE=name - Complete guided TDD cycle"
	@echo "  make tdd-quick                   - Quick verification for development"
	@echo "  make tdd-dashboard              - Show TDD status and metrics"
	@echo ""'
    
    # Use sed to add after the evidence-report line
    sed -i '/evidence-report/a\'"$help_addition" "$PROJECT_ROOT/Makefile"
    
    success "Help section updated with new TDD commands"
}

# Add comprehensive TDD commands to Makefile
add_tdd_commands() {
    log "Adding comprehensive TDD commands to Makefile..."
    
    # Read the enhancement file and append to Makefile
    cat "$PROJECT_ROOT/Makefile.tdd-enhancement" >> "$PROJECT_ROOT/Makefile"
    
    success "Comprehensive TDD commands added to Makefile"
}

# Install Node.js dependencies for browser testing
install_browser_test_deps() {
    log "Installing Node.js dependencies for browser testing..."
    
    # Create package.json if it doesn't exist
    if [ ! -f "$PROJECT_ROOT/package.json" ]; then
        cat > "$PROJECT_ROOT/package.json" << EOF
{
  "name": "gotrs-test-automation",
  "version": "1.0.0", 
  "description": "Browser testing dependencies for GOTRS TDD automation",
  "scripts": {
    "install-playwright": "playwright install chromium",
    "test-browser": "echo 'Browser tests ready'"
  },
  "dependencies": {
    "playwright": "^1.40.0"
  },
  "devDependencies": {}
}
EOF
        success "package.json created for browser testing dependencies"
    fi
    
    # Try to install Playwright using the existing container approach
    if command -v docker >/dev/null 2>&1 || command -v podman >/dev/null 2>&1; then
        log "Installing Playwright in container..."
        # Use the existing container-first approach
        make toolbox-build > /dev/null 2>&1 || true
        success "Playwright dependencies prepared (will install in container when needed)"
    else
        warning "No container runtime found - browser tests may not work without manual Playwright installation"
    fi
}

# Create git hooks for TDD enforcement
install_git_hooks() {
    log "Installing git hooks for TDD enforcement..."
    
    if [ -d "$PROJECT_ROOT/.git" ]; then
        # Pre-commit hook
        cat > "$PROJECT_ROOT/.git/hooks/pre-commit" << 'EOF'
#!/bin/bash
# TDD Pre-commit Hook
# Prevents commits with false success claims or hidden failures

echo "🔒 Running TDD pre-commit verification..."

# Run anti-gaslighting check
if ! ./scripts/anti-gaslighting-detector.sh quick; then
    echo "❌ Pre-commit failed: Gaslighting detected"
    echo "Fix all issues before committing"
    exit 1
fi

echo "✅ Pre-commit TDD verification passed"
exit 0
EOF
        chmod +x "$PROJECT_ROOT/.git/hooks/pre-commit"
        
        # Pre-push hook
        cat > "$PROJECT_ROOT/.git/hooks/pre-push" << 'EOF'
#!/bin/bash
# TDD Pre-push Hook
# Comprehensive verification before pushing

echo "🚀 Running TDD pre-push verification..."

# Run comprehensive TDD verification
if ! ./scripts/tdd-comprehensive.sh comprehensive; then
    echo "❌ Pre-push failed: Comprehensive verification failed"
    echo "All quality gates must pass before pushing"
    exit 1
fi

echo "✅ Pre-push TDD verification passed"
exit 0
EOF
        chmod +x "$PROJECT_ROOT/.git/hooks/pre-push"
        
        success "Git hooks installed for TDD enforcement"
    else
        warning "Not a git repository - git hooks not installed"
    fi
}

# Initialize TDD environment
initialize_tdd_environment() {
    log "Initializing TDD environment..."
    
    # Run the comprehensive TDD initialization
    "$PROJECT_ROOT/scripts/comprehensive-tdd-integration.sh" init
    
    success "TDD environment initialized"
}

# Create documentation
create_tdd_documentation() {
    log "Creating TDD automation documentation..."
    
    cat > "$PROJECT_ROOT/TDD-AUTOMATION.md" << 'EOF'
# GOTRS TDD Automation

This project includes comprehensive Test-Driven Development automation that prevents false success claims and enforces evidence-based verification.

## Anti-Gaslighting Protection

The TDD automation specifically addresses the "Claude the intern" pattern where success is claimed despite:
- Failing tests
- Server errors (500, 404) 
- JavaScript console errors
- Missing UI elements
- Authentication bypasses
- Template rendering errors
- Compilation failures

## Quick Start

1. **Initialize TDD cycle:**
   ```bash
   make tdd-test-first-init FEATURE="My New Feature"
   ```

2. **Generate failing test:**
   ```bash
   make tdd-generate-test
   ```

3. **Implement minimal code to make test pass**

4. **Run comprehensive verification:**
   ```bash
   make tdd-comprehensive
   ```

## Key Commands

- `make tdd-comprehensive` - Run ALL quality gates with evidence
- `make anti-gaslighting` - Detect false success claims
- `make tdd-full-cycle FEATURE=name` - Complete guided TDD cycle
- `make tdd-dashboard` - Show TDD status and metrics
- `make tdd-quick` - Quick verification for development

## Quality Gates

All 11 quality gates must pass for success claims:

1. **Compilation** - Go code compiles without errors
2. **Unit Tests** - All unit tests pass with >70% coverage
3. **Integration Tests** - Database and service integration works
4. **Security Tests** - Authentication and password security verified
5. **Service Health** - Health endpoint responds correctly
6. **Database Tests** - Migrations and connectivity verified
7. **Template Tests** - Template rendering without errors
8. **API Tests** - HTTP endpoints respond correctly (>80% success rate)
9. **Browser Tests** - JavaScript console error-free
10. **Performance Tests** - Response times under thresholds
11. **Regression Tests** - Historical failures prevented

## Historical Failure Prevention

The automation specifically checks for these historical failure patterns:
- Password echoing in logs or output
- Template syntax errors breaking pages
- Authentication bugs allowing bypasses
- JavaScript console errors breaking UI
- Missing UI elements making pages unusable
- 500 server errors indicating backend problems
- 404 not found errors for expected endpoints

## Evidence Collection

All verifications collect evidence in `generated/evidence/` with:
- Timestamped JSON evidence files
- HTML reports with comprehensive results
- Anti-gaslighting violation reports
- TDD cycle tracking and metrics

## Zero Tolerance Policy

Success is only claimed when:
- ALL quality gates pass (100% success rate)
- ZERO historical failure patterns detected
- Complete evidence collected and verified
- No false positive test results

This prevents premature success claims and ensures genuine system functionality.
EOF
    
    success "TDD automation documentation created: TDD-AUTOMATION.md"
}

# Verify installation
verify_installation() {
    log "Verifying TDD automation installation..."
    
    local verification_results=()
    local verification_success=true
    
    # Check scripts are executable
    local scripts=(
        "tdd-comprehensive.sh"
        "anti-gaslighting-detector.sh" 
        "tdd-test-first-enforcer.sh"
        "comprehensive-tdd-integration.sh"
        "tdd-enforcer.sh"
    )
    
    for script in "${scripts[@]}"; do
        if [ -x "$PROJECT_ROOT/scripts/$script" ]; then
            verification_results+=("✅ $script executable")
        else
            verification_results+=("❌ $script not executable")
            verification_success=false
        fi
    done
    
    # Check Makefile commands
    if grep -q "tdd-comprehensive:" "$PROJECT_ROOT/Makefile"; then
        verification_results+=("✅ Makefile updated with TDD commands")
    else
        verification_results+=("❌ Makefile not updated")
        verification_success=false
    fi
    
    # Check TDD environment
    if [ -f "$PROJECT_ROOT/generated/tdd-config.json" ]; then
        verification_results+=("✅ TDD environment initialized")
    else
        verification_results+=("❌ TDD environment not initialized")
        verification_success=false
    fi
    
    # Display results
    echo ""
    echo "🔍 Installation Verification Results:"
    echo "===================================="
    for result in "${verification_results[@]}"; do
        echo "  $result"
    done
    echo ""
    
    if [ "$verification_success" = true ]; then
        success "TDD automation installation verified successfully"
        return 0
    else
        fail "TDD automation installation verification failed"
        return 1
    fi
}

# Show usage instructions
show_usage_instructions() {
    echo ""
    echo "🎉 TDD AUTOMATION INSTALLATION COMPLETE"
    echo "======================================="
    echo ""
    echo "Next steps:"
    echo "1. Try the TDD dashboard: make tdd-dashboard"
    echo "2. Start a TDD cycle: make tdd-test-first-init FEATURE='Test Feature'"
    echo "3. Run comprehensive verification: make tdd-comprehensive"
    echo "4. Check for false claims: make anti-gaslighting"
    echo ""
    echo "Available commands:"
    echo "- make tdd-comprehensive    - Full quality gate verification"
    echo "- make anti-gaslighting     - Detect false success claims"
    echo "- make tdd-full-cycle       - Complete guided TDD cycle"
    echo "- make tdd-dashboard        - Show TDD status and metrics"
    echo "- make tdd-quick           - Quick development verification"
    echo ""
    echo "Documentation: cat TDD-AUTOMATION.md"
    echo "Evidence files: ls generated/evidence/"
    echo ""
    echo "🚨 ZERO TOLERANCE for false success claims!"
    echo "All quality gates must pass for legitimate success."
}

# Main installation process
main() {
    echo ""
    echo "🧪 INSTALLING COMPREHENSIVE TDD AUTOMATION"
    echo "==========================================="
    echo "Installing evidence-based test automation that prevents false success claims"
    echo ""
    
    # Step 1: Backup existing files
    log "Step 1/7: Backing up existing files..."
    backup_file=$(backup_makefile)
    
    # Step 2: Update Makefile help section
    log "Step 2/7: Updating Makefile help section..."
    update_makefile_help
    
    # Step 3: Add TDD commands
    log "Step 3/7: Adding comprehensive TDD commands..."
    add_tdd_commands
    
    # Step 4: Install browser test dependencies
    log "Step 4/7: Installing browser test dependencies..."
    install_browser_test_deps
    
    # Step 5: Install git hooks
    log "Step 5/7: Installing git hooks..."
    install_git_hooks
    
    # Step 6: Initialize TDD environment
    log "Step 6/7: Initializing TDD environment..."
    initialize_tdd_environment
    
    # Step 7: Create documentation
    log "Step 7/7: Creating documentation..."
    create_tdd_documentation
    
    # Verify installation
    if verify_installation; then
        show_usage_instructions
        success "TDD automation installation completed successfully"
        exit 0
    else
        fail "TDD automation installation failed verification"
        exit 1
    fi
}

# Run installation
main "$@"