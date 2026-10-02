#!/usr/bin/env bash
set -euo pipefail

BENCH_COUNT="${BENCH_COUNT:-3}"
BENCH_TIME="${BENCH_TIME:-1s}"
# Defaults: every benchmark in every package that defines one (derived from the
# tracked *_test.go files, so the list cannot go stale).
BENCH_REGEX="${BENCH_REGEX:-.}"
BENCH_OUT="${BENCH_OUT:-generated/benchmarks/go-$(date -u +%Y%m%dT%H%M%SZ).txt}"
if [ -z "${BENCH_PACKAGES:-}" ]; then
	BENCH_PACKAGES=$(git grep -lE '^func Benchmark' -- '*_test.go' | xargs -rn1 dirname | sort -u | sed 's|^|./|' | tr '\n' ' ')
fi

mkdir -p "$(dirname "$BENCH_OUT")"

read -r -a packages <<<"$BENCH_PACKAGES"

{
	echo "# GoatFlow Go benchmark baseline"
	echo "# generated_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
	echo "# git_commit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
	echo "# go_version=$(go version)"
	echo "# bench_count=$BENCH_COUNT"
	echo "# bench_time=$BENCH_TIME"
	echo "# bench_regex=$BENCH_REGEX"
	echo "# bench_packages=$BENCH_PACKAGES"
	echo
	go test \
		-run '^$' \
		-bench "$BENCH_REGEX" \
		-benchmem \
		-benchtime "$BENCH_TIME" \
		-count "$BENCH_COUNT" \
		"${packages[@]}"
} | tee "$BENCH_OUT"

echo
echo "Benchmark results written to $BENCH_OUT"
