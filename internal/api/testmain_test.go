package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/ticketnumber"
)

func TestMain(m *testing.M) {
	ensureTestEnvironment()

	// Fail fast (3s) with clear error - don't waste time on 45s timeouts
	// Tests MUST have a database available - no skipping allowed
	if err := waitForTestDatabase(3 * time.Second); err != nil {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "╔══════════════════════════════════════════════════════════════════╗")
		fmt.Fprintln(os.Stderr, "║  FATAL: TEST DATABASE UNAVAILABLE                               ║")
		fmt.Fprintln(os.Stderr, "╠══════════════════════════════════════════════════════════════════╣")
		fmt.Fprintln(os.Stderr, "║  API tests require the test database to be running.             ║")
		fmt.Fprintln(os.Stderr, "║  Tests cannot be skipped - a real database is required.         ║")
		fmt.Fprintln(os.Stderr, "║                                                                 ║")
		fmt.Fprintln(os.Stderr, "║  To start the database:                                         ║")
		fmt.Fprintln(os.Stderr, "║    make test-db-up                                              ║")
		fmt.Fprintln(os.Stderr, "║                                                                 ║")
		fmt.Fprintln(os.Stderr, "║  Then run tests:                                                ║")
		fmt.Fprintln(os.Stderr, "║    make toolbox-exec ARGS=\"go test ./internal/api/...\"          ║")
		fmt.Fprintln(os.Stderr, "╚══════════════════════════════════════════════════════════════════╝")
		fmt.Fprintln(os.Stderr, "")
		os.Exit(1)
	}

	// Reset database to canonical state before running tests
	if err := resetTestDatabase(); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: Failed to reset test database: %v\n", err)
	}

	// Initialize ticket number generator for all tests that create tickets
	if err := initTestTicketNumberGenerator(); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: Failed to initialize ticket number generator: %v\n", err)
	}

	code := m.Run()
	database.CloseTestDB()
	os.Exit(code)
}

func init() {
	ensureTestEnvironment()
}

func ensureTestEnvironment() {
	_ = os.Unsetenv("DATABASE_URL")
	setDefaultEnv("APP_ENV", "test")
	// Set templates directory for tests that render HTML
	setDefaultEnv("TEMPLATES_DIR", "/workspace/templates")
	setDefaultEnv("TEST_DB_DRIVER", "mysql")
	setDefaultEnv("TEST_DB_HOST", "mariadb-test")
	setDefaultEnv("TEST_DB_PORT", "3306")
	setDefaultEnv("TEST_DB_NAME", "otrs_test")
	setDefaultEnv("TEST_DB_USER", "otrs")
	// TEST_DB_PASSWORD must be set via environment - no default
	if os.Getenv("TEST_DB_PASSWORD") == "" && os.Getenv("TEST_DB_MYSQL_PASSWORD") == "" {
		fmt.Fprintln(os.Stderr, "ERROR: TEST_DB_PASSWORD or TEST_DB_MYSQL_PASSWORD must be set in .env")
		os.Exit(1)
	}
	setDefaultEnv("TEST_DB_PASSWORD", os.Getenv("TEST_DB_MYSQL_PASSWORD"))
}

func waitForTestDatabase(timeout time.Duration) error {
	driver := strings.ToLower(os.Getenv("TEST_DB_DRIVER"))

	var waitErr error
	switch driver {
	case "", "postgres", "postgresql":
		waitErr = waitForPostgresDatabase(timeout)
	case "mysql", "mariadb":
		waitErr = waitForMySQLDatabase(timeout)
	default:
		waitErr = waitForPostgresDatabase(timeout)
	}

	if waitErr != nil {
		return waitErr
	}

	database.ResetDB()
	return database.InitTestDB()
}

func waitForPostgresDatabase(timeout time.Duration) error {
	host := os.Getenv("TEST_DB_HOST")
	port := os.Getenv("TEST_DB_PORT")
	user := os.Getenv("TEST_DB_USER")
	password := os.Getenv("TEST_DB_PASSWORD")
	dbName := os.Getenv("TEST_DB_NAME")
	sslMode := os.Getenv("TEST_DB_SSLMODE")
	if sslMode == "" {
		sslMode = os.Getenv("TEST_DB_SSL_MODE")
	}
	if sslMode == "" {
		sslMode = "disable"
	}

	if host == "" || port == "" {
		return fmt.Errorf("test database host/port not configured")
	}

	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		currentHost := host
		if resolved := resolveHost(host); resolved != "" && resolved != host {
			currentHost = resolved
			_ = os.Setenv("TEST_DB_HOST", currentHost)
		}

		dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", currentHost, port, user, password, dbName, sslMode)

		sqlDB, err := sql.Open("postgres", dsn)
		if err == nil {
			err = sqlDB.Ping()
			sqlDB.Close()
			if err == nil {
				return nil
			}
			lastErr = err
		} else {
			lastErr = err
		}

		time.Sleep(500 * time.Millisecond)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("timeout waiting for database")
	}
	return lastErr
}

