package main

import "errors"

// errRunAsRoot is returned when goats is started as root: GoatFlow, its
// runner and its plugins always run as an unprivileged user.
var errRunAsRoot = errors.New("GoatFlow must not run as root (uid 0): run it as an unprivileged user, " +
	"as the container images and Helm chart do (uid 1000)")

// checkNotRoot refuses euid 0. Off Unix, os.Geteuid returns -1.
func checkNotRoot(euid int) error {
	if euid == 0 {
		return errRunAsRoot
	}
	return nil
}
