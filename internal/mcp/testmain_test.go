package mcp

import (
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/goatkit/goatflow/internal/database"
)

var (
	initDBOnce sync.Once
	initDBErr  error
)

func TestMain(m *testing.M) {
	// Ensure test environment
	if os.Getenv("TEST_DB_PASSWORD") == "" && os.Getenv("TEST_DB_MYSQL_PASSWORD") == "" {
		fmt.Fprintln(os.Stderr, "WARNING: TEST_DB_PASSWORD not set, integration tests will be skipped")
	}
	if os.Getenv("TEST_DB_PASSWORD") == "" && os.Getenv("TEST_DB_MYSQL_PASSWORD") != "" {
		os.Setenv("TEST_DB_PASSWORD", os.Getenv("TEST_DB_MYSQL_PASSWORD"))
	}

	// Initialize test database
	if err := database.InitTestDB(); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: Failed to init test DB: %v\n", err)
	}

	// Run tests
	code := m.Run()

	database.CloseTestDB()
	os.Exit(code)
}

// requireDatabase skips the test if database is not available
func requireDatabase(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	if db, err := database.GetDB(); err == nil && db != nil {
		return
	}

	initDBOnce.Do(func() {
		initDBErr = database.InitTestDB()
	})

	if initDBErr != nil {
		t.Skipf("skipping integration test: %v", initDBErr)
	}

	if db, err := database.GetDB(); err != nil || db == nil {
		t.Skip("Database not available, skipping MCP authorization tests")
	}
}
