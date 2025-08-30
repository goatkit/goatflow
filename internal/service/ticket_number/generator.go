package ticket_number

import (
	"database/sql"
	"errors"
	"fmt"
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
	// Start transaction for atomic operation
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	
	var counter int64
	
	// Try to get current counter value with lock (SELECT FOR UPDATE)
	err = tx.QueryRow(database.ConvertPlaceholders(`
		SELECT counter FROM ticket_number_counter 
		WHERE counter_uid = $1
		FOR UPDATE
	`), counterUID).Scan(&counter)
	
	if err == sql.ErrNoRows {
		// Counter doesn't exist, create it with value 1
		// Use INSERT IGNORE for MySQL compatibility
		_, err = tx.Exec(database.ConvertPlaceholders(`
			INSERT IGNORE INTO ticket_number_counter (counter, counter_uid, create_time)
			VALUES (1, $1, NOW())
		`), counterUID)
		
		if err != nil {
			return 0, fmt.Errorf("failed to insert counter: %w", err)
		}
		
		// Now get the counter (it either existed or was just created)
		err = tx.QueryRow(database.ConvertPlaceholders(`
			SELECT counter FROM ticket_number_counter 
			WHERE counter_uid = $1
			FOR UPDATE
		`), counterUID).Scan(&counter)
		
		if err != nil {
			return 0, fmt.Errorf("failed to get counter after insert: %w", err)
		}
	} else if err != nil {
		return 0, fmt.Errorf("failed to get counter: %w", err)
	}
	
	// Increment counter
	counter++
	_, err = tx.Exec(database.ConvertPlaceholders(`
		UPDATE ticket_number_counter 
		SET counter = $1 
		WHERE counter_uid = $2
	`), counter, counterUID)
	
	if err != nil {
		return 0, fmt.Errorf("failed to update counter: %w", err)
	}
	
	// Commit transaction
	if err = tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit: %w", err)
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