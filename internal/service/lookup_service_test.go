package service

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/database"
)

func lookupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)
	return db
}

func countRows(t *testing.T, db *sql.DB, query string) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(query)).Scan(&n))
	return n
}

func insertTicketType(t *testing.T, db *sql.DB, name string) int64 {
	t.Helper()
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO ticket_type (name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)
		RETURNING id`), name)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM ticket_type WHERE id = ?`), id)
	})
	return id
}

func findItem(items []models.LookupItem, id int) (models.LookupItem, bool) {
	for _, it := range items {
		if it.ID == id {
			return it, true
		}
	}
	return models.LookupItem{}, false
}

// Every list in the form data mirrors its table: no invented default queues,
// types or priorities, and no states dropped to a fixed "5-state workflow".
func TestLookupFormDataMirrorsTables(t *testing.T) {
	db := lookupTestDB(t)

	stateName := fmt.Sprintf("lookup state %d", time.Now().UnixNano())
	stateID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO ticket_state (name, comments, type_id, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, '', (SELECT id FROM ticket_state_type WHERE name = 'open'), 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)
		RETURNING id`), stateName)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM ticket_state WHERE id = ?`), stateID)
	})
	typeName := fmt.Sprintf("LookupType%d", time.Now().UnixNano())
	typeID := insertTicketType(t, db, typeName)

	data, err := NewLookupService().GetTicketFormDataWithLang("en")
	require.NoError(t, err)

	assert.Len(t, data.Statuses, countRows(t, db, `SELECT COUNT(*) FROM ticket_state`))
	assert.Len(t, data.Priorities, countRows(t, db, `SELECT COUNT(*) FROM ticket_priority WHERE valid_id = 1`))
	assert.Len(t, data.Queues, countRows(t, db, `SELECT COUNT(*) FROM queue WHERE valid_id = 1`))
	assert.Len(t, data.Types, countRows(t, db, `SELECT COUNT(*) FROM ticket_type WHERE valid_id = 1`))

	st, ok := findItem(data.Statuses, int(stateID))
	require.True(t, ok, "new ticket_state row missing from statuses")
	assert.Equal(t, stateName, st.Value)

	typ, ok := findItem(data.Types, int(typeID))
	require.True(t, ok, "new ticket_type row missing from types")
	assert.Equal(t, typeName, typ.Value)
	assert.Equal(t, typeName, typ.Label)
	assert.True(t, typ.Active)

	for _, q := range data.Queues {
		var name string
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
			`SELECT name FROM queue WHERE id = ? AND valid_id = 1`), q.ID).Scan(&name), "queue id %d not a valid queue row", q.ID)
	}
}

// Results are cached per language until InvalidateCache.
func TestLookupFormDataCacheInvalidation(t *testing.T) {
	db := lookupTestDB(t)
	svc := NewLookupService()

	_, err := svc.GetTicketFormDataWithLang("en")
	require.NoError(t, err)

	typeID := insertTicketType(t, db, fmt.Sprintf("LookupCache%d", time.Now().UnixNano()))

	cached, err := svc.GetTicketFormDataWithLang("en")
	require.NoError(t, err)
	_, ok := findItem(cached.Types, int(typeID))
	assert.False(t, ok, "cached data must not see rows inserted after the first load")

	svc.InvalidateCache()
	fresh, err := svc.GetTicketFormDataWithLang("en")
	require.NoError(t, err)
	_, ok = findItem(fresh.Types, int(typeID))
	assert.True(t, ok, "invalidated cache must reload from the database")
}
