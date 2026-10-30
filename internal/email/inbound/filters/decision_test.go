package filters

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/goatkit/goatflow/internal/platform/email/inbound/connector"
	"github.com/goatkit/goatflow/internal/platform/sysconfig"
)

type fakeDecider struct {
	resp  string
	err   error
	calls int
	args  []byte
	ctx   context.Context
}

func (f *fakeDecider) Call(ctx context.Context, pluginName, fn string, args []byte) ([]byte, error) {
	f.calls++
	f.ctx = ctx
	f.args = args
	if pluginName != decisionPlugin || fn != decisionFunction {
		return nil, errors.New("unexpected function " + pluginName + "." + fn)
	}
	return []byte(f.resp), f.err
}

const decisionMail = "From: Ann Example <ann@example.com>\r\nSubject: =?UTF-8?Q?Charged_twice_=E2=80=93_refund?=\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nMy card was charged twice for order A-104. Please refund the duplicate.\r\n"

func decisionConfig() sysconfig.DecisionFilterConfig {
	cfg := sysconfig.DefaultDecisionFilterConfig()
	cfg.Enabled = true
	cfg.ProviderID = 7
	cfg.Threshold = 0.6
	// Option names out of alphabetical order: the request must keep them.
	cfg.Queue = sysconfig.DecisionFilterField{Enabled: true, Options: []sysconfig.DecisionFilterOption{
		{Name: "Support::Technical", Description: "Bugs and outages"},
		{Name: "Billing", Description: "Charges, invoices, refunds"},
		{Name: "Accounts"},
	}}
	cfg.Priority = sysconfig.DecisionFilterField{Enabled: true, Instructions: "How urgent is this?", Options: []sysconfig.DecisionFilterOption{
		{Name: "3 normal"}, {Name: "4 high"},
	}}
	cfg.Type = sysconfig.DecisionFilterField{Enabled: false, Options: []sysconfig.DecisionFilterOption{{Name: "Incident"}, {Name: "Request"}}}
	return cfg
}

func runDecision(t *testing.T, ctx context.Context, cfg sysconfig.DecisionFilterConfig, caller PluginCaller, annotations map[string]any) *MessageContext {
	t.Helper()
	f := NewDecisionFilter(func() PluginCaller { return caller }, func() (sysconfig.DecisionFilterConfig, error) { return cfg, nil }, nil)
	m := &MessageContext{Message: &connector.FetchedMessage{UID: "u1", Raw: []byte(decisionMail)}, Annotations: annotations}
	if err := f.Apply(ctx, m); err != nil {
		t.Fatalf("Apply must fail open, got %v", err)
	}
	return m
}

func TestDecisionFilterAppliesConfidentAnswers(t *testing.T) {
	caller := &fakeDecider{resp: `{"answers":{
		"queue":{"type":"choice","choice":"Billing","confidence":0.91},
		"priority":{"type":"choice","choice":"4 high","confidence":0.42}}}`}
	m := runDecision(t, context.Background(), decisionConfig(), caller, map[string]any{})

	if got := m.Annotations[AnnotationQueueNameOverride]; got != "Billing" {
		t.Errorf("queue = %v, want Billing", got)
	}
	if _, ok := m.Annotations[AnnotationPriorityNameOverride]; ok {
		t.Errorf("priority below threshold 0.6 was applied: %v", m.Annotations[AnnotationPriorityNameOverride])
	}

	var req struct {
		ProviderID     int64 `json:"provider_id"`
		TimeoutSeconds int   `json:"timeout_seconds"`
		State          struct{ From, Subject, Body string }
		Questions      map[string]json.RawMessage
	}
	if err := json.Unmarshal(caller.args, &req); err != nil {
		t.Fatalf("request is not JSON: %v\n%s", err, caller.args)
	}
	if req.ProviderID != 7 || req.TimeoutSeconds != sysconfig.DecisionFilterDefaultTimeout {
		t.Errorf("provider_id=%d timeout_seconds=%d", req.ProviderID, req.TimeoutSeconds)
	}
	if req.State.Subject != "Charged twice – refund" || !strings.Contains(req.State.From, "ann@example.com") || !strings.HasPrefix(req.State.Body, "My card was charged twice") {
		t.Errorf("state = %+v", req.State)
	}
	if _, ok := req.Questions["type"]; ok || len(req.Questions) != 2 {
		t.Errorf("questions = %v, want queue and priority only", req.Questions)
	}
	// The option order fixes the answer letters, so it must be the configured
	// order, not Go's sorted map order.
	wantQueue := `{"type":"choice","instructions":"Which queue should handle this email?","criteria":{"Support::Technical":"Bugs and outages","Billing":"Charges, invoices, refunds","Accounts":null}}`
	if got := string(req.Questions["queue"]); got != wantQueue {
		t.Errorf("queue question\n got %s\nwant %s", got, wantQueue)
	}
	if got := string(req.Questions["priority"]); !strings.Contains(got, `"instructions":"How urgent is this?"`) {
		t.Errorf("priority question lost its instructions: %s", got)
	}
}

