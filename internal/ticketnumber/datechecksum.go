package ticketnumber

import (
    "context"
    "fmt"
)

// DateChecksum: yyyymmdd + SystemID + zero-padded counter (min 5) + single check digit.
// Check digit: sum over digits with multipliers 1,2,1,2... ; sum each product directly; checksum = 10 - (sum % 10); if result == 10 => 1.
type DateChecksum struct { cfg Config; clock Clock }
func NewDateChecksum(cfg Config, clk Clock) *DateChecksum { return &DateChecksum{cfg: cfg, clock: clk} }
func (g *DateChecksum) Name() string { return "DateChecksum" }
func (g *DateChecksum) IsDateBased() bool { return true }
func (g *DateChecksum) Next(ctx context.Context, store CounterStore) (string, error) {
    tp := g.clock.Now()
    c, err := store.Add(ctx, true, 1)
    if err != nil { return "", err }
    min := g.cfg.MinCounterSize
    if min <= 0 { min = 5 }
    body := fmt.Sprintf("%04d%02d%02d%s%0*d", tp.Year, tp.Month, tp.Day, g.cfg.SystemID, min, c)
    cs := checksumAlt(body)
    return body + fmt.Sprintf("%d", cs), nil
}
func checksumAlt(s string) int {
    sum := 0
    mult := 1
    for i:=0;i<len(s);i++ {
        d := int(s[i]-'0')
        sum += mult * d
        mult++
        if mult == 3 { mult = 1 }
    }
    sum %= 10
    c := 10 - sum
    if c == 10 { c = 1 }
    return c
}
