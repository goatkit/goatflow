//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/goatkit/goatflow/internal/platform/landlock"
	"golang.org/x/sys/unix"
)

// systemReadPaths are what a plugin may read and execute besides its own
// directory: libraries, tools, certificates and name resolution, and the
// cgroup and CPU files some libraries size themselves from. /proc is not
// among them: other processes stay invisible (no listing, command lines or
// status). The Go runtime does not need /proc, as GOMAXPROCS and GOMEMLIMIT
// come from the environment; os.Executable fails in a plugin.
var systemReadPaths = []string{
	"/usr", "/lib", "/lib64", "/bin", "/sbin", "/etc",
	"/sys/fs/cgroup", "/sys/devices/system/cpu", "/dev/urandom", "/dev/random",
}

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
	if err := sandbox(l, path); err != nil {
		return fmt.Errorf("sandbox: %w", err)
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

// sandbox confines the plugin with Landlock: read and execute its own
// directory and the system paths, write only l.writeDir and /dev/null.
// Without Landlock it warns and leaves the plugin unconfined.
func sandbox(l limits, plugin string) error {
	abi := landlock.ABI()
	if abi < landlock.FilesABI {
		fmt.Fprintf(os.Stderr, "goatflow-plugin-exec: warning: Landlock is not available, the plugin runs without the file and process sandbox; it needs Linux %s or later with Landlock enabled\n", landlock.FullKernel)
		return nil
	}
	return landlock.Sandbox{
		ReadExec:  append(systemReadPaths, filepath.Dir(plugin)),
		ReadWrite: []string{l.writeDir, "/dev/null"},
		DenyTCP:   l.denyNetwork,
	}.Restrict(abi)
}
