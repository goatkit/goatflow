package ticket_number

import (
	"database/sql"
	"fmt"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// AutoIncrementConfig holds configuration for auto-increment generator
type AutoIncrementConfig struct {
	Prefix    string
	MinDigits int
	StartFrom int64
}

// AutoIncrementGenerator generates simple sequential ticket numbers
// Format: PREFIX + padded number (e.g., T-0001000)
type AutoIncrementGenerator struct {
	db         *sql.DB
	config     AutoIncrementConfig
	counterUID string
}

// NewAutoIncrementGenerator creates a new auto-increment generator
func NewAutoIncrementGenerator(db *sql.DB, config AutoIncrementConfig) *AutoIncrementGenerator {
	// Set defaults
	if config.MinDigits == 0 {
		config.MinDigits = 7
	}
	if config.StartFrom == 0 {
		config.StartFrom = 1
	}
	
	return &AutoIncrementGenerator{
		db:         db,
		config:     config,
		counterUID: "auto_increment",
	}
}

// Generate creates a new ticket number
func (g *AutoIncrementGenerator) Generate() (string, error) {
	// Get next counter value
	counter, err := g.getNextCounterWithStart()
	if err != nil {
		return "", fmt.Errorf("failed to get next counter: %w", err)
	}
	
	// Format with prefix and padding
	format := fmt.Sprintf("%%s%%0%dd", g.config.MinDigits)
	ticketNumber := fmt.Sprintf(format, g.config.Prefix, counter)
	
	return ticketNumber, nil
}

// Reset resets the counter to the starting value
func (g *AutoIncrementGenerator) Reset() error {
	return resetCounter(g.db, g.counterUID, g.config.StartFrom-1)
}

// getNextCounterWithStart gets the next counter, handling the StartFrom value
func (g *AutoIncrementGenerator) getNextCounterWithStart() (int64, error) {
	var counter int64
	
	// First, check if counter exists
	err := g.db.QueryRow(database.ConvertPlaceholders(`
		SELECT counter FROM ticket_number_counter 
		WHERE counter_uid = $1
	`), g.counterUID).Scan(&counter)
	
	if err == sql.ErrNoRows {
		// Counter doesn't exist, use the shared getNextCounter function
		// which handles MySQL/PostgreSQL differences
		return getNextCounter(g.db, g.counterUID)
	}
	
	if err != nil {
		return 0, err
	}
	
	// Counter exists, use the shared getNextCounter function
	return getNextCounter(g.db, g.counterUID)
	
	if err != nil {
		return 0, err
	}
	
	return counter, nil
}