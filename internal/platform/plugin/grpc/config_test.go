package grpc

import (
	"testing"

	"github.com/goatkit/goatflow/internal/platform/version"
)

// Plugins gate features on host_version, so it must be the running GoatFlow.
func TestBuildPluginConfigPassesHostVersion(t *testing.T) {
	t.Chdir(t.TempDir()) // buildPluginConfig creates data/plugins/<name>
	old := version.Version
	version.Version = "0.12.3"
	t.Cleanup(func() { version.Version = old })

	if got := buildPluginConfig("probe")["host_version"]; got != "0.12.3" {
		t.Errorf("host_version = %q, want %q", got, "0.12.3")
	}
}
