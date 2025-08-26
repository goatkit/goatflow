#!/bin/bash
# Install comprehensive git hooks for GOTRS project
# This script sets up pre-commit hooks for security, quality, and legal compliance

set -e

HOOKS_DIR=".git/hooks"
PRE_COMMIT_HOOK="$HOOKS_DIR/pre-commit"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}Installing comprehensive GOTRS git hooks...${NC}"

# Ensure we're in a git repository
if [ ! -d ".git" ]; then
    echo -e "${RED}Error: Not in a git repository root${NC}"
    exit 1
fi

# Create hooks directory if it doesn't exist
mkdir -p "$HOOKS_DIR"

# Create comprehensive pre-commit hook
cat > "$PRE_COMMIT_HOOK" << 'EOF'
#!/bin/bash
# GOTRS Pre-commit hook - Comprehensive security and quality checks

set -e  # Exit on any error

echo "🔍 Running pre-commit checks..."

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Check 1: Scan for secrets
echo -e "${BLUE}🔐 Scanning for secrets...${NC}"

# Check if gitleaks is available
if command -v gitleaks &> /dev/null; then
    gitleaks protect --staged --verbose
    if [ $? -ne 0 ]; then
        echo -e "${RED}❌ Secrets detected in staged changes! Commit aborted.${NC}"
        echo "   Review the findings above and remove any secrets from staged files."
        echo "   Use 'gitleaks protect --staged --verbose' to re-scan."
        exit 1
    fi
else
    # Try with Docker/Podman
    CONTAINER_CMD=$(command -v podman 2> /dev/null || command -v docker 2> /dev/null)
    if [ -n "$CONTAINER_CMD" ]; then
        $CONTAINER_CMD run --rm -v "$(pwd):/workspace" -w /workspace \
            zricethezav/gitleaks:latest protect --staged --verbose
        if [ $? -ne 0 ]; then
            echo -e "${RED}❌ Secrets detected! Commit aborted.${NC}"
            exit 1
        fi
    else
        echo -e "${YELLOW}⚠️  Warning: gitleaks not found. Install it or use Docker/Podman.${NC}"
        echo "   Skipping secret scan (not recommended)."
    fi
fi

echo -e "${GREEN}✅ No secrets detected${NC}"

# Check 2: Prevent binary files from being committed
echo -e "${BLUE}🚫 Checking for binary files...${NC}"

# Get list of staged files
STAGED_FILES=$(git diff --cached --name-only)

