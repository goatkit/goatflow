package postmaster

import (
    "context"
    "errors"
    "testing"

    "github.com/gotrs-io/gotrs-ce/internal/email/inbound/connector"
    "github.com/gotrs-io/gotrs-ce/internal/email/inbound/filters"
)

type stubProcessor struct {
	meta *filters.MessageContext
	msg  *connector.FetchedMessage
}

func (s *stubProcessor) Process(_ context.Context, msg *connector.FetchedMessage, meta *filters.MessageContext) (Result, error) {
	s.meta = meta
	s.msg = msg
	return Result{Action: "ok"}, nil
}

type stubFilter struct {
	err error
}

func (f stubFilter) ID() string { return "stub" }

func (f stubFilter) Apply(_ context.Context, m *filters.MessageContext) error {
	if f.err != nil {
		return f.err
	}
	m.Annotations["seen"] = true
	return nil
}

func TestServiceHandleRunsChainAndProcessor(t *testing.T) {
	proc := &stubProcessor{}
	svc := Service{
		FilterChain: filters.NewChain(stubFilter{}),
		Handler:     proc,
	}
	msg := &connector.FetchedMessage{Raw: []byte("Subject: hi\r\n\r\nBody")}
	msg.WithAccount(connector.Account{ID: 1})
	if err := svc.Handle(context.Background(), msg); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if proc.meta == nil || proc.msg == nil {
		t.Fatalf("expected processor to receive inputs")
	}
	if !proc.meta.Annotations["seen"].(bool) {
		t.Fatalf("expected filter to annotate context")
	}
	if proc.meta.Account.ID != 1 {
		t.Fatalf("expected account snapshot to propagate")
	}
}

func TestServiceHandlePropagatesFilterError(t *testing.T) {
	svc := Service{
		FilterChain: filters.NewChain(stubFilter{err: errors.New("fail")}),
		Handler:     &stubProcessor{},
	}
	msg := &connector.FetchedMessage{Raw: []byte("Subject: hi\r\n\r\nBody")}
	if err := svc.Handle(context.Background(), msg); err == nil {
		t.Fatalf("expected filter error to propagate")
	}
}
