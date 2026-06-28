#!/bin/bash
# Activate tracked git hooks for GoatFlow
#
# Hooks live in .githooks/ (tracked in the repo) and are activated by setting
# core.hooksPath. This means hooks are available immediately after cloning —
# you just need to run this script once (or `make setup-hooks`).
#
# After cloning:
#   make setup-hooks
# or:
#   git config core.hooksPath .githooks

set -e

if [ ! -d ".git" ]; then
    echo "Error: Not in a git repository root" >&2
    exit 1
fi

git config core.hooksPath .githooks

echo "✅ Git hooks activated (.githooks/)"
echo ""
echo "Active hooks:"
echo "  - pre-commit: Secret scanning (gitleaks), binary file prevention,"
echo "    large file warnings, attribution blocking, SQL guard"
echo ""
echo "To bypass (use with caution): git commit --no-verify"
echo "To deactivate: git config --unset core.hooksPath"