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
# Packages are listed from the top-level directories a clean checkout has:
# git-ignored ones (tmp/, node_modules/, storage/, ...) are never walked.
# tmp/ is the toolbox TMPDIR, so it holds the go-build* work dirs of
# concurrent or killed `go test` runs; one broken or unreadable package
# under a `./...` walk makes `go list` print nothing at all.
#
# Every phase runs; the script exits non-zero if any phase failed, and
# stops before testing if the package set cannot be worked out.
#
# Args: extra `go test` flags, e.g. -count=1 (test-unit); omitted for
#       test-fast so Go's result cache applies.

set -u
export PATH=/usr/local/go/bin:$PATH
EXTRA_FLAGS="$*"

CORE_EXCLUDE='tests/e2e|tests/integration|internal/email/integration|internal/platform/template'
DB_SYMBOLS='database\.(GetDB|InitTestDB|SetDB|ResetDB|CloseTestDB)'

die() {
	echo "unit-test-phases: $*" >&2
	exit 1
}

status=0
FAILED=""
fail() {
	status=1
	FAILED="$FAILED $1"
}
finish() {
	[ "$status" -eq 0 ] || echo "unit-test-phases: FAILED:$FAILED" >&2
	exit "$status"
}

# Both the package walk and the DB classification read the git work tree.
git rev-parse --is-inside-work-tree >/dev/null 2>&1 || die "not inside a git work tree"
MODULE=$(go list -m) || die "go list -m failed"

echo "Running template tests..."
go test -timeout=1m -buildvcs=false -v -p "$(nproc)" ./internal/platform/template/... $EXTRA_FLAGS || fail templates

# `./...` skips dot, underscore and testdata directories; so do the roots.
ROOTS=()
for d in */; do
	d=${d%/}
	case $d in _* | testdata) continue ;; esac
	git check-ignore -q -- "$d" && continue
	ROOTS+=("./$d/...")
done
compgen -G '*.go' >/dev/null && ROOTS+=(.)
[ ${#ROOTS[@]} -gt 0 ] || die "no package directories found"

# -e keeps a package that fails to load in the list, so `go test` fails it
# instead of the whole listing coming back empty.
LISTED=$(go list -e "${ROOTS[@]}") || die "go list failed"
while read -r p; do
	case $p in
	"$MODULE" | "$MODULE"/*) ;;
	*) die "go list returned '$p', not a package of $MODULE (unreadable directory?)" ;;
	esac
done <<<"$LISTED"
CORE_PKGS=$(echo "$LISTED" | grep -Ev "$CORE_EXCLUDE")
[ -n "$CORE_PKGS" ] || die "no core packages to test"

# Directories whose test files reference the shared DB globals (git grep
# exits 1 when nothing matches).
DB_FILES=$(git grep -lE "$DB_SYMBOLS" -- '*_test.go')
[ $? -le 1 ] || die "git grep failed"
DB_DIRS=$(echo "$DB_FILES" | xargs -rn1 dirname | sort -u | sed 's|^|./|')

if [ -z "$DB_DIRS" ]; then
	echo "Running core packages"
	go test -timeout=15m -buildvcs=false -v -p "$(nproc)" $CORE_PKGS $EXTRA_FLAGS || fail core
	finish
fi

# Import paths of DB packages that build in the default context (per-dir so
# one tag-gated directory cannot fail the whole listing).
DB_PKGS=""
for d in $DB_DIRS; do
	p=$(go list "$d" 2>/dev/null)
	[ -n "$p" ] && DB_PKGS="$DB_PKGS $p"
done
DB_PKGS=$(echo $DB_PKGS | tr ' ' '\n' | grep -Fx -f <(echo "$CORE_PKGS" | tr ' ' '\n' | sort -u) | sort -u | tr '\n' ' ')

NON_DB_PKGS="$CORE_PKGS"
if [ -n "$DB_PKGS" ]; then
	NON_DB_PKGS=$(echo "$CORE_PKGS" | tr ' ' '\n' | grep -Fxv -f <(echo "$DB_PKGS" | tr ' ' '\n' | sort -u) | tr '\n' ' ')
fi

if [ -n "$NON_DB_PKGS" ]; then
	echo "Running non-DB core packages (parallel)"
	go test -timeout=15m -buildvcs=false -v -p "$(nproc)" $NON_DB_PKGS $EXTRA_FLAGS || fail non-db
fi

if [ -n "$DB_PKGS" ]; then
	echo "Running DB core packages (serialized -p 1: shared test DB)"
	go test -timeout=15m -buildvcs=false -v -p 1 $DB_PKGS $EXTRA_FLAGS || fail db
fi

finish
