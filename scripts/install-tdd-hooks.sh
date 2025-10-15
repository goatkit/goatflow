#!/bin/bash
#
# Install TDD Pre-commit Hooks
# Prevents commits without proper TDD verification
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
HOOKS_DIR="$PROJECT_ROOT/.git/hooks"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${GREEN}Installing TDD pre-commit hooks...${NC}"

# Check if we're in a git repository
if [ ! -d "$PROJECT_ROOT/.git" ]; then
    echo -e "${RED}Error: Not a git repository${NC}"
    exit 1
fi

# Create hooks directory if it doesn't exist
mkdir -p "$HOOKS_DIR"

# Create pre-commit hook for TDD enforcement
cat > "$HOOKS_DIR/pre-commit" << 'EOF'
#!/bin/bash
#
# TDD Pre-commit Hook
# Enforces TDD discipline and quality gates
#

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}TDD Pre-commit Hook: Enforcing quality gates...${NC}"

# Get the project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Change to project root
cd "$PROJECT_ROOT"

# Check if TDD workflow is active
if [ -f ".tdd-state" ]; then
    echo -e "${YELLOW}TDD workflow active - checking compliance...${NC}"
    
    # Read TDD state
    PHASE=$(jq -r '.phase' .tdd-state 2>/dev/null || echo "unknown")
    VERIFICATION_PASSED=$(jq -r '.verification_passed' .tdd-state 2>/dev/null || echo "false")
    
    # Check if verification has passed
    if [ "$VERIFICATION_PASSED" != "true" ]; then
        echo -e "${RED}❌ TDD VIOLATION: Cannot commit without passing verification${NC}"
        echo ""
        echo "Current TDD phase: $PHASE"
        echo "Verification status: Not passed"
        echo ""
        echo "Required steps:"
        echo "1. Run: make tdd-verify"
        echo "2. Ensure ALL 7 quality gates pass (100% success rate)"
        echo "3. Fix any failing gates"
        echo "4. Re-run verification until all gates pass"
        echo ""
        echo "No commits allowed without evidence of working code."
        echo "This prevents the 'Claude the intern' pattern of premature claims."
        exit 1
    fi
    
    # Check if verification is recent (within last 10 minutes)
    LAST_VERIFICATION=$(jq -r '.timestamp' .tdd-state 2>/dev/null || echo "0")
    CURRENT_TIME=$(date +%s)
    if [ -n "$LAST_VERIFICATION" ] && [ "$LAST_VERIFICATION" != "null" ]; then
        VERIFICATION_TIME=$(date -d "$LAST_VERIFICATION" +%s 2>/dev/null || echo "0")
        TIME_DIFF=$((CURRENT_TIME - VERIFICATION_TIME))
        
        # If verification is older than 10 minutes, require re-verification
        if [ $TIME_DIFF -gt 600 ]; then
            echo -e "${YELLOW}⚠️  Verification is older than 10 minutes${NC}"
            echo "Please re-run: make tdd-verify"
            echo "This ensures your commit reflects the current state."
            exit 1
        fi
    fi
    
    echo -e "${GREEN}✅ TDD verification passed - commit allowed${NC}"
fi

# Run basic compilation check
echo "Checking Go compilation..."
if ! go build -o /tmp/gotrs-server-build ./cmd/server >/dev/null 2>&1; then
    echo -e "${RED}❌ Go compilation failed${NC}"
    echo "Fix compilation errors before committing."
    exit 1
fi
rm -f /tmp/gotrs-server-build

# Run go fmt check
echo "Checking Go formatting..."
if [ -n "$(gofmt -l .)" ]; then
    echo -e "${YELLOW}⚠️  Go files not formatted${NC}"
    echo "Run: go fmt ./..."
    exit 1
fi

# Check for obvious issues in Go files
echo "Running basic Go checks..."
if go vet ./... 2>&1 | grep -q .; then
    echo -e "${YELLOW}⚠️  Go vet found issues${NC}"
    echo "Fix vet issues before committing."
    exit 1
fi

# Success
echo -e "${GREEN}✅ Pre-commit checks passed${NC}"
exit 0
EOF

# Make the hook executable
chmod +x "$HOOKS_DIR/pre-commit"

# Create pre-push hook for additional verification
cat > "$HOOKS_DIR/pre-push" << 'EOF'
#!/bin/bash
#
# TDD Pre-push Hook  
# Additional verification before pushing
#

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}TDD Pre-push Hook: Final verification...${NC}"

# Get the project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Change to project root
cd "$PROJECT_ROOT"

