package plugin

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"
)

// HealthPingFunc is the reserved function name the manager probes
// against every loaded plugin to check liveness.
//
// The probe uses the existing Plugin.Call path — no protocol changes,
// no mandatory plugin-side handler. A plugin that doesn't recognise
// this function name will return an "unknown function" error, which
// the health checker treats as HEALTHY (any quick response means the
// gRPC/net-rpc channel is alive and the plugin process is responsive).
// Only a context-deadline-exceeded error counts as a failure — that's
// what distinguishes a zombie plugin from a merely unhelpful one.
//
// Plugins that want to surface rich health info can implement a handler
// for this name and return a JSON object — the manager stores it on
// PluginHealth.Payload so admin UIs can render it. Plugins that don't
// implement it remain healthy as before; the payload is just empty.
const HealthPingFunc = "__health_ping__"

// defaultHealthInterval is how often the manager probes each plugin
// when StartHealthChecker is invoked without an explicit interval.
// 60s keeps log noise low while still catching a dead plugin within
// ~3 minutes (3 consecutive failures × 60s).
const defaultHealthInterval = 60 * time.Second

// defaultHealthProbeTimeout is the per-plugin ceiling on how long we
// wait for a Ping response before counting it as a failure. Must be
// shorter than defaultHealthInterval to avoid overlapping probes.
const defaultHealthProbeTimeout = 5 * time.Second

// healthFailureThreshold is how many consecutive failed probes it
// takes to mark a plugin unhealthy. Set to 3 so transient glitches
// (network blip, momentary GC pause) don't trip the alarm — a
// genuinely stuck plugin will accumulate failures fast.
const healthFailureThreshold = 3

// Auto-restart tuning. Backoff doubles each attempt: 5s, 10s, 20s, 40s,
// 80s, 160s, capped at restartBackoffMax. Once a plugin restarts
// successfully and stays healthy, the backoff resets to restartBackoffInitial.
const (
	restartBackoffInitial = 5 * time.Second
	restartBackoffMax     = 5 * time.Minute

	// crashLoopWindow is the rolling window over which we count
	// restart attempts when deciding whether a plugin is in a
	// crash-loop. Pick a window long enough to catch "broken at
	// startup, restart succeeds briefly, then dies again" but short
	// enough that a plugin recovering after one bad deploy isn't
	// permanently abandoned.
	crashLoopWindow = 10 * time.Minute

	// crashLoopMaxAttempts is how many restart attempts within
	// crashLoopWindow we tolerate before giving up. After this point,
	// further auto-restarts are suppressed and the plugin is marked
	// "abandoned" — admin must intervene (fix the plugin, click
	// Restart in the UI, or simply call Manager.ResetCrashLoop).
	crashLoopMaxAttempts = 5
)

// Restarter is the hook the manager uses to actually re-spawn a
// plugin process after a health failure. It's an interface so the
// manager doesn't depend on the loader package — the loader (which
// already knows how to discover and (re)spawn WASM/gRPC plugins)
// implements this and is wired in via SetRestarter at startup.
type Restarter interface {
	Reload(ctx context.Context, name string) error
}

// PluginHealth is a snapshot of a plugin's current health state.
// Exposed via Manager.HealthStatus for admin UIs / dashboards.
type PluginHealth struct {
	Healthy             bool      `json:"healthy"`
	LastCheck           time.Time `json:"last_check"`
	LastSuccess         time.Time `json:"last_success,omitempty"`
	ConsecutiveFailures int       `json:"consecutive_failures"`
	LastError           string    `json:"last_error,omitempty"`

	// Payload is the most recent JSON body returned by the plugin's
	// __health_ping__ handler, or nil if the plugin returned a
	// non-JSON / empty body. Plugins that want to surface custom
	// health detail (queue depth, downstream connectivity, version
	// info) should return a JSON object from their ping handler.
	Payload json.RawMessage `json:"payload,omitempty"`

	// Restart bookkeeping (for auto-recovery). All optional — only
	// meaningful when StartAutoRecovery has been wired in.
	RestartAttempts    int       `json:"restart_attempts,omitempty"`
	LastRestartAt      time.Time `json:"last_restart_at,omitempty"`
	NextRestartAt      time.Time `json:"next_restart_at,omitempty"`
	CrashLoopAbandoned bool      `json:"crash_loop_abandoned,omitempty"`
}

