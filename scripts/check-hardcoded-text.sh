#!/bin/bash
# check-hardcoded-text.sh - Scan pongo2 templates for hardcoded UI text that should use i18n
#
# This script finds text that appears to be user-facing content but isn't wrapped
# in {{ t("...") }} translation calls.
#
# Usage: ./scripts/check-hardcoded-text.sh [--fix] [--verbose] [--strict]
#   --fix      Show suggested fixes (doesn't auto-apply)
#   --verbose  Show all matches including context
#   --strict   Fail on any findings (for CI)

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
TEMPLATES_DIR="$PROJECT_ROOT/templates"

# Colors
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

VERBOSE=false
FIX=false
STRICT=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --verbose) VERBOSE=true; shift ;;
        --fix) FIX=true; shift ;;
        --strict) STRICT=true; shift ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

echo -e "${CYAN}Scanning templates for hardcoded UI text...${NC}"
echo ""

# Temporary file for results
RESULTS_FILE=$(mktemp)
trap "rm -f $RESULTS_FILE" EXIT

# === PATTERN 1: Hardcoded text in alert/confirm JavaScript calls ===
echo "Checking for hardcoded text in JavaScript alert/confirm..."
for file in $(find "$TEMPLATES_DIR" -name "*.pongo2" -type f); do
    grep -n "alert('[^']*')" "$file" 2>/dev/null | while read -r match; do
        # Skip if it uses template vars
        if echo "$match" | grep -q '{{'; then continue; fi
        line_num=$(echo "$match" | cut -d: -f1)
        text=$(echo "$match" | grep -oP "alert\('\K[^']+" || echo "")
        if [[ -n "$text" ]] && [[ ${#text} -gt 5 ]]; then
            echo "JS_ALERT|$file|$line_num|$text" >> "$RESULTS_FILE"
        fi
    done
    grep -n "confirm('[^']*')" "$file" 2>/dev/null | while read -r match; do
        if echo "$match" | grep -q '{{'; then continue; fi
        line_num=$(echo "$match" | cut -d: -f1)
        text=$(echo "$match" | grep -oP "confirm\('\K[^']+" || echo "")
        if [[ -n "$text" ]] && [[ ${#text} -gt 5 ]]; then
            echo "JS_CONFIRM|$file|$line_num|$text" >> "$RESULTS_FILE"
        fi
    done
done

# === PATTERN 2: Hardcoded phrases in templates ===
echo "Checking for hardcoded phrases..."
PHRASES=(
    "tickets selected"
    "ticket selected"
    "items selected"
    "No tickets found"
    "No results found"
    "Are you sure"
    "Change Status"
    "Change Priority"
    "Clear selection"
    "Loading..."
    "Please wait"
    "Click here"
    "Learn more"
    "View all"
    "No data"
    "Nothing found"
    "Please select"
    "Please enter"
    "Select all"
    "matching tickets"
    "Select tickets"
    "bulk actions"
    "Dynamic Field"
    "Apply DF"
    'placeholder="Contains'
    "Open Tickets"
    "Avg Response"
    "Active Agents"
    "Ticket Volume"
    "Queue Distribution"
    "Queue Status"
    "Active Alerts"
)

for file in $(find "$TEMPLATES_DIR" -name "*.pongo2" -type f); do
    for phrase in "${PHRASES[@]}"; do
        grep -in "$phrase" "$file" 2>/dev/null | while read -r match; do
            # Skip if line contains i18n pattern or template variable output
            if echo "$match" | grep -qE '\{\{[[:space:]]*t\(|default:"|:\{\{[[:space:]]*[a-zA-Z_]'; then continue; fi
            # Skip if the phrase is just part of a variable name (e.g., {{ loading_message }})
            if echo "$match" | grep -qE '\{\{[[:space:]]*[a-zA-Z_]*loading[a-zA-Z_]*[[:space:]]*\}\}'; then continue; fi
            # Skip if it's in a data attribute (already handled by JS)
            if echo "$match" | grep -q 'data-singular\|data-plural'; then continue; fi
            # Skip HTML comments
            if echo "$match" | grep -qE '<!--.*-->|<!--'; then continue; fi
            # Skip template comments
            if echo "$match" | grep -qE '\{#.*#\}|\{#'; then continue; fi
            # Skip console.log/error/warn statements
            if echo "$match" | grep -qE 'console\.(log|error|warn|info|debug)\('; then continue; fi
            # Skip JavaScript variable assignments (loading: false, loading = true, etc.)
            if echo "$match" | grep -qE '[a-zA-Z_]+[[:space:]]*[:=][[:space:]]*(true|false|null|[0-9])'; then continue; fi
            # Skip macro parameter defaults
            if echo "$match" | grep -qE '\{%[[:space:]]*macro[[:space:]]'; then continue; fi
            # Skip Alpine.js directives (x-show="loading", etc.)
            if echo "$match" | grep -qE 'x-show="[a-zA-Z_]+"'; then continue; fi
            # Skip HTML id/class attributes with "loading"
            if echo "$match" | grep -qE '(id|class)="[^"]*loading[^"]*"'; then continue; fi
            # Skip img loading="lazy" attribute
            if echo "$match" | grep -qE 'loading="lazy"'; then continue; fi
            # Skip JS dataset property access
            if echo "$match" | grep -qE '\.loading[[:space:]]*[=!]'; then continue; fi
            # Skip JS function names containing "loading"
            if echo "$match" | grep -qE 'function[[:space:]]+[a-zA-Z]*[Ll]oading'; then continue; fi
            # Skip JS single-line comments (including mid-line comments and full-line comments)
            if echo "$match" | grep -qE '(^[0-9]+:[[:space:]]*//|[[:space:]]// )'; then continue; fi
            # Skip showToast messages (JS toast notifications)
            if echo "$match" | grep -qE "showToast\("; then continue; fi
            # Skip innerHTML error messages
            if echo "$match" | grep -qE '\.innerHTML[[:space:]]*='; then continue; fi
            # Skip document.readyState checks
            if echo "$match" | grep -qE 'document\.readyState'; then continue; fi
            # Skip JavaScript object literals (demo data, config objects)
            if echo "$match" | grep -qE "message:[[:space:]]*['\"]|affected_item:[[:space:]]*['\"]|action_required:[[:space:]]*['\"]"; then continue; fi
            # Skip JavaScript object key-value pairs with string values
            if echo "$match" | grep -qE "queue_name:[[:space:]]*['\"]"; then continue; fi
            # Skip JavaScript getElementById/querySelector references
            if echo "$match" | grep -qE "getElementById\(['\"]|querySelector\(['\"]|querySelectorAll\(['\"]"; then continue; fi
            # Skip JavaScript object property labels (for token snippets, config objects)
            if echo "$match" | grep -qE "label:[[:space:]]*['\"]|description:[[:space:]]*['\"]|hint:[[:space:]]*['\"]"; then continue; fi
            # Skip lowercase "dynamic field" in descriptive text (only capitalized versions need i18n)
            if [[ "$phrase" == "Dynamic Field" ]] && echo "$match" | grep -qv "Dynamic Field"; then continue; fi
            # Skip JavaScript ternary fallbacks (e.g., textSpan ? textSpan.dataset.singular : 'ticket selected')
            if echo "$match" | grep -qE "\?[^:]+:[[:space:]]*['\"][^'\"]+['\"]"; then continue; fi
            line_num=$(echo "$match" | cut -d: -f1)
            echo "PHRASE|$file|$line_num|$phrase" >> "$RESULTS_FILE"
        done
    done
done

# === PATTERN 3: Hardcoded title attributes ===
echo "Checking for hardcoded title attributes..."
for file in $(find "$TEMPLATES_DIR" -name "*.pongo2" -type f); do
    grep -n 'title="[A-Z][a-zA-Z ]*"' "$file" 2>/dev/null | while read -r match; do
        # Skip if uses i18n
        if echo "$match" | grep -qE '\{\{[[:space:]]*t\(|default:"'; then continue; fi
        # Skip macro parameter defaults
        if echo "$match" | grep -qE '\{%[[:space:]]*macro[[:space:]]'; then continue; fi
        # Skip include with parameters
        if echo "$match" | grep -qE '\{%[[:space:]]*include[[:space:]]'; then continue; fi
        line_num=$(echo "$match" | cut -d: -f1)
        title=$(echo "$match" | grep -oP 'title="\K[^"]+' | head -1 || echo "")
        # Skip short or technical titles
        if [[ -n "$title" ]] && [[ ${#title} -gt 3 ]] && [[ ! "$title" =~ ^[A-Z]+$ ]]; then
            echo "TITLE|$file|$line_num|$title" >> "$RESULTS_FILE"
        fi
    done
done

# === PATTERN 4: Hardcoded placeholder attributes ===
echo "Checking for hardcoded placeholder attributes..."
for file in $(find "$TEMPLATES_DIR" -name "*.pongo2" -type f); do
    grep -n 'placeholder="[A-Z][a-zA-Z ]*"' "$file" 2>/dev/null | while read -r match; do
        # Skip if uses i18n
        if echo "$match" | grep -qE '\{\{[[:space:]]*t\(|default:"'; then continue; fi
        line_num=$(echo "$match" | cut -d: -f1)
        placeholder=$(echo "$match" | grep -oP 'placeholder="\K[^"]+' | head -1 || echo "")
        if [[ -n "$placeholder" ]] && [[ ${#placeholder} -gt 5 ]]; then
            echo "PLACEHOLDER|$file|$line_num|$placeholder" >> "$RESULTS_FILE"
        fi
    done
done

# === PATTERN 5: Hardcoded text in block title ===
echo "Checking for hardcoded block titles..."
for file in $(find "$TEMPLATES_DIR" -name "*.pongo2" -type f); do
    grep -n '{% block title %}[^{]*{% endblock %}' "$file" 2>/dev/null | while read -r match; do
        # Skip if uses i18n
        if echo "$match" | grep -qE '\{\{[[:space:]]*t\('; then continue; fi
        line_num=$(echo "$match" | cut -d: -f1)
        # Extract text between block title and endblock
        text=$(echo "$match" | sed -n 's/.*{% block title %}\([^{]*\){% endblock %}.*/\1/p' | sed 's/^ *//;s/ *$//')
        if [[ -n "$text" ]] && [[ ${#text} -gt 2 ]]; then
            echo "BLOCK_TITLE|$file|$line_num|$text" >> "$RESULTS_FILE"
        fi
    done
done

# === PATTERN 6: Hardcoded text in heading tags (h1-h6) ===
echo "Checking for hardcoded heading text..."
for file in $(find "$TEMPLATES_DIR" -name "*.pongo2" -type f); do
    grep -n '<h[1-6][^>]*>[A-Z][^<]*</h[1-6]>' "$file" 2>/dev/null | while read -r match; do
        # Skip if uses i18n
        if echo "$match" | grep -qE '\{\{[[:space:]]*t\('; then continue; fi
        line_num=$(echo "$match" | cut -d: -f1)
        # Extract text between heading tags
        text=$(echo "$match" | sed -n 's/.*<h[1-6][^>]*>\([^<]*\)<\/h[1-6]>.*/\1/p' | sed 's/^ *//;s/ *$//')
        if [[ -n "$text" ]] && [[ ${#text} -gt 2 ]] && [[ "$text" =~ [A-Za-z] ]]; then
            echo "HEADING|$file|$line_num|$text" >> "$RESULTS_FILE"
        fi
    done
done

# === PATTERN 7: Hardcoded button text ===
echo "Checking for hardcoded button text..."
for file in $(find "$TEMPLATES_DIR" -name "*.pongo2" -type f); do
    grep -n '<button[^>]*>[A-Z][^<]*</button>' "$file" 2>/dev/null | while read -r match; do
        # Skip if uses i18n
        if echo "$match" | grep -qE '\{\{[[:space:]]*t\('; then continue; fi
        # Skip if it contains icons or other elements
        if echo "$match" | grep -qE '<svg|<i |<span'; then continue; fi
        line_num=$(echo "$match" | cut -d: -f1)
        text=$(echo "$match" | sed -n 's/.*<button[^>]*>\([^<]*\)<\/button>.*/\1/p' | sed 's/^ *//;s/ *$//')
        if [[ -n "$text" ]] && [[ ${#text} -gt 2 ]] && [[ "$text" =~ [A-Za-z] ]]; then
            echo "BUTTON|$file|$line_num|$text" >> "$RESULTS_FILE"
        fi
    done
done

echo ""
echo "=========================================="
echo ""

# Process and display results
FOUND_ISSUES=0
if [[ -s "$RESULTS_FILE" ]]; then
    # Sort and deduplicate, then process
    sort -u "$RESULTS_FILE" | while IFS='|' read -r type file line_num text; do
        if [[ -z "$type" ]]; then continue; fi

        echo -e "${YELLOW}$file:$line_num${NC}"
        case "$type" in
            JS_ALERT)
                echo -e "  Type: JavaScript alert()"
                echo -e "  Text: ${RED}$text${NC}"
                ;;
            JS_CONFIRM)
                echo -e "  Type: JavaScript confirm()"
                echo -e "  Text: ${RED}$text${NC}"
                ;;
            PHRASE)
                echo -e "  Type: Hardcoded phrase"
                echo -e "  Text: ${RED}$text${NC}"
                ;;
            TITLE)
                echo -e "  Type: title attribute"
                echo -e "  Text: ${RED}$text${NC}"
                ;;
            PLACEHOLDER)
                echo -e "  Type: placeholder attribute"
                echo -e "  Text: ${RED}$text${NC}"
                ;;
            BLOCK_TITLE)
                echo -e "  Type: {% block title %}"
                echo -e "  Text: ${RED}$text${NC}"
                ;;
            HEADING)
                echo -e "  Type: Heading tag (h1-h6)"
                echo -e "  Text: ${RED}$text${NC}"
                ;;
            BUTTON)
                echo -e "  Type: Button text"
                echo -e "  Text: ${RED}$text${NC}"
                ;;
        esac

        if $FIX; then
            # Convert to lowercase, replace spaces with underscores for key suggestion
            suggested_key=$(echo "$text" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9]/_/g' | sed 's/__*/_/g' | sed 's/^_//' | sed 's/_$//')
            echo -e "  ${GREEN}Suggestion: {{ t(\"...$suggested_key\")|default:\"$text\" }}${NC}"
        fi
        echo ""
    done

    # Count issues
    FOUND_ISSUES=$(sort -u "$RESULTS_FILE" | grep -c . || echo 0)
fi

if [[ $FOUND_ISSUES -eq 0 ]]; then
    echo -e "${GREEN}No hardcoded UI text found.${NC}"
    exit 0
else
    echo -e "${YELLOW}Found $FOUND_ISSUES potential hardcoded text issue(s).${NC}"
    echo ""
    echo "To see suggested fixes, run:"
    echo "  $0 --fix"
    echo ""

    if $STRICT; then
        echo -e "${RED}Failing due to --strict mode.${NC}"
        exit 1
    fi

    exit 0
fi
