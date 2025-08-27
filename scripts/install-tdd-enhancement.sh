#!/bin/bash
#
# INSTALL TDD ENHANCEMENT
# Manually integrate TDD enhancements into the existing Makefile
# Simplified version that avoids sed issues
#

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
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

warning() {
    echo -e "${YELLOW}⚠️ INSTALL:${NC} $1"
}

# Append TDD enhancements to existing Makefile
append_tdd_enhancements() {
    log "Appending TDD enhancements to Makefile..."
    
    # Check if already installed
    if grep -q "tdd-comprehensive:" "$PROJECT_ROOT/Makefile" 2>/dev/null; then
        warning "TDD enhancements already appear to be installed"
        return 0
    fi
    
    # Append the enhancement file to the Makefile
    if [ -f "$PROJECT_ROOT/Makefile.tdd-enhancement" ]; then
        cat "$PROJECT_ROOT/Makefile.tdd-enhancement" >> "$PROJECT_ROOT/Makefile"
        success "TDD enhancements appended to Makefile"
        
        # Clean up the enhancement file
        rm -f "$PROJECT_ROOT/Makefile.tdd-enhancement"
        return 0
    else
        warning "TDD enhancement file not found, creating inline..."
        
        # Add enhancements directly
        cat >> "$PROJECT_ROOT/Makefile" << 'EOF'

#########################################
# COMPREHENSIVE TDD AUTOMATION
#########################################

# Run comprehensive TDD verification with ALL quality gates
tdd-comprehensive:
	@echo "🧪 Running COMPREHENSIVE TDD verification..."
	@echo "Zero tolerance for false positives and premature success claims"
	@./scripts/tdd-comprehensive.sh comprehensive

# Anti-gaslighting detection - prevents false success claims
anti-gaslighting:
	@echo "🚨 Running anti-gaslighting detection..."
	@echo "Detecting premature success claims and hidden failures..."
	@./scripts/anti-gaslighting-detector.sh detect

# Initialize test-first TDD cycle with proper enforcement
tdd-test-first-init:
	@if [ -z "$(FEATURE)" ]; then \
		echo "Error: FEATURE required. Usage: make tdd-test-first-init FEATURE='Feature Name'"; \
		exit 1; \
	fi
	@echo "🔴 Initializing test-first TDD cycle for: $(FEATURE)"
	@./scripts/tdd-test-first-enforcer.sh init "$(FEATURE)"

# Generate failing test for TDD cycle
tdd-generate-test:
	@if [ ! -f .tdd-state ]; then \
		echo "Error: TDD not initialized. Run 'make tdd-test-first-init FEATURE=name' first"; \
		exit 1; \
	fi
	@echo "📝 Generating failing test..."
	@./scripts/tdd-test-first-enforcer.sh generate-test unit

# Complete guided TDD cycle with comprehensive verification
tdd-full-cycle:
	@if [ -z "$(FEATURE)" ]; then \
		echo "Error: FEATURE required. Usage: make tdd-full-cycle FEATURE='Feature Name'"; \
		exit 1; \
	fi
	@echo "🔄 Starting full TDD cycle for: $(FEATURE)"
	@./scripts/comprehensive-tdd-integration.sh full-cycle "$(FEATURE)"

# Quick verification for development (fast feedback)
tdd-quick:
	@echo "⚡ Running quick TDD verification..."
	@./scripts/comprehensive-tdd-integration.sh quick

# Show TDD dashboard with current status and metrics
tdd-dashboard:
	@./scripts/comprehensive-tdd-integration.sh dashboard

# Initialize comprehensive TDD environment  
tdd-comprehensive-init:
	@echo "🚀 Initializing comprehensive TDD environment..."
	@./scripts/comprehensive-tdd-integration.sh init

# Verify system integrity (prevents gaslighting)  
verify-integrity:
	@echo "🔍 Verifying system integrity..."
	@echo "Checking for false success claims and hidden failures..."
	@./scripts/anti-gaslighting-detector.sh detect
	@echo "Running comprehensive verification..."
	@./scripts/tdd-comprehensive.sh comprehensive

# Clean TDD state (reset cycle)
tdd-clean:
	@echo "🧹 Cleaning TDD state..."
	@rm -f .tdd-state
	@echo "TDD cycle reset. Start new cycle with 'make tdd-test-first-init FEATURE=name'"
EOF
        
        success "TDD enhancements added directly to Makefile"
        return 0
    fi
}