// healthState carries the in-flight health bookkeeping for one
// plugin. It lives next to the registeredPlugin in the manager's
// map, guarded by the manager's mutex.
type healthState struct {
	healthy             bool
	lastCheck           time.Time
	lastSuccess         time.Time
	consecutiveFailures int
	lastError           string
	payload             json.RawMessage

	// restartAttempts counts consecutive failed restart attempts
	// (resets to 0 when a restart succeeds and the plugin returns to
	// healthy). Used to compute the next backoff.
	restartAttempts int
	lastRestartAt   time.Time
	nextRestartAt   time.Time

	// restartHistory is a rolling window of restart-attempt
	// timestamps, capped by crashLoopMaxAttempts. Older entries are
	// trimmed in-place; once the window is full, crashLoopAbandoned
	// trips and auto-recovery stops.
	restartHistory     []time.Time
	crashLoopAbandoned bool

	// inflightRestart guards against double-dispatching a restart for
	// the same plugin — health-checker pass N+1 may run before pass N's
	// restart goroutine has finished. Always accessed under Manager.mu.
	inflightRestart bool
}

// StartHealthChecker launches a background goroutine that periodically
// probes every loaded plugin and updates its health state. Returns a
// stop function that callers should defer at the end of the program
// lifetime to tear the checker down cleanly.
//
// Passing a zero interval or probeTimeout uses the package defaults
// (60s / 5s). The checker is safe to start at most once per manager —
// subsequent calls return a no-op stop function. This mirrors how
// OnPluginLoaded and other manager-level hooks are wired up, where
// the cmd/goats/main.go startup owns the lifecycle.
//
// If a Restarter has been wired in via SetRestarter, the checker also
// drives auto-recovery: unhealthy plugins are scheduled for restart
// with exponential backoff, and a crash-loop guard suppresses repeated
// restarts for plugins that won't stabilise. Without a Restarter the
// checker is purely observational — it logs and exposes status but
// never restarts anything.
func (m *Manager) StartHealthChecker(interval, probeTimeout time.Duration) func() {
	m.mu.Lock()
	if m.healthCheckerStarted {
		m.mu.Unlock()
		return func() {}
	}
	m.healthCheckerStarted = true
	m.mu.Unlock()

	if interval <= 0 {
		interval = defaultHealthInterval
	}
	if probeTimeout <= 0 {
		probeTimeout = defaultHealthProbeTimeout
	}

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				m.probeAllPluginsHealth(probeTimeout)
				m.dispatchAutoRestarts()
			}
		}
	}()

	return func() {
		close(stop)
		<-done
	}
}

// SetRestarter wires in the loader so the health checker can attempt
// auto-recovery on health failures. Safe to call any time; without it,
// the checker only observes and logs.
func (m *Manager) SetRestarter(r Restarter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.restarter = r
}

// probeAllPluginsHealth runs a single probe pass across every loaded
// plugin. Snapshots the plugin list under the read lock, probes
// outside the lock (probes do blocking RPC calls — never hold the
// manager mutex across them), then applies results under the write
// lock.
func (m *Manager) probeAllPluginsHealth(probeTimeout time.Duration) {
	m.mu.RLock()
	names := make([]string, 0, len(m.plugins))
	for name := range m.plugins {
		names = append(names, name)
	}
	m.mu.RUnlock()

	for _, name := range names {
		m.probeOnePluginHealth(name, probeTimeout)
	}
}

