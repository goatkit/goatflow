//go:build linux

package grpc

import (
	"os"
	"strings"
	"testing"

	"github.com/goatkit/goatflow/internal/plugin"
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

func TestBuildPluginEnv_NoNetwork(t *testing.T) {
	policy := plugin.ResourcePolicy{
		Status:      "approved",
		Permissions: []plugin.Permission{}, // No HTTP
	}
	env := buildPluginEnv(policy, "test-plugin")

	found := false
	for _, kv := range env {
		if kv == "GOATFLOW_NO_NETWORK=1" {
			found = true
		}
	}
	if !found {
		t.Error("expected GOATFLOW_NO_NETWORK=1 when no HTTP permission")
	}
}

func TestBuildPluginEnv_WithHTTP(t *testing.T) {
	policy := plugin.ResourcePolicy{
		Status: "approved",
		Permissions: []plugin.Permission{
			{Type: "http"},
		},
	}
	env := buildPluginEnv(policy, "test-plugin")

	for _, kv := range env {
		if kv == "GOATFLOW_NO_NETWORK=1" {
			t.Error("GOATFLOW_NO_NETWORK should not be set when HTTP permission exists")
		}
	}
}
