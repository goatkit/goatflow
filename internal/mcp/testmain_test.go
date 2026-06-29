//go:build mcp_legacy_tests

package mcp

import (
	"fmt"
	"os"
	"testing"

	"github.com/goatkit/goatflow/internal/platform/database"
)

func TestMain(m *testing.M) {
	// Ensure test environment
	if os.Getenv("TEST_DB_PASSWORD") == "" && os.Getenv("TEST_DB_MYSQL_PASSWORD") == "" {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "╔══════════════════════════════════════════════════════════════════╗")
		fmt.Fprintln(os.Stderr, "║  FATAL: TEST DATABASE UNAVAILABLE                               ║")
		fmt.Fprintln(os.Stderr, "╠══════════════════════════════════════════════════════════════════╣")
		fmt.Fprintln(os.Stderr, "║  MCP tests require the test database to be running.             ║")
		fmt.Fprintln(os.Stderr, "║  Tests cannot be skipped - a real database is required.         ║")
		fmt.Fprintln(os.Stderr, "║                                                                 ║")
		fmt.Fprintln(os.Stderr, "║  To start the database:                                         ║")
		fmt.Fprintln(os.Stderr, "║    make test-db-up                                              ║")
		fmt.Fprintln(os.Stderr, "║                                                                 ║")
		fmt.Fprintln(os.Stderr, "║  Then run tests:                                                ║")
		fmt.Fprintln(os.Stderr, "║    make toolbox-exec ARGS=\"go test ./internal/mcp/...\"          ║")
		fmt.Fprintln(os.Stderr, "╚══════════════════════════════════════════════════════════════════╝")
		fmt.Fprintln(os.Stderr, "")
		os.Exit(1)
	}
	if os.Getenv("TEST_DB_PASSWORD") == "" && os.Getenv("TEST_DB_MYSQL_PASSWORD") != "" {
		os.Setenv("TEST_DB_PASSWORD", os.Getenv("TEST_DB_MYSQL_PASSWORD"))
	}

	// Initialize test database — fail hard, don't skip
	if err := database.InitTestDB(); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: Failed to init test DB: %v\n", err)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Clean up MCP test fixtures so other packages aren't affected
	cleanupMCPTestData()

	database.CloseTestDB()
	os.Exit(code)
}

// cleanupMCPTestData removes all MCP test fixtures from the shared test database.
// This prevents cross-package test pollution when running the full suite.
func cleanupMCPTestData() {
	db, err := database.GetDB()
	if err != nil || db == nil {
		return
	}
	db.Exec("SET FOREIGN_KEY_CHECKS=0")
	db.Exec(database.ConvertPlaceholders("DELETE FROM ticket WHERE id >= 80000 AND id < 90000"))
	db.Exec(database.ConvertPlaceholders("DELETE FROM article WHERE ticket_id >= 80000 AND ticket_id < 90000"))
	db.Exec(database.ConvertPlaceholders("DELETE FROM ticket_history WHERE ticket_id >= 80000 AND ticket_id < 90000"))
	db.Exec(database.ConvertPlaceholders("DELETE FROM user_api_tokens WHERE user_id >= 80000 AND user_id < 90000"))
	db.Exec(database.ConvertPlaceholders("DELETE FROM group_user WHERE user_id >= 80000 AND user_id < 90000"))
	db.Exec(database.ConvertPlaceholders("DELETE FROM group_customer WHERE customer_id LIKE 'mcptest-%'"))
	db.Exec(database.ConvertPlaceholders("DELETE FROM customer_user WHERE login LIKE '%mcptest%'"))
	db.Exec(database.ConvertPlaceholders("DELETE FROM customer_company WHERE customer_id LIKE 'mcptest-%'"))
	db.Exec(database.ConvertPlaceholders("DELETE FROM queue WHERE id >= 80000 AND id < 90000"))
	db.Exec(database.ConvertPlaceholders("DELETE FROM `groups` WHERE id >= 80000 AND id < 90000"))
	db.Exec(database.ConvertPlaceholders("DELETE FROM users WHERE id >= 80000 AND id < 90000"))
	db.Exec("SET FOREIGN_KEY_CHECKS=1")
}
