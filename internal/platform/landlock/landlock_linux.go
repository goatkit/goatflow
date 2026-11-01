//go:build linux

// Package landlock confines a process with the Linux Landlock LSM, which
// needs no privileges: it works for a non-root user inside a standard
// container, where namespaces are refused.
package landlock

import (
	"errors"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	fsReadExec = unix.LANDLOCK_ACCESS_FS_EXECUTE | unix.LANDLOCK_ACCESS_FS_READ_FILE | unix.LANDLOCK_ACCESS_FS_READ_DIR
	// fsFileRights are the rights that apply to a file, not a directory.
	fsFileRights = unix.LANDLOCK_ACCESS_FS_EXECUTE | unix.LANDLOCK_ACCESS_FS_WRITE_FILE |
		unix.LANDLOCK_ACCESS_FS_READ_FILE | unix.LANDLOCK_ACCESS_FS_TRUNCATE | unix.LANDLOCK_ACCESS_FS_IOCTL_DEV
)

// ABI returns the kernel's Landlock ABI version, or 0 when Landlock is not
// available (kernel older than 5.13, Landlock not enabled, or the call
// blocked by a seccomp profile).
func ABI() int {
	v, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, 0, 0, unix.LANDLOCK_CREATE_RULESET_VERSION)
	if errno != 0 {
		return 0
	}
	return int(v)
}

// fsRights is every filesystem right the given ABI version handles.
func fsRights(abi int) uint64 {
	rights := uint64(1)<<13 - 1 // ABI 1: execute .. make_sym
	if abi >= 2 {
		rights |= unix.LANDLOCK_ACCESS_FS_REFER
	}
	if abi >= 3 {
		rights |= unix.LANDLOCK_ACCESS_FS_TRUNCATE
	}
	if abi >= 5 {
		rights |= unix.LANDLOCK_ACCESS_FS_IOCTL_DEV
	}
	return rights
}

// Restrict confines the calling thread and everything it later executes:
// files beneath s.ReadExec can be read and executed, files beneath
// s.ReadWrite can be used fully, and nothing else on the filesystem is
// reachable. The thread can no longer read the memory or environment of
// processes outside the sandbox and, from ABI 6, cannot signal them. With
// s.DenyTCP (ABI 4 and later) it cannot bind or connect TCP sockets.
// Paths that do not exist are skipped. The caller must stay on this thread
// (runtime.LockOSThread) until it execs.
func (s Sandbox) Restrict(abi int) error {
	if abi < FilesABI {
		return errors.New("landlock is not available")
	}
	attr := unix.LandlockRulesetAttr{Handled_access_fs: fsRights(abi)}
	if s.DenyTCP && abi >= TCPABI {
		attr.Handled_access_net = unix.LANDLOCK_ACCESS_NET_BIND_TCP | unix.LANDLOCK_ACCESS_NET_CONNECT_TCP
	}
	if abi >= SignalsABI {
		attr.Scoped = unix.LANDLOCK_SCOPE_SIGNAL | unix.LANDLOCK_SCOPE_ABSTRACT_UNIX_SOCKET
	}
	fd, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET,
		uintptr(unsafe.Pointer(&attr)), unsafe.Sizeof(attr), 0) // #nosec G103 -- landlock_create_ruleset(2) takes a pointer to the attributes
	if errno != 0 {
		return fmt.Errorf("create ruleset: %w", errno)
	}
	ruleset := int(fd)
	defer unix.Close(ruleset)

	for _, p := range s.ReadExec {
		if err := allow(ruleset, p, fsReadExec); err != nil {
			return err
		}
	}
	for _, p := range s.ReadWrite {
		if err := allow(ruleset, p, attr.Handled_access_fs); err != nil {
			return err
		}
	}
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return fmt.Errorf("set no_new_privs: %w", err)
	}
	if _, _, errno := unix.Syscall(unix.SYS_LANDLOCK_RESTRICT_SELF, uintptr(ruleset), 0, 0); errno != 0 {
		return fmt.Errorf("restrict self: %w", errno)
	}
	return nil
}

// allow adds a rule granting access beneath path; a file gets only the
// rights that apply to files.
func allow(ruleset int, path string, access uint64) error {
	fi, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("sandbox path %s: %w", path, err)
	}
	if !fi.IsDir() {
		access &= fsFileRights
	}
	pfd, err := unix.Open(path, unix.O_PATH|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("open sandbox path %s: %w", path, err)
	}
	defer unix.Close(pfd)
	rule := unix.LandlockPathBeneathAttr{Allowed_access: access, Parent_fd: int32(pfd)} // #nosec G115 -- file descriptors fit in int32
	if _, _, errno := unix.Syscall6(unix.SYS_LANDLOCK_ADD_RULE, uintptr(ruleset),
		unix.LANDLOCK_RULE_PATH_BENEATH, uintptr(unsafe.Pointer(&rule)), 0, 0, 0); errno != 0 { // #nosec G103 -- landlock_add_rule(2) takes a pointer to the rule
		return fmt.Errorf("allow %s: %w", path, errno)
	}
	return nil
}
