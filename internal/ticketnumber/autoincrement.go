// Package ticketnumber provides ticket number generation implementations.
package ticketnumber

import (
	"context"
	"fmt"
)

type AutoIncrement struct{ cfg Config }

func NewAutoIncrement(cfg Config) *AutoIncrement { return &AutoIncrement{cfg: cfg} }
func (g *AutoIncrement) Name() string            { return "AutoIncrement" }
func (g *AutoIncrement) IsDateBased() bool       { return false }
func (g *AutoIncrement) Next(ctx context.Context, store CounterStore) (string, error) {
	min := g.cfg.MinCounterSize
	if min <= 0 {
		min = 5
	}
	c, err := store.Add(ctx, false, 1)
	if err != nil {
		return "", err
	}
	// for compatibility, inserts a literal '0' between SystemID and the zero-padded counter for AutoIncrement
	return fmt.Sprintf("%s0%0*d", g.cfg.SystemID, min, c), nil
}
