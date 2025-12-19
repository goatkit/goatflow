package filters

import (
	"context"
	"errors"
	"testing"
)

type recordingFilter struct {
	id     string
	calls  *[]string
	failOn bool
}

func (r recordingFilter) ID() string { return r.id }

func (r recordingFilter) Apply(_ context.Context, _ *MessageContext) error {
	*r.calls = append(*r.calls, r.id)
	if r.failOn {
		return errors.New("boom")
	}
	return nil
}

func TestChainRunsFilters(t *testing.T) {
	var calls []string
	chain := NewChain(
		recordingFilter{id: "a", calls: &calls},
		recordingFilter{id: "b", calls: &calls},
	)
	ctx := &MessageContext{}
	if err := chain.Run(context.Background(), ctx); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(calls) != 2 || calls[0] != "a" || calls[1] != "b" {
		t.Fatalf("unexpected call order %+v", calls)
	}
}

func TestChainStopsOnError(t *testing.T) {
	var calls []string
	chain := NewChain(
		recordingFilter{id: "a", calls: &calls, failOn: true},
		recordingFilter{id: "b", calls: &calls},
	)
	if err := chain.Run(context.Background(), &MessageContext{}); err == nil {
		t.Fatalf("expected error from failing filter")
	}
	if len(calls) != 1 || calls[0] != "a" {
		t.Fatalf("expected chain to stop after failure, got %+v", calls)
	}
}
