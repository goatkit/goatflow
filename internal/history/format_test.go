package history

import (
	"testing"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

func TestNormalizeHistoryName_NewTicket(t *testing.T) {
	entry := models.TicketHistoryEntry{
		HistoryType:  "NewTicket",
		Name:         "%%2025082610000014%%Junk%%3 normal%%open%%2",
		QueueName:    "Junk",
		StateName:    "open",
		PriorityName: "3 normal",
	}

	got := NormalizeHistoryName(entry)
	expected := "Ticket created (#2025082610000014) • Junk • open • Priority 3 normal"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestNormalizeHistoryName_EmailAgent(t *testing.T) {
	entry := models.TicketHistoryEntry{
		HistoryType: "EmailAgent",
		Name:        "%%testuser1@gotrs.local, , ",
	}

	got := NormalizeHistoryName(entry)
	expected := "Agent email sent to testuser1@gotrs.local"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestNormalizeHistoryName_AgentNotification(t *testing.T) {
	entry := models.TicketHistoryEntry{
		HistoryType: "SendAgentNotification",
		Name:        "%%Ticket email delivery failure notification%%nigel%%Email",
	}

	got := NormalizeHistoryName(entry)
	expected := "Agent notification “Ticket email delivery failure notification” (recipient nigel, via Email)"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestNormalizeHistoryName_NoLegacyPayload(t *testing.T) {
	entry := models.TicketHistoryEntry{
		HistoryType: "TicketCreate",
		Name:        "Ticket created",
	}

	got := NormalizeHistoryName(entry)
	if got != "Ticket created" {
		t.Fatalf("unexpected normalization result: %q", got)
	}
}

func TestNormalizeHistoryName_EmptyName(t *testing.T) {
	entry := models.TicketHistoryEntry{
		HistoryType: "NewTicket",
		Name:        "",
	}

	if got := NormalizeHistoryName(entry); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestNormalizeHistoryName_IgnoresEmptyTokens(t *testing.T) {
	entry := models.TicketHistoryEntry{
		HistoryType: "NewTicket",
		Name:        "%%%%2024%%",
	}

	got := NormalizeHistoryName(entry)
	if got != "Ticket created (#2024)" {
		t.Fatalf("expected simplified token handling, got %q", got)
	}
}

func TestSplitLegacyPayload(t *testing.T) {
	parts := splitLegacyPayload("%%foo%% %%bar ,,%%baz%%")
	expected := []string{"foo", "bar", "baz"}
	if len(parts) != len(expected) {
		t.Fatalf("expected %d parts, got %d", len(expected), len(parts))
	}
	for i := range expected {
		if parts[i] != expected[i] {
			t.Fatalf("at %d expected %q got %q", i, expected[i], parts[i])
		}
	}
}

func TestFormatNewTicketFallbacks(t *testing.T) {
	entry := models.TicketHistoryEntry{
		HistoryType:  "NewTicket",
		Name:         "%%2025%%Queue%%Priority%%State%%",
		QueueName:    "", // force fallback to payload parts
		StateName:    "",
		PriorityName: "",
	}

	got := NormalizeHistoryName(entry)
	expected := "Ticket created (#2025) • Queue • State • Priority"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestNormalizeHistoryName_DefaultJoin(t *testing.T) {
	entry := models.TicketHistoryEntry{
		HistoryType: "SomeLegacyType",
		Name:        "%%foo%%bar%%baz",
	}

	got := NormalizeHistoryName(entry)
	expected := "foo • bar • baz"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestNormalizeHistoryName_TrimsCommaSpace(t *testing.T) {
	entry := models.TicketHistoryEntry{
		HistoryType: "EmailAgent",
		Name:        "%%%%foo@example.com,, %%%%",
	}

	got := NormalizeHistoryName(entry)
	expected := "Agent email sent to foo@example.com"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestNormalizeHistoryName_DoesNotChangeWhenRawIsReadable(t *testing.T) {
	entry := models.TicketHistoryEntry{
		HistoryType: "Misc",
		Name:        "Reset of unlock time.",
		CreatedAt:   time.Now(),
	}

	if NormalizeHistoryName(entry) != "Reset of unlock time." {
		t.Fatalf("expected plain string to remain unchanged")
	}
}
