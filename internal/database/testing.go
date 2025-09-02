package database

import (
	"database/sql"
    "fmt"
	"log"

	"github.com/gotrs-io/gotrs-ce/internal/services/adapter"
)

// testDB is kept for API compatibility; we do not own the lifecycle
var testDB *sql.DB

// InitTestDB initializes a database connection for tests using the
// project service adapter. It is safe to call multiple times.
// It does not create schema by default; individual tests should
// create required tables with CREATE TABLE IF NOT EXISTS as needed.
func InitTestDB() error {
	// Ensure the service registry and database are configured
	if err := adapter.AutoConfigureDatabase(); err != nil {
		// Not fatal; adapter.GetDB may still return a direct connection
		log.Printf("InitTestDB: AutoConfigureDatabase warning: %v", err)
	}

    db, err := GetDB()
    if err != nil {
        return err
    }
    if db == nil {
        return fmt.Errorf("no database connection available")
    }
	// Basic connectivity check
	if err := db.Ping(); err != nil {
		return err
	}
	// Keep a reference for CloseTestDB (no-op close semantics)
	testDB = db
	return nil
}

// CloseTestDB is a no-op to avoid interfering with the global
// service-managed database connection. Left for API compatibility.
func CloseTestDB() {
	// Intentionally no-op. Tests that open dedicated connections must
	// manage their own lifecycle.
}

// InitDB is kept for backward-compatibility with older tests; delegates to InitTestDB.
func InitDB() error {
    return InitTestDB()
}

// SetDB allows tests to inject a mock *sql.DB for functions that call GetDB.
// Use ResetDB to restore the previous value.
func SetDB(db *sql.DB) {
    testDB = db
    // Also set the global adapter's DB if available
}

// ResetDB clears the test-injected DB.
func ResetDB() {
    testDB = nil
}