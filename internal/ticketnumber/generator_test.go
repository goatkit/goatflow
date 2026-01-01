package ticketnumber

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// dummy store to drive expected counters sequentially per scope.
type memStore struct {
	mu     sync.Mutex
	global int64
	day    int64
}

func (m *memStore) Add(_ context.Context, dateScoped bool, offset int64) (int64, error) {
	if offset < 1 {
		return 0, errors.New("bad offset")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if dateScoped {
		m.day += offset
		return m.day, nil
	}
	m.global += offset
	return m.global, nil
}

func TestAutoIncrementSequence(t *testing.T) {
	g := NewAutoIncrement(Config{SystemID: "10", MinCounterSize: 5})
	ms := &memStore{}
	ctx := context.Background()
	got1, _ := g.Next(ctx, ms)
	got2, _ := g.Next(ctx, ms)
	if got1 != "10000001" || got2 != "10000002" {
		t.Fatalf("unexpected %s %s", got1, got2)
	}
}

func TestDateSequenceFormatted(t *testing.T) {
	clk := fixedClock{t: TimeParts{Year: 2025, Month: 10, Day: 03}}
	g := NewDate(Config{SystemID: "10", MinCounterSize: 5, DateUseFormattedCounter: true}, clk)
	ms := &memStore{}
	ctx := context.Background()
	a, _ := g.Next(ctx, ms)
	b, _ := g.Next(ctx, ms)
	if a != "202510031000001" || b != "202510031000002" {
		t.Fatalf("unexpected %s %s", a, b)
	}
}

func TestDateChecksumSequence(t *testing.T) {
	clk := fixedClock{t: TimeParts{Year: 2025, Month: 10, Day: 03}}
	g := NewDateChecksum(Config{SystemID: "10", MinCounterSize: 5}, clk)
	ms := &memStore{}
	ctx := context.Background()
	a, _ := g.Next(ctx, ms) // counter 1 => padded 00001
	b, _ := g.Next(ctx, ms) // counter 2 => 00002
	if a == b {
		t.Fatalf("checksum numbers should differ: %s %s", a, b)
	}
	// Validate structure: yyyymmdd + systemid + 5 digits + 1 checksum
	if len(a) != 8+len("10")+5+1 {
		t.Fatalf("unexpected length %d", len(a))
	}
	// Recompute checksum of body and compare last digit
	body := a[:len(a)-1]
	gotCS := int(a[len(a)-1] - '0')
	want := checksumAlt(body)
	if gotCS != want {
		t.Fatalf("checksum mismatch got=%d want=%d for %s", gotCS, want, a)
	}
}

func TestRandomDeterministicSeed(t *testing.T) {
	g1 := NewRandom(Config{SystemID: "10"}, 12345)
	g2 := NewRandom(Config{SystemID: "10"}, 12345)
	ms := &memStore{}
	ctx := context.Background()
	a1, _ := g1.Next(ctx, ms)
	a2, _ := g2.Next(ctx, ms)
	if a1 != a2 {
		t.Fatalf("expected deterministic same first value %s %s", a1, a2)
	}
	// Ensure 10 random digits after system id
	if len(a1) != len("10")+10 {
		t.Fatalf("random length mismatch %d", len(a1))
	}
}
