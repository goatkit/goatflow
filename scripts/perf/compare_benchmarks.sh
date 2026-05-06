#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 2 ]; then
	echo "Usage: scripts/perf/compare_benchmarks.sh BASELINE CANDIDATE" >&2
	exit 2
fi

baseline="$1"
candidate="$2"

if [ ! -f "$baseline" ]; then
	echo "Baseline benchmark file not found: $baseline" >&2
	exit 2
fi

if [ ! -f "$candidate" ]; then
	echo "Candidate benchmark file not found: $candidate" >&2
	exit 2
fi

if command -v benchstat >/dev/null 2>&1; then
	benchstat "$baseline" "$candidate"
else
	go run golang.org/x/perf/cmd/benchstat@latest "$baseline" "$candidate"
fi
