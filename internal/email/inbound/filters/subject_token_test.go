package filters

import (
	"context"
	"testing"

	"github.com/goatkit/goatflow/internal/email/inbound/connector"
)

func TestSubjectTokenFilterSetsAnnotation(t *testing.T) {
	filter := NewSubjectTokenFilter(nil)
	msg := &connector.FetchedMessage{Raw: []byte("Subject: Re: [Ticket#2025012345] Issue\r\n\r\nBody")}
	ctx := &MessageContext{Message: msg}

	if err := filter.Apply(context.Background(), ctx); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if ctx.Annotations == nil {
		t.Fatalf("expected annotations to be populated")
	}
	got, ok := ctx.Annotations[AnnotationFollowUpTicketNumber]
	if !ok {
		t.Fatalf("expected follow-up ticket annotation")
	}
	if got.(string) != "2025012345" {
		t.Fatalf("unexpected ticket number %v", got)
	}
}

func TestSubjectTokenFilterDecodesEncodedHeader(t *testing.T) {
	filter := NewSubjectTokenFilter(nil)
	encodedSubject := "Subject: =?UTF-8?Q?Re=3A_[Ticket#12345]_hi?=\r\n\r\nBody"
	msg := &connector.FetchedMessage{Raw: []byte(encodedSubject)}
	ctx := &MessageContext{Message: msg}

	if err := filter.Apply(context.Background(), ctx); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if ctx.Annotations[AnnotationFollowUpTicketNumber] != "12345" {
		t.Fatalf("expected decoded ticket id, got %+v", ctx.Annotations[AnnotationFollowUpTicketNumber])
	}
}

func TestSubjectTokenFilterIgnoresMissingToken(t *testing.T) {
	filter := NewSubjectTokenFilter(nil)
	msg := &connector.FetchedMessage{Raw: []byte("Subject: Hello world\r\n\r\nBody")}
	ctx := &MessageContext{Message: msg}

	if err := filter.Apply(context.Background(), ctx); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if ctx.Annotations != nil {
		if _, ok := ctx.Annotations[AnnotationFollowUpTicketNumber]; ok {
			t.Fatalf("expected no follow-up annotation")
		}
	}
}