if [ -n "$STAGED_FILES" ]; then
    # Define binary file patterns to block
    BINARY_PATTERNS=(
        # Compiled executables
        '\.exe$' '\.dll$' '\.so$' '\.dylib$' '\.a$' '\.lib$' '\.bin$'
        
        # Archives and compressed files
        '\.tar$' '\.tar\.gz$' '\.tgz$' '\.zip$' '\.rar$' '\.7z$' '\.gz$' '\.bz2$' '\.xz$'
        
        # Package files
        '\.deb$' '\.rpm$' '\.dmg$' '\.msi$' '\.pkg$' '\.AppImage$'
        
        # Database files
        '\.db$' '\.sqlite$' '\.sqlite3$'
        
        # Large data files
        '\.dump$' '\.backup$' '\.bak$' '\.orig$'
        
        # Media files (if large)
        '\.avi$' '\.mov$' '\.mp4$' '\.mkv$' '\.wmv$' '\.flv$'
        '\.mp3$' '\.wav$' '\.flac$' '\.aac$' '\.ogg$'
        
        # VM and container images
        '\.iso$' '\.img$' '\.qcow2$' '\.vmdk$' '\.vdi$'
        '\.ova$' '\.ovf$'
    )
    
    # Common binary file names (without extension)
    BINARY_NAMES=(
        'server' 'goats' 'gotrs' 'generator' 'gotrs-babelfish'
        'a\.out' 'core' 'core\.[0-9]+'
    )
    
    BLOCKED_FILES=()
    LARGE_FILES=()
    
    while IFS= read -r file; do
        # Skip if file doesn't exist (deleted files)
        if [ ! -f "$file" ]; then
            continue
        fi
        
        # Check file patterns
        for pattern in "${BINARY_PATTERNS[@]}"; do
            if echo "$file" | grep -qE "$pattern"; then
                BLOCKED_FILES+=("$file (matches pattern: $pattern)")
                continue 2  # Skip to next file
            fi
        done
        
        # Check binary names
        basename_file=$(basename "$file")
        for name_pattern in "${BINARY_NAMES[@]}"; do
            if echo "$basename_file" | grep -qE "^${name_pattern}$"; then
                BLOCKED_FILES+=("$file (matches binary name: $name_pattern)")
                continue 2  # Skip to next file
            fi
        done
        
        # Check file size (block files >10MB)
        file_size=$(stat -f%z "$file" 2>/dev/null || stat -c%s "$file" 2>/dev/null || echo 0)
        if [ "$file_size" -gt 10485760 ]; then  # 10MB in bytes
            size_mb=$((file_size / 1048576))
            LARGE_FILES+=("$file (${size_mb}MB)")
        fi
        
        # Check if file is binary using git's method
        if git diff --cached --numstat "$file" | grep -q '^-[[:space:]]*-[[:space:]]'; then
            # This is a binary file according to git
            # But allow some common binary types that are OK in repos
            if ! echo "$file" | grep -qE '\.(png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|otf|eot)$'; then
                BLOCKED_FILES+=("$file (detected as binary by git)")
            fi
        fi
        
    done <<< "$STAGED_FILES"
    
    # Report blocked files
    if [ ${#BLOCKED_FILES[@]} -gt 0 ]; then
        echo -e "${RED}❌ Binary files detected in staged changes:${NC}"
        for blocked_file in "${BLOCKED_FILES[@]}"; do
            echo -e "${RED}   - $blocked_file${NC}"
        done
        echo ""
        echo "These files are blocked to prevent repository bloat."
        echo "If you need to commit binary files:"
        echo "1. Move them to a separate binary storage (e.g., Git LFS)"
        echo "2. Add them to .gitignore if they're build artifacts"
        echo "3. Use 'git rm --cached <file>' to unstage them"
        exit 1
    fi
    
    # Report large files as warnings
    if [ ${#LARGE_FILES[@]} -gt 0 ]; then
        echo -e "${YELLOW}⚠️  Large files detected (>10MB):${NC}"
        for large_file in "${LARGE_FILES[@]}"; do
            echo -e "${YELLOW}   - $large_file${NC}"
        done
        echo -e "${YELLOW}Consider using Git LFS for large files.${NC}"
    fi
fi

echo -e "${GREEN}✅ No prohibited binary files detected${NC}"

# Check 3: Attribution detection (existing functionality)
echo -e "${BLUE}🤖 Checking for attribution...${NC}"

# Get the commit message from stdin or from file
if [ -t 0 ]; then
    # If stdin is a terminal, read from git's commit message file
    COMMIT_MSG_FILE="$1"
    if [ -n "$COMMIT_MSG_FILE" ] && [ -f "$COMMIT_MSG_FILE" ]; then
        COMMIT_MSG=$(cat "$COMMIT_MSG_FILE")
    else
        # Fallback: check if there's a temporary commit message
        COMMIT_MSG=$(git log --format=%B -n 1 HEAD 2>/dev/null || echo "")
    fi
else
    # Read from stdin
    COMMIT_MSG=$(cat)
fi

# Check for attribution patterns in commit message
if echo "$COMMIT_MSG" | grep -qi -E "(Claude|Anthropic|Co-Authored-By.*Claude|Generated.*Claude|🤖.*Claude|AI.*generated|claude\.ai)"; then
    echo -e "${RED}❌ COMMIT REJECTED: Attribution detected!${NC}"
    echo -e "${RED}Found forbidden patterns:${NC}"
    echo "$COMMIT_MSG" | grep -i -E "(Claude|Anthropic|Co-Authored-By.*Claude|Generated.*Claude|🤖.*Claude|AI.*generated|claude\.ai)" | sed 's/^/  - /'
    echo ""
    echo -e "${YELLOW}Your commit message contains attribution which is not allowed.${NC}"
    echo -e "${YELLOW}Please remove all attribution lines from your commit message.${NC}"
    echo ""
    echo -e "${YELLOW}Common attribution patterns to remove:${NC}"
    echo "  - Co-Authored-By: ..."
    echo "  - Generated with ..."
    echo "  - 🤖 symbols"
    echo "  - References to AI/Claude/Anthropic"
    exit 1
fi

echo -e "${GREEN}✅ No attribution detected${NC}"

echo -e "${GREEN}🎉 All pre-commit checks passed!${NC}"
exit 0
EOF

# Make the hook executable
chmod +x "$PRE_COMMIT_HOOK"

echo -e "${GREEN}✅ Comprehensive git hooks installed successfully!${NC}"
echo ""
echo "Installed hooks:"
echo "  - pre-commit: Multi-layered security and quality checks"
echo "    • Secret scanning with Gitleaks"
echo "    • Binary file prevention"
echo "    • Large file warnings (>10MB)"
echo "    • Attribution pattern blocking"
echo ""
echo "The hook will run automatically before each commit."
echo "To bypass (use with caution): git commit --no-verify"
echo ""
echo "To test the hook manually:"
echo "  $PRE_COMMIT_HOOK"
echo ""
echo -e "${BLUE}🔒 Your repository is now protected from:${NC}"
echo "  • Accidental secret commits"
echo "  • Binary bloat"
echo "  • Attribution leaks"
echo "  • Repository size issues"