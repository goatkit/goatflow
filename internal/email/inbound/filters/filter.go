package filters

import (
	"context"

	"github.com/goatkit/goatflow/internal/email/inbound/connector"
)

// MessageContext is the mutable envelope filters operate on.
type MessageContext struct {
	Account     connector.Account
	Message     *connector.FetchedMessage
	Annotations map[string]any
}

// Filter mutates a message before it hits PostMaster.
type Filter interface {
	ID() string
	Apply(ctx context.Context, m *MessageContext) error
}

// Chain executes filters in order, short-circuiting on error.
type Chain struct {
	filters []Filter
}

// NewChain returns a filter chain that runs the provided filters sequentially.
func NewChain(fs ...Filter) Chain {
	return Chain{filters: fs}
}

// Run executes the chain.
func (c Chain) Run(ctx context.Context, m *MessageContext) error {
	for _, f := range c.filters {
		if err := f.Apply(ctx, m); err != nil {
			return err
		}
	}
	return nil
}
