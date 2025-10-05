//go:build integration

package ticketnumber

import (
    "context"
    "database/sql"
    "os"
    "sync"
    "testing"
    _ "github.com/lib/pq"
)

func openTestDB(t *testing.T) *sql.DB {
    dsn := os.Getenv("TEST_PG_DSN")
    if dsn == "" { t.Skip("TEST_PG_DSN not set") }
    db, err := sql.Open("postgres", dsn)
    if err != nil { t.Skip("db not available") }
    return db
}

func TestConcurrency_AllGenerators(t *testing.T) {
    db := openTestDB(t)
    defer db.Close()
    store := NewDBStore(db, "10")
    ctx := context.Background()
    // Simple clock
    clk := fixedClock{t: TimeParts{Year: 2025, Month: 10, Day: 04}}
    gens := []Generator{
        NewAutoIncrement(Config{SystemID: "10", MinCounterSize: 5}),
        NewDate(Config{SystemID: "10", MinCounterSize: 5, DateUseFormattedCounter: true}, clk),
        NewDateChecksum(Config{SystemID: "10", MinCounterSize: 5}, clk),
        NewRandom(Config{SystemID: "10"}, 1234),
    }
    for _, g := range gens {
        t.Run(g.Name(), func(t *testing.T) {
            var wg sync.WaitGroup
            results := make(map[string]struct{})
            var mu sync.Mutex
            n := 40
            for i:=0;i<n;i++ {
                wg.Add(1)
                go func() {
                    defer wg.Done()
                    tn, err := g.Next(ctx, store)
                    if err != nil { t.Errorf("next failed: %v", err); return }
                    mu.Lock(); if _, ok := results[tn]; ok { t.Errorf("duplicate %s", tn) } else { results[tn]=struct{}{} }; mu.Unlock()
                }()
            }
            wg.Wait()
            if len(results) != n { t.Fatalf("expected %d unique got %d", n, len(results)) }
        })
    }
}
