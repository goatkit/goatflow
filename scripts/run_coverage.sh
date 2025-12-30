#!/usr/bin/env bash
set -euo pipefail

mkdir -p generated

# Disable VCS stamping globally to avoid git safe directory issues in containers
export GOFLAGS="-buildvcs=false"

# Mark workspace as safe for git operations (needed in CI containers)
git config --global --add safe.directory /workspace 2>/dev/null || true

PKGS=$(go list ./... | grep -Ev '/tests/|/tools/test-utilities|/examples$|/internal/api($|/)|/internal/i18n$|/tmp$')
if [[ -z "${PKGS}" ]]; then
	echo "No packages selected for coverage" >&2
	exit 1
fi

go test -buildvcs=false -v -race -coverprofile=generated/coverage.out -covermode=atomic ${PKGS}
