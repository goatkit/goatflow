package plugin

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeRestarter records every Reload call and lets the test inject an
// outcome. By default Reload succeeds and flips the underlying fake
// plugin back to healthy on the next probe — set wantErr to simulate a
// restart that fails to bring the plugin back.
type fakeRestarter struct {
	mu      sync.Mutex
	calls   []string
	wantErr error
	onCall  func(name string) // optional hook, runs before returning
	count   atomic.Int64
}

func (r *fakeRestarter) Reload(ctx context.Context, name string) error {
	r.count.Add(1)
	r.mu.Lock()
	r.calls = append(r.calls, name)
	hook := r.onCall
	wantErr := r.wantErr
	r.mu.Unlock()
	if hook != nil {
		hook(name)
	}
	return wantErr
}

func (r *fakeRestarter) callsFor(name string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, c := range r.calls {
		if c == name {
			n++
		}
	}
	return n
}

// TestProbeCapturesRichPayload verifies that a JSON body returned from
// the plugin's __health_ping__ handler is stored on PluginHealth.
func TestProbeCapturesRichPayload(t *testing.T) {
	m := newTestManager()
	addTestPlugin(m, &fakePlugin{
		name:     "rich",
		callResp: []byte(`{"queue_depth":42,"version":"1.2.3"}`),
	})

	m.probeOnePluginHealth("rich", 100*time.Millisecond)

	h, _ := m.HealthStatus("rich")
	if !h.Healthy {
		t.Fatalf("expected healthy, got %+v", h)
	}
	if string(h.Payload) != `{"queue_depth":42,"version":"1.2.3"}` {
		t.Errorf("expected payload preserved, got %q", string(h.Payload))
	}
}

// TestProbeIgnoresNonJSONPayload verifies that a non-JSON body is
// silently dropped (still healthy, no panic) — we don't want a
// chatty plugin to break the dashboard with garbage.
func TestProbeIgnoresNonJSONPayload(t *testing.T) {
	m := newTestManager()
	addTestPlugin(m, &fakePlugin{
		name:     "chatty",
		callResp: []byte(`not json`),
	})

	m.probeOnePluginHealth("chatty", 100*time.Millisecond)

	h, _ := m.HealthStatus("chatty")
	if !h.Healthy {
		t.Errorf("plugin should still be healthy, got %+v", h)
	}
	if h.Payload != nil {
		t.Errorf("expected nil payload for non-JSON body, got %q", string(h.Payload))
	}
}

// TestAutoRestartTriggersOnUnhealthy verifies that once a plugin trips
// the failure threshold and a Restarter is wired in, dispatchAutoRestarts
// asks the restarter to reload the plugin.
func TestAutoRestartTriggersOnUnhealthy(t *testing.T) {
	m := newTestManager()
	r := &fakeRestarter{}
	m.SetRestarter(r)

	p := &fakePlugin{name: "zombie", callDelay: 200 * time.Millisecond}
	addTestPlugin(m, p)

	// Trip the failure threshold.
	for i := 0; i < healthFailureThreshold; i++ {
		m.probeOnePluginHealth("zombie", 50*time.Millisecond)
	}
	if h, _ := m.HealthStatus("zombie"); h.Healthy {
		t.Fatalf("setup: expected unhealthy first")
	}

	m.dispatchAutoRestarts()

	// Restart runs in a goroutine; wait briefly for it to land.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && r.callsFor("zombie") == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if r.callsFor("zombie") != 1 {
		t.Errorf("expected 1 restart call, got %d", r.callsFor("zombie"))
	}

	h, _ := m.HealthStatus("zombie")
	if h.RestartAttempts != 1 {
		t.Errorf("expected RestartAttempts=1, got %d", h.RestartAttempts)
	}
	if h.NextRestartAt.IsZero() {
		t.Errorf("expected NextRestartAt set after dispatch")
	}
}

