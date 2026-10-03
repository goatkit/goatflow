package plugin_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/goatkit/goatflow/internal/platform/plugin"
	"github.com/goatkit/goatflow/internal/platform/version"
)

// minHostPlugin declares a MinHostVersion and counts Init calls.
type minHostPlugin struct {
	version, minHost string
	inits            int
}

func (p *minHostPlugin) GKRegister() plugin.GKRegistration {
	return plugin.GKRegistration{Name: "min-host-probe", Version: p.version, MinHostVersion: p.minHost}
}
func (p *minHostPlugin) Init(context.Context, plugin.HostAPI) error { p.inits++; return nil }
func (p *minHostPlugin) Call(context.Context, string, json.RawMessage) (json.RawMessage, error) {
	return nil, nil
}
func (p *minHostPlugin) Shutdown(context.Context) error { return nil }

func TestRegisterRefusesPluginNeedingNewerHost(t *testing.T) {
	old := version.Version
	version.Version = "0.10.0"
	t.Cleanup(func() { version.Version = old })

	ctx := context.Background()
	mgr := plugin.NewManager(&mockHostAPI{})

	tooNew := &minHostPlugin{version: "2.0.0", minHost: "0.11.0"}
	err := mgr.Register(ctx, tooNew)
	var incompat *version.IncompatibleError
	if !errors.As(err, &incompat) {
		t.Fatalf("Register error = %v, want *version.IncompatibleError", err)
	}
	if want := "min-host-probe v2.0.0 requires GoatFlow >= 0.11.0, you have 0.10.0"; err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
	if tooNew.inits != 0 {
		t.Errorf("refused plugin was initialised %d times", tooNew.inits)
	}
	if _, ok := mgr.Get("min-host-probe"); ok {
		t.Error("refused plugin is registered")
	}
	logged := false
	for _, e := range plugin.GetLogBuffer().GetByPlugin("min-host-probe") {
		if e.Level == "error" && strings.Contains(e.Message, "requires GoatFlow >= 0.11.0") {
			logged = true
		}
	}
	if !logged {
		t.Error("refusal not recorded in the plugin log buffer")
	}

	ok := &minHostPlugin{version: "1.0.0", minHost: "0.10.0"}
	if err := mgr.Register(ctx, ok); err != nil {
		t.Fatalf("Register of compatible plugin: %v", err)
	}

	// Hot reload to a version needing a newer host keeps the running one.
	if err := mgr.ReplacePlugin(ctx, "min-host-probe", tooNew); !errors.As(err, &incompat) {
		t.Fatalf("ReplacePlugin error = %v, want *version.IncompatibleError", err)
	}
	if got, _ := mgr.Get("min-host-probe"); got != plugin.Plugin(ok) {
		t.Error("refused replacement displaced the running plugin")
	}
}
