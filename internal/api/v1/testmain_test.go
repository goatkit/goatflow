package v1

import (
	"fmt"
	"os"
	"testing"

	"github.com/goatkit/goatflow/internal/database"
)

func TestMain(m *testing.M) {
	// Ensure test environment
	if os.Getenv("TEST_DB_PASSWORD") == "" && os.Getenv("TEST_DB_MYSQL_PASSWORD") == "" {
		fmt.Fprintln(os.Stderr, "ERROR: TEST_DB_PASSWORD or TEST_DB_MYSQL_PASSWORD must be set in .env")
		os.Exit(1)
	}
	if os.Getenv("TEST_DB_PASSWORD") == "" {
		os.Setenv("TEST_DB_PASSWORD", os.Getenv("TEST_DB_MYSQL_PASSWORD"))
	}

	// Initialize test database
	if err := database.InitTestDB(); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: Failed to init test DB: %v\n", err)
	}

	// Reset database to canonical state before running v1 tests
	if err := resetTestDatabase(); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: Failed to reset test DB: %v\n", err)
	}

	// Run tests
	code := m.Run()

	database.CloseTestDB()
	os.Exit(code)
}

// resetTestDatabase resets the test database to canonical state.
// This ensures v1 tests have clean, predictable data regardless of what
// other test packages may have done to the database.
func resetTestDatabase() error {
	db, err := database.GetDB()
	if err != nil || db == nil {
		return fmt.Errorf("no database connection")
	}

	// Disable foreign key checks for cleanup
	db.Exec("SET FOREIGN_KEY_CHECKS = 0")
	defer db.Exec("SET FOREIGN_KEY_CHECKS = 1")

	// Clean tickets created by other tests (preserve IDs 1-1000)
	db.Exec("DELETE FROM ticket_history WHERE ticket_id > 1000")
	db.Exec("DELETE FROM article_data_mime WHERE article_id IN (SELECT id FROM article WHERE ticket_id > 1000)")
	db.Exec("DELETE FROM article WHERE ticket_id > 1000")
	db.Exec("DELETE FROM ticket WHERE id > 1000")

	// Restore canonical test tickets (IDs 1, 2, 3, 123)
	now := "NOW()"
	
	// Ensure tickets exist in expected state (not archived)
	db.Exec(`INSERT INTO ticket (id, tn, title, queue_id, ticket_lock_id, type_id, user_id, 
		responsible_user_id, ticket_priority_id, ticket_state_id, customer_id, customer_user_id,
		timeout, until_time, escalation_time, escalation_update_time, escalation_response_time,
		escalation_solution_time, archive_flag, create_time, create_by, change_time, change_by)
		VALUES (1, 'RAW-0001', 'First Raw queue ticket', 2, 1, 1, 1, 1, 3, 2,
		'test-customer', 'test@example.com', 0, 0, 0, 0, 0, 0, 0, ` + now + `, 1, ` + now + `, 1)
		ON DUPLICATE KEY UPDATE ticket_state_id = 2, archive_flag = 0`)

	db.Exec(`INSERT INTO ticket (id, tn, title, queue_id, ticket_lock_id, type_id, user_id, 
		responsible_user_id, ticket_priority_id, ticket_state_id, customer_id, customer_user_id,
		timeout, until_time, escalation_time, escalation_update_time, escalation_response_time,
		escalation_solution_time, archive_flag, create_time, create_by, change_time, change_by)
		VALUES (2, 'RAW-0002', 'Second Raw queue ticket', 2, 1, 1, 1, 1, 3, 2,
		'test-customer', 'test@example.com', 0, 0, 0, 0, 0, 0, 0, ` + now + `, 1, ` + now + `, 1)
		ON DUPLICATE KEY UPDATE ticket_state_id = 2, archive_flag = 0`)

	db.Exec(`INSERT INTO ticket (id, tn, title, queue_id, ticket_lock_id, type_id, user_id, 
		responsible_user_id, ticket_priority_id, ticket_state_id, customer_id, customer_user_id,
		timeout, until_time, escalation_time, escalation_update_time, escalation_response_time,
		escalation_solution_time, archive_flag, create_time, create_by, change_time, change_by)
		VALUES (3, 'JUNK-0001', 'Junk queue ticket', 3, 1, 1, 1, 1, 3, 2,
		'test-customer', 'test@example.com', 0, 0, 0, 0, 0, 0, 0, ` + now + `, 1, ` + now + `, 1)
		ON DUPLICATE KEY UPDATE ticket_state_id = 2, archive_flag = 0`)

	db.Exec(`INSERT INTO ticket (id, tn, title, queue_id, ticket_lock_id, type_id, user_id, 
		responsible_user_id, ticket_priority_id, ticket_state_id, customer_id, customer_user_id,
		timeout, until_time, escalation_time, escalation_update_time, escalation_response_time,
		escalation_solution_time, archive_flag, create_time, create_by, change_time, change_by)
		VALUES (123, 'TEST-0123', 'Test Ticket for Attachments', 1, 1, 1, 1, 1, 3, 2,
		'test-customer', 'test@example.com', 0, 0, 0, 0, 0, 0, 0, ` + now + `, 1, ` + now + `, 1)
		ON DUPLICATE KEY UPDATE ticket_state_id = 2, archive_flag = 0`)

	// Ensure user 1 has permissions on all queues
	db.Exec(`INSERT IGNORE INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
		VALUES (1, 1, 'rw', NOW(), 1, NOW(), 1)`)

	// Ensure testuser (id 15) exists and has permissions
	db.Exec(`INSERT INTO users (id, login, pw, valid_id, create_time, create_by, change_time, change_by)
		VALUES (15, 'testuser', 'test', 1, NOW(), 1, NOW(), 1)
		ON DUPLICATE KEY UPDATE valid_id = 1`)
	db.Exec(`INSERT IGNORE INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
		VALUES (15, 1, 'rw', NOW(), 1, NOW(), 1)`)

	return nil
}