// TestAutoRestartBackoffSkipsUntilDue verifies that a second
// dispatchAutoRestarts pass within the backoff window does NOT trigger
// a second restart.
func TestAutoRestartBackoffSkipsUntilDue(t *testing.T) {
	m := newTestManager()
	r := &fakeRestarter{}
	m.SetRestarter(r)

	p := &fakePlugin{name: "stuck", callDelay: 200 * time.Millisecond}
	addTestPlugin(m, p)

	for i := 0; i < healthFailureThreshold; i++ {
		m.probeOnePluginHealth("stuck", 50*time.Millisecond)
	}

	m.dispatchAutoRestarts()
	// Wait for first goroutine to land.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && r.callsFor("stuck") == 0 {
		time.Sleep(10 * time.Millisecond)
	}

	// Second pass — backoff hasn't elapsed (5s default initial wait).
	m.dispatchAutoRestarts()
	time.Sleep(50 * time.Millisecond)

	if r.callsFor("stuck") != 1 {
		t.Errorf("expected 1 restart within backoff window, got %d", r.callsFor("stuck"))
	}
}

// TestCrashLoopGuardAbandons verifies that after crashLoopMaxAttempts
// restarts within the window, the plugin is marked abandoned and no
// further restarts are dispatched.
func TestCrashLoopGuardAbandons(t *testing.T) {
	m := newTestManager()
	r := &fakeRestarter{wantErr: errors.New("plugin won't load")}
	m.SetRestarter(r)

	p := &fakePlugin{name: "broken", callDelay: 200 * time.Millisecond}
	addTestPlugin(m, p)

	// Drive past the failure threshold once.
	for i := 0; i < healthFailureThreshold; i++ {
		m.probeOnePluginHealth("broken", 50*time.Millisecond)
	}

	// Bypass the backoff timer by directly calling startRestart in a
	// loop — production has the ticker doing this slowly, but the test
	// just exercises the budget logic.
	for i := 0; i < crashLoopMaxAttempts+2; i++ {
		// Wait for any in-flight restart to finish.
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			m.mu.RLock()
			busy := m.plugins["broken"].health.inflightRestart
			m.mu.RUnlock()
			if !busy {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		// Reset nextRestartAt so dispatch picks it up immediately.
		m.mu.Lock()
		m.plugins["broken"].health.nextRestartAt = time.Time{}
		m.mu.Unlock()
		m.dispatchAutoRestarts()
	}

	// Wait for any remaining goroutine to finish.
	time.Sleep(50 * time.Millisecond)

	h, _ := m.HealthStatus("broken")
	if !h.CrashLoopAbandoned {
		t.Errorf("expected CrashLoopAbandoned=true after %d failures, got %+v", crashLoopMaxAttempts+2, h)
	}

	// Subsequent dispatches must be no-ops.
	prev := r.count.Load()
	m.mu.Lock()
	m.plugins["broken"].health.nextRestartAt = time.Time{}
	m.mu.Unlock()
	m.dispatchAutoRestarts()
	time.Sleep(50 * time.Millisecond)
	if r.count.Load() != prev {
		t.Errorf("abandoned plugin should not restart again; count went %d → %d", prev, r.count.Load())
	}
}

// TestResetCrashLoopUnsticks verifies that the admin "reset" path
// clears abandonment and lets auto-recovery resume.
func TestResetCrashLoopUnsticks(t *testing.T) {
	m := newTestManager()
	addTestPlugin(m, &fakePlugin{name: "stuck"})

	m.mu.Lock()
	m.plugins["stuck"].health.crashLoopAbandoned = true
	m.plugins["stuck"].health.restartAttempts = 7
	m.mu.Unlock()

	if !m.ResetCrashLoop("stuck") {
		t.Fatalf("ResetCrashLoop returned false for known plugin")
	}

	h, _ := m.HealthStatus("stuck")
	if h.CrashLoopAbandoned {
		t.Errorf("expected abandonment cleared, got %+v", h)
	}
	if h.RestartAttempts != 0 {
		t.Errorf("expected restart attempts cleared, got %d", h.RestartAttempts)
	}
}

// TestRestartBackoffShape pins the documented backoff sequence so a
// future tweak to the constants breaks this test loudly.
func TestRestartBackoffShape(t *testing.T) {
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{1, 5 * time.Second},
		{2, 10 * time.Second},
		{3, 20 * time.Second},
		{4, 40 * time.Second},
		{5, 80 * time.Second},
		{6, 160 * time.Second},
		{7, restartBackoffMax}, // would be 320s, capped
		{99, restartBackoffMax},
	}
	for _, c := range cases {
		got := restartBackoff(c.attempt)
		if got != c.want {
			t.Errorf("attempt %d: got %v, want %v", c.attempt, got, c.want)
		}
	}
}