func waitForMySQLDatabase(timeout time.Duration) error {
	host := os.Getenv("TEST_DB_HOST")
	port := os.Getenv("TEST_DB_PORT")
	user := os.Getenv("TEST_DB_USER")
	password := os.Getenv("TEST_DB_PASSWORD")
	dbName := os.Getenv("TEST_DB_NAME")

	if host == "" || port == "" {
		return fmt.Errorf("test database host/port not configured")
	}

	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		currentHost := host
		if resolved := resolveHost(host); resolved != "" && resolved != host {
			currentHost = resolved
			_ = os.Setenv("TEST_DB_HOST", currentHost)
		}

		dsn := buildMySQLDSN(user, password, currentHost, port, dbName)
		sqlDB, err := sql.Open("mysql", dsn)
		if err == nil {
			err = sqlDB.Ping()
			sqlDB.Close()
			if err == nil {
				return nil
			}
			lastErr = err
		} else {
			lastErr = err
		}

		time.Sleep(500 * time.Millisecond)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("timeout waiting for database")
	}
	return lastErr
}

func buildMySQLDSN(user, password, host, port, dbName string) string {
	params := []string{"parseTime=true", "multiStatements=true", "timeout=2s"}
	paramStr := strings.Join(params, "&")

	escapedUser := url.QueryEscape(user)
	escapedPassword := url.QueryEscape(password)
	escapedDBName := url.QueryEscape(dbName)

	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?%s", escapedUser, escapedPassword, host, port, escapedDBName, paramStr)
}

