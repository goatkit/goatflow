package ticketnumber

import (
    "context"
    "fmt"
)

type Date struct { cfg Config; clock Clock }

func NewDate(cfg Config, clk Clock) *Date { return &Date{cfg: cfg, clock: clk} }
func (g *Date) Name() string { return "Date" }
func (g *Date) IsDateBased() bool { return true }
func (g *Date) Next(ctx context.Context, store CounterStore) (string, error) {
    tp := g.clock.Now()
    counter, err := store.Add(ctx, true, 1)
    if err != nil { return "", err }
    if g.cfg.DateUseFormattedCounter {
        min := g.cfg.MinCounterSize
        if min <= 0 { min = 5 }
        return fmt.Sprintf("%04d%02d%02d%s%0*d", tp.Year, tp.Month, tp.Day, g.cfg.SystemID, min, counter), nil
    }
    return fmt.Sprintf("%04d%02d%02d%s%d", tp.Year, tp.Month, tp.Day, g.cfg.SystemID, counter), nil
}
