//go:build db

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/notifications"
)

type stubHub struct {
	mu    sync.Mutex
	store map[int][]notifications.PendingReminder
}

func newStubHub() *stubHub {
	return &stubHub{store: make(map[int][]notifications.PendingReminder)}
}

func (s *stubHub) Dispatch(_ context.Context, recipients []int, reminder notifications.PendingReminder) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range recipients {
		if id <= 0 {
			continue
		}
		s.store[id] = append(s.store[id], reminder)
	}
	return nil
}

func (s *stubHub) Consume(userID int) []notifications.PendingReminder {
	if userID <= 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	reminders := s.store[userID]
	delete(s.store, userID)
	return reminders
}

func TestHandlePendingReminderFeedUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/api/notifications/pending", nil)
	c.Request = req

	handlePendingReminderFeed(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("json parse failed: %v", err)
	}
	if success, _ := body["success"].(bool); success {
		t.Fatalf("expected success=false")
	}
}

func TestHandlePendingReminderFeedReturnsReminders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	stub := newStubHub()
	pending := time.Date(2025, 10, 16, 18, 0, 0, 0, time.UTC)
	stub.store[5] = []notifications.PendingReminder{{
		TicketID:     77,
		TicketNumber: "202510161000077",
		Title:        "Follow-up",
		QueueID:      3,
		QueueName:    "Escalations",
		PendingUntil: pending,
		StateName:    "pending reminder",
	}}

	prev := notifications.SetHub(stub)
	defer notifications.SetHub(prev)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/api/notifications/pending", nil)
	c.Request = req
	c.Set("user_id", uint(5))

	handlePendingReminderFeed(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("json parse failed: %v", err)
	}

	success, _ := body["success"].(bool)
	if !success {
		t.Fatalf("expected success=true")
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data payload")
	}
	reminders, ok := data["reminders"].([]any)
	if !ok || len(reminders) != 1 {
		t.Fatalf("expected 1 reminder, got %v", data["reminders"])
	}
	reminder, _ := reminders[0].(map[string]any)
	if ticket, _ := reminder["ticket_id"].(float64); ticket != 77 {
		t.Fatalf("unexpected ticket id %v", ticket)
	}
	if pendingStr, _ := reminder["pending_until"].(string); pendingStr != pending.UTC().Format(time.RFC3339) {
		t.Fatalf("unexpected pending_until %s", pendingStr)
	}

	if consumed := stub.Consume(5); len(consumed) != 0 {
		t.Fatalf("expected hub to drain reminders")
	}
}
