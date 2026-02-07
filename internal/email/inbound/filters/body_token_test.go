package filters

import (
	"context"
	"strings"
	"testing"

	"github.com/goatkit/goatflow/internal/email/inbound/connector"
)

func TestBodyTokenFilterAnnotatesPlainText(t *testing.T) {
	filter := NewBodyTokenFilter(nil)
	raw := strings.Join([]string{
		"Subject: Hi",
		"",
		"Please keep reference [Ticket#202500111] in future replies",
	}, "\r\n")
	msg := &connector.FetchedMessage{Raw: []byte(raw)}
	ctx := &MessageContext{Message: msg}
	if err := filter.Apply(context.Background(), ctx); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	got, ok := ctx.Annotations[AnnotationFollowUpTicketNumber]
	if !ok || got != "202500111" {
		t.Fatalf("expected ticket annotation, got %+v", ctx.Annotations)
	}
}

func TestBodyTokenFilterHandlesHTMLBody(t *testing.T) {
	filter := NewBodyTokenFilter(nil)
	raw := strings.Join([]string{
		"Subject: Update",
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
		"",
		"<p>Thread marker <strong>[Ticket#202588888]</strong></p>",
	}, "\r\n")
	msg := &connector.FetchedMessage{Raw: []byte(raw)}
	ctx := &MessageContext{Message: msg}
	if err := filter.Apply(context.Background(), ctx); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if ctx.Annotations[AnnotationFollowUpTicketNumber] != "202588888" {
		t.Fatalf("expected ticket 202588888, got %+v", ctx.Annotations)
	}
}

func TestBodyTokenFilterRespectsExistingAnnotation(t *testing.T) {
	filter := NewBodyTokenFilter(nil)
	raw := strings.Join([]string{
		"Subject: Hi",
		"",
		"Body [Ticket#9090]",
	}, "\r\n")
	msg := &connector.FetchedMessage{Raw: []byte(raw)}
	ctx := &MessageContext{
		Message:     msg,
		Annotations: map[string]any{AnnotationFollowUpTicketNumber: "12345"},
	}
	if err := filter.Apply(context.Background(), ctx); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if ctx.Annotations[AnnotationFollowUpTicketNumber] != "12345" {
		t.Fatalf("expected annotation to remain 12345, got %v", ctx.Annotations[AnnotationFollowUpTicketNumber])
	}
}
