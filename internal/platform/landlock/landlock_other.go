//go:build !linux

package landlock

import "errors"

// ABI is always 0: Landlock is Linux-only.
func ABI() int { return 0 }

// Restrict always fails: Landlock is Linux-only.
func (s Sandbox) Restrict(int) error { return errors.New("landlock is Linux-only") }