# If TDD workflow is active, require comprehensive verification
if [ -f ".tdd-state" ]; then
    echo -e "${YELLOW}TDD workflow active - comprehensive verification required...${NC}"
    
    # Check if evidence reports exist
    if [ ! -d "generated/evidence" ] || [ -z "$(find generated/evidence -name "*.json" -type f)" ]; then
        echo -e "${RED}❌ No TDD evidence found${NC}"
        echo "Run: make tdd-verify"
        echo "Cannot push without evidence of quality gate compliance."
        exit 1
    fi
    
    # Check for recent evidence (within last hour)
    RECENT_EVIDENCE=$(find generated/evidence -name "*.json" -type f -newermt "1 hour ago" | wc -l)
    if [ "$RECENT_EVIDENCE" -eq 0 ]; then
        echo -e "${YELLOW}⚠️  No recent TDD evidence found${NC}"
        echo "Run: make tdd-verify"
        echo "Ensure verification is current before pushing."
        exit 1
    fi
    
    echo -e "${GREEN}✅ TDD evidence found - push allowed${NC}"
fi

# Run quick test to ensure basic functionality
echo "Running quick compilation and basic tests..."

# Compile check
if ! go build -o /tmp/gotrs-server-build ./cmd/server >/dev/null 2>&1; then
    echo -e "${RED}❌ Compilation failed${NC}"
    exit 1
fi
rm -f /tmp/gotrs-server-build

# Run a subset of fast tests
if ! go test -short -timeout 30s ./... >/dev/null 2>&1; then
    echo -e "${RED}❌ Quick tests failed${NC}"
    echo "Run full test suite to identify issues."
    exit 1
fi

echo -e "${GREEN}✅ Pre-push verification passed${NC}"
exit 0
EOF

# Make the hook executable
chmod +x "$HOOKS_DIR/pre-push"

# Create commit-msg hook to enforce commit message format
cat > "$HOOKS_DIR/commit-msg" << 'EOF'
#!/bin/bash
#
# TDD Commit Message Hook
# Enforces commit message format and TDD compliance
#

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

COMMIT_MSG_FILE=$1
COMMIT_MSG=$(cat "$COMMIT_MSG_FILE")

# Get the project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Change to project root
cd "$PROJECT_ROOT"

# Skip if it's a merge commit
if echo "$COMMIT_MSG" | grep -q "^Merge "; then
    exit 0
fi

# Skip if it's a revert commit  
if echo "$COMMIT_MSG" | grep -q "^Revert "; then
    exit 0
fi

# If TDD workflow is active, enforce TDD commit format
if [ -f ".tdd-state" ]; then
    FEATURE=$(jq -r '.feature' .tdd-state 2>/dev/null || echo "")
    PHASE=$(jq -r '.phase' .tdd-state 2>/dev/null || echo "")
    
    # Check if commit message mentions TDD phase or feature
    if ! echo "$COMMIT_MSG" | grep -qi "tdd\|test\|implement\|refactor"; then
        echo -e "${YELLOW}⚠️  TDD workflow active but commit message doesn't indicate TDD phase${NC}"
        echo ""
        echo "Current TDD state:"
        echo "  Feature: $FEATURE"
        echo "  Phase: $PHASE"
        echo ""
        echo "Consider including TDD context in your commit message:"
        echo "  - 'TDD: Add failing tests for $FEATURE'"
        echo "  - 'TDD: Implement $FEATURE to pass tests'"
        echo "  - 'TDD: Refactor $FEATURE for better design'"
        echo ""
        echo "Continue anyway? [y/N]"
        read -r response
        if [[ ! "$response" =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi
fi

# Basic commit message validation
if [ ${#COMMIT_MSG} -lt 10 ]; then
    echo -e "${RED}❌ Commit message too short (minimum 10 characters)${NC}"
    exit 1
fi

# Check for common bad patterns
if echo "$COMMIT_MSG" | grep -qi "^fix\|^wip\|^tmp\|^test$"; then
    echo -e "${YELLOW}⚠️  Consider a more descriptive commit message${NC}"
    echo "Current: $COMMIT_MSG"
    echo ""
    echo "Continue anyway? [y/N]"
    read -r response
    if [[ ! "$response" =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

echo -e "${GREEN}✅ Commit message validated${NC}"
exit 0
EOF

# Make the hook executable
chmod +x "$HOOKS_DIR/commit-msg"

echo -e "${GREEN}✅ TDD hooks installed successfully!${NC}"
echo ""
echo "Installed hooks:"
echo "  - pre-commit:  Enforces TDD verification and basic quality checks"
echo "  - pre-push:    Additional verification before pushing"
echo "  - commit-msg:  Validates commit messages and TDD compliance"
echo ""
echo "These hooks will:"
echo "  ✓ Prevent commits without TDD verification when in TDD workflow"
echo "  ✓ Ensure code compiles before committing"
echo "  ✓ Check Go formatting and basic issues"
echo "  ✓ Require evidence of quality gate compliance"
echo "  ✓ Prevent the 'Claude the intern' pattern of premature claims"
echo ""
echo -e "${YELLOW}To bypass hooks in emergencies: git commit --no-verify${NC}"
echo -e "${YELLOW}But this defeats the purpose of TDD discipline!${NC}"