//go:build linux

package grpc

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/goatkit/goatflow/internal/platform/plugin"
)

const (
	cgroupRoot = "/sys/fs/cgroup"
	// hostLeaf is the child cgroup goatflow moves its own cgroup's processes
	// into, so that cgroup may enable controllers for plugin cgroups.
	hostLeaf     = "goatflow-host"
	cpuMaxPeriod = 100000 // cpu.max period in µs
)

// pluginCgroup is the cgroup v2 a plugin process is started in, with hard
// cpu.max and memory.max limits.
type pluginCgroup struct {
	dir  string
	fd   int // open directory, for CLONE_INTO_CGROUP
	once sync.Once
}

var (
	cgroupsOnce sync.Once
	cgroupsBase string // parent of plugin cgroups; "" = unavailable
	cgroupSeq   atomic.Uint64
)

// pluginCgroupBase returns the cgroup under which plugin cgroups are created,
// or "" when hard limits are unavailable. Detection runs once; launcher
// starts a throwaway process to prove a child can be cloned into a cgroup.
func pluginCgroupBase(launcher string) string {
	cgroupsOnce.Do(func() {
		base, err := delegatedCgroup(launcher)
		if err != nil {
			log.Printf("plugin cgroups unavailable, plugins get launcher limits only: %v", err)
			return
		}
		cgroupsBase = base
		log.Printf("plugin cgroups: hard CPU and memory limits under %s", base)
	})
	return cgroupsBase
}

// delegatedCgroup checks this process may manage cgroup v2 children with the
// cpu and memory controllers and prepares its cgroup for them.
func delegatedCgroup(launcher string) (string, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(cgroupRoot, &st); err != nil {
		return "", err
	}
	if st.Type != unix.CGROUP2_SUPER_MAGIC {
		return "", fmt.Errorf("%s is not a cgroup v2 mount", cgroupRoot)
	}
	own, err := ownCgroup()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cgroupRoot, own)
	if _, err := os.Stat(filepath.Join(dir, "memory.max")); err != nil {
		return "", fmt.Errorf("goatflow runs in the root cgroup or memory is not delegated (%s)", dir)
	}
	base, err := prepareBase(dir)
	if err != nil {
		return "", err
	}
	if err := probeCloneIntoCgroup(base, launcher); err != nil {
		return "", fmt.Errorf("starting a process in a cgroup: %w", err)
	}
	return base, nil
}

// ownCgroup is this process's cgroup v2 path.
func ownCgroup() (string, error) {
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if path, ok := strings.CutPrefix(line, "0::"); ok && strings.HasPrefix(path, "/") {
			return path, nil
		}
	}
	return "", errors.New("no cgroup v2 entry in /proc/self/cgroup")
}

// prepareBase makes dir able to host plugin cgroups: a cgroup with processes
// cannot enable controllers for children, so its processes move into the
// hostLeaf child first. When this process already sits in a hostLeaf (an
// earlier goatflow process prepared the parent) the parent is used.
func prepareBase(dir string) (string, error) {
	base := dir
	if filepath.Base(dir) == hostLeaf {
		base = filepath.Dir(dir)
	} else {
		if err := moveProcesses(dir, filepath.Join(dir, hostLeaf)); err != nil {
			return "", err
		}
	}
	controllers, err := os.ReadFile(filepath.Join(base, "cgroup.controllers")) // #nosec G304 -- cgroupfs path from /proc/self/cgroup
	if err != nil {
		return "", err
	}
	have := strings.Fields(string(controllers))
	for _, c := range []string{"cpu", "memory"} {
		if !slices.Contains(have, c) {
			return "", fmt.Errorf("controller %s not available in %s", c, base)
		}
	}
	if err := writeCgroupFile(base, "cgroup.subtree_control", "+cpu +memory"); err != nil {
		return "", err
	}
	return base, nil
}

