// Package postmaster provides email parsing, filtering, and dispatch orchestration.
package postmaster

import (
	"context"

	"github.com/gotrs-io/gotrs-ce/internal/email/inbound/connector"
	"github.com/gotrs-io/gotrs-ce/internal/email/inbound/filters"
)

// Processor orchestrates PostMaster-style parsing, filtering, and dispatching.
type Processor interface {
	Process(ctx context.Context, msg *connector.FetchedMessage, meta *filters.MessageContext) (Result, error)
}

// Result tracks what happened to a message.
type Result struct {
	TicketID  int
	ArticleID int
	Action    string // new_ticket, follow_up, ignored, bounced, etc.
	Err       error
}

// Service wires connectors, filters, and ticket services together.
type Service struct {
	FilterChain filters.Chain
	Handler     Processor
}

// Handle implements connector.Handler by running the filter chain then processor.
func (s Service) Handle(ctx context.Context, msg *connector.FetchedMessage) error {
	ctxMsg := &filters.MessageContext{
		Account:     msg.AccountSnapshot(),
		Message:     msg,
		Annotations: map[string]any{},
	}
	if err := s.FilterChain.Run(ctx, ctxMsg); err != nil {
		return err
	}
	_, err := s.Handler.Process(ctx, msg, ctxMsg)
	return err
}
