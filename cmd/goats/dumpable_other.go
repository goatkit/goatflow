//go:build !linux

package main

// closeProcFiles has nothing to do off Linux.
func closeProcFiles() {}
