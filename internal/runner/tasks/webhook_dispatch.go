package tasks

import (
	"context"
	"database/sql"
	"log"
	"sync/atomic"
	"time"

	"github.com/goatkit/goatflow/internal/platform/runner"
	"github.com/goatkit/goatflow/internal/webhooks"
)

// WebhookDispatchTask publishes ticket/article events to outbound webhooks and
// sends due deliveries (first attempts and retries).
type WebhookDispatchTask struct {
	service *webhooks.Service
	running atomic.Bool
}

// NewWebhookDispatchTask creates the webhook dispatch task.
func NewWebhookDispatchTask(db *sql.DB) runner.Task {
	return &WebhookDispatchTask{service: webhooks.NewService(db)}
}

// Name returns the task name.
func (t *WebhookDispatchTask) Name() string { return "webhook-dispatch" }

// Schedule runs the task every 10 seconds.
func (t *WebhookDispatchTask) Schedule() string { return "*/10 * * * * *" }

// Timeout bounds one run; sends still pending afterwards continue next run.
func (t *WebhookDispatchTask) Timeout() time.Duration { return 5 * time.Minute }

// Run publishes new events and sends due deliveries. A run that starts while
// the previous one is still sending returns immediately.
func (t *WebhookDispatchTask) Run(ctx context.Context) error {
	if !t.running.CompareAndSwap(false, true) {
		return nil
	}
	defer t.running.Store(false)
	queued, attempted, err := t.service.RunOnce(ctx)
	if queued > 0 || attempted > 0 {
		log.Printf("webhook-dispatch: queued %d deliveries, attempted %d", queued, attempted)
	}
	return err
}