func moveProcesses(from, to string) error {
	if err := os.Mkdir(to, 0o755); err != nil && !errors.Is(err, fs.ErrExist) { // #nosec G301 -- cgroup directory
		return err
	}
	procs, err := os.ReadFile(filepath.Join(from, "cgroup.procs")) // #nosec G304 -- cgroupfs path from /proc/self/cgroup
	if err != nil {
		return err
	}
	for _, pid := range strings.Fields(string(procs)) {
		if err := writeCgroupFile(to, "cgroup.procs", pid); err != nil && !errors.Is(err, unix.ESRCH) {
			return fmt.Errorf("move process %s to %s: %w", pid, to, err)
		}
	}
	return nil
}

// probeCloneIntoCgroup starts the launcher (which exits at once without
// arguments) in a throwaway cgroup under base.
func probeCloneIntoCgroup(base, launcher string) error {
	cg, err := createCgroup(filepath.Join(base, "probe-"+strconv.Itoa(os.Getpid())))
	if err != nil {
		return err
	}
	defer cg.remove()
	cmd := exec.Command(launcher) // #nosec G204 -- the host's own launcher binary
	cmd.SysProcAttr = &syscall.SysProcAttr{UseCgroupFD: true, CgroupFD: cg.fd}
	if err := cmd.Start(); err != nil {
		return err
	}
	_ = cmd.Wait() // it exits non-zero by design
	return nil
}

// newPluginCgroup creates the cgroup for one start of the named plugin with
// limits as hard caps, or returns nil when hard limits are unavailable.
func newPluginCgroup(name string, limits plugin.LaunchLimits, launcher string) *pluginCgroup {
	base := pluginCgroupBase(launcher)
	if base == "" {
		return nil
	}
	dir := filepath.Join(base, fmt.Sprintf("plugin-%s.%d", name, cgroupSeq.Add(1)))
	cg, err := createCgroup(dir)
	if err == nil {
		err = cg.limit(limits)
		if err != nil {
			cg.remove()
		}
	}
	if err != nil {
		log.Printf("plugin %s: no cgroup, launcher limits only: %v", name, err)
		return nil
	}
	return cg
}

func createCgroup(dir string) (*pluginCgroup, error) {
	if err := os.Mkdir(dir, 0o755); err != nil { // #nosec G301 -- cgroup directory
		return nil, err
	}
	fd, err := unix.Open(dir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		_ = unix.Rmdir(dir)
		return nil, err
	}
	return &pluginCgroup{dir: dir, fd: fd}, nil
}

func (cg *pluginCgroup) limit(l plugin.LaunchLimits) error {
	quota := int64(l.CPUMillis) * cpuMaxPeriod / 1000
	if err := writeCgroupFile(cg.dir, "cpu.max", fmt.Sprintf("%d %d", quota, cpuMaxPeriod)); err != nil {
		return err
	}
	return writeCgroupFile(cg.dir, "memory.max", strconv.FormatInt(int64(l.MemoryMB)<<20, 10))
}

// remove deletes the cgroup once its process has gone; safe to call twice
// and on nil.
func (cg *pluginCgroup) remove() {
	if cg == nil {
		return
	}
	cg.once.Do(func() {
		_ = unix.Close(cg.fd)
		// The exited process can take a moment to leave the cgroup.
		for range 50 {
			err := unix.Rmdir(cg.dir)
			if err == nil || errors.Is(err, unix.ENOENT) {
				return
			}
			if !errors.Is(err, unix.EBUSY) {
				log.Printf("remove plugin cgroup %s: %v", cg.dir, err)
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		log.Printf("remove plugin cgroup %s: still in use", cg.dir)
	})
}

func writeCgroupFile(dir, name, value string) error {
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY, 0) // #nosec G304 -- fixed cgroupfs file names
	if err != nil {
		return err
	}
	if _, err := f.WriteString(value); err != nil {
		_ = f.Close()
		return fmt.Errorf("write %s to %s: %w", value, f.Name(), err)
	}
	return f.Close()
}
