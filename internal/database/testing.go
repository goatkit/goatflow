package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/gotrs-io/gotrs-ce/internal/services/adapter"
)

// testDB is kept for API compatibility; we do not own the lifecycle.
var (
	testDBMu       sync.RWMutex
	testDB         *sql.DB
	testDBOverride bool
)

// IsTestDBOverride reports whether a test database has been explicitly injected via SetDB.
func IsTestDBOverride() bool {
	testDBMu.RLock()
	defer testDBMu.RUnlock()
	return testDBOverride && testDB != nil
}

// InitTestDB initializes a database connection for tests using the
// project service adapter. It is safe to call multiple times.
// It does not create schema by default; individual tests should
// create required tables with CREATE TABLE IF NOT EXISTS as needed.
func InitTestDB() error {
	// In test environment with no DB configured, fast-return success (DB-less)
	if v := os.Getenv("APP_ENV"); v == "test" {
		if os.Getenv("TEST_DB_HOST") == "" && os.Getenv("TEST_DB_NAME") == "" && os.Getenv("DATABASE_URL") == "" {
			// Leave testDB nil so callers that require DB can detect absence,
			// but allow DB-less tests to proceed quickly.
			return nil
		}
	}
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
	testDBMu.Lock()
	testDB = db
	testDBOverride = false
	testDBMu.Unlock()
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

// dbStack holds saved DB states for nested SetDB/ResetDB calls.
type dbState struct {
	db       *sql.DB
	override bool
}

var dbStack []dbState

// SetDB allows tests to inject a mock *sql.DB for functions that call GetDB.
// Use ResetDB to restore the previous value. Uses a stack to handle nested calls.
func SetDB(db *sql.DB) {
	testDBMu.Lock()
	dbStack = append(dbStack, dbState{db: testDB, override: testDBOverride})
	testDB = db
	testDBOverride = db != nil
	testDBMu.Unlock()
}

// ResetDB restores the test-injected DB to the previously saved value.
// This allows tests that inject mocks to not interfere with other tests
// that expect the real test database to be available.
func ResetDB() {
	testDBMu.Lock()
	if len(dbStack) > 0 {
		state := dbStack[len(dbStack)-1]
		dbStack = dbStack[:len(dbStack)-1]
		testDB = state.db
		testDBOverride = state.override
	} else {
		testDB = nil
		testDBOverride = false
	}
	testDBMu.Unlock()
}
