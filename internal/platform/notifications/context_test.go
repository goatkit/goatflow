package notifications

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
)

func renderContextTestDB(t *testing.T) *sql.DB {
	t.Helper()
	if os.Getenv("GOATFLOW_TEST_DB_READY") == "" {
		t.Skip("integration test: needs the test database (GOATFLOW_TEST_DB_READY)")
	}
	require.NoError(t, database.InitTestDB())
	db, err := database.GetDB()
	require.NoError(t, err)
	return db
}

// closedDB returns a handle for the driver under test that has been closed, so
// every query fails the way a lost connection pool does.
func closedDB(t *testing.T) *sql.DB {
	t.Helper()
	driver := "postgres"
	if database.IsMySQL() {
		driver = "mysql"
	}
	db, err := sql.Open(driver, "")
	require.NoError(t, err)
	require.NoError(t, db.Close())
	return db
}

func TestBuildRenderContextNilDB(t *testing.T) {
	rc, err := BuildRenderContext(context.Background(), nil, "someone@example.test", 1)
	require.Error(t, err)
	require.Nil(t, rc)
}

func TestBuildRenderContextClosedDB(t *testing.T) {
	db := closedDB(t)

	rc, err := BuildRenderContext(context.Background(), db, "someone@example.test", 0)
	require.Error(t, err, "customer lookup on a closed pool must fail, not yield the bare login")
	require.Nil(t, rc)

	rc, err = BuildRenderContext(context.Background(), db, "", 1)
	require.Error(t, err, "agent lookup on a closed pool must fail, not yield empty names")
	require.Nil(t, rc)
}

func TestBuildRenderContextRealRows(t *testing.T) {
	db := renderContextTestDB(t)
	now := time.Now().UTC()
	suffix := fmt.Sprintf("%d", now.UnixNano())
	customerLogin := "rcx-cust-" + suffix
	agentLogin := "rcx-agent-" + suffix

	_, err := db.Exec(database.ConvertPlaceholders(`
		INSERT INTO customer_user (login, email, customer_id, first_name, last_name, valid_id,
			create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'rcx-co', 'Ada', 'Lovelace', 1, ?, 1, ?, 1)`),
		customerLogin, customerLogin+"@example.test", now, now)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM customer_user WHERE login = ?`), customerLogin)
	})

	agentID64, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 'x', 'Grace', 'Hopper', 1, ?, 1, ?, 1) RETURNING id`), agentLogin, now, now)
	require.NoError(t, err)
	agentID := int(agentID64)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM users WHERE id = ?`), agentID)
	})

	t.Run("customer and agent rows give their real names", func(t *testing.T) {
		rc, err := BuildRenderContext(context.Background(), db, customerLogin, agentID)
		require.NoError(t, err)
		require.Equal(t, &RenderContext{
			CustomerFullName: "Ada Lovelace",
			AgentFirstName:   "Grace",
			AgentLastName:    "Hopper",
		}, rc)
	})

	t.Run("customer matched by email", func(t *testing.T) {
		rc, err := BuildRenderContext(context.Background(), db, customerLogin+"@example.test", 0)
		require.NoError(t, err)
		require.Equal(t, "Ada Lovelace", rc.CustomerFullName)
	})

	t.Run("customer login without customer_user row falls back to the login", func(t *testing.T) {
		rc, err := BuildRenderContext(context.Background(), db, "walk-in-"+suffix+"@example.test", agentID)
		require.NoError(t, err)
		require.Equal(t, "walk-in-"+suffix+"@example.test", rc.CustomerFullName)
		require.Equal(t, "Grace", rc.AgentFirstName)
	})

	t.Run("agent id without users row is an error", func(t *testing.T) {
		var maxID int
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT COALESCE(MAX(id), 0) FROM users`)).Scan(&maxID))
		rc, err := BuildRenderContext(context.Background(), db, customerLogin, maxID+1000)
		require.Error(t, err)
		require.Nil(t, rc)
	})
}
