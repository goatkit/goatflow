package v1

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/goatkit/goatflow/internal/platform/database"
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

	// Clean up api/v1 auth test fixtures so other packages aren't affected
	cleanupV1TestData()

	database.CloseTestDB()
	os.Exit(code)
}

// cleanupV1TestData removes all v1 auth test fixtures from the shared test database.
// This prevents cross-package test pollution when running the full suite.
func cleanupV1TestData() {
	db, err := database.GetDB()
	if err != nil || db == nil {
		return
	}
	_ = withoutFKChecks(db, func(exec func(query string, args ...any) error) {
		_ = exec("DELETE FROM user_api_tokens WHERE user_id >= 90000 OR name LIKE 'agent-%' OR name LIKE 'customer-%'")
		_ = exec("DELETE FROM ticket_history WHERE ticket_id >= 90000")
		_ = exec("DELETE FROM article_data_mime WHERE article_id IN (SELECT id FROM article WHERE ticket_id >= 90000)")
		_ = exec("DELETE FROM article WHERE ticket_id >= 90000")
		_ = exec("DELETE FROM ticket WHERE id >= 90000")
		_ = exec("DELETE FROM group_user WHERE user_id >= 90000 OR group_id >= 90000")
		_ = exec("DELETE FROM group_customer WHERE customer_id LIKE 'authtest-%'")
		_ = exec("DELETE FROM customer_user WHERE login LIKE '%authtest%'")
		_ = exec("DELETE FROM customer_company WHERE customer_id LIKE 'authtest-%'")
		_ = exec("DELETE FROM queue WHERE id >= 90000 OR name LIKE 'AuthTest-%'")
		_ = exec("DELETE FROM `groups` WHERE id >= 90000 OR name LIKE 'AuthTest-%'")
		_ = exec("DELETE FROM users WHERE id >= 90000 OR login LIKE 'authtest-%'")
	})
}

// resetTestDatabase resets the test database to canonical state.
// This ensures v1 tests have clean, predictable data regardless of what
// other test packages may have done to the database.
func resetTestDatabase() error {
	db, err := database.GetDB()
	if err != nil || db == nil {
		return fmt.Errorf("no database connection")
	}

	var errs []error
	connErr := withoutFKChecks(db, func(exec func(query string, args ...any) error) {
		run := func(query string, args ...any) {
			if err := exec(query, args...); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", strings.Join(strings.Fields(query), " "), err))
			}
		}

		// Clean tickets created by other tests (preserve IDs 1-1000)
		run("DELETE FROM ticket_history WHERE ticket_id > 1000")
		run("DELETE FROM article_data_mime WHERE article_id IN (SELECT id FROM article WHERE ticket_id > 1000)")
		run("DELETE FROM article WHERE ticket_id > 1000")
		run("DELETE FROM ticket WHERE id > 1000")

		// Ensure queues referenced by test tickets exist (queue 1 already seeded)
		for id, name := range map[int]string{2: "Raw", 3: "Junk"} {
			run(`INSERT IGNORE INTO queue (id, name, group_id, unlock_timeout, first_response_time,
				first_response_notify, update_time, update_notify, solution_time, solution_notify,
				system_address_id, calendar_name, default_sign_key, salutation_id, signature_id,
				follow_up_id, follow_up_lock, comments, valid_id, create_time, create_by, change_time, change_by)
				VALUES (?, ?, 1, 0, 0, 0, 0, 0, 0, 0, 1, NULL, NULL, 1, 1, 1, 1, ?, 1, NOW(), 1, NOW(), 1)`,
				id, name, name+" queue for testing")
			run("UPDATE queue SET name = ? WHERE id = ?", name, id)
		}

		// Ensure tickets exist in expected state (not archived)
		tickets := []struct {
			id      int
			tn      string
			title   string
			queueID int
		}{
			{1, "RAW-0001", "First Raw queue ticket", 2},
			{2, "RAW-0002", "Second Raw queue ticket", 2},
			{3, "JUNK-0001", "Junk queue ticket", 3},
			{123, "TEST-0123", "Test Ticket for Attachments", 1},
		}
		for _, tk := range tickets {
			run(`INSERT IGNORE INTO ticket (id, tn, title, queue_id, ticket_lock_id, type_id, user_id,
				responsible_user_id, ticket_priority_id, ticket_state_id, customer_id, customer_user_id,
				timeout, until_time, escalation_time, escalation_update_time, escalation_response_time,
				escalation_solution_time, archive_flag, create_time, create_by, change_time, change_by)
				VALUES (?, ?, ?, ?, 1, 1, 1, 1, 3, 2,
				'test-customer', 'test@example.com', 0, 0, 0, 0, 0, 0, 0, NOW(), 1, NOW(), 1)`,
				tk.id, tk.tn, tk.title, tk.queueID)
			run("UPDATE ticket SET ticket_state_id = 2, archive_flag = 0 WHERE id = ?", tk.id)
		}

		// Ensure testuser (id 15) exists and is valid
		run(`INSERT IGNORE INTO users (id, login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
			VALUES (15, 'testuser', 'test', 'Test', 'User', 1, NOW(), 1, NOW(), 1)`)
		run("UPDATE users SET valid_id = 1 WHERE id = 15")

		// Ensure users 1 and 15 have rw on group 1. group_user has no unique
		// key, so replace the grant instead of INSERT IGNORE.
		for _, userID := range []int{1, 15} {
			run("DELETE FROM group_user WHERE user_id = ? AND group_id = 1 AND permission_key = 'rw'", userID)
			run(`INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
				VALUES (?, 1, 'rw', NOW(), 1, NOW(), 1)`, userID)
		}

		// Rows above may carry explicit ids; PostgreSQL sequences do not follow
		// explicit ids, so move them past the highest id.
		if database.IsPostgreSQL() {
			run("SELECT setval(pg_get_serial_sequence('queue', 'id'), (SELECT COALESCE(MAX(id), 1) FROM queue))")
			run("SELECT setval(pg_get_serial_sequence('ticket', 'id'), (SELECT COALESCE(MAX(id), 1) FROM ticket))")
			run("SELECT setval(pg_get_serial_sequence('users', 'id'), (SELECT COALESCE(MAX(id), 1) FROM users))")
		}
	})
	return errors.Join(append(errs, connErr)...)
}
