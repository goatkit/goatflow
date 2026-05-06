#!/usr/bin/env bash
set -euo pipefail

BENCH_COUNT="${BENCH_COUNT:-3}"
BENCH_TIME="${BENCH_TIME:-1s}"
BENCH_REGEX="${BENCH_REGEX:-Benchmark(Sanitize|StripHTML|GetConfig|GetDSN|IsProduction|IsBusinessDay|IsWithinBusinessHours|SetPassword|CheckPassword|RecordRequest|GetStats|ValidateResponse|ValidateJSONSchema|Routing|TemplateLoading|DashboardPage|LinkChecker|LDAPService_ValidateConfig|LDAPService_GetUserAttributes)}"
BENCH_OUT="${BENCH_OUT:-generated/benchmarks/go-$(date -u +%Y%m%dT%H%M%SZ).txt}"
BENCH_PACKAGES="${BENCH_PACKAGES:-./internal/utils ./internal/config ./internal/models ./internal/routing ./internal/middleware ./internal/api ./internal/service}"

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
