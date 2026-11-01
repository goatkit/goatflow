//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"golang.org/x/sys/unix"
)

// Niceness and the seccomp filter belong to the calling thread, so main stays
// on one thread from limit setup through exec.
func init() {
	runtime.LockOSThread()
}

func run(args []string) error {
	l, argv, err := parse(args)
	if err != nil {
		return err
	}
	path, err := exec.LookPath(argv[0])
	if err != nil {
		return fmt.Errorf("plugin binary: %w", err)
	}
	limit := uint64(l.memoryMB) << 20 // #nosec G115 -- memoryMB is checked positive
	if err := unix.Setrlimit(unix.RLIMIT_DATA, &unix.Rlimit{Cur: limit, Max: limit}); err != nil {
		return fmt.Errorf("set memory limit: %w", err)
	}
	if err := lowerPriority(); err != nil {
		return fmt.Errorf("lower priority: %w", err)
	}
	if l.denyNetwork {
		if err := denyNetwork(); err != nil {
			return fmt.Errorf("deny network: %w", err)
		}
	}
	// #nosec G204 -- the host chooses the plugin binary; running it is this command's purpose
	if err := unix.Exec(path, argv, childEnv(os.Environ(), l)); err != nil {
		return fmt.Errorf("exec %s: %w", path, err)
	}
	return nil
}

// lowerPriority raises this thread's nice value by 10, up to 19.
func lowerPriority() error {
	prio, err := unix.Getpriority(unix.PRIO_PROCESS, 0)
	if err != nil {
		return err
	}
	// The raw getpriority syscall returns 20 - nice.
	return unix.Setpriority(unix.PRIO_PROCESS, 0, min(20-prio+10, 19))
}
