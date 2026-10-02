package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
)

const requiredTimeUnitsSetting = "Ticket::Frontend::AgentTicketNote###RequiredTimeUnits"

// closedSysconfigDB returns a closed handle for the driver under test: every
// query on it fails the way a lost connection pool does.
func closedSysconfigDB(t *testing.T) *sql.DB {
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

// insertSysconfigDefault adds a sysconfig_default row for the test and returns its id.
func insertSysconfigDefault(t *testing.T, db *sql.DB, name, value string) int64 {
	t.Helper()
	now := time.Now().UTC()
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO sysconfig_default (
			name, description, navigation, is_invisible, is_readonly, is_required,
			is_valid, has_configlevel, user_modification_possible, user_modification_active,
			xml_content_raw, xml_content_parsed, xml_filename, effective_value,
			is_dirty, exclusive_lock_guid, create_time, create_by, change_time, change_by
		) VALUES (?, 'sysconfig value test', 'Core::Test', 0, 0, 0,
			1, 0, 0, 0, '', '', 'Test.xml', ?, 0, '', ?, 1, ?, 1) RETURNING id`), name, value, now, now)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM sysconfig_modified WHERE name = ?`), name)
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM sysconfig_default WHERE name = ?`), name)
	})
	return id
}

func TestSysconfigValueWithoutUsableDatabase(t *testing.T) {
	name := "Test::Sysconfig::NoDB"

	_, ok, err := sysconfigValue(nil, name)
	require.Error(t, err)
	require.False(t, ok)

	_, ok, err = sysconfigValue(closedSysconfigDB(t), name)
	require.Error(t, err, "a failed query must not read as 'setting not configured'")
	require.False(t, ok)

	_, err = isTimeUnitsRequired(closedSysconfigDB(t))
	require.Error(t, err)
}

func TestSysconfigValueModifiedOverridesDefault(t *testing.T) {
	db, err := database.GetDB()
	require.NoError(t, err)
	name := fmt.Sprintf("Test::Sysconfig::Precedence%d", time.Now().UnixNano())

	_, ok, err := sysconfigValue(db, name)
	require.NoError(t, err)
	require.False(t, ok, "no row in either table means not configured")

	defaultID := insertSysconfigDefault(t, db, name, "false")
	value, ok, err := sysconfigValue(db, name)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "false", value)

	now := time.Now().UTC()
	_, err = db.Exec(database.ConvertPlaceholders(`
		INSERT INTO sysconfig_modified
		(sysconfig_default_id, name, user_id, effective_value, is_valid, user_modification_active,
		 is_dirty, reset_to_default, create_time, create_by, change_time, change_by)
		VALUES (?, ?, NULL, ?, 1, 0, 0, 0, ?, 1, ?, 1)`), defaultID, name, "true", now, now)
	require.NoError(t, err)

	value, ok, err = sysconfigValue(db, name)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "true", value)

	enabled, ok, err := sysconfigBool(db, name)
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, enabled)
}

func postNoteJSON(ticketID string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/tickets/:id/notes", handleAddTicketNote)
	body, _ := json.Marshal(map[string]any{"content": "time units lookup test", "internal": true})
	req := httptest.NewRequest(http.MethodPost, "/tickets/"+ticketID+"/notes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// A note must not be saved when the RequiredTimeUnits setting cannot be read:
// guessing "not required" would accept notes the admin made time-mandatory.
func TestAddTicketNoteFailsWhenTimeUnitsSettingUnreadable(t *testing.T) {
	t.Run("corrupt setting value", func(t *testing.T) {
		db, err := database.GetDB()
		require.NoError(t, err)
		var existing int
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
			`SELECT COUNT(*) FROM sysconfig_default WHERE name = ?`), requiredTimeUnitsSetting).Scan(&existing))
		require.Zero(t, existing, "test DB already defines %s", requiredTimeUnitsSetting)
		insertSysconfigDefault(t, db, requiredTimeUnitsSetting, "sometimes")
		ticketID := createWriteTestTicket(t, db, nil)

		w := postNoteJSON(fmt.Sprint(ticketID))

		require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
		require.JSONEq(t, `{"success":false,"error":"Failed to load note settings"}`, w.Body.String())
		require.Zero(t, countRows(t, db, `SELECT COUNT(*) FROM article WHERE ticket_id = ?`, ticketID),
			"note saved although the time units setting could not be read")
	})

	t.Run("database query fails", func(t *testing.T) {
		database.SetDB(closedSysconfigDB(t))
		t.Cleanup(database.ResetDB)

		w := postNoteJSON("TN-NOT-RESOLVED")

		require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
		require.JSONEq(t, `{"success":false,"error":"Failed to load note settings"}`, w.Body.String())
	})
}
