package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// fakePlugin implements the Plugin interface with a configurable Call
// delay, so tests can simulate healthy/unhealthy plugins deterministically.
type fakePlugin struct {
	name         string
	callDelay    time.Duration // how long Call blocks before returning
	callErr      error         // returned unconditionally when delay elapses (unless nil)
	callResp     []byte        // response body for Call (used to test payload capture)
	callCount    atomic.Int64  // counts how many times Call was invoked
	shutdownHang bool          // if true, Shutdown blocks until ctx is done
}

func (f *fakePlugin) GKRegister() GKRegistration {
	return GKRegistration{Name: f.name}
}

func (f *fakePlugin) Init(ctx context.Context, host HostAPI) error { return nil }

func (f *fakePlugin) Call(ctx context.Context, fn string, args json.RawMessage) (json.RawMessage, error) {
	f.callCount.Add(1)
	if f.callDelay > 0 {
		select {
		case <-time.After(f.callDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if f.callErr != nil {
		return nil, f.callErr
	}
	if f.callResp != nil {
		return json.RawMessage(f.callResp), nil
	}
	return json.RawMessage(`"ok"`), nil
}

func (f *fakePlugin) Shutdown(ctx context.Context) error {
	if f.shutdownHang {
		<-ctx.Done()
		return ctx.Err()
	}
	return nil
}

// newTestManager builds a bare manager with no HostAPI — good enough
// for tests that only exercise health + shutdown without DB hits.
func newTestManager() *Manager {
	return &Manager{
		plugins:   make(map[string]*registeredPlugin),
		policies:  make(map[string]*ResourcePolicy),
		sandboxes: make(map[string]*SandboxedHostAPI),
	}
}

func addTestPlugin(m *Manager, p *fakePlugin) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.plugins[p.name] = &registeredPlugin{
		plugin:   p,
		manifest: GKRegistration{Name: p.name},
		enabled:  true,
		health:   healthInitForRegister(),
	}
}

// TestProbeHealthySucceeds verifies that a plugin which returns
// quickly (within the probe timeout) is marked healthy.
func TestProbeHealthySucceeds(t *testing.T) {
	m := newTestManager()
	addTestPlugin(m, &fakePlugin{name: "fast"})

	m.probeOnePluginHealth("fast", 100*time.Millisecond)

	h, ok := m.HealthStatus("fast")
	if !ok {
		t.Fatalf("plugin not registered")
	}
	if !h.Healthy {
		t.Errorf("expected healthy, got %+v", h)
	}
	if h.ConsecutiveFailures != 0 {
		t.Errorf("expected 0 consecutive failures, got %d", h.ConsecutiveFailures)
	}
	if h.LastSuccess.IsZero() {
		t.Errorf("expected LastSuccess to be set")
	}
}

// TestProbeUnknownFunctionStillHealthy verifies the "any response
// means alive" contract — a plugin that doesn't implement the health
// ping handler (and so returns an error) is still considered healthy,
// because the round-trip worked.
func TestProbeUnknownFunctionStillHealthy(t *testing.T) {
	m := newTestManager()
	addTestPlugin(m, &fakePlugin{
		name:    "no-ping-handler",
		callErr: errors.New("unknown function: __health_ping__"),
	})

	m.probeOnePluginHealth("no-ping-handler", 100*time.Millisecond)

	h, _ := m.HealthStatus("no-ping-handler")
	if !h.Healthy {
		t.Errorf("plugin returning any error should still be healthy (gRPC alive); got %+v", h)
	}
}

// TestProbeTimeoutCountsFailure verifies that a plugin which blocks
// past the probe timeout is recorded as a failure (but not yet marked
// unhealthy until the threshold is reached).
func TestProbeTimeoutCountsFailure(t *testing.T) {
	m := newTestManager()
	addTestPlugin(m, &fakePlugin{name: "slow", callDelay: 200 * time.Millisecond})

	m.probeOnePluginHealth("slow", 50*time.Millisecond)

	h, _ := m.HealthStatus("slow")
	if h.ConsecutiveFailures != 1 {
		t.Errorf("expected 1 consecutive failure, got %d", h.ConsecutiveFailures)
	}
	if !h.Healthy {
		// After 1 failure (below threshold) health stays at whatever
		// it was. On a freshly-registered plugin that's `false`; the
		// initial state. Check lastError is set.
	}
	if h.LastError == "" {
		t.Errorf("expected LastError to be populated after timeout")
	}
}

// TestProbeThresholdTripsUnhealthy verifies that after
// healthFailureThreshold consecutive timeouts, Healthy flips to false.
func TestProbeThresholdTripsUnhealthy(t *testing.T) {
	m := newTestManager()
	p := &fakePlugin{name: "zombie"}
	addTestPlugin(m, p)

	// Warm up: one successful probe → healthy.
	p.callDelay = 0
	m.probeOnePluginHealth("zombie", 100*time.Millisecond)
	if h, _ := m.HealthStatus("zombie"); !h.Healthy {
		t.Fatalf("setup: expected healthy after first good probe")
	}

	// Fail threshold times.
	p.callDelay = 200 * time.Millisecond
	for i := 0; i < healthFailureThreshold; i++ {
		m.probeOnePluginHealth("zombie", 50*time.Millisecond)
	}

	h, _ := m.HealthStatus("zombie")
	if h.Healthy {
		t.Errorf("expected unhealthy after %d consecutive failures, got %+v", healthFailureThreshold, h)
	}
	if h.ConsecutiveFailures < healthFailureThreshold {
		t.Errorf("expected ≥ %d consecutive failures, got %d", healthFailureThreshold, h.ConsecutiveFailures)
	}
}

// TestProbeRecoverFromUnhealthy verifies that a successful probe
// after being unhealthy resets the counter and flips back to healthy.
func TestProbeRecoverFromUnhealthy(t *testing.T) {
	m := newTestManager()
	p := &fakePlugin{name: "recovering", callDelay: 200 * time.Millisecond}
	addTestPlugin(m, p)

	// Drive to unhealthy.
	for i := 0; i < healthFailureThreshold; i++ {
		m.probeOnePluginHealth("recovering", 50*time.Millisecond)
	}
	if h, _ := m.HealthStatus("recovering"); h.Healthy {
		t.Fatalf("setup: expected unhealthy first")
	}

	// Then a single successful probe.
	p.callDelay = 0
	m.probeOnePluginHealth("recovering", 100*time.Millisecond)

	h, _ := m.HealthStatus("recovering")
	if !h.Healthy {
		t.Errorf("expected healthy after recovery, got %+v", h)
	}
	if h.ConsecutiveFailures != 0 {
		t.Errorf("expected counter reset, got %d", h.ConsecutiveFailures)
	}
	if h.LastError != "" {
		t.Errorf("expected LastError cleared, got %q", h.LastError)
	}
}

// TestProbeUnknownPluginSkips verifies that probing a name not in the
// manager just returns silently (no panic, no state change).
func TestProbeUnknownPluginSkips(t *testing.T) {
	m := newTestManager()
	m.probeOnePluginHealth("ghost", 100*time.Millisecond)
	if _, ok := m.HealthStatus("ghost"); ok {
		t.Errorf("HealthStatus should report not-found for unregistered plugin")
	}
}

// TestAllHealthStatusesSnapshot verifies the map-wide getter returns
// data for every registered plugin.
func TestAllHealthStatusesSnapshot(t *testing.T) {
	m := newTestManager()
	addTestPlugin(m, &fakePlugin{name: "a"})
	addTestPlugin(m, &fakePlugin{name: "b"})

	m.probeOnePluginHealth("a", 100*time.Millisecond)
	m.probeOnePluginHealth("b", 100*time.Millisecond)

	all := m.AllHealthStatuses()
	if len(all) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(all))
	}
	for name, h := range all {
		if !h.Healthy {
			t.Errorf("expected %q healthy, got %+v", name, h)
		}
	}
}

// TestStartHealthCheckerStopsCleanly verifies the checker goroutine
// exits when stop is invoked, and that starting it twice is a no-op.
func TestStartHealthCheckerStopsCleanly(t *testing.T) {
	m := newTestManager()
	p := &fakePlugin{name: "ticker"}
	addTestPlugin(m, p)

	stop1 := m.StartHealthChecker(20*time.Millisecond, 10*time.Millisecond)
	// Second call must be a no-op stop — we should not spin up two goroutines.
	stop2 := m.StartHealthChecker(20*time.Millisecond, 10*time.Millisecond)

	// Let the ticker fire at least once.
	time.Sleep(60 * time.Millisecond)

	before := p.callCount.Load()
	if before == 0 {
		t.Fatalf("expected at least one probe, got 0")
	}

	stop1()
	stop2() // stop2 should be a no-op; this just asserts it doesn't panic.

	// Give any in-flight goroutine a moment to exit, then confirm the
	// count isn't growing after stop1 closed the channel.
	time.Sleep(60 * time.Millisecond)
	after := p.callCount.Load()
	growth := after - before
	// Allow at most one more probe that was in-flight when stop fired.
	if growth > 1 {
		t.Errorf("expected ≤1 in-flight probe after stop, got %d", growth)
	}
}
