//go:build !linux

package main

import (
	"fmt"
	"runtime"
)

func run([]string) error {
	return fmt.Errorf("plugin limits are not supported on %s", runtime.GOOS)
}
