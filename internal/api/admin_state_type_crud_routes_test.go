package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// These exercise the exact requests templates/pages/admin/states.pongo2 and
// types.pongo2 send, through the production YAML router.

func adminCRUDRequest(t *testing.T, router *gin.Engine, method, path, contentType string, body io.Reader) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	AddTestAuthCookie(req, GetTestAuthToken(t))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return w.Code, resp
}

func insertTicketUsing(t *testing.T, db *sql.DB, column string, value int64) int64 {
	t.Helper()
	stateID, typeID := int64(1), int64(1)
	if column == "ticket_state_id" {
		stateID = value
	} else {
		typeID = value
	}
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO ticket (tn, title, queue_id, ticket_lock_id, type_id, user_id, responsible_user_id,
			ticket_priority_id, ticket_state_id, timeout, until_time, escalation_time, escalation_update_time,
			escalation_response_time, escalation_solution_time, archive_flag, create_time, create_by, change_time, change_by)
		VALUES (?, 'crud in-use', 1, 1, ?, 1, 1, 3, ?, 0, 0, 0, 0, 0, 0, 0, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)
		RETURNING id`), fmt.Sprintf("CRUD%d", time.Now().UnixNano()%1e12), typeID, stateID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM ticket WHERE id = ?`), id)
	})
	return id
}

// validTicketTypeCount returns the number of valid ticket_type rows and drops
// the lookup cache so the next /api/lookups request reflects them.
func validTicketTypeCount(t *testing.T) int {
	t.Helper()
	db := getTestDB(t)
	var n int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		`SELECT COUNT(*) FROM ticket_type WHERE valid_id = 1`)).Scan(&n))
	GetLookupService().InvalidateCache()
	return n
}

func validTicketTypeNames(t *testing.T) []string {
	t.Helper()
	rows, err := getTestDB(t).Query(database.ConvertPlaceholders(`SELECT name FROM ticket_type WHERE valid_id = 1`))
	require.NoError(t, err)
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		require.NoError(t, rows.Scan(&n))
		names = append(names, n)
	}
	require.NoError(t, rows.Err())
	return names
}

func TestAdminStateCRUDRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := getTestDB(t)
	router := NewSimpleRouterWithDB(db)

	name := fmt.Sprintf("crud state %d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM ticket_state WHERE name IN (?, ?)`), name, name+" renamed")
	})

	var pendingTypeID int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		`SELECT id FROM ticket_state_type WHERE name = 'pending reminder'`)).Scan(&pendingTypeID))

	body := fmt.Sprintf(`{"name":%q,"type_id":%d,"comments":"made by test","valid_id":1}`, name, pendingTypeID)
	code, resp := adminCRUDRequest(t, router, http.MethodPost, "/admin/states/create", "application/json", strings.NewReader(body))
	require.Equal(t, http.StatusCreated, code, resp)
	assert.Equal(t, true, resp["success"])

	var id int64
	var typeID, validID int
	var comments sql.NullString
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		`SELECT id, type_id, comments, valid_id FROM ticket_state WHERE name = ?`), name).Scan(&id, &typeID, &comments, &validID))
	assert.Equal(t, pendingTypeID, typeID)
	assert.Equal(t, "made by test", comments.String)
	assert.Equal(t, 1, validID)

	code, resp = adminCRUDRequest(t, router, http.MethodPost, "/admin/states/create", "application/json", strings.NewReader(body))
	assert.Equal(t, http.StatusConflict, code, resp)

	update := fmt.Sprintf(`{"name":%q,"type_id":%d,"comments":null,"valid_id":1}`, name+" renamed", pendingTypeID)
	for range 2 { // the identical second save must not report "not found"
		code, resp = adminCRUDRequest(t, router, http.MethodPut, fmt.Sprintf("/admin/states/%d/update", id), "application/json", strings.NewReader(update))
		require.Equal(t, http.StatusOK, code, resp)
	}
	var gotName string
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT name FROM ticket_state WHERE id = ?`), id).Scan(&gotName))
	assert.Equal(t, name+" renamed", gotName)

	code, _ = adminCRUDRequest(t, router, http.MethodPut, "/admin/states/32000/update", "application/json", strings.NewReader(update))
	assert.Equal(t, http.StatusNotFound, code)

	ticketID := insertTicketUsing(t, db, "ticket_state_id", id)
	code, resp = adminCRUDRequest(t, router, http.MethodDelete, fmt.Sprintf("/admin/states/%d/delete", id), "", nil)
	assert.Equal(t, http.StatusConflict, code, resp)
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT valid_id FROM ticket_state WHERE id = ?`), id).Scan(&validID))
	assert.Equal(t, 1, validID)

	_, err := db.Exec(database.ConvertPlaceholders(`DELETE FROM ticket WHERE id = ?`), ticketID)
	require.NoError(t, err)
	code, resp = adminCRUDRequest(t, router, http.MethodDelete, fmt.Sprintf("/admin/states/%d/delete", id), "", nil)
	require.Equal(t, http.StatusOK, code, resp)
	assert.Equal(t, true, resp["success"])
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT valid_id FROM ticket_state WHERE id = ?`), id).Scan(&validID))
	assert.Equal(t, 2, validID)
}

func TestAdminTypeCRUDRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := getTestDB(t)
	router := NewSimpleRouterWithDB(db)

	name := fmt.Sprintf("crud type %d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM ticket_type WHERE name IN (?, ?)`), name, name+" renamed")
	})
	form := func(n string, valid int) io.Reader {
		return strings.NewReader(url.Values{"name": {n}, "valid_id": {fmt.Sprint(valid)}}.Encode())
	}
	const formCT = "application/x-www-form-urlencoded"

	code, resp := adminCRUDRequest(t, router, http.MethodPost, "/admin/types/create", formCT, form(name, 1))
	require.Equal(t, http.StatusCreated, code, resp)
	assert.Equal(t, true, resp["success"])

	var id int64
	var validID int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		`SELECT id, valid_id FROM ticket_type WHERE name = ?`), name).Scan(&id, &validID))
	assert.Equal(t, 1, validID)

	code, resp = adminCRUDRequest(t, router, http.MethodPost, "/admin/types/create", formCT, form(name, 1))
	assert.Equal(t, http.StatusBadRequest, code, resp)

	for range 2 {
		code, resp = adminCRUDRequest(t, router, http.MethodPost, fmt.Sprintf("/admin/types/%d/update", id), formCT, form(name+" renamed", 1))
		require.Equal(t, http.StatusOK, code, resp)
	}
	var gotName string
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT name FROM ticket_type WHERE id = ?`), id).Scan(&gotName))
	assert.Equal(t, name+" renamed", gotName)

	code, _ = adminCRUDRequest(t, router, http.MethodPost, "/admin/types/32000/update", formCT, form(name+" other", 1))
	assert.Equal(t, http.StatusNotFound, code)

	ticketID := insertTicketUsing(t, db, "type_id", id)
	code, resp = adminCRUDRequest(t, router, http.MethodPost, fmt.Sprintf("/admin/types/%d/delete", id), "", nil)
	assert.Equal(t, http.StatusBadRequest, code, resp)
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT valid_id FROM ticket_type WHERE id = ?`), id).Scan(&validID))
	assert.Equal(t, 1, validID)

	_, err := db.Exec(database.ConvertPlaceholders(`DELETE FROM ticket WHERE id = ?`), ticketID)
	require.NoError(t, err)
	code, resp = adminCRUDRequest(t, router, http.MethodPost, fmt.Sprintf("/admin/types/%d/delete", id), "", nil)
	require.Equal(t, http.StatusOK, code, resp)
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT valid_id FROM ticket_type WHERE id = ?`), id).Scan(&validID))
	assert.Equal(t, 2, validID)
}