// probeOnePluginHealth sends the ping, records the outcome, and logs
// state transitions. Kept separate from the loop so it's easy to test
// in isolation.
func (m *Manager) probeOnePluginHealth(name string, probeTimeout time.Duration) {
	m.mu.RLock()
	rp, exists := m.plugins[name]
	m.mu.RUnlock()
	if !exists {
		// Plugin was unregistered between snapshot and probe — skip.
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	resp, err := rp.plugin.Call(ctx, HealthPingFunc, json.RawMessage(nil))

	// Only a context-deadline-exceeded indicates the plugin failed to
	// respond in time (i.e. is zombie / wedged). Any other error —
	// including "unknown function: __health_ping__" from plugins that
	// don't recognise the name — means the RPC round-trip worked and
	// the plugin is alive.
	failed := ctx.Err() == context.DeadlineExceeded

	// Capture rich payload only on a successful response. Validate it
	// parses as JSON before storing — plugins that don't implement the
	// handler return an error, which we already treat as alive.
	var payload json.RawMessage
	if !failed && err == nil && len(resp) > 0 {
		if json.Valid(resp) {
			payload = append(json.RawMessage(nil), resp...)
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Re-check: plugin could have been unregistered while we probed.
	rp, exists = m.plugins[name]
	if !exists {
		return
	}

	now := time.Now()
	prev := rp.health
	rp.health.lastCheck = now

	if failed {
		rp.health.consecutiveFailures++
		if err != nil {
			rp.health.lastError = err.Error()
		}
		if rp.health.consecutiveFailures >= healthFailureThreshold {
			rp.health.healthy = false
		}
	} else {
		rp.health.consecutiveFailures = 0
		rp.health.lastSuccess = now
		rp.health.lastError = ""
		rp.health.healthy = true
		rp.health.payload = payload
		// A healthy probe after a successful restart attempt: clear
		// crash-loop bookkeeping so the next failure starts fresh.
		if rp.health.restartAttempts > 0 {
			slog.Info("plugin recovered after restart",
				"plugin", name,
				"attempts", rp.health.restartAttempts)
			rp.health.restartAttempts = 0
			rp.health.nextRestartAt = time.Time{}
			rp.health.restartHistory = nil
		}
	}

	// Log transitions only — avoid spamming a line every 60s for
	// healthy plugins. First-time success transitions from the
	// zero-value `healthy=false`, so on startup we get one line per
	// plugin confirming the checker works.
	switch {
	case !prev.healthy && rp.health.healthy:
		slog.Info("plugin became healthy", "plugin", name,
			"consecutive_failures_before_recovery", prev.consecutiveFailures)
	case prev.healthy && !rp.health.healthy:
		slog.Warn("plugin marked unhealthy", "plugin", name,
			"consecutive_failures", rp.health.consecutiveFailures,
			"last_error", rp.health.lastError)
	case failed && rp.health.consecutiveFailures == 1:
		// First failure after being healthy — debug log, don't warn
		// yet (could be a transient blip).
		slog.Debug("plugin health probe failed", "plugin", name, "error", rp.health.lastError)
	}
}

// dispatchAutoRestarts looks for unhealthy plugins whose backoff has
// elapsed and asks the configured Restarter to restart them. Skips
// plugins that are already abandoned (crash-loop guard tripped) or
// have a restart already in flight from a previous pass.
//
// Each restart runs in its own goroutine so a slow Reload can't block
// the next probe interval — plugins are independent.
func (m *Manager) dispatchAutoRestarts() {
	m.mu.RLock()
	r := m.restarter
	if r == nil {
		m.mu.RUnlock()
		return
	}
	type todo struct {
		name    string
		attempt int
	}
	now := time.Now()
	var toRestart []todo
	for name, rp := range m.plugins {
		if rp.health.healthy || rp.health.crashLoopAbandoned {
			continue
		}
		if rp.health.consecutiveFailures < healthFailureThreshold {
			continue
		}
		if !rp.health.nextRestartAt.IsZero() && now.Before(rp.health.nextRestartAt) {
			continue
		}
		if rp.health.inflightRestart {
			continue
		}
		toRestart = append(toRestart, todo{name: name, attempt: rp.health.restartAttempts + 1})
	}
	m.mu.RUnlock()

	for _, t := range toRestart {
		m.startRestart(r, t.name, t.attempt)
	}
}

// startRestart claims the inflight flag, schedules the next backoff
// window, and dispatches the actual Reload in a background goroutine.
// Returns immediately so the caller (the health-checker tick) doesn't
// block on plugin spawning.
func (m *Manager) startRestart(r Restarter, name string, attempt int) {
	m.mu.Lock()
	rp, exists := m.plugins[name]
	if !exists {
		m.mu.Unlock()
		return
	}
	if rp.health.inflightRestart {
		// Another goroutine grabbed it between dispatchAutoRestarts
		// reading the flag and us taking the write lock.
		m.mu.Unlock()
		return
	}
	rp.health.inflightRestart = true

	// Crash-loop check: trim history to the rolling window, append
	// this attempt, and if we've blown the budget, abandon.
	now := time.Now()
	cutoff := now.Add(-crashLoopWindow)
	trimmed := rp.health.restartHistory[:0]
	for _, t := range rp.health.restartHistory {
		if t.After(cutoff) {
			trimmed = append(trimmed, t)
		}
	}
	trimmed = append(trimmed, now)
	rp.health.restartHistory = trimmed

	if len(trimmed) > crashLoopMaxAttempts {
		rp.health.crashLoopAbandoned = true
		rp.health.inflightRestart = false
		slog.Error("plugin abandoned after crash-loop",
			"plugin", name,
			"attempts_in_window", len(trimmed),
			"window", crashLoopWindow)
		m.mu.Unlock()
		return
	}

	rp.health.restartAttempts = attempt
	rp.health.lastRestartAt = now
	rp.health.nextRestartAt = now.Add(restartBackoff(attempt))
	m.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		slog.Warn("auto-restarting unhealthy plugin", "plugin", name, "attempt", attempt)
		if err := r.Reload(ctx, name); err != nil {
			slog.Warn("plugin auto-restart failed",
				"plugin", name,
				"attempt", attempt,
				"error", err)
		} else {
			slog.Info("plugin auto-restart dispatched", "plugin", name, "attempt", attempt)
		}
		// Drop the inflight flag whether we succeeded or not — the
		// next probe pass will see the result and either clear
		// counters (success) or schedule the next attempt (failure).
		// Reload may have replaced the registeredPlugin entirely
		// (ReplacePlugin); look up by name fresh under the lock.
		m.mu.Lock()
		if rp, exists := m.plugins[name]; exists {
			rp.health.inflightRestart = false
		}
		m.mu.Unlock()
	}()
}

// restartBackoff returns the wait time for the Nth (1-indexed) restart
// attempt: 5s, 10s, 20s, ... capped at restartBackoffMax.
func restartBackoff(attempt int) time.Duration {
	if attempt <= 1 {
		return restartBackoffInitial
	}
	d := restartBackoffInitial
	for i := 1; i < attempt; i++ {
		d *= 2
		if d >= restartBackoffMax {
			return restartBackoffMax
		}
	}
	return d
}

// ResetCrashLoop clears the crash-loop-abandoned flag and restart
// history for a plugin. Called by the admin UI's "Retry" / "Reset"
// button, or after the operator has fixed the underlying problem.
// Returns false if the plugin isn't registered.
func (m *Manager) ResetCrashLoop(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	rp, ok := m.plugins[name]
	if !ok {
		return false
	}
	rp.health.crashLoopAbandoned = false
	rp.health.restartAttempts = 0
	rp.health.restartHistory = nil
	rp.health.nextRestartAt = time.Time{}
	return true
}

// HealthStatus returns the current health snapshot for a named plugin.
// Returns (zero, false) if the plugin isn't registered; otherwise
// returns the most recent probe result.
func (m *Manager) HealthStatus(name string) (PluginHealth, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rp, ok := m.plugins[name]
	if !ok {
		return PluginHealth{}, false
	}
	return snapshotHealth(rp), true
}

// AllHealthStatuses returns a name→health map snapshot for every
// registered plugin, suitable for rendering a dashboard widget or
// responding to an admin API call.
func (m *Manager) AllHealthStatuses() map[string]PluginHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]PluginHealth, len(m.plugins))
	for name, rp := range m.plugins {
		out[name] = snapshotHealth(rp)
	}
	return out
}

// snapshotHealth copies the in-memory health state into the public
// PluginHealth struct. Caller must hold m.mu (read or write).
func snapshotHealth(rp *registeredPlugin) PluginHealth {
	return PluginHealth{
		Healthy:             rp.health.healthy,
		LastCheck:           rp.health.lastCheck,
		LastSuccess:         rp.health.lastSuccess,
		ConsecutiveFailures: rp.health.consecutiveFailures,
		LastError:           rp.health.lastError,
		Payload:             rp.health.payload,
		RestartAttempts:     rp.health.restartAttempts,
		LastRestartAt:       rp.health.lastRestartAt,
		NextRestartAt:       rp.health.nextRestartAt,
		CrashLoopAbandoned:  rp.health.crashLoopAbandoned,
	}
}

// healthInitForRegister seeds a freshly-registered plugin's health
// state. Called by Register/ReplacePlugin so a newly-loaded plugin
// starts out unhealthy (false) until the first probe confirms it —
// avoids false-positive "healthy" reports in the interval between
// registration and the first scheduled probe. Ensures the first
// successful probe emits the "plugin became healthy" log line, which
// is a useful startup confirmation signal in ops logs.
func healthInitForRegister() healthState {
	return healthState{
		healthy: false,
	}
}
