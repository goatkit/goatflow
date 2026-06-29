package notifications

import (
	"context"
	"testing"
	"time"
)

func TestMemoryHubDispatchAndConsume(t *testing.T) {
	hub := NewMemoryHub()

	reminder := PendingReminder{
		TicketID:     101,
		TicketNumber: "202510161000009",
		Title:        "Pending reminder",
		QueueID:      2,
		QueueName:    "Support",
		PendingUntil: time.Date(2025, 10, 16, 11, 0, 0, 0, time.UTC),
		StateName:    "pending reminder",
	}

	if err := hub.Dispatch(context.Background(), []int{7}, reminder); err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	out := hub.Consume(7)
	if len(out) != 1 {
		t.Fatalf("expected 1 reminder, got %d", len(out))
	}
	if out[0].TicketID != 101 {
		t.Fatalf("unexpected ticket id %d", out[0].TicketID)
	}

	// Second consume should be empty (queue cleared).
	out = hub.Consume(7)
	if len(out) != 0 {
		t.Fatalf("expected queue to be empty")
	}
}

func TestMemoryHubDeduplicatesByTicket(t *testing.T) {
	hub := NewMemoryHub()

	first := PendingReminder{TicketID: 1, TicketNumber: "TN1", Title: "First"}
	updated := PendingReminder{TicketID: 1, TicketNumber: "TN1", Title: "Updated"}

	if err := hub.Dispatch(context.Background(), []int{5}, first); err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}
	if err := hub.Dispatch(context.Background(), []int{5}, updated); err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	out := hub.Consume(5)
	if len(out) != 1 {
		t.Fatalf("expected 1 reminder after dedupe, got %d", len(out))
	}
	if out[0].Title != "Updated" {
		t.Fatalf("expected updated reminder, got %s", out[0].Title)
	}
}
