package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/notifications"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
)

type stubTicketRepository struct {
	reminders []*models.PendingReminder
	limit     int
}

func (s *stubTicketRepository) AutoClosePendingTickets(ctx context.Context, now time.Time, transitions map[string]string, systemUserID int) (*repository.AutoCloseResult, error) {
	return nil, nil
}

func (s *stubTicketRepository) FindDuePendingReminders(ctx context.Context, now time.Time, limit int) ([]*models.PendingReminder, error) {
	s.limit = limit
	return s.reminders, nil
}

type stubReminderHub struct {
	mu         sync.Mutex
	dispatched []struct {
		recipients []int
		reminder   notifications.PendingReminder
	}
}

func (s *stubReminderHub) Dispatch(ctx context.Context, recipients []int, reminder notifications.PendingReminder) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	storedRecipients := append([]int(nil), recipients...)
	s.dispatched = append(s.dispatched, struct {
		recipients []int
		reminder   notifications.PendingReminder
	}{recipients: storedRecipients, reminder: reminder})
	return nil
}

func (s *stubReminderHub) Consume(userID int) []notifications.PendingReminder {
	s.mu.Lock()
	defer s.mu.Unlock()
	return nil
}

func TestHandlePendingReminderDispatches(t *testing.T) {
	cronEngine := cron.New(cron.WithLocation(time.UTC))
	t.Cleanup(func() { cronEngine.Stop() })

	repo := &stubTicketRepository{
		reminders: []*models.PendingReminder{{
			TicketID:          10,
			TicketNumber:      "202510161000010",
			Title:             "Follow up",
			QueueID:           2,
			QueueName:         "Support",
			PendingUntil:      time.Now().Add(-time.Minute).UTC(),
			ResponsibleUserID: intPtr(4),
			StateName:         "pending reminder",
		}},
	}
	hub := &stubReminderHub{}

	svc := NewService(nil,
		WithCron(cronEngine),
		WithTicketAutoCloser(repo),
		WithReminderHub(hub),
	)

	job := &models.ScheduledJob{Config: map[string]any{"limit": 25}}
	if err := svc.handlePendingReminder(context.Background(), job); err != nil {
		t.Fatalf("handlePendingReminder returned error: %v", err)
	}

	hub.mu.Lock()
	defer hub.mu.Unlock()
	if len(hub.dispatched) != 1 {
		t.Fatalf("expected 1 dispatch, got %d", len(hub.dispatched))
	}
	record := hub.dispatched[0]
	if len(record.recipients) != 1 || record.recipients[0] != 4 {
		t.Fatalf("unexpected recipients: %+v", record.recipients)
	}
	if record.reminder.TicketID != 10 {
		t.Fatalf("unexpected reminder ticket id %d", record.reminder.TicketID)
	}
}

func TestHandlePendingReminderOwnerFallback(t *testing.T) {
	cronEngine := cron.New(cron.WithLocation(time.UTC))
	t.Cleanup(func() { cronEngine.Stop() })

	repo := &stubTicketRepository{
		reminders: []*models.PendingReminder{{
			TicketID:     11,
			TicketNumber: "202510161000011",
			Title:        "Call customer",
			QueueID:      2,
			QueueName:    "Support",
			PendingUntil: time.Now().Add(-time.Minute).UTC(),
			OwnerUserID:  intPtr(7),
			StateName:    "pending reminder",
		}},
	}
	hub := &stubReminderHub{}

	svc := NewService(nil,
		WithCron(cronEngine),
		WithTicketAutoCloser(repo),
		WithReminderHub(hub),
	)

	job := &models.ScheduledJob{}
	if err := svc.handlePendingReminder(context.Background(), job); err != nil {
		t.Fatalf("handlePendingReminder returned error: %v", err)
	}

	hub.mu.Lock()
	defer hub.mu.Unlock()
	if len(hub.dispatched) != 1 {
		t.Fatalf("expected 1 dispatch, got %d", len(hub.dispatched))
	}
	record := hub.dispatched[0]
	if len(record.recipients) != 1 || record.recipients[0] != 7 {
		t.Fatalf("expected fallback recipient 7, got %+v", record.recipients)
	}
}

func TestHandlePendingReminderRespectsLimit(t *testing.T) {
	cronEngine := cron.New(cron.WithLocation(time.UTC))
	t.Cleanup(func() { cronEngine.Stop() })

	repo := &stubTicketRepository{}
	hub := &stubReminderHub{}

	svc := NewService(nil,
		WithCron(cronEngine),
		WithTicketAutoCloser(repo),
		WithReminderHub(hub),
	)

	job := &models.ScheduledJob{Config: map[string]any{"limit": 15}}
	if err := svc.handlePendingReminder(context.Background(), job); err != nil {
		t.Fatalf("handlePendingReminder returned error: %v", err)
	}

	if repo.limit != 15 {
		t.Fatalf("expected limit 15, got %d", repo.limit)
	}
}

func intPtr(v int) *int {
	return &v
}