// TestNoRestartWithoutRestarter verifies that the auto-recovery is
// purely opt-in: with no Restarter wired in, an unhealthy plugin just
// sits unhealthy and nobody touches it.
func TestNoRestartWithoutRestarter(t *testing.T) {
	m := newTestManager()
	p := &fakePlugin{name: "lonely", callDelay: 200 * time.Millisecond}
	addTestPlugin(m, p)

	for i := 0; i < healthFailureThreshold; i++ {
		m.probeOnePluginHealth("lonely", 50*time.Millisecond)
	}

	m.dispatchAutoRestarts() // should be a no-op
	time.Sleep(20 * time.Millisecond)

	h, _ := m.HealthStatus("lonely")
	if h.RestartAttempts != 0 {
		t.Errorf("expected no restarts without Restarter, got %d attempts", h.RestartAttempts)
	}
}

// TestRestartSucceedsClearsCounters verifies that after a successful
// restart (plugin reports healthy on next probe), the restart bookkeeping
// resets so a future failure starts the backoff fresh.
func TestRestartSucceedsClearsCounters(t *testing.T) {
	m := newTestManager()
	r := &fakeRestarter{}
	m.SetRestarter(r)

	p := &fakePlugin{name: "flaky", callDelay: 200 * time.Millisecond}
	addTestPlugin(m, p)
	for i := 0; i < healthFailureThreshold; i++ {
		m.probeOnePluginHealth("flaky", 50*time.Millisecond)
	}
	m.dispatchAutoRestarts()
	time.Sleep(50 * time.Millisecond)

	if h, _ := m.HealthStatus("flaky"); h.RestartAttempts != 1 {
		t.Fatalf("setup: expected RestartAttempts=1, got %d", h.RestartAttempts)
	}

	// Plugin recovers — drop the delay and probe.
	p.callDelay = 0
	m.probeOnePluginHealth("flaky", 100*time.Millisecond)

	h, _ := m.HealthStatus("flaky")
	if !h.Healthy {
		t.Fatalf("expected healthy after recovery, got %+v", h)
	}
	if h.RestartAttempts != 0 {
		t.Errorf("expected RestartAttempts cleared, got %d", h.RestartAttempts)
	}
	if !h.NextRestartAt.IsZero() {
		t.Errorf("expected NextRestartAt cleared, got %v", h.NextRestartAt)
	}
}

// TestRestarterErrorLogged verifies that an error returned by the
// Restarter (e.g. plugin binary missing) doesn't panic and doesn't
// prevent the next attempt — only the crash-loop budget does.
func TestRestarterErrorLogged(t *testing.T) {
	m := newTestManager()
	r := &fakeRestarter{wantErr: errors.New("boom")}
	m.SetRestarter(r)

	p := &fakePlugin{name: "errboom", callDelay: 200 * time.Millisecond}
	addTestPlugin(m, p)
	for i := 0; i < healthFailureThreshold; i++ {
		m.probeOnePluginHealth("errboom", 50*time.Millisecond)
	}

	m.dispatchAutoRestarts()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		m.mu.RLock()
		busy := m.plugins["errboom"].health.inflightRestart
		m.mu.RUnlock()
		if !busy {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if r.callsFor("errboom") == 0 {
		t.Errorf("expected restarter to have been called even though it errors")
	}
	// Sanity: the error message would have ended up in slog. We don't
	// assert log content, just that we didn't panic and inflight cleared.
	m.mu.RLock()
	if m.plugins["errboom"].health.inflightRestart {
		t.Errorf("inflight flag should be cleared after error")
	}
	m.mu.RUnlock()
}
