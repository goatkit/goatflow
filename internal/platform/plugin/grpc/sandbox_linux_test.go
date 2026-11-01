//go:build linux

package grpc

import (
	"os"
	"strings"
	"testing"

	"github.com/goatkit/goatflow/internal/platform/plugin"
)

func TestBuildPluginEnv_PrefixStripping(t *testing.T) {
	// Set a GOATFLOW_PLUGIN_ var and verify it appears both with and without prefix.
	os.Setenv("GOATFLOW_PLUGIN_ADB_SERVER_HOST", "host.docker.internal")
	defer os.Unsetenv("GOATFLOW_PLUGIN_ADB_SERVER_HOST")

	policy := plugin.ResourcePolicy{Status: "approved"}
	env := buildPluginEnv(policy, "testplugin")

	var hasPrefixed, hasStripped bool
	for _, kv := range env {
		if kv == "GOATFLOW_PLUGIN_ADB_SERVER_HOST=host.docker.internal" {
			hasPrefixed = true
		}
		if kv == "ADB_SERVER_HOST=host.docker.internal" {
			hasStripped = true
		}
	}

	if !hasPrefixed {
		t.Error("expected GOATFLOW_PLUGIN_ADB_SERVER_HOST in env")
	}
	if !hasStripped {
		t.Error("expected stripped ADB_SERVER_HOST in env")
	}
}

func TestBuildPluginEnv_BasicEntries(t *testing.T) {
	policy := plugin.ResourcePolicy{Status: "approved"}
	env := buildPluginEnv(policy, "test-plugin")

	lookup := make(map[string]string)
	for _, kv := range env {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			lookup[parts[0]] = parts[1]
		}
	}

	if _, ok := lookup["PATH"]; !ok {
		t.Error("expected PATH in env")
	}
	if _, ok := lookup["HOME"]; !ok {
		t.Error("expected HOME in env")
	}
	if _, ok := lookup["TMPDIR"]; !ok {
		t.Error("expected TMPDIR in env")
	}
}

func hasEnv(env []string, kv string) bool {
	for _, e := range env {
		if e == kv {
			return true
		}
	}
	return false
}

func TestBuildPluginEnv_NoNetwork(t *testing.T) {
	// The host-mediated http permission does not open the plugin's own network.
	policy := plugin.ResourcePolicy{
		Status:      "approved",
		Permissions: []plugin.Permission{{Type: "http", Access: "*"}},
	}
	if !hasEnv(buildPluginEnv(policy, "test-plugin"), "GOATFLOW_NO_NETWORK=1") {
		t.Error("expected GOATFLOW_NO_NETWORK=1 without network permission")
	}
}

func TestBuildPluginEnv_WithNetwork(t *testing.T) {
	policy := plugin.ResourcePolicy{
		Status:      "approved",
		Permissions: []plugin.Permission{{Type: plugin.PermissionNetwork}},
	}
	if hasEnv(buildPluginEnv(policy, "test-plugin"), "GOATFLOW_NO_NETWORK=1") {
		t.Error("GOATFLOW_NO_NETWORK should not be set with network permission")
	}
}

func TestBuildPluginEnv_LauncherSettingStaysWithHost(t *testing.T) {
	t.Setenv(launcherEnv, "/opt/goatflow/goatflow-plugin-exec")
	env := buildPluginEnv(plugin.ResourcePolicy{Status: "approved"}, "test-plugin")
	for _, kv := range env {
		if strings.Contains(kv, "goatflow-plugin-exec") {
			t.Errorf("launcher setting forwarded to the plugin: %s", kv)
		}
	}
}

func TestLauncherArgsCarryPolicyLimits(t *testing.T) {
	policy := plugin.ResourcePolicy{MemoryMB: 256, CPUMillis: 500}
	got := strings.Join(launcherArgs(policy.Launch(), "/app/plugins/x/x"), " ")
	if want := "--memory-mb 256 --cpu-millis 500 --deny-network -- /app/plugins/x/x"; got != want {
		t.Errorf("args = %q, want %q", got, want)
	}
	withNetwork := plugin.ResourcePolicy{Permissions: []plugin.Permission{{Type: plugin.PermissionNetwork}}}
	got = strings.Join(launcherArgs(withNetwork.Launch(), "/p"), " ")
	if want := "--memory-mb 512 --cpu-millis 1000 -- /p"; got != want {
		t.Errorf("args with defaults and network = %q, want %q", got, want)
	}
}

func TestSafePluginName(t *testing.T) {
	for _, name := range []string{"hostcaller", "goatkit-llm", "My_Plugin.v2"} {
		if !safePluginName(name) {
			t.Errorf("%q refused", name)
		}
	}
	for _, name := range []string{"", ".", "..", "../x", "a/b", "a b", ".hidden", strings.Repeat("a", 129)} {
		if safePluginName(name) {
			t.Errorf("%q accepted", name)
		}
	}
}
