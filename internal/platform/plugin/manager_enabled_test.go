package plugin_test

import (
	"context"
	"errors"
	"testing"

	"github.com/goatkit/goatflow/internal/platform/plugin"
	"github.com/goatkit/goatflow/internal/platform/plugin/example"
)

// failingDBHost is a host whose database is unreachable.
type failingDBHost struct{ mockHostAPI }

func (h *failingDBHost) DBQuery(ctx context.Context, query string, args ...any) ([]map[string]any, error) {
	return nil, errors.New("connection refused")
}

// A failed enabled-state lookup must not enable a plugin: an administrator
// may have disabled it, and a DB outage would silently switch it back on.
func TestRegisterLeavesPluginDisabledWhenEnabledLookupFails(t *testing.T) {
	mgr := plugin.NewManager(&failingDBHost{})
	if err := mgr.Register(context.Background(), example.NewHelloPlugin()); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if mgr.IsEnabled("hello") {
		t.Fatal("plugin enabled although its enabled state could not be read")
	}
}

// With a reachable DB and no sysconfig row the documented default applies.
func TestRegisterEnablesPluginWithoutSysconfigRow(t *testing.T) {
	mgr := plugin.NewManager(&mockHostAPI{})
	if err := mgr.Register(context.Background(), example.NewHelloPlugin()); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if !mgr.IsEnabled("hello") {
		t.Fatal("plugin without a sysconfig row should default to enabled")
	}
}
