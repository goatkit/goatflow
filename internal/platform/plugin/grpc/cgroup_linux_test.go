//go:build linux

package grpc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goatkit/goatflow/internal/platform/plugin"
	"github.com/goatkit/goatflow/internal/platform/plugin/grpc/grpctest"
)

// TestPluginRunsInItsOwnLimitedCgroup needs a delegated, writable cgroup v2
// (e.g. a privileged container with a private cgroup namespace).
func TestPluginRunsInItsOwnLimitedCgroup(t *testing.T) {
	bin := grpctest.BuildPlugin(t, "./internal/platform/plugin/grpc/testdata/hostcaller", "hostcaller")
	launcher, err := launcherPath()
	if err != nil {
		t.Fatal(err)
	}
	if pluginCgroupBase(launcher) == "" {
		t.Skip("no delegated cgroup v2: plugins get launcher limits only")
	}

	policy := plugin.ResourcePolicy{PluginName: "hostcaller", Status: "approved", MemoryMB: 300, CPUMillis: 1500}
	p, err := LoadGRPCPlugin(bin, "hostcaller", policy)
	if err != nil {
		t.Fatalf("LoadGRPCPlugin: %v", err)
	}
	if !p.CgroupEnforced() {
		t.Fatal("plugin started without a cgroup although cgroups are available")
	}
	dir := p.cgroup.dir

	procCgroup, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", p.client.ReattachConfig().Pid))
	if err != nil {
		t.Fatal(err)
	}
	if got := filepath.Join(cgroupRoot, strings.TrimPrefix(strings.TrimSpace(string(procCgroup)), "0::")); got != dir {
		t.Errorf("plugin process is in %s, want %s", got, dir)
	}
	for file, want := range map[string]string{"cpu.max": "150000 100000", "memory.max": "314572800"} {
		got, err := os.ReadFile(filepath.Join(dir, file))
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(string(got)) != want {
			t.Errorf("%s = %q, want %q", file, strings.TrimSpace(string(got)), want)
		}
	}

	_ = p.Shutdown(context.Background())
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("plugin cgroup %s left behind after shutdown (stat err %v)", dir, err)
	}
}
