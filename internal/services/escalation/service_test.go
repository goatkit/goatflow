package escalation

import (
	"testing"
)

func TestStateTypeIDToName(t *testing.T) {
	s := &Service{}

	tests := []struct {
		typeID int
		want   string
	}{
		{1, "new"},
		{2, "open"},
		{3, "closed"},
		{4, "pending reminder"},
		{5, "pending auto"},
		{6, "removed"},
		{7, "merged"},
		{99, "open"}, // Unknown defaults to open
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := s.stateTypeIDToName(tt.typeID)
			if got != tt.want {
				t.Errorf("stateTypeIDToName(%d) = %q, want %q", tt.typeID, got, tt.want)
			}
		})
	}
}

func TestEscalationPreferencesDefault(t *testing.T) {
	prefs := &EscalationPreferences{}

	// Verify zero values
	if prefs.FirstResponseTime != 0 {
		t.Errorf("FirstResponseTime should default to 0, got %d", prefs.FirstResponseTime)
	}
	if prefs.UpdateTime != 0 {
		t.Errorf("UpdateTime should default to 0, got %d", prefs.UpdateTime)
	}
	if prefs.SolutionTime != 0 {
		t.Errorf("SolutionTime should default to 0, got %d", prefs.SolutionTime)
	}
	if prefs.Calendar != "" {
		t.Errorf("Calendar should default to empty, got %q", prefs.Calendar)
	}
}

func TestTicketInfoDefaults(t *testing.T) {
	ticket := &TicketInfo{
		ID:        1,
		QueueID:   1,
		StateType: "new",
	}

	// SLAID should be nil by default
	if ticket.SLAID != nil {
		t.Error("SLAID should be nil by default")
	}
}

func TestEscalationPreferencesWithValues(t *testing.T) {
	prefs := &EscalationPreferences{
		FirstResponseTime:   60,  // 1 hour
		FirstResponseNotify: 80,  // 80% warning
		UpdateTime:          120, // 2 hours
		UpdateNotify:        70,  // 70% warning
		SolutionTime:        480, // 8 hours
		SolutionNotify:      90,  // 90% warning
		Calendar:            "1", // Calendar 1
	}

	if prefs.FirstResponseTime != 60 {
		t.Errorf("FirstResponseTime = %d, want 60", prefs.FirstResponseTime)
	}
	if prefs.UpdateTime != 120 {
		t.Errorf("UpdateTime = %d, want 120", prefs.UpdateTime)
	}
	if prefs.SolutionTime != 480 {
		t.Errorf("SolutionTime = %d, want 480", prefs.SolutionTime)
	}
	if prefs.Calendar != "1" {
		t.Errorf("Calendar = %q, want %q", prefs.Calendar, "1")
	}
}
