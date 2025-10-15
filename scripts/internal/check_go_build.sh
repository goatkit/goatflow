#!/bin/bash
# Shell helper to compile goats inside toolbox without leaving host binaries.
set -euo pipefail

export PATH=/usr/local/go/bin:$PATH
export GOFLAGS=-buildvcs=false

workdir=$(pwd)
if [ "${workdir##*/}" != "workspace" ]; then
    cd /workspace
fi

tmp_bin=$(mktemp /tmp/goats-build-XXXXXX)
cleanup() { /bin/rm -f "$tmp_bin"; }
trap cleanup EXIT

go build -o "$tmp_bin" ./cmd/goats
