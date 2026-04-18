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
// Plugins that want to do real health work (check DB connectivity,
// external dependencies, etc.) can implement a handler for this name
// in their Call dispatch and return a richer status. The manager
// currently ignores the response body and only cares that something
// came back in time.
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

// PluginHealth is a snapshot of a plugin's current health state.
// Exposed via Manager.HealthStatus for admin UIs / dashboards.
type PluginHealth struct {
	Healthy             bool      `json:"healthy"`
	LastCheck           time.Time `json:"last_check"`
	LastSuccess         time.Time `json:"last_success,omitempty"`
	ConsecutiveFailures int       `json:"consecutive_failures"`
	LastError           string    `json:"last_error,omitempty"`
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
// The checker does NOT auto-restart unhealthy plugins. That will be
// added in a future release once the restart-policy design is
// settled (backoff shape, crash-loop guard, interaction with
// hot-reload-on-binary-change). For now the job is to surface bad
// state via logs + HealthStatus so operators notice, and so the
// admin UI can render a warning.
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
			}
		}
	}()

	return func() {
		close(stop)
		<-done
	}
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
	_, err := rp.plugin.Call(ctx, HealthPingFunc, json.RawMessage(nil))

	// Only a context-deadline-exceeded indicates the plugin failed to
	// respond in time (i.e. is zombie / wedged). Any other error —
	// including "unknown function: __health_ping__" from plugins that
	// don't recognise the name — means the RPC round-trip worked and
	// the plugin is alive.
	failed := ctx.Err() == context.DeadlineExceeded

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
	return PluginHealth{
		Healthy:             rp.health.healthy,
		LastCheck:           rp.health.lastCheck,
		LastSuccess:         rp.health.lastSuccess,
		ConsecutiveFailures: rp.health.consecutiveFailures,
		LastError:           rp.health.lastError,
	}, true
}

// AllHealthStatuses returns a name→health map snapshot for every
// registered plugin, suitable for rendering a dashboard widget or
// responding to an admin API call.
func (m *Manager) AllHealthStatuses() map[string]PluginHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]PluginHealth, len(m.plugins))
	for name, rp := range m.plugins {
		out[name] = PluginHealth{
			Healthy:             rp.health.healthy,
			LastCheck:           rp.health.lastCheck,
			LastSuccess:         rp.health.lastSuccess,
			ConsecutiveFailures: rp.health.consecutiveFailures,
			LastError:           rp.health.lastError,
		}
	}
	return out
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

