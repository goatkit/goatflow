package ticket_number

import (
	"database/sql"
	"fmt"
	"time"
)

// DateConfig holds configuration for date-based generator.
type DateConfig struct {
	IncludeHour   bool
	CounterDigits int
	ResetDaily    bool
}

// Format: YYYYMMDDHH + counter (e.g., 2025082814000001).
type DateGenerator struct {
	db     *sql.DB
	config DateConfig
}

// NewDateGenerator creates a new date-based generator.
func NewDateGenerator(db *sql.DB, config DateConfig) *DateGenerator {
	// Set defaults
	if config.CounterDigits == 0 {
		config.CounterDigits = 6
	}

	return &DateGenerator{
		db:     db,
		config: config,
	}
}

// Generate creates a new ticket number.
func (g *DateGenerator) Generate() (string, error) {
	now := time.Now()

	// Build date prefix
	var datePrefix string
	if g.config.IncludeHour {
		datePrefix = fmt.Sprintf("%04d%02d%02d%02d",
			now.Year(), now.Month(), now.Day(), now.Hour())
	} else {
		datePrefix = fmt.Sprintf("%04d%02d%02d",
			now.Year(), now.Month(), now.Day())
	}

	// Get counter UID (includes date for daily reset)
	counterUID := g.getCounterUID(now)

	// Get next counter value
	counter, err := getNextCounter(g.db, counterUID)
	if err != nil {
		return "", fmt.Errorf("failed to get next counter: %w", err)
	}

	// Format counter with padding
	counterFormat := fmt.Sprintf("%%0%dd", g.config.CounterDigits)
	counterStr := fmt.Sprintf(counterFormat, counter)

	// Combine date and counter
	ticketNumber := datePrefix + counterStr

	return ticketNumber, nil
}

// Reset resets the counter (happens automatically with daily counter UIDs).
func (g *DateGenerator) Reset() error {
	// For date-based generators with daily reset,
	// we use a new counter_uid each day, so no explicit reset needed
	if g.config.ResetDaily {
		return nil
	}

	// For non-daily reset, reset the main counter
	counterUID := g.getCounterUID(time.Now())
	return resetCounter(g.db, counterUID, 0)
}

// getCounterUID returns the counter UID for this generator.
func (g *DateGenerator) getCounterUID(t time.Time) string {
	if g.config.ResetDaily {
		// Include date in UID for daily reset
		return fmt.Sprintf("date_%04d%02d%02d", t.Year(), t.Month(), t.Day())
	}
	// Single counter for all dates
	return "date_persistent"
}
