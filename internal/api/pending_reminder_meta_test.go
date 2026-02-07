package api

import (
	"testing"
	"time"

	"github.com/goatkit/goatflow/internal/models"
)

func TestComputePendingReminderMetaScheduled(t *testing.T) {
	due := time.Date(2025, 10, 17, 15, 0, 0, 0, time.UTC)
	ticket := &models.Ticket{UntilTime: int(due.Unix())}
	now := due.Add(-time.Hour)

	meta := computePendingReminderMeta(ticket, "pending reminder", pendingReminderStateTypeID, now)

	if !meta.pending {
		t.Fatalf("expected pending reminder state to be recognized")
	}
	if meta.at != due.Format("2006-01-02 15:04:05 UTC") {
		t.Fatalf("unexpected reminder timestamp %q", meta.at)
	}
	if meta.relative != "1h" {
		t.Fatalf("expected humanized duration to be 1h, got %q", meta.relative)
	}
	if meta.overdue {
		t.Fatalf("did not expect reminder to be marked overdue")
	}
}

func TestComputePendingReminderMetaMissingUntilTime(t *testing.T) {
	ticket := &models.Ticket{}
	now := time.Date(2025, 10, 17, 15, 0, 0, 0, time.UTC)

	meta := computePendingReminderMeta(ticket, "pending reminder", pendingReminderStateTypeID, now)

	if !meta.pending {
		t.Fatalf("expected pending reminder state to be recognized")
	}
	// When UntilTime is 0, the function now defaults to now + 24 hours
	// rather than returning a fallback label
	if !meta.hasTime {
		// hasTime should be false when no explicit time is set
	} else {
		t.Fatalf("expected hasTime to be false for missing until_time")
	}
	if meta.at == "" {
		t.Fatalf("expected at to be a formatted timestamp, got empty string")
	}
	if meta.relative == "" {
		t.Fatalf("expected relative to be set to humanized duration")
	}
	if meta.message != "Default reminder time (24h from now)" {
		t.Fatalf("expected message about default time, got %q", meta.message)
	}
}
