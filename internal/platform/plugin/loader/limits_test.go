package loader_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/goatkit/goatflow/internal/platform/plugin"
	"github.com/goatkit/goatflow/internal/platform/plugin/loader"
	"github.com/goatkit/goatflow/internal/platform/plugin/wasm/wasmtest"
)

// loadLimitsPlugin loads a test WASM plugin that grows its memory by 4MB on
// "grow" and loops forever on "spin".
func loadLimitsPlugin(t *testing.T, manifest string) (*plugin.Manager, *loader.Loader) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "limits.wasm"), wasmtest.Module(manifest, 64), 0o600); err != nil {
		t.Fatal(err)
	}
	mgr := plugin.NewManager(&mockHostAPI{})
	l := loader.NewLoader(dir, mgr, nil)
	if n, errs := l.LoadAll(ctx); n != 1 || len(errs) > 0 {
		t.Fatalf("LoadAll = %d, %v", n, errs)
	}
	t.Cleanup(func() { _ = mgr.ShutdownAll(ctx) })
	return mgr, l
}

func grows(t *testing.T, mgr *plugin.Manager) string {
	t.Helper()
	out, err := mgr.Call(context.Background(), "limits", "grow", nil)
	if err != nil {
		t.Fatalf("grow: %v", err)
	}
	return string(out)
}

func spinsUntilDeadline(t *testing.T, mgr *plugin.Manager) {
	t.Helper()
	start := time.Now()
	_, err := mgr.Call(context.Background(), "limits", "spin", nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("spin error = %v, want deadline exceeded", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("spin returned after %s", elapsed)
	}
}

// TestWASMPluginRunsUnderDeclaredLimits: the memory and call timeout a WASM
// plugin declares in its registration hold once it is loaded and after a
// reload.
func TestWASMPluginRunsUnderDeclaredLimits(t *testing.T) {
	mgr, l := loadLimitsPlugin(t,
		`{"name":"limits","version":"1.0.0","resources":{"memory_mb":1,"call_timeout":"100ms"}}`)

	if got := grows(t, mgr); got != "false" {
		t.Errorf("4MB growth under a declared 1MB = %s, want false", got)
	}
	spinsUntilDeadline(t, mgr)

	if err := l.Reload(context.Background(), "limits"); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := grows(t, mgr); got != "false" {
		t.Errorf("after reload: 4MB growth under a declared 1MB = %s, want false", got)
	}
	spinsUntilDeadline(t, mgr)
}

// TestWASMPluginPolicyChanges: without declarations a WASM plugin gets the
// default 512MB; a stored policy's call timeout applies at once and its
// memory limit from the next reload.
func TestWASMPluginPolicyChanges(t *testing.T) {
	mgr, l := loadLimitsPlugin(t, `{"name":"limits","version":"1.0.0"}`)

	if got := grows(t, mgr); got != "true" {
		t.Errorf("4MB growth under the default limit = %s, want true", got)
	}

	mgr.SetPolicy("limits", plugin.ResourcePolicy{MemoryMB: 1, CallTimeout: "100ms"})
	spinsUntilDeadline(t, mgr)
	if got := grows(t, mgr); got != "true" {
		t.Errorf("memory limit changed without a reload: grow = %s", got)
	}

	if err := l.Reload(context.Background(), "limits"); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := grows(t, mgr); got != "false" {
		t.Errorf("after reload: 4MB growth under a stored 1MB = %s, want false", got)
	}
}
