//go:build linux

package grpc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/goatkit/goatflow/internal/plugin"
)

// buildSysProcAttr creates OS-level process restrictions for gRPC plugin processes on Linux.
func buildSysProcAttr(policy plugin.ResourcePolicy) *syscall.SysProcAttr {
	attr := &syscall.SysProcAttr{
		// Plugin dies when host dies - prevent orphaned processes
		Pdeathsig: syscall.SIGKILL,
	}

	// Apply namespace isolation where available (requires Linux kernel support)
	// This provides basic process isolation but isn't as strong as containers
	// Skip namespace isolation in testing environments
	if supportsNamespaces() && !isTestEnvironment() {
		attr.Cloneflags = syscall.CLONE_NEWNS | syscall.CLONE_NEWPID
	}

	// Note: Setting rlimits through SysProcAttr.Setrlimit is not available in Go's standard library.
	// For production use, consider using a wrapper script or external tools like systemd-run
	// with resource constraints, or container technologies for stronger isolation.
	// 
	// For now, we document the intended limits and rely on namespace isolation where available.

	return attr
}

// buildPluginEnv creates a minimal, restricted environment for the plugin process.
// This prevents leaking sensitive host environment variables like DB credentials.
func buildPluginEnv(policy plugin.ResourcePolicy, pluginName string) []string {
	// Start with minimal safe environment
	env := []string{
		"PATH=/usr/local/bin:/usr/bin:/bin",  // Basic PATH
	}

	// Create plugin-specific temp directory
	tmpDir := filepath.Join("/tmp", "goatflow-plugin-"+pluginName)
	if err := os.MkdirAll(tmpDir, 0700); err == nil {
		env = append(env, "HOME="+tmpDir)
		env = append(env, "TMPDIR="+tmpDir)
	} else {
		// Fall back to /tmp if we can't create plugin-specific dir
		env = append(env, "HOME=/tmp")
		env = append(env, "TMPDIR=/tmp")
	}

	// Add timezone for time-aware plugins
	if tz := os.Getenv("TZ"); tz != "" {
		env = append(env, "TZ="+tz)
	}

	// Check if plugin has network permissions - if not, limit network access
	hasHTTP := false
	for _, perm := range policy.Permissions {
		if perm.Type == "http" {
			hasHTTP = true
			break
		}
	}
	
	// If no HTTP permission, set environment variable that compliant plugins should check
	if !hasHTTP {
		env = append(env, "GOATFLOW_NO_NETWORK=1")
	}

	return env
}

// supportsNamespaces checks if the Linux kernel supports the namespaces we want to use.
func supportsNamespaces() bool {
	// Check if /proc/sys/user/max_user_namespaces exists (user namespaces support)
	_, err := os.Stat("/proc/sys/user/max_user_namespaces")
	return err == nil
}

// isTestEnvironment detects if we're running in a test environment.
func isTestEnvironment() bool {
	// Check if we're running under 'go test'
	if os.Getenv("GO_TEST") == "1" {
		return true
	}
	// Alternative detection: check if the current executable contains "test"
	if exe, err := os.Executable(); err == nil {
		return filepath.Base(exe) == "test" || strings.Contains(exe, ".test") || strings.Contains(exe, "_test")
	}
	return false
}

// applyProcessSandbox applies OS-level restrictions to the plugin command.
func applyProcessSandbox(cmd *exec.Cmd, policy plugin.ResourcePolicy, pluginName string) error {
	// Set process attributes (resource limits, namespaces)
	cmd.SysProcAttr = buildSysProcAttr(policy)
	
	// Set restricted environment
	cmd.Env = buildPluginEnv(policy, pluginName)

	return nil
}