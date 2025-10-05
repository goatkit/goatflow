package ticketnumber

import (
    "context"
    "crypto/rand"
    "encoding/binary"
    mrand "math/rand"
    "fmt"
)

// Random generator: SystemID + 8 random digits (collision resolved by retry via store if needed).
type Random struct { cfg Config; src *mrand.Rand }

func NewRandom(cfg Config, seed int64) *Random {
    var seeded int64
    if seed != 0 { seeded = seed } else {
        var b [8]byte; _, _ = rand.Read(b[:]); seeded = int64(binary.LittleEndian.Uint64(b[:]))
    }
    return &Random{cfg: cfg, src: mrand.New(mrand.NewSource(seeded))}
}
func (g *Random) Name() string { return "Random" }
func (g *Random) IsDateBased() bool { return false }
func (g *Random) Next(ctx context.Context, store CounterStore) (string, error) {
    // Advance counter to preserve ordering semantics; though random doesn't need counter we keep Add for uniformity (offset 1 global)
    _, _ = store.Add(ctx, false, 1)
    // 10 random digits; ensure leading zeros allowed. Random: int rand 9999999999 then sprintf 10 digits)
    n := g.src.Int63() % 10000000000
    return fmt.Sprintf("%s%010d", g.cfg.SystemID, n), nil
}