func TestDecisionFilterLeavesExplicitRulesAlone(t *testing.T) {
	caller := &fakeDecider{resp: `{"answers":{"priority":{"choice":"4 high","confidence":0.9}}}`}
	m := runDecision(t, context.Background(), decisionConfig(), caller, map[string]any{AnnotationQueueIDOverride: 3})

	if got := m.Annotations[AnnotationQueueIDOverride]; got != 3 {
		t.Errorf("queue id override changed to %v", got)
	}
	if _, ok := m.Annotations[AnnotationQueueNameOverride]; ok {
		t.Error("decision set a queue name despite an explicit queue rule")
	}
	if strings.Contains(string(caller.args), `"queue"`) {
		t.Errorf("queue was asked although a rule decided it: %s", caller.args)
	}
	if got := m.Annotations[AnnotationPriorityNameOverride]; got != "4 high" {
		t.Errorf("priority = %v, want 4 high", got)
	}
}

func TestDecisionFilterSkipsWithoutCalling(t *testing.T) {
	disabled := decisionConfig()
	disabled.Enabled = false
	nothingLeft := decisionConfig()
	nothingLeft.Priority.Enabled = false

	soon, cancel := context.WithTimeout(context.Background(), decisionPollReserve+time.Second)
	defer cancel()

	cases := map[string]struct {
		ctx         context.Context
		cfg         sysconfig.DecisionFilterConfig
		annotations map[string]any
	}{
		"disabled":                {context.Background(), disabled, map[string]any{}},
		"follow-up":               {context.Background(), decisionConfig(), map[string]any{AnnotationFollowUpTicketNumber: "2026101810000012"}},
		"ignored":                 {context.Background(), decisionConfig(), map[string]any{AnnotationIgnoreMessage: true}},
		"all fields decided":      {context.Background(), nothingLeft, map[string]any{AnnotationQueueNameOverride: "Raw"}},
		"poll deadline too close": {soon, decisionConfig(), map[string]any{}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			caller := &fakeDecider{resp: `{"answers":{}}`}
			runDecision(t, c.ctx, c.cfg, caller, c.annotations)
			if caller.calls != 0 {
				t.Errorf("llm_decide called %d times", caller.calls)
			}
		})
	}
}

func TestDecisionFilterCallTimeoutLeavesPollReserve(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), decisionPollReserve+10*time.Second)
	defer cancel()
	caller := &fakeDecider{resp: `{"answers":{}}`}
	runDecision(t, ctx, decisionConfig(), caller, map[string]any{})

	deadline, ok := caller.ctx.Deadline()
	if !ok {
		t.Fatal("call has no deadline")
	}
	if left := time.Until(deadline); left > 10*time.Second {
		t.Errorf("call may run %s, past the poll reserve", left)
	}
	if !strings.Contains(string(caller.args), `"timeout_seconds":9`) && !strings.Contains(string(caller.args), `"timeout_seconds":10`) {
		t.Errorf("plugin timeout not shortened to the poll budget: %s", caller.args)
	}
}

func TestDecisionFilterFailsOpen(t *testing.T) {
	cases := map[string]PluginCaller{
		"plugin missing":    &fakeDecider{err: errors.New(`plugin "goatkit-llm" not found`)},
		"error body":        &fakeDecider{resp: `{"error":"no active decide provider","status":503}`},
		"unreadable body":   &fakeDecider{resp: `<html>`},
		"no answers":        &fakeDecider{resp: `{}`},
		"unknown option":    &fakeDecider{resp: `{"answers":{"queue":{"choice":"Junk","confidence":0.99}}}`},
		"no plugin manager": nil,
	}
	for name, caller := range cases {
		t.Run(name, func(t *testing.T) {
			m := runDecision(t, context.Background(), decisionConfig(), caller, map[string]any{})
			if len(m.Annotations) != 0 {
				t.Errorf("annotations set: %v", m.Annotations)
			}
		})
	}
}

func TestDecisionFilterConfigErrorSkips(t *testing.T) {
	caller := &fakeDecider{resp: `{"answers":{}}`}
	f := NewDecisionFilter(func() PluginCaller { return caller }, func() (sysconfig.DecisionFilterConfig, error) {
		return sysconfig.DecisionFilterConfig{Enabled: true}, errors.New("bad json")
	}, nil)
	m := &MessageContext{Message: &connector.FetchedMessage{Raw: []byte(decisionMail)}, Annotations: map[string]any{}}
	if err := f.Apply(context.Background(), m); err != nil || caller.calls != 0 {
		t.Fatalf("err=%v calls=%d, want skip", err, caller.calls)
	}
}
