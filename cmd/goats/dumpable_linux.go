//go:build linux

package main

import (
	"log"

	"golang.org/x/sys/unix"
)

// closeProcFiles marks the process non-dumpable, so its /proc entries
// (environ, mem, maps, fd) belong to root and other processes running as the
// same user, such as plugin processes, cannot read its environment or memory.
func closeProcFiles() {
	if err := unix.Prctl(unix.PR_SET_DUMPABLE, 0, 0, 0, 0); err != nil {
		log.Printf("WARNING: could not mark the process non-dumpable: %v", err)
	}
}
