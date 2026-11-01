//go:build linux

package grpc

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/goatkit/goatflow/internal/platform/plugin"
)

const (
	// launcherName is the binary that applies a plugin's launch limits and
	// then execs it; it ships next to goats.
	launcherName = "goatflow-plugin-exec"
	// launcherEnv overrides where the launcher is found.
	launcherEnv = "GOATFLOW_PLUGIN_LAUNCHER"
)

// pluginCommand returns the command that starts the plugin at execPath
// through the launcher with policy's launch limits, and the cgroup it starts
// in (nil when hard cgroup limits are unavailable). It fails when the
// launcher cannot be found: plugins never start without their limits.
func pluginCommand(execPath, pluginName string, policy plugin.ResourcePolicy) (*exec.Cmd, *pluginCgroup, error) {
	if !safePluginName(pluginName) {
		return nil, nil, fmt.Errorf("plugin name %q is not usable as a path component", pluginName)
	}
	launcher, err := launcherPath()
	if err != nil {
		return nil, nil, err
	}
	limits := policy.Launch()

	// #nosec G204 -- launcher and plugin binary are chosen by the host, not by request input
	cmd := exec.Command(launcher, launcherArgs(limits, execPath)...)
	cmd.SysProcAttr = buildSysProcAttr()
	cmd.Env = buildPluginEnv(policy, pluginName)

	cg := newPluginCgroup(pluginName, limits, launcher)
	if cg != nil {
		cmd.SysProcAttr.UseCgroupFD = true
		cmd.SysProcAttr.CgroupFD = cg.fd
	}
	return cmd, cg, nil
}

// launcherArgs is the launcher's argv (after its own name) for the plugin.
func launcherArgs(limits plugin.LaunchLimits, execPath string) []string {
	args := make([]string, 0, 7)
	args = append(args,
		"--memory-mb", strconv.Itoa(limits.MemoryMB),
		"--cpu-millis", strconv.Itoa(limits.CPUMillis))
	if !limits.Network {
		args = append(args, "--deny-network")
	}
	return append(args, "--", execPath)
}

// launcherPath finds the launcher: $GOATFLOW_PLUGIN_LAUNCHER, else next to
// this executable, else on PATH.
func launcherPath() (string, error) {
	if p := os.Getenv(launcherEnv); p != "" {
		if err := checkExecutable(p); err != nil {
			return "", fmt.Errorf("plugin launcher %s=%s: %w", launcherEnv, p, err)
		}
		return p, nil
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), launcherName)
		if checkExecutable(p) == nil {
			return p, nil
		}
	}
	p, err := exec.LookPath(launcherName)
	if err != nil {
		return "", fmt.Errorf("plugin launcher %s not found next to the goatflow binary or on PATH (set %s): %w",
			launcherName, launcherEnv, err)
	}
	return p, nil
}

func checkExecutable(path string) error {
	fi, err := os.Stat(path) // #nosec G703 -- launcher path comes from host configuration
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() || fi.Mode().Perm()&0o111 == 0 {
		return errors.New("not an executable file")
	}
	return nil
}

// safePluginName reports whether name can be used as a single path component.
func safePluginName(name string) bool {
	if name == "" || len(name) > 128 || name[0] == '.' {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
		default:
			return false
		}
	}
	return true
}

// buildSysProcAttr creates OS-level process restrictions for gRPC plugin processes on Linux.
func buildSysProcAttr() *syscall.SysProcAttr {
	attr := &syscall.SysProcAttr{
		// Plugin dies when host dies - prevent orphaned processes
		Pdeathsig: syscall.SIGKILL,
	}

	// Apply namespace isolation where available and permitted (requires Linux
	// kernel support and privileges).
	if supportsNamespaces() {
		attr.Cloneflags = syscall.CLONE_NEWNS | syscall.CLONE_NEWPID
	}
	return attr
}

// buildPluginEnv creates a minimal, restricted environment for the plugin process.
// This prevents leaking sensitive host environment variables like DB credentials.
func buildPluginEnv(policy plugin.ResourcePolicy, pluginName string) []string {
	// Start with minimal safe environment
	env := []string{
		"PATH=/usr/local/bin:/usr/bin:/bin", // Basic PATH
	}

	// Create plugin-specific temp directory under the writable app temp dir.
	// /tmp is NOT world-writable in the alpine runtime image (mode 0755,
	// root-owned), so go-plugin cannot create its unix socket there and the
	// plugin exits before the handshake. /app/tmp is created and chowned to
	// appuser in the Dockerfile runtime-base stage.
	const pluginTmpBase = "/app/tmp"
	tmpDir := filepath.Join(pluginTmpBase, "goatflow-plugin-"+pluginName)
	if err := os.MkdirAll(tmpDir, 0700); err == nil {
		env = append(env, "HOME="+tmpDir)
		env = append(env, "TMPDIR="+tmpDir)
	} else {
		// Fall back to the writable app temp base (never /tmp — it is not writable).
		env = append(env, "HOME="+pluginTmpBase)
		env = append(env, "TMPDIR="+pluginTmpBase)
	}

	// Add timezone for time-aware plugins
	if tz := os.Getenv("TZ"); tz != "" {
		env = append(env, "TZ="+tz)
	}

	// Forward plugin-specific env vars (GOATFLOW_PLUGIN_* prefix).
	// Variables are forwarded as-is AND with the prefix stripped, so e.g.
	// GOATFLOW_PLUGIN_ADB_SERVER_HOST=localhost appears in the plugin
	// environment as both GOATFLOW_PLUGIN_ADB_SERVER_HOST and ADB_SERVER_HOST.
	// This lets plugins use standard env var names while keeping the
	// namespaced originals available for disambiguation. The launcher
	// setting is the host's, not the plugin's.
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "GOATFLOW_PLUGIN_") && !strings.HasPrefix(kv, launcherEnv+"=") {
			env = append(env, kv)
			// Strip prefix: GOATFLOW_PLUGIN_FOO=bar -> FOO=bar
			stripped := strings.TrimPrefix(kv, "GOATFLOW_PLUGIN_")
			env = append(env, stripped)
		}
	}

	// Tell the plugin its own network connections are refused (the launcher
	// enforces it).
	if !policy.Launch().Network {
		env = append(env, "GOATFLOW_NO_NETWORK=1")
	}

	return env
}

// supportsNamespaces checks if namespace isolation is available and permitted.
// Inside Docker containers, namespace creation is usually denied (EPERM)
// even though the kernel supports it.
func supportsNamespaces() bool {
	// Check kernel support
	if _, err := os.Stat("/proc/sys/user/max_user_namespaces"); err != nil {
		return false
	}

	// CLONE_NEWNS|CLONE_NEWPID without a user namespace needs CAP_SYS_ADMIN:
	// a non-root process gets EPERM and the plugin would never start.
	if os.Geteuid() != 0 {
		return false
	}

	// Detect container environment where namespace creation is typically denied.
	// Check /.dockerenv (Docker) or /run/.containerenv (Podman).
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return false
	}
	if _, err := os.Stat("/run/.containerenv"); err == nil {
		return false
	}

	// Check cgroup for container signatures
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		s := string(data)
		if strings.Contains(s, "docker") || strings.Contains(s, "containerd") ||
			strings.Contains(s, "kubepods") || strings.Contains(s, "lxc") {
			return false
		}
	}

	return true
}
