package mcp

import (
	"fmt"
	"os"
	"testing"

	"github.com/goatkit/goatflow/internal/database"
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

	database.CloseTestDB()
	os.Exit(code)
}
