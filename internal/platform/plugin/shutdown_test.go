package plugin

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestShutdownAllPerPluginTimeout verifies that a plugin whose
// Shutdown hangs is given its ResourcePolicy.ShutdownTimeout window
// and then moved past — it doesn't block the whole ShutdownAll pass.
func TestShutdownAllPerPluginTimeout(t *testing.T) {
	m := newTestManager()

	fast := &fakePlugin{name: "fast"}
	slow := &fakePlugin{name: "slow", shutdownHang: true}
	addTestPlugin(m, fast)
	addTestPlugin(m, slow)

	// Apply a short shutdown timeout to the slow plugin so the test
	// doesn't wait 10s (the default). 100ms is plenty for a fake
	// plugin whose Shutdown is a context-block; 500ms is a safe
	// upper bound on how long the test should run.
	m.policies["slow"] = &ResourcePolicy{PluginName: "slow", ShutdownTimeout: "100ms"}

	start := time.Now()
	err := m.ShutdownAll(context.Background())
	elapsed := time.Since(start)

	// Expect an error that names the slow plugin and references a
	// deadline — but ShutdownAll should have completed, not hung.
	if err == nil {
		t.Fatalf("expected shutdown error for hung plugin, got nil")
	}
	if !strings.Contains(err.Error(), "slow") {
		t.Errorf("expected error to name slow plugin, got %v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("ShutdownAll took %v, expected ~100ms (per-plugin timeout)", elapsed)
	}

	// And the map should be cleared either way.
	if len(m.plugins) != 0 {
		t.Errorf("expected plugins map cleared post-shutdown, got %d entries", len(m.plugins))
	}
}

// TestShutdownAllDefaultTimeout verifies that a plugin with no
// ResourcePolicy entry still gets a bounded shutdown via the
// defaultShutdownTimeout constant — no plugin can wedge us forever.
func TestShutdownAllDefaultTimeout(t *testing.T) {
	m := newTestManager()
	addTestPlugin(m, &fakePlugin{name: "polite"}) // returns quickly

	start := time.Now()
	if err := m.ShutdownAll(context.Background()); err != nil {
		t.Fatalf("unexpected error shutting down fast plugin: %v", err)
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Errorf("fast plugin shutdown took too long")
	}
}

// TestShutdownAllParallel verifies that N hung plugins each with a
// 200ms per-plugin budget shut down in ~200ms (parallel) rather than
// ~N×200ms (the old serial behaviour).
func TestShutdownAllParallel(t *testing.T) {
	m := newTestManager()
	const n = 5
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("hung-%d", i)
		addTestPlugin(m, &fakePlugin{name: name, shutdownHang: true})
		m.policies[name] = &ResourcePolicy{PluginName: name, ShutdownTimeout: "200ms"}
	}

	start := time.Now()
	err := m.ShutdownAll(context.Background())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("expected error from hung plugins, got nil")
	}
	// Serial would be ~n×200ms = 1s. Parallel should be ~200ms with a
	// generous slop budget for goroutine scheduling on a busy CI host.
	if elapsed > 600*time.Millisecond {
		t.Errorf("ShutdownAll took %v for %d plugins — expected parallel (~200ms), got serial-ish", elapsed, n)
	}

	if len(m.plugins) != 0 {
		t.Errorf("expected plugins map cleared, got %d entries", len(m.plugins))
	}
}

// TestShutdownAllOuterCtxCaps verifies that a caller-provided
// context timeout caps the total duration — each per-plugin timeout
// gets clamped by it.
func TestShutdownAllOuterCtxCaps(t *testing.T) {
	m := newTestManager()
	for i, name := range []string{"a", "b", "c", "d"} {
		p := &fakePlugin{name: name, shutdownHang: true}
		addTestPlugin(m, p)
		// Give each plugin a 1s per-plugin budget.
		m.policies[name] = &ResourcePolicy{PluginName: name, ShutdownTimeout: "1s"}
		_ = i
	}

	// But cap the whole thing at 200ms. Without the outer ctx this
	// would take ~4s (4 plugins × 1s).
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	_ = m.ShutdownAll(ctx)
	elapsed := time.Since(start)

	if elapsed > 500*time.Millisecond {
		t.Errorf("ShutdownAll respected per-plugin 1s timeout but should have been clamped by outer 200ms ctx; took %v", elapsed)
	}
}
