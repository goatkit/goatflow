package filters

import (
	"context"
	"strings"
	"testing"

	"github.com/goatkit/goatflow/internal/email/inbound/connector"
)

func TestHeaderTokenFilterDetectsGoatFlowHeader(t *testing.T) {
	filter := NewHeaderTokenFilter(nil)
	raw := strings.Join([]string{
		"Subject: Hi",
		"X-GoatFlow-TicketNumber: 20259001",
		"",
		"Body",
	}, "\r\n")
	ctx := &MessageContext{Message: &connector.FetchedMessage{Raw: []byte(raw)}}
	if err := filter.Apply(context.Background(), ctx); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if ctx.Annotations[AnnotationFollowUpTicketNumber] != "20259001" {
		t.Fatalf("expected ticket 20259001, got %+v", ctx.Annotations)
	}
}

func TestHeaderTokenFilterHandlesBracketedValue(t *testing.T) {
	filter := NewHeaderTokenFilter(nil)
	raw := strings.Join([]string{
		"Subject: Hi",
		"X-OTRS-TicketNumber: [Ticket#20253333]",
		"",
		"Body",
	}, "\r\n")
	ctx := &MessageContext{Message: &connector.FetchedMessage{Raw: []byte(raw)}}
	if err := filter.Apply(context.Background(), ctx); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if ctx.Annotations[AnnotationFollowUpTicketNumber] != "20253333" {
		t.Fatalf("expected ticket 20253333, got %+v", ctx.Annotations)
	}
}

func TestHeaderTokenFilterRespectsExistingAnnotation(t *testing.T) {
	filter := NewHeaderTokenFilter(nil)
	ctx := &MessageContext{
		Message:     &connector.FetchedMessage{Raw: []byte("Subject: Hi\r\nX-GoatFlow-TicketNumber: 111\r\n\r\nBody")},
		Annotations: map[string]any{AnnotationFollowUpTicketNumber: "555"},
	}
	if err := filter.Apply(context.Background(), ctx); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if ctx.Annotations[AnnotationFollowUpTicketNumber] != "555" {
		t.Fatalf("expected annotation 555 to remain, got %+v", ctx.Annotations)
	}
}
