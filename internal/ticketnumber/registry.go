package ticketnumber

import (
    "errors"
    "strings"
    "time"
)

// Resolve maps a configured generator name and systemID to a concrete Generator.
// Names must match schema options exactly (case-insensitive match for leniency).
// Valid: Increment, Date, DateChecksum, Random.
func Resolve(name string, systemID string, clk Clock) (Generator, error) {
    if clk == nil { clk = realClock{} }
    n := strings.TrimSpace(name)
    switch strings.ToLower(n) {
    case "increment":
        return NewAutoIncrement(Config{SystemID: systemID, MinCounterSize: 5}), nil
    case "date":
        return NewDate(Config{SystemID: systemID, MinCounterSize: 5, DateUseFormattedCounter: true}, clk), nil
    case "datechecksum":
        return NewDateChecksum(Config{SystemID: systemID, MinCounterSize: 5}, clk), nil
    case "random":
        return NewRandom(Config{SystemID: systemID}, time.Now().UnixNano()), nil
    default:
        return nil, errors.New("unknown ticket number generator: " + name)
    }
}

type realClock struct{}
func (realClock) Now() TimeParts {
    t := time.Now().UTC()
    return TimeParts{Year: t.Year(), Month: int(t.Month()), Day: t.Day()}
}
