package ticket_number

import (
	"database/sql"
	"fmt"
	"hash/crc32"
	"strings"
	"time"
)

// DateChecksumConfig holds configuration for date-checksum generator
type DateChecksumConfig struct {
	Separator      string
	CounterDigits  int
	ChecksumLength int
	ResetDaily     bool
}

// DateChecksumGenerator generates ticket numbers with checksum
// Format: YYYYMMDD-counter-checksum (e.g., 20250828-000001-42)
type DateChecksumGenerator struct {
	db     *sql.DB
	config DateChecksumConfig
}

// NewDateChecksumGenerator creates a new date-checksum generator
func NewDateChecksumGenerator(db *sql.DB, config DateChecksumConfig) *DateChecksumGenerator {
	// Set defaults
	if config.Separator == "" {
		config.Separator = "-"
	}
	if config.CounterDigits == 0 {
		config.CounterDigits = 6
	}
	if config.ChecksumLength == 0 {
		config.ChecksumLength = 2
	}
	
	return &DateChecksumGenerator{
		db:     db,
		config: config,
	}
}

// Generate creates a new ticket number with checksum
func (g *DateChecksumGenerator) Generate() (string, error) {
	now := time.Now()
	
	// Build date part
	datePart := fmt.Sprintf("%04d%02d%02d", now.Year(), now.Month(), now.Day())
	
	// Get counter UID (includes date for daily reset)
	counterUID := g.getCounterUID(now)
	
	// Get next counter value
	counter, err := getNextCounter(g.db, counterUID)
	if err != nil {
		return "", fmt.Errorf("failed to get next counter: %w", err)
	}
	
	// Format counter with padding
	counterFormat := fmt.Sprintf("%%0%dd", g.config.CounterDigits)
	counterPart := fmt.Sprintf(counterFormat, counter)
	
	// Calculate checksum
	checksumInput := datePart + counterPart
	checksum := g.calculateChecksum(checksumInput)
	
	// Combine all parts
	ticketNumber := fmt.Sprintf("%s%s%s%s%s",
		datePart,
		g.config.Separator,
		counterPart,
		g.config.Separator,
		checksum)
	
	return ticketNumber, nil
}

// Reset resets the counter (happens automatically with daily counter UIDs)
func (g *DateChecksumGenerator) Reset() error {
	// For date-based generators with daily reset,
	// we use a new counter_uid each day, so no explicit reset needed
	if g.config.ResetDaily {
		return nil
	}
	
	// For non-daily reset, reset the main counter
	counterUID := g.getCounterUID(time.Now())
	return resetCounter(g.db, counterUID, 0)
}

// Validate checks if a ticket number has a valid checksum
func (g *DateChecksumGenerator) Validate(ticketNumber string) bool {
	parts := strings.Split(ticketNumber, g.config.Separator)
	if len(parts) != 3 {
		return false
	}
	
	// Recalculate checksum
	checksumInput := parts[0] + parts[1]
	expectedChecksum := g.calculateChecksum(checksumInput)
	
	return parts[2] == expectedChecksum
}

// calculateChecksum calculates a checksum for the given input
func (g *DateChecksumGenerator) calculateChecksum(input string) string {
	// Use CRC32 for checksum
	checksum := crc32.ChecksumIEEE([]byte(input))
	
	// Convert to string and truncate to desired length
	checksumStr := fmt.Sprintf("%010d", checksum) // 10 digits max
	
	// Take last N digits
	if len(checksumStr) > g.config.ChecksumLength {
		checksumStr = checksumStr[len(checksumStr)-g.config.ChecksumLength:]
	}
	
	// Pad if necessary
	format := fmt.Sprintf("%%0%dd", g.config.ChecksumLength)
	result := fmt.Sprintf(format, atoi(checksumStr))
	
	return result
}

// getCounterUID returns the counter UID for this generator
func (g *DateChecksumGenerator) getCounterUID(t time.Time) string {
	if g.config.ResetDaily {
		// Include date in UID for daily reset
		return fmt.Sprintf("date_checksum_%04d%02d%02d", t.Year(), t.Month(), t.Day())
	}
	// Single counter for all dates
	return "date_checksum_persistent"
}

// atoi converts string to int, returns 0 on error
func atoi(s string) int {
	var result int
	fmt.Sscanf(s, "%d", &result)
	return result
}