//go:build linux && !amd64 && !arm64

package main

import (
	"fmt"
	"runtime"
)

func denyNetwork() error {
	return fmt.Errorf("no seccomp filter for %s", runtime.GOARCH)
}
