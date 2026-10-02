package tasks

import (
	"context"
	"database/sql"
	"log"
	"sync/atomic"
	"time"

	"github.com/goatkit/goatflow/internal/notificationevents"
	"github.com/goatkit/goatflow/internal/platform/runner"
)

// NotificationEventsTask evaluates the ticket notification rules (Admin ->
// Ticket Notifications) for new ticket events and queues the emails on
// mail_queue, which the email queue task sends.
type NotificationEventsTask struct {
	service *notificationevents.Service
	running atomic.Bool
}

// NewNotificationEventsTask creates the notification event task.
func NewNotificationEventsTask(db *sql.DB) runner.Task {
	logger := log.New(log.Writer(), "[NOTIFICATION-EVENTS] ", log.LstdFlags)
	return &NotificationEventsTask{service: notificationevents.NewService(db, logger)}
}

// Name returns the task name.
func (t *NotificationEventsTask) Name() string { return "notification-events" }

// Schedule runs the task every 10 seconds.
func (t *NotificationEventsTask) Schedule() string { return "*/10 * * * * *" }

// Timeout bounds one run; events left over continue next run.
func (t *NotificationEventsTask) Timeout() time.Duration { return 5 * time.Minute }

// Run evaluates the events written since the previous run. A run that starts
// while the previous one is still evaluating returns immediately.
func (t *NotificationEventsTask) Run(ctx context.Context) error {
	if !t.running.CompareAndSwap(false, true) {
		return nil
	}
	defer t.running.Store(false)
	queued, err := t.service.RunOnce(ctx)
	if queued > 0 {
		log.Printf("notification-events: queued %d emails", queued)
	}
	return err
}
