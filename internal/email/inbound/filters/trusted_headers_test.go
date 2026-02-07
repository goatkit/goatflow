package filters

import (
	"context"
	"testing"

	"github.com/goatkit/goatflow/internal/email/inbound/connector"
)

func TestTrustedHeadersFilterAppliesOverrides(t *testing.T) {
	filter := NewTrustedHeadersFilter(nil)
	msg := &connector.FetchedMessage{Raw: []byte("X-OTRS-QueueName: Support\r\nX-GoatFlow-QueueID: 12\r\nX-OTRS-PriorityID: 5\r\nX-GoatFlow-Title: =?utf-8?B?VGVzdA==?=\r\nX-OTRS-CustomerUserID: vip@example.com\r\nX-OTRS-CustomerID: VIP\r\n\r\nbody")}
	msg.WithAccount(connector.Account{AllowTrustedHeaders: true})
	ctx := &MessageContext{Account: msg.AccountSnapshot(), Message: msg, Annotations: map[string]any{}}
	if err := filter.Apply(context.Background(), ctx); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if got, ok := ctx.Annotations[AnnotationQueueIDOverride].(int); !ok || got != 12 {
		t.Fatalf("expected queue id override 12, got %v", ctx.Annotations[AnnotationQueueIDOverride])
	}
	if got, ok := ctx.Annotations[AnnotationQueueNameOverride].(string); !ok || got != "Support" {
		t.Fatalf("expected queue name override Support, got %v", ctx.Annotations[AnnotationQueueNameOverride])
	}
	if got, ok := ctx.Annotations[AnnotationPriorityIDOverride].(int); !ok || got != 5 {
		t.Fatalf("expected priority override 5, got %v", ctx.Annotations[AnnotationPriorityIDOverride])
	}
	if got, ok := ctx.Annotations[AnnotationTitleOverride].(string); !ok || got != "Test" {
		t.Fatalf("expected title override decoded, got %v", ctx.Annotations[AnnotationTitleOverride])
	}
	if got, ok := ctx.Annotations[AnnotationCustomerUserOverride].(string); !ok || got != "vip@example.com" {
		t.Fatalf("expected customer user override, got %v", ctx.Annotations[AnnotationCustomerUserOverride])
	}
	if got, ok := ctx.Annotations[AnnotationCustomerIDOverride].(string); !ok || got != "VIP" {
		t.Fatalf("expected customer id override, got %v", ctx.Annotations[AnnotationCustomerIDOverride])
	}
}

func TestTrustedHeadersFilterSkipsWhenNotAllowed(t *testing.T) {
	filter := NewTrustedHeadersFilter(nil)
	msg := &connector.FetchedMessage{Raw: []byte("X-GoatFlow-Queue: Support\r\n\r\nBody")}
	msg.WithAccount(connector.Account{AllowTrustedHeaders: false})
	ctx := &MessageContext{Account: msg.AccountSnapshot(), Message: msg, Annotations: map[string]any{}}
	if err := filter.Apply(context.Background(), ctx); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if len(ctx.Annotations) != 0 {
		t.Fatalf("expected no annotations when trusted headers disabled")
	}
}

func TestTrustedHeadersFilterCapturesIgnoreFlag(t *testing.T) {
	filter := NewTrustedHeadersFilter(nil)
	msg := &connector.FetchedMessage{Raw: []byte("X-OTRS-Ignore: yes\r\n\r\nBody")}
	msg.WithAccount(connector.Account{AllowTrustedHeaders: true})
	ctx := &MessageContext{Account: msg.AccountSnapshot(), Message: msg, Annotations: map[string]any{}}
	if err := filter.Apply(context.Background(), ctx); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	val, ok := ctx.Annotations[AnnotationIgnoreMessage]
	if !ok {
		t.Fatalf("expected ignore annotation to be set")
	}
	boolVal, _ := val.(bool)
	if !boolVal {
		t.Fatalf("expected ignore annotation true, got %v", val)
	}
}

func TestTrustedHeadersFilterCapturesCustomHeaders(t *testing.T) {
	filter := NewTrustedHeadersFilter(nil, "X-GoatFlow-Tag", "x-custom-flag")
	msg := &connector.FetchedMessage{Raw: []byte("X-GoatFlow-Tag: vip\r\nx-custom-flag: true\r\n\r\nBody")}
	msg.WithAccount(connector.Account{AllowTrustedHeaders: true})
	ctx := &MessageContext{Account: msg.AccountSnapshot(), Message: msg, Annotations: map[string]any{}}
	if err := filter.Apply(context.Background(), ctx); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if got := ctx.Annotations[annotationTrustedHeaderKey("X-GoatFlow-Tag")]; got != "vip" {
		t.Fatalf("expected custom header tag, got %v", got)
	}
	if got := ctx.Annotations[annotationTrustedHeaderKey("x-custom-flag")]; got != "true" {
		t.Fatalf("expected custom header flag, got %v", got)
	}
}
