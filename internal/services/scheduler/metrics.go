package scheduler

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/gotrs-io/gotrs-ce/internal/email/inbound/connector"
)

type emailPollMetrics struct {
	runs           prometheus.Counter
	activeAccounts prometheus.Gauge
	processed      *prometheus.CounterVec
	durations      prometheus.Observer
}

var (
	emailPollMetricsOnce sync.Once
	emailPollMetricsInst *emailPollMetrics
)

func globalEmailPollMetrics() *emailPollMetrics {
	emailPollMetricsOnce.Do(func() {
		emailPollMetricsInst = newEmailPollMetrics()
	})
	return emailPollMetricsInst
}

func newEmailPollMetrics() *emailPollMetrics {
	return &emailPollMetrics{
		runs: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "gotrs",
			Subsystem: "scheduler",
			Name:      "email_poll_runs_total",
			Help:      "Total email poller executions",
		}),
		activeAccounts: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "gotrs",
			Subsystem: "scheduler",
			Name:      "email_poll_active_accounts",
			Help:      "Active mailboxes observed during the latest poll",
		}),
		processed: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "gotrs",
			Subsystem: "scheduler",
			Name:      "email_poll_accounts_total",
			Help:      "Accounts processed by the email poller, labeled by result and connector",
		}, []string{"status", "connector"}),
		durations: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: "gotrs",
			Subsystem: "scheduler",
			Name:      "email_poll_duration_seconds",
			Help:      "Duration of email poller executions",
			Buckets:   prometheus.DefBuckets,
		}),
	}
}

func (m *emailPollMetrics) recordRun(active int) func() {
	if m == nil {
		return func() {}
	}
	m.runs.Inc()
	m.activeAccounts.Set(float64(active))
	timer := prometheus.NewTimer(m.durations)
	return func() {
		timer.ObserveDuration()
	}
}

func (m *emailPollMetrics) recordAccount(account connector.Account, success bool) {
	if m == nil {
		return
	}
	status := "success"
	if !success {
		status = "failure"
	}
	connectorName := account.Type
	if connectorName == "" {
		connectorName = "unknown"
	}
	m.processed.WithLabelValues(status, connectorName).Inc()
}
