// Package grpctest builds gRPC plugin binaries and the plugin launcher for
// tests that start real plugin processes.
package grpctest

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

// LauncherEnv names the launcher the gRPC runtime starts plugins through.
const LauncherEnv = "GOATFLOW_PLUGIN_LAUNCHER"

var (
	launcherOnce sync.Once
	launcherDir  string
	launcherPath string
	launcherErr  error
)

// Run runs the tests, then removes the launcher UseLauncher built. Call it
// from TestMain: os.Exit(grpctest.Run(m)).
func Run(m *testing.M) int {
	code := m.Run()
	if launcherDir != "" {
		_ = os.RemoveAll(launcherDir)
	}
	return code
}

// UseLauncher builds cmd/goatflow-plugin-exec once per test binary and
// points LauncherEnv at it for the rest of t.
func UseLauncher(t testing.TB) {
	t.Helper()
	launcherOnce.Do(func() {
		launcherDir, launcherErr = os.MkdirTemp("", "goatflow-plugin-exec-")
		if launcherErr != nil {
			return
		}
		launcherPath = filepath.Join(launcherDir, "goatflow-plugin-exec")
		launcherErr = build(launcherPath, "./cmd/goatflow-plugin-exec")
	})
	if launcherErr != nil {
		t.Fatalf("build plugin launcher: %v", launcherErr)
	}
	t.Setenv(LauncherEnv, launcherPath)
}

// BuildPlugin builds the plugin main package pkg (relative to the repo root)
// into a temp dir, makes the launcher available (UseLauncher) and returns the
// plugin binary's path.
func BuildPlugin(t testing.TB, pkg, name string) string {
	t.Helper()
	UseLauncher(t)
	path := filepath.Join(t.TempDir(), name)
	if runtime.GOOS == "windows" {
		path += ".exe"
	}
	if err := build(path, pkg); err != nil {
		t.Fatalf("build plugin %s: %v", pkg, err)
	}
	return path
}

func build(out, pkg string) error {
	_, file, _, _ := runtime.Caller(0)
	cmd := exec.Command("go", "build", "-o", out, pkg) // #nosec G204 -- test-only build of repo packages
	cmd.Dir = filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..")
	cmd.Env = os.Environ()
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w\n%s", err, output)
	}
	return nil
}
