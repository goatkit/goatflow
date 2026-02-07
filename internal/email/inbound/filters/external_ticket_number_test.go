package filters

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/goatkit/goatflow/internal/email/inbound/connector"
)

func TestExternalTicketNumberFilterMatchesSubject(t *testing.T) {
	rule := ExternalTicketRule{
		Name:          "subject",
		Pattern:       regexp.MustCompile(`Case ([0-9]{8})`),
		SearchSubject: true,
	}
	filter := NewExternalTicketNumberFilter([]ExternalTicketRule{rule}, nil)
	if filter == nil {
		t.Fatalf("expected filter instance")
	}
	raw := strings.Join([]string{
		"Subject: Vendor Case 20251234",
		"",
		"Body",
	}, "\r\n")
	ctx := &MessageContext{Message: &connector.FetchedMessage{Raw: []byte(raw)}}
	if err := filter.Apply(context.Background(), ctx); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if ctx.Annotations[AnnotationFollowUpTicketNumber] != "20251234" {
		t.Fatalf("expected ticket 20251234, got %+v", ctx.Annotations)
	}
}

func TestExternalTicketNumberFilterMatchesBody(t *testing.T) {
	rule := ExternalTicketRule{
		Name:       "body",
		Pattern:    regexp.MustCompile(`Case#([0-9]{4})`),
		SearchBody: true,
	}
	filter := NewExternalTicketNumberFilter([]ExternalTicketRule{rule}, nil)
	raw := strings.Join([]string{
		"Subject: Update",
		"",
		"Case#7788 escalation",
	}, "\r\n")
	ctx := &MessageContext{Message: &connector.FetchedMessage{Raw: []byte(raw)}}
	if err := filter.Apply(context.Background(), ctx); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if ctx.Annotations[AnnotationFollowUpTicketNumber] != "7788" {
		t.Fatalf("expected ticket 7788, got %+v", ctx.Annotations)
	}
}

func TestExternalTicketNumberFilterMatchesHeader(t *testing.T) {
	rule := ExternalTicketRule{
		Name:    "header",
		Pattern: regexp.MustCompile(`CaseID-([0-9]+)`),
		Headers: []string{"X-Vendor-Case"},
	}
	filter := NewExternalTicketNumberFilter([]ExternalTicketRule{rule}, nil)
	raw := strings.Join([]string{
		"Subject: Ping",
		"X-Vendor-Case: CaseID-4455",
		"",
		"Body",
	}, "\r\n")
	ctx := &MessageContext{Message: &connector.FetchedMessage{Raw: []byte(raw)}}
	if err := filter.Apply(context.Background(), ctx); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if ctx.Annotations[AnnotationFollowUpTicketNumber] != "4455" {
		t.Fatalf("expected ticket 4455, got %+v", ctx.Annotations)
	}
}

func TestExternalTicketNumberFilterRespectsExistingTicket(t *testing.T) {
	rule := ExternalTicketRule{
		Name:          "subject",
		Pattern:       regexp.MustCompile(`Case ([0-9]+)`),
		SearchSubject: true,
	}
	filter := NewExternalTicketNumberFilter([]ExternalTicketRule{rule}, nil)
	ctx := &MessageContext{
		Message:     &connector.FetchedMessage{Raw: []byte("Subject: Case 123\r\n\r\nBody")},
		Annotations: map[string]any{AnnotationFollowUpTicketNumber: "999"},
	}
	if err := filter.Apply(context.Background(), ctx); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if ctx.Annotations[AnnotationFollowUpTicketNumber] != "999" {
		t.Fatalf("expected annotation to remain 999, got %+v", ctx.Annotations)
	}
}
