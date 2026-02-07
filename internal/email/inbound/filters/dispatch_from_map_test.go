package filters

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/goatkit/goatflow/internal/email/inbound/connector"
)

type stubDispatchProvider struct {
	rules map[int][]DispatchRule
}

func (s stubDispatchProvider) RulesFor(accountID int) []DispatchRule {
	return append([]DispatchRule(nil), s.rules[accountID]...)
}

func TestDispatchFromMapFilterMatchesQueueID(t *testing.T) {
	provider := stubDispatchProvider{rules: map[int][]DispatchRule{
		42: {
			{Match: "*@vip.example.com", QueueID: 7, PriorityID: 5},
		},
	}}
	filter := NewDispatchFromMapFilter(provider, nil)
	msg := &connector.FetchedMessage{Raw: []byte("From: Jane <agent@vip.example.com>\r\n\r\nbody")}
	msg.WithAccount(connector.Account{ID: 42, DispatchingMode: "from"})
	ctx := &MessageContext{Account: msg.AccountSnapshot(), Message: msg, Annotations: map[string]any{}}
	if err := filter.Apply(context.Background(), ctx); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if got := ctx.Annotations[AnnotationQueueIDOverride]; got != 7 {
		t.Fatalf("expected queue id override 7, got %v", got)
	}
	if got := ctx.Annotations[AnnotationPriorityIDOverride]; got != 5 {
		t.Fatalf("expected priority override 5, got %v", got)
	}
}

func TestDispatchFromMapFilterRespectsQueueMode(t *testing.T) {
	provider := stubDispatchProvider{rules: map[int][]DispatchRule{
		1: {{Match: "*", QueueID: 9}},
	}}
	filter := NewDispatchFromMapFilter(provider, nil)
	msg := &connector.FetchedMessage{Raw: []byte("From: ops@example.com\r\n\r\nbody")}
	msg.WithAccount(connector.Account{ID: 1, DispatchingMode: "queue"})
	ctx := &MessageContext{Account: msg.AccountSnapshot(), Message: msg, Annotations: map[string]any{}}
	if err := filter.Apply(context.Background(), ctx); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if len(ctx.Annotations) != 0 {
		t.Fatalf("expected no overrides for queue dispatching mode")
	}
}

func TestDispatchFromMapFilterQueueNameOverride(t *testing.T) {
	provider := stubDispatchProvider{rules: map[int][]DispatchRule{
		5: {{Match: "*@example.com", QueueName: "VIP"}},
	}}
	filter := NewDispatchFromMapFilter(provider, nil)
	msg := &connector.FetchedMessage{Raw: []byte("From: Root <root@example.com>\r\n\r\nbody")}
	msg.WithAccount(connector.Account{ID: 5, DispatchingMode: "from"})
	ctx := &MessageContext{Account: msg.AccountSnapshot(), Message: msg, Annotations: map[string]any{}}
	if err := filter.Apply(context.Background(), ctx); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if got := ctx.Annotations[AnnotationQueueNameOverride]; got != "VIP" {
		t.Fatalf("expected queue name override VIP, got %v", got)
	}
}

func TestFileDispatchRuleProviderLoadsConfig(t *testing.T) {
	content := []byte(`accounts:
  "42":
    - match: "*@vip.example.com"
      queue: "Premium"
      priority_id: 4
  7:
    - match: "*@corp.example.com"
      queue_id: 12
`)
	tmp := t.TempDir()
	path := filepath.Join(tmp, "dispatch.yaml")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	provider, err := NewFileDispatchRuleProvider(path)
	if err != nil {
		t.Fatalf("NewFileDispatchRuleProvider returned error: %v", err)
	}
	rules := provider.RulesFor(42)
	if len(rules) != 1 || rules[0].QueueName != "Premium" || rules[0].PriorityID != 4 {
		t.Fatalf("unexpected rules %+v", rules)
	}
	if rules := provider.RulesFor(7); len(rules) != 1 || rules[0].QueueID != 12 {
		t.Fatalf("expected queue id 12, got %+v", rules)
	}
}

func TestFileDispatchRuleProviderMissingFile(t *testing.T) {
	provider, err := NewFileDispatchRuleProvider("/path/does/not/exist.yaml")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if provider != nil {
		t.Fatalf("expected nil provider for missing file")
	}
}
