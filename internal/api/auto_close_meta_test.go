package api

import (
	"testing"
	"time"

	"github.com/goatkit/goatflow/internal/models"
)

func TestComputeAutoCloseMetaPendingAutoWithoutUntilTime(t *testing.T) {
	ticket := &models.Ticket{
		TicketNumber:     "202510161000008",
		TicketStateID:    7,
		TicketPriorityID: 3,
		UntilTime:        0,
	}

	now := time.Date(2025, 10, 16, 0, 0, 0, 0, time.UTC)
	meta := computeAutoCloseMeta(ticket, "pending auto close+", pendingAutoStateTypeID, now)

	if !meta.pending {
		t.Fatalf("expected pending auto state to be recognized")
	}
	if meta.at == "" {
		t.Fatalf("expected auto-close display metadata even when until_time is zero")
	}
}
