#!/bin/bash
# Unit test phases for the core packages.
#
# Packages whose tests mutate the shared test DB must not run concurrently:
# they reset/truncate seeded rows, so overlapping packages corrupt each
# other's fixtures (404 "Ticket not found", "no rows" flakes). This script
# splits the core packages into two phases:
#   1. non-DB packages in parallel (-p NPROC)
#   2. DB packages serialized (-p 1)
#
# The DB set is classified dynamically from test files, so a new
# DB-touching package is serialized automatically without touching this
# script. A directory whose test files are excluded by build constraints
# contributes no tests in the default build context and is left out of
# both phases' explicit sets (it stays in the parallel set, running zero
# tests).
#
# Args: extra `go test` flags, e.g. -count=1 (test-unit); omitted for
#       test-fast so Go's result cache applies.

set -u
export PATH=/usr/local/go/bin:$PATH
EXTRA_FLAGS="$*"

CORE_EXCLUDE='tests/e2e|tests/integration|internal/email/integration|internal/platform/template'
DB_SYMBOLS='database\.(GetDB|InitTestDB|SetDB|ResetDB|CloseTestDB)'

echo "Running template tests..."
go test -timeout=1m -buildvcs=false -v -p "$(nproc)" ./internal/platform/template/... $EXTRA_FLAGS || true

CORE_PKGS=$(go list ./... | rg -v "$CORE_EXCLUDE")

# Directories whose test files reference the shared DB globals.
DB_DIRS=$(git grep -lE "$DB_SYMBOLS" -- '*_test.go' | xargs -rn1 dirname | sort -u | sed 's|^|./|')

if [ -z "$DB_DIRS" ]; then
	echo "Running core packages"
	go test -timeout=15m -buildvcs=false -v -p "$(nproc)" $CORE_PKGS $EXTRA_FLAGS
	exit $?
fi

# Import paths of DB packages that build in the default context (per-dir so
# one tag-gated directory cannot fail the whole listing).
DB_PKGS=""
for d in $DB_DIRS; do
	p=$(go list "$d" 2>/dev/null)
	[ -n "$p" ] && DB_PKGS="$DB_PKGS $p"
done
DB_PKGS=$(echo $DB_PKGS | tr ' ' '\n' | grep -Fx -f <(echo "$CORE_PKGS" | tr ' ' '\n' | sort -u) | sort -u | tr '\n' ' ')

NON_DB_PKGS=""
if [ -n "$DB_PKGS" ]; then
	NON_DB_PKGS=$(echo "$CORE_PKGS" | tr ' ' '\n' | grep -Fxv -f <(echo "$DB_PKGS" | tr ' ' '\n' | sort -u) | tr '\n' ' ')
fi

if [ -n "$NON_DB_PKGS" ]; then
	echo "Running non-DB core packages (parallel)"
	go test -timeout=15m -buildvcs=false -v -p "$(nproc)" $NON_DB_PKGS $EXTRA_FLAGS
fi

if [ -n "$DB_PKGS" ]; then
	echo "Running DB core packages (serialized -p 1: shared test DB)"
	go test -timeout=15m -buildvcs=false -v -p 1 $DB_PKGS $EXTRA_FLAGS
fi
