package ticket_number

import (
	"database/sql"
	"errors"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// TicketNumberGenerator defines the interface for all ticket number generators
type TicketNumberGenerator interface {
	// Generate creates a new unique ticket number
	Generate() (string, error)
	
	// Reset resets the counter (for daily/monthly resets)
	Reset() error
}

// Common errors
var (
	ErrGeneratorNotConfigured = errors.New("ticket number generator not configured")
	ErrCounterUpdateFailed    = errors.New("failed to update counter")
	ErrInvalidConfiguration   = errors.New("invalid generator configuration")
)

// getNextCounter atomically increments and returns the next counter value
// Uses the OTRS ticket_number_counter table
func getNextCounter(db *sql.DB, counterUID string) (int64, error) {
	var counter int64
	
	// Try to increment existing counter
	err := db.QueryRow(database.ConvertPlaceholders(`
		UPDATE ticket_number_counter 
		SET counter = counter + 1 
		WHERE counter_uid = $1
		RETURNING counter
	`), counterUID).Scan(&counter)
	
	if err == sql.ErrNoRows {
		// Counter doesn't exist, create it with value 1
		err = db.QueryRow(database.ConvertPlaceholders(`
			INSERT INTO ticket_number_counter (counter, counter_uid, create_time)
			VALUES (1, $1, NOW())
			ON CONFLICT (counter_uid) DO UPDATE 
			SET counter = ticket_number_counter.counter + 1
			RETURNING counter
		`), counterUID).Scan(&counter)
	}
	
	if err != nil {
		return 0, err
	}
	
	return counter, nil
}

// resetCounter resets a counter to a specific value
func resetCounter(db *sql.DB, counterUID string, value int64) error {
	_, err := db.Exec(database.ConvertPlaceholders(`
		INSERT INTO ticket_number_counter (counter, counter_uid, create_time)
		VALUES ($1, $2, NOW())
		ON CONFLICT (counter_uid) DO UPDATE 
		SET counter = $1, create_time = NOW()
	`), value, counterUID)
	
	return err
}