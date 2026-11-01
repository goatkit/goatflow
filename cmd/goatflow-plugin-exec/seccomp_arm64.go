//go:build linux && arm64

package main

import "golang.org/x/sys/unix"

const (
	auditArch = unix.AUDIT_ARCH_AARCH64
	denyX32   = false
)
