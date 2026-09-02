package api

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// registerProcessMetrics is wired into the default registry once at package
// init so /metrics always exposes the goatflow_process_* gauges regardless of
// how the app was started (server, runner, tests).
func init() {
	registerProcessMetrics()
}

// HandleMetrics serves the Prometheus exposition format from the default
// registerer. Cache metrics (cache_hits_total, cache_misses_total,
// cache_errors_total, cache_set_total, cache_delete_total,
// cache_latency_seconds, cache_size) are already recorded into the default
// registry by the Valkey cache via promauto, so they appear here with no
// extra wiring.
func HandleMetrics(c *gin.Context) {
	promhttp.Handler().ServeHTTP(c.Writer, c.Request)
}

// registerProcessMetrics adds app-level gauges that are hard to derive from
// anywhere else: whether the process is up and the process start time.
// Re-registering (e.g. double init in tests) is a benign no-op — the
// existing registration simply wins.
func registerProcessMetrics() {
	reg := prometheus.DefaultRegisterer

	// goatflow_up — constant 1 while the process is alive. Prometheus'
	// standard "is the target up" signal.
	if err := reg.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "goatflow_up",
			Help: "Whether the GoatFlow process is up (1) or not (0).",
		},
		func() float64 { return 1 },
	)); err != nil {
		// Already registered — benign.
		_ = err
	}

	// goatflow_process_start_time_seconds — Unix timestamp the process
	// started; dashboards compute uptime as now() - start_time.
	if err := reg.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "goatflow_process_start_time_seconds",
			Help: "Unix timestamp (seconds) at which the process started.",
		},
		func() float64 { return float64(processStart.Unix()) },
	)); err != nil {
		// Already registered — benign.
		_ = err
	}
}
