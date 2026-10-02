package loader_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/plugin"
	"github.com/goatkit/goatflow/internal/platform/plugin/loader"
)

// wasmLoadCounter counts the loader's "loading WASM plugin" records, emitted
// once per WASM module compiled and instantiated.
type wasmLoadCounter struct{ n atomic.Int64 }

func (c *wasmLoadCounter) Enabled(context.Context, slog.Level) bool { return true }
func (c *wasmLoadCounter) Handle(_ context.Context, r slog.Record) error {
	if r.Message == "loading WASM plugin" {
		c.n.Add(1)
	}
	return nil
}
func (c *wasmLoadCounter) WithAttrs([]slog.Attr) slog.Handler { return c }
func (c *wasmLoadCounter) WithGroup(string) slog.Handler      { return c }

// TestLazyWASMPluginLoadedOnce: hello.wasm registers as "hello-wasm", not
// under its file name. The loader must track it under the registered name
// once loaded, otherwise every Discovered()/EnsureLoaded() sweep misses the
// registry and loads the module again.
func TestLazyWASMPluginLoadedOnce(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	src := filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "plugins", "hello-wasm", "hello.wasm")
	wasmBytes, err := os.ReadFile(src)
	require.NoError(t, err)

	pluginDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(pluginDir, "hello-wasm"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "hello-wasm", "hello.wasm"), wasmBytes, 0o644))

	ctx := context.Background()
	mgr := plugin.NewManager(&mockHostAPI{})
	counter := &wasmLoadCounter{}
	l := loader.NewLoader(pluginDir, mgr, slog.New(counter), loader.WithLazyLoading())
	mgr.SetLazyLoader(l)
	t.Cleanup(func() { _ = mgr.Unregister(ctx, "hello-wasm") })

	_, err = l.DiscoverAll()
	require.NoError(t, err)
	require.Equal(t, []string{"hello"}, l.Discovered(), "unloaded WASM plugin is tracked by file name")
	require.Zero(t, counter.n.Load())

	hasWidget := func() bool {
		return slices.ContainsFunc(mgr.AllWidgets("dashboard"), func(w plugin.PluginWidget) bool {
			return w.PluginName == "hello-wasm" && w.ID == "hello-wasm-widget"
		})
	}

	// First sweep lazily loads the plugin.
	require.True(t, hasWidget())
	require.EqualValues(t, 1, counter.n.Load())

	args, _ := json.Marshal(map[string]string{"name": "GoatKit"})
	var ensureErr error
	for range 3 {
		require.True(t, hasWidget())
		for _, name := range l.Discovered() {
			ensureErr = errors.Join(ensureErr, l.EnsureLoaded(ctx, name))
		}
		_, err := mgr.Call(ctx, "hello-wasm", "hello", args)
		require.NoError(t, err)
	}
	require.EqualValues(t, 1, counter.n.Load(), "repeated calls must not reload the plugin")
	require.NoError(t, ensureErr)
	require.Equal(t, []string{"hello-wasm"}, l.Discovered(), "loaded WASM plugin is tracked by registered name")

	// Re-discovery (as after a plugin upload) must not resurrect the file-name key.
	_, err = l.DiscoverAll()
	require.NoError(t, err)
	require.Equal(t, []string{"hello-wasm"}, l.Discovered())

	// A disabled plugin is still registered: sweeps must not load it again.
	require.NoError(t, mgr.Disable("hello-wasm"))
	ensureErr = nil
	for range 3 {
		require.False(t, hasWidget())
		ensureErr = errors.Join(ensureErr, l.EnsureLoaded(ctx, "hello-wasm"))
	}
	require.EqualValues(t, 1, counter.n.Load(), "disabled plugin must not be reloaded")
	require.NoError(t, ensureErr)
}