func resolveHost(host string) string {
	if host == "" {
		return host
	}

	if strings.EqualFold(host, "localhost") {
		return "127.0.0.1"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if ips, err := net.DefaultResolver.LookupHost(ctx, host); err == nil && len(ips) > 0 {
		return ips[0]
	}

	if _, err := exec.LookPath("getent"); err == nil {
		if output, err := exec.Command("getent", "hosts", host).Output(); err == nil {
			fields := strings.Fields(string(output))
			if len(fields) > 0 {
				return fields[0]
			}
		}
	}

	switch strings.ToLower(host) {
	case "host.docker.internal", "host.containers.internal":
		return "127.0.0.1"
	}

	return host
}

func setDefaultEnv(key, value string) {
	if os.Getenv(key) != "" {
		return
	}
	_ = os.Setenv(key, value)
}

// ResetTestDB is a test helper that resets the database to canonical state.
// Call this via t.Cleanup(ResetTestDB) at the start of tests that modify data.
func ResetTestDB() {
	if err := resetTestDatabase(); err != nil {
		// Log but don't fail - cleanup errors shouldn't fail the next test
		fmt.Fprintf(os.Stderr, "WARNING: resetTestDatabase failed: %v\n", err)
	}
}

// WithCleanDB resets the database to canonical state at the START of the test
// and registers a cleanup hook to reset again after the test completes.
// Usage: WithCleanDB(t) at the start of any test that modifies the database.
func WithCleanDB(t *testing.T) {
	t.Helper()
	ResetTestDB() // Reset at start to ensure clean state
	t.Cleanup(ResetTestDB)
}

// - Ticket 123 exists for attachment tests.
func resetTestDatabase() error {
	// Reinitialize the real test database connection in case a previous test
	// injected a mock DB that wasn't properly cleaned up
	if err := database.InitTestDB(); err != nil {
		return fmt.Errorf("failed to initialize test DB: %w", err)
	}

	db, err := database.GetDB()
	if err != nil {
		return fmt.Errorf("failed to get DB connection: %w", err)
	}

	// All cleanup runs on one dedicated connection: disabling FK checks is
	// session state (MySQL FOREIGN_KEY_CHECKS, PostgreSQL
	// session_replication_role), so it must not leak into the pool.
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("failed to get DB connection: %w", err)
	}
	defer conn.Close()

	var errs []error
	exec := func(query string, args ...any) {
		if _, err := conn.ExecContext(ctx, database.ConvertPlaceholders(query), args...); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", strings.Join(strings.Fields(query), " "), err))
		}
	}

	// Disable foreign key checks for cleanup
	exec("SET FOREIGN_KEY_CHECKS = 0")
	defer conn.ExecContext(ctx, database.ConvertPlaceholders("SET FOREIGN_KEY_CHECKS = 1"))

	// Clean ALL tickets - we'll recreate canonical test data
	exec("DELETE FROM ticket_history")
	exec("DELETE FROM article_data_mime")
	exec("DELETE FROM article_data_mime_attachment")
	exec("DELETE FROM article")
	exec("DELETE FROM ticket")

	// Clean API tokens (all test tokens)
	exec("DELETE FROM user_api_tokens")

	// Clean test states (preserve IDs 1-5 and the OTRS default states from 000029)
	exec("DELETE FROM ticket_state WHERE id > 5 AND name NOT IN ('pending auto close+', 'pending auto close-', 'removed', 'merged')")

	// Clean test types (preserve IDs 1-5)
	exec("DELETE FROM ticket_type WHERE id > 5")

	// Clean test queues (preserve the seeded IDs 1-6: migration 000002 + test_integration seed)
	exec("DELETE FROM queue WHERE id > 6")

	// Clean test groups (preserve the seeded IDs 1-5: users, admin, stats, support, testgroup)
	exec("DELETE FROM group_user WHERE group_id > 5")
	exec("DELETE FROM `groups` WHERE id > 5")

	// Clean test users (preserve the seeded agents: 1-2, 15 testuser, e2e-2fa-agent)
	exec("DELETE FROM group_user WHERE user_id IN (SELECT id FROM users WHERE id > 2 AND id <> 15 AND login <> 'e2e-2fa-agent')")
	exec("DELETE FROM users WHERE id > 2 AND id <> 15 AND login <> 'e2e-2fa-agent'")

	// Clean test dynamic fields
	exec("DELETE FROM dynamic_field_value WHERE id > 0")
	exec("DELETE FROM dynamic_field WHERE id > 10")

	// Restore canonical state names
	canonicalStates := map[int]string{
		1: "new",
		2: "open",
		3: "pending reminder",
		4: "closed successful",
		5: "closed unsuccessful",
	}
	for id, name := range canonicalStates {
		exec("UPDATE ticket_state SET name = ? WHERE id = ?", name, id)
	}

	// Restore canonical type names
	canonicalTypes := map[int]string{
		1: "Unclassified",
		2: "Incident",
		3: "Service Request",
		4: "Problem",
		5: "Change Request",
	}
	for id, name := range canonicalTypes {
		exec("UPDATE ticket_type SET name = ? WHERE id = ?", name, id)
	}

	// Restore canonical priority names
	canonicalPriorities := map[int]string{
		1: "1 very low",
		2: "2 low",
		3: "3 normal",
		4: "4 high",
		5: "5 very high",
	}
	for id, name := range canonicalPriorities {
		exec("UPDATE ticket_priority SET name = ? WHERE id = ?", name, id)
	}

	// Restore canonical queue names, valid_id, and group_id (from migrations)
	// group_id = 1 (users group) for all canonical queues
	canonicalQueues := map[int]string{
		1: "Postmaster",
		2: "Raw",
		3: "Junk",
		4: "Misc",
	}
	for id, name := range canonicalQueues {
		exec(`INSERT IGNORE INTO queue (id, name, group_id, system_address_id, salutation_id, signature_id,
			follow_up_id, follow_up_lock, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, ?, 1, 1, 1, 1, 1, 0, 1, NOW(), 1, NOW(), 1)`, id, name)
		exec("UPDATE queue SET name = ?, group_id = 1, valid_id = 1 WHERE id = ?", name, id)
	}

	// Seed canonical test tickets
	// Raw queue (id=2) has 2 tickets
	exec(`INSERT INTO ticket (id, tn, title, queue_id, ticket_lock_id, type_id, user_id, 
		responsible_user_id, ticket_priority_id, ticket_state_id, customer_id, customer_user_id,
		timeout, until_time, escalation_time, escalation_update_time, escalation_response_time,
		escalation_solution_time, archive_flag, create_time, create_by, change_time, change_by)
		VALUES (1, 'RAW-0001', 'First Raw queue ticket', 2, 1, 1, 1, 1, 3, 2,
		'test-customer', 'test@example.com', 0, 0, 0, 0, 0, 0, 0, NOW(), 1, NOW(), 1)`)
	exec(`INSERT INTO ticket (id, tn, title, queue_id, ticket_lock_id, type_id, user_id, 
		responsible_user_id, ticket_priority_id, ticket_state_id, customer_id, customer_user_id,
		timeout, until_time, escalation_time, escalation_update_time, escalation_response_time,
		escalation_solution_time, archive_flag, create_time, create_by, change_time, change_by)
		VALUES (2, 'RAW-0002', 'Second Raw queue ticket', 2, 1, 1, 1, 1, 3, 2,
		'test-customer', 'test@example.com', 0, 0, 0, 0, 0, 0, 0, NOW(), 1, NOW(), 1)`)

	// Junk queue (id=3) has 1 ticket
	exec(`INSERT INTO ticket (id, tn, title, queue_id, ticket_lock_id, type_id, user_id, 
		responsible_user_id, ticket_priority_id, ticket_state_id, customer_id, customer_user_id,
		timeout, until_time, escalation_time, escalation_update_time, escalation_response_time,
		escalation_solution_time, archive_flag, create_time, create_by, change_time, change_by)
		VALUES (3, 'JUNK-0001', 'Junk queue ticket', 3, 1, 1, 1, 1, 3, 2,
		'test-customer', 'test@example.com', 0, 0, 0, 0, 0, 0, 0, NOW(), 1, NOW(), 1)`)

	// Ticket 123 for attachment tests
	exec(`INSERT INTO ticket (id, tn, title, queue_id, ticket_lock_id, type_id, user_id, 
		responsible_user_id, ticket_priority_id, ticket_state_id, customer_id, customer_user_id,
		timeout, until_time, escalation_time, escalation_update_time, escalation_response_time,
		escalation_solution_time, archive_flag, create_time, create_by, change_time, change_by)
		VALUES (123, 'TEST-0123', 'Test Ticket for Attachments', 1, 1, 1, 1, 1, 3, 2,
		'test-customer', 'test@example.com', 0, 0, 0, 0, 0, 0, 0, NOW(), 1, NOW(), 1)`)

	// Seed support group (id=4) for admin user tests
	exec(`INSERT IGNORE INTO groups (id, name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (4, 'support', 1, NOW(), 1, NOW(), 1)`)

	// Seed testuser (id=15) for admin user tests
	exec(`INSERT IGNORE INTO users (id, login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (15, 'testuser', 'test', 'Test', 'Agent', 1, NOW(), 1, NOW(), 1)`)

	// Ensure canonical groups exist (users=1, admin=2, stats=3, support=4)
	exec(`INSERT IGNORE INTO groups (id, name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (1, 'users', 1, NOW(), 1, NOW(), 1)`)
	exec(`INSERT IGNORE INTO groups (id, name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (2, 'admin', 1, NOW(), 1, NOW(), 1)`)
	exec(`INSERT IGNORE INTO groups (id, name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (3, 'stats', 1, NOW(), 1, NOW(), 1)`)

	// CRITICAL: Grant user 1 (root) full permissions for queue access middleware
	// The queue_access middleware checks group_user for permission_key
	// 'rw' supersedes all other permissions (ro, create, move_into, etc.)
	// group_user has no unique key, so replace the grant instead of INSERT IGNORE.
	for groupID := 1; groupID <= 4; groupID++ {
		exec("DELETE FROM group_user WHERE user_id = 1 AND group_id = ? AND permission_key = 'rw'", groupID)
		exec(`INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
			VALUES (1, ?, 'rw', NOW(), 1, NOW(), 1)`, groupID)
	}

	// Rows above were inserted with explicit ids; PostgreSQL sequences do not
	// follow explicit ids, so move them past the highest id.
	if database.IsPostgreSQL() {
		for _, table := range []string{"ticket", "queue", "groups", "users"} {
			exec(fmt.Sprintf("SELECT setval(pg_get_serial_sequence('%[1]s', 'id'), (SELECT COALESCE(MAX(id), 1) FROM %[1]s))", table))
		}
	}

	// Re-initialize the ticket number generator in case a previous test set it to nil
	if err := initTestTicketNumberGenerator(); err != nil {
		errs = append(errs, fmt.Errorf("failed to re-initialize ticket number generator: %w", err))
	}

	return errors.Join(errs...)
}

// initTestTicketNumberGenerator initializes the global ticket number generator.
// Called once in TestMain before all tests run.
func initTestTicketNumberGenerator() error {
	db, err := database.GetDB()
	if err != nil {
		return fmt.Errorf("failed to get DB: %w", err)
	}
	if db == nil {
		return nil // No DB, skip
	}

	gen, err := ticketnumber.Resolve("DateChecksum", "10", nil)
	if err != nil {
		return fmt.Errorf("failed to create generator: %w", err)
	}

	repository.SetTicketNumberGenerator(gen, ticketnumber.NewDBStore(db, "10"))
	return nil
}
