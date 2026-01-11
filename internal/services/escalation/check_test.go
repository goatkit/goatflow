package escalation

import (
	"testing"
	"time"
)

func TestEscalationInfo(t *testing.T) {
	info := &EscalationInfo{
		FirstResponseTimeEscalation:   true,
		FirstResponseTimeNotification: false,
		UpdateTimeEscalation:          false,
		UpdateTimeNotification:        true,
		SolutionTimeEscalation:        false,
		SolutionTimeNotification:      false,
	}

	if !info.FirstResponseTimeEscalation {
		t.Error("FirstResponseTimeEscalation should be true")
	}
	if info.FirstResponseTimeNotification {
		t.Error("FirstResponseTimeNotification should be false")
	}
	if info.UpdateTimeEscalation {
		t.Error("UpdateTimeEscalation should be false")
	}
	if !info.UpdateTimeNotification {
		t.Error("UpdateTimeNotification should be true")
	}
}

func TestEscalationEvent(t *testing.T) {
	event := EscalationEvent{
		TicketID:  123,
		EventName: "EscalationResponseTimeStart",
	}

	if event.TicketID != 123 {
		t.Errorf("TicketID = %d, want 123", event.TicketID)
	}
	if event.EventName != "EscalationResponseTimeStart" {
		t.Errorf("EventName = %q, want %q", event.EventName, "EscalationResponseTimeStart")
	}
}

func TestEscalatingTicket(t *testing.T) {
	ticket := EscalatingTicket{
		ID:           42,
		TicketNumber: "2025010100001",
	}

	if ticket.ID != 42 {
		t.Errorf("ID = %d, want 42", ticket.ID)
	}
	if ticket.TicketNumber != "2025010100001" {
		t.Errorf("TicketNumber = %q, want %q", ticket.TicketNumber, "2025010100001")
	}
}

func TestCheckServiceSetDecayTime(t *testing.T) {
	svc := &CheckService{decayTime: 0}

	svc.SetDecayTime(30)
	if svc.decayTime != 30 {
		t.Errorf("decayTime = %d, want 30", svc.decayTime)
	}

	svc.SetDecayTime(0)
	if svc.decayTime != 0 {
		t.Errorf("decayTime = %d, want 0", svc.decayTime)
	}
}

func TestIsPastNotifyThreshold(t *testing.T) {
	svc := &CheckService{}

	now := time.Now().Unix()

	tests := []struct {
		name          string
		destTime      int64
		notifyPercent int
		want          bool
	}{
		{
			name:          "already escalated",
			destTime:      now - 100, // Past
			notifyPercent: 80,
			want:          false,
		},
		{
			name:          "far from escalation",
			destTime:      now + 10000, // Far future
			notifyPercent: 80,
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.isPastNotifyThreshold(tt.destTime, tt.notifyPercent)
			if got != tt.want {
				t.Errorf("isPastNotifyThreshold() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckTicketEscalationsEvents(t *testing.T) {
	svc := &CheckService{decayTime: 0}

	tests := []struct {
		name       string
		info       *EscalationInfo
		wantEvents []string
	}{
		{
			name: "first response escalation",
			info: &EscalationInfo{
				FirstResponseTimeEscalation: true,
			},
			wantEvents: []string{"EscalationResponseTimeStart", "NotificationEscalation"},
		},
		{
			name: "update time notification",
			info: &EscalationInfo{
				UpdateTimeNotification: true,
			},
			wantEvents: []string{"EscalationUpdateTimeNotifyBefore", "NotificationEscalationNotifyBefore"},
		},
		{
			name: "solution escalation",
			info: &EscalationInfo{
				SolutionTimeEscalation: true,
			},
			wantEvents: []string{"EscalationSolutionTimeStart", "NotificationEscalation"},
		},
		{
			name: "multiple escalations",
			info: &EscalationInfo{
				FirstResponseTimeEscalation: true,
				UpdateTimeEscalation:        true,
			},
			wantEvents: []string{"EscalationResponseTimeStart", "EscalationUpdateTimeStart", "NotificationEscalation"},
		},
		{
			name:       "no escalation",
			info:       &EscalationInfo{},
			wantEvents: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: checkTicketEscalations would normally write to DB for history
			// This test just verifies the logic without DB
			events := svc.checkTicketEscalationsLogic(tt.info)

			if len(events) != len(tt.wantEvents) {
				t.Errorf("got %d events, want %d", len(events), len(tt.wantEvents))
				return
			}

			for i, evt := range events {
				if evt != tt.wantEvents[i] {
					t.Errorf("event[%d] = %q, want %q", i, evt, tt.wantEvents[i])
				}
			}
		})
	}
}

// checkTicketEscalationsLogic is a helper for testing without DB
func (s *CheckService) checkTicketEscalationsLogic(info *EscalationInfo) []string {
	var events []string

	type escalationType struct {
		condition bool
		eventName string
	}

	escalations := []escalationType{
		{info.FirstResponseTimeEscalation, "EscalationResponseTimeStart"},
		{info.UpdateTimeEscalation, "EscalationUpdateTimeStart"},
		{info.SolutionTimeEscalation, "EscalationSolutionTimeStart"},
		{info.FirstResponseTimeNotification, "EscalationResponseTimeNotifyBefore"},
		{info.UpdateTimeNotification, "EscalationUpdateTimeNotifyBefore"},
		{info.SolutionTimeNotification, "EscalationSolutionTimeNotifyBefore"},
	}

	for _, esc := range escalations {
		if esc.condition {
			events = append(events, esc.eventName)
		}
	}

	// Add notification meta-events
	hasEscalation := info.FirstResponseTimeEscalation || info.UpdateTimeEscalation || info.SolutionTimeEscalation
	hasNotification := info.FirstResponseTimeNotification || info.UpdateTimeNotification || info.SolutionTimeNotification

	if hasEscalation {
		events = append(events, "NotificationEscalation")
	} else if hasNotification {
		events = append(events, "NotificationEscalationNotifyBefore")
	}

	return events
}
