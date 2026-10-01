package v1

import (
	"context"
	"database/sql"
	"sync"
	"testing"

	"github.com/goatkit/goatflow/internal/platform/database"
)

var (
	initDBOnce sync.Once
	initDBErr  error
)

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

	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skipf("skipping integration test: %v", err)
	}
}

// withoutFKChecks runs fn on one dedicated connection with foreign key checks
// disabled. The switch is session state (MySQL FOREIGN_KEY_CHECKS, PostgreSQL
// session_replication_role), so the statements must share its connection and
// the switch must be restored before the connection returns to the pool.
// exec converts placeholders for the active driver.
func withoutFKChecks(db *sql.DB, fn func(exec func(query string, args ...any) error)) error {
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	exec := func(query string, args ...any) error {
		_, err := conn.ExecContext(ctx, database.ConvertPlaceholders(query), args...)
		return err
	}
	if err := exec("SET FOREIGN_KEY_CHECKS = 0"); err != nil {
		return err
	}
	fn(exec)
	return exec("SET FOREIGN_KEY_CHECKS = 1")
}