# Initialize TDD environment
initialize_tdd_environment() {
    log "Initializing TDD environment..."
    
    # Create directories
    mkdir -p "$PROJECT_ROOT/generated/tdd-logs"
    mkdir -p "$PROJECT_ROOT/generated/evidence"
    mkdir -p "$PROJECT_ROOT/generated/test-results"
    mkdir -p "$PROJECT_ROOT/generated/anti-gaslighting"
    mkdir -p "$PROJECT_ROOT/generated/tdd-enforcer"
    mkdir -p "$PROJECT_ROOT/generated/tdd-integration"
    mkdir -p "$PROJECT_ROOT/tests/integration"
    mkdir -p "$PROJECT_ROOT/tests/api"
    mkdir -p "$PROJECT_ROOT/tests/e2e"
    
    # Create .gitignore
    if [ ! -f "$PROJECT_ROOT/generated/.gitignore" ]; then
        cat > "$PROJECT_ROOT/generated/.gitignore" << 'EOF'
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
    "tdd_enforcer": "scripts/tdd-enforcer.sh",
    "comprehensive_tdd": "scripts/tdd-comprehensive.sh", 
    "anti_gaslighting": "scripts/anti-gaslighting-detector.sh",
    "test_first_enforcer": "scripts/tdd-test-first-enforcer.sh"
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
    
    success "TDD environment initialized"
}

# Test the installation
test_installation() {
    log "Testing TDD automation installation..."
    
    # Test that make commands work
    if make -n tdd-comprehensive > /dev/null 2>&1; then
        success "make tdd-comprehensive command available"
    else
        warning "make tdd-comprehensive command not working"
        return 1
    fi
    
    if make -n anti-gaslighting > /dev/null 2>&1; then
        success "make anti-gaslighting command available"
    else
        warning "make anti-gaslighting command not working"
        return 1
    fi
    
    if make -n tdd-dashboard > /dev/null 2>&1; then
        success "make tdd-dashboard command available"
    else
        warning "make tdd-dashboard command not working" 
        return 1
    fi
    
    success "TDD automation installation test passed"
    return 0
}

# Main installation
main() {
    echo ""
    echo "🧪 INSTALLING COMPREHENSIVE TDD ENHANCEMENTS"
    echo "============================================="
    echo "Adding comprehensive TDD automation to existing GOTRS infrastructure"
    echo ""
    
    # Step 1: Append TDD enhancements
    log "Step 1/3: Adding TDD enhancements to Makefile..."
    append_tdd_enhancements
    
    # Step 2: Initialize environment
    log "Step 2/3: Initializing TDD environment..."
    initialize_tdd_environment
    
    # Step 3: Test installation
    log "Step 3/3: Testing installation..."
    if test_installation; then
        echo ""
        echo "🎉 TDD AUTOMATION INSTALLATION COMPLETE"
        echo "======================================="
        echo ""
        echo "Available TDD commands:"
        echo "- make tdd-comprehensive    - Full quality gate verification"
        echo "- make anti-gaslighting     - Detect false success claims"
        echo "- make tdd-dashboard        - Show TDD status and metrics"
        echo "- make tdd-test-first-init FEATURE='name' - Start TDD cycle"
        echo "- make tdd-quick           - Quick development verification"
        echo ""
        echo "Try: make tdd-dashboard"
        echo ""
        success "TDD automation ready for use!"
        exit 0
    else
        echo ""
        echo "❌ TDD AUTOMATION INSTALLATION ISSUES"
        echo "====================================="
        echo "Some commands may not work properly. Check the Makefile manually."
        exit 1
    fi
}

# Run installation
main "$@"