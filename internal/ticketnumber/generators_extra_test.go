package ticketnumber

import (
    "context"
    "testing"
)

// Reuse memStore from generator_test.go (same package) for counter progression.

func TestDateSequenceUnformatted(t *testing.T) {
    clk := fixedClock{t: TimeParts{Year: 2025, Month: 10, Day: 3}}
    g := NewDate(Config{SystemID: "10", MinCounterSize: 5, DateUseFormattedCounter: false}, clk)
    ms := &memStore{}
    ctx := context.Background()
    a, _ := g.Next(ctx, ms)
    b, _ := g.Next(ctx, ms)
    if a != "20251003101" || b != "20251003102" { t.Fatalf("unexpected unformatted date numbers %s %s", a, b) }
}

func TestResolve_CaseInsensitive(t *testing.T) {
    cases := []struct{ in, want string }{
        {"increment", "AutoIncrement"},
        {"DATE", "Date"},
        {"datechecksum", "DateChecksum"},
        {"rAnDoM", "Random"},
    }
    for _, c := range cases {
        g, err := Resolve(c.in, "10", fixedClock{t: TimeParts{Year: 2025, Month: 10, Day: 4}})
        if err != nil { t.Fatalf("resolve error for %s: %v", c.in, err) }
        if g.Name() != c.want { t.Fatalf("expected canonical name %s got %s", c.want, g.Name()) }
    }
}

func TestAutoIncrementDefaultMinSize(t *testing.T) {
    g := NewAutoIncrement(Config{SystemID: "10", MinCounterSize: 0}) // should default to 5
    ms := &memStore{}
    a, _ := g.Next(context.Background(), ms)
    if a != "10000001" { t.Fatalf("expected default padded value got %s", a) }
}

func TestRandomUniquenessSmallSample(t *testing.T) {
    g := NewRandom(Config{SystemID: "10"}, 99999) // deterministic
    ms := &memStore{}
    seen := make(map[string]struct{})
    for i:=0;i<100;i++ {
        n, _ := g.Next(context.Background(), ms)
        if len(n) != len("10")+10 { t.Fatalf("length mismatch for %s", n) }
        if _, ok := seen[n]; ok { t.Fatalf("duplicate random number %s", n) }
        seen[n] = struct{}{}
    }
}

func TestDateChecksumMultiple(t *testing.T) {
    clk := fixedClock{t: TimeParts{Year: 2025, Month: 10, Day: 4}}
    g := NewDateChecksum(Config{SystemID: "10", MinCounterSize: 5}, clk)
    ms := &memStore{}
    ctx := context.Background()
    prev := ""
    for i:=0;i<10;i++ {
        v, _ := g.Next(ctx, ms)
        if len(v) != 8+len("10")+5+1 { t.Fatalf("unexpected length %d for %s", len(v), v) }
        body := v[:len(v)-1]
        cs := int(v[len(v)-1]-'0')
        if cs != checksumAlt(body) { t.Fatalf("checksum mismatch for %s", v) }
        if v == prev { t.Fatalf("duplicate consecutive number %s", v) }
        prev = v
    }
}
