#!/bin/bash
# SQL Guard - Check for database portability issues
#
# This project uses sqlx with QueryBuilder.Rebind() for safe SQL:
# - Write queries with ? placeholders
# - Rebind() converts to $1/$2 (PostgreSQL) or ? (MySQL)
# - User values passed as args, never concatenated
#
# This script catches code that bypasses this safety:
# 1. Raw $N placeholders (PostgreSQL-only, bypasses Rebind)
# 2. ILIKE keyword (PostgreSQL-only, use LOWER() or adapter)

set -e

RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
NC='\033[0m'

ERRORS=0
WARNINGS=0

# Default: check all internal Go files (production code)
# --staged: only check staged files (for pre-commit hook)
# --all: check everything including tests
if [ "$1" = "--staged" ]; then
    FILES=$(git diff --cached --name-only --diff-filter=ACM | grep '\.go$' || true)
elif [ "$1" = "--all" ]; then
    FILES=$(find ./internal -name '*.go' -not -path './vendor/*' 2>/dev/null || true)
else
    FILES=$(find ./internal -name '*.go' -not -path './vendor/*' -not -name '*_test.go' 2>/dev/null || true)
fi

if [ -z "$FILES" ]; then
    echo -e "${GREEN}No Go files to check${NC}"
    exit 0
fi

declare -a FINDINGS=()

while IFS= read -r file; do
    [ -z "$file" ] && continue
    [ ! -f "$file" ] && continue

    # Skip database adapter files - they handle the conversion
    if echo "$file" | grep -qE '(database/adapter|database/sql_compat|database/querybuilder)'; then
        continue
    fi

    # 1. Raw $N placeholders in SQL (bypasses Rebind, PostgreSQL-only)
    # Should use ? with qb.Rebind() instead
    matches=$(grep -nE '\$[0-9]+' "$file" 2>/dev/null | \
        grep -iE '(SELECT|INSERT|UPDATE|DELETE|WHERE|FROM|JOIN|AND|OR)' | \
        grep -v '// sql-ok' | \
        grep -v 'ConvertPlaceholders' | \
        grep -v 'Rebind' | \
        grep -v '\$2[aby]\$' || true)  # Exclude bcrypt hash prefixes
    if [ -n "$matches" ]; then
        while IFS= read -r match; do
            FINDINGS+=("${RED}❌ $file: Raw \$N placeholder bypasses Rebind() - use ? placeholders${NC}")
            FINDINGS+=("    $match")
            ((ERRORS++)) || true
        done <<< "$matches"
    fi

    # 2. ILIKE keyword (PostgreSQL-only)
    # Should use LOWER(col) LIKE LOWER(?) for MySQL compatibility
    # Or use the database adapter's case-insensitive search method
    matches=$(grep -nE '\bILIKE\b' "$file" 2>/dev/null | \
        grep -v '// sql-ok' | \
        grep -v 'database/adapter' || true)
    if [ -n "$matches" ]; then
        while IFS= read -r match; do
            FINDINGS+=("${YELLOW}⚠️  $file: ILIKE is PostgreSQL-only - use LOWER() LIKE LOWER() for MySQL${NC}")
            FINDINGS+=("    $match")
            ((WARNINGS++)) || true
        done <<< "$matches"
    fi

done <<< "$FILES"

# Output findings
for finding in "${FINDINGS[@]}"; do
    echo -e "$finding"
done

# Summary
if [ $ERRORS -gt 0 ]; then
    echo ""
    echo -e "${RED}❌ SQL guard: $ERRORS portability error(s), $WARNINGS warning(s)${NC}"
    echo ""
    echo "Fix: Use ? placeholders with qb.Rebind() instead of raw \$N"
    echo "     Use LOWER(col) LIKE LOWER(?) instead of ILIKE"
    echo "     Add '// sql-ok' comment to suppress false positives"
    exit 1
elif [ $WARNINGS -gt 0 ]; then
    echo ""
    echo -e "${YELLOW}⚠️  SQL guard: $WARNINGS portability warning(s)${NC}"
    exit 0
else
    echo -e "${GREEN}✅ SQL guard passed - no portability issues${NC}"
    exit 0
fi
