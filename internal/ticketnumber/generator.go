package ticketnumber

import "context"

// Generator defines contract for ticket number generators.
type Generator interface {
    Name() string
    Next(ctx context.Context, store CounterStore) (string, error)
    IsDateBased() bool
}

// CounterStore abstraction over ticket_number_counter algorithm.
type CounterStore interface {
    // Add returns next counter given offset (>=1).
    Add(ctx context.Context, dateScoped bool, offset int64) (int64, error)
}

// Config needed by generators.
type Config struct {
    SystemID                string
    MinCounterSize          int
    DateUseFormattedCounter bool
}

// Clock allows deterministic testing.
type Clock interface { Now() TimeParts }

// TimeParts minimal date parts.
type TimeParts struct { Year int; Month int; Day int }
