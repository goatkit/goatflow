package database

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"

	"github.com/gotrs-io/gotrs-ce/internal/services/adapter"
)

// GetDB returns the database connection singleton from the service registry.
// Service registry is the single source of truth for database connections.
func GetDB() (*sql.DB, error) {
	testDBMu.RLock()
	override := testDBOverride
	current := testDB
	testDBMu.RUnlock()

	if override && current != nil {
		return current, nil
	}

	if current != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		err := current.PingContext(ctx)
		cancel()
		if err == nil {
			return current, nil
		}
		// Drop stale pointer and fall back to the service-managed connection.
		testDBMu.Lock()
		if testDB == current {
			testDB = nil
		}
		testDBMu.Unlock()
	}

	db, err := adapter.GetDB()
	if err != nil {
		return nil, err
	}

	testDBMu.Lock()
	if !testDBOverride {
		testDB = db
	}
	testDBMu.Unlock()

	return db, nil
}
