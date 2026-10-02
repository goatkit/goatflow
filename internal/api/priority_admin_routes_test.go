package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// The admin priorities page (pages/admin/priorities.pongo2) and the lookups
// page (pages/admin/lookups.pongo2) create, update and delete priorities
// through /api/v1/priorities. These run through the real YAML router.
func TestPriorityWriteRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := getTestDB(t)
	router := NewSimpleRouterWithDB(db)
	adminToken := GetTestAuthToken(t)

	send := func(t *testing.T, token, method, path string, body any) *httptest.ResponseRecorder {
		t.Helper()
		var buf bytes.Buffer
		if body != nil {
			require.NoError(t, json.NewEncoder(&buf).Encode(body))
		}
		req := httptest.NewRequest(method, path, &buf)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	name := fmt.Sprintf("RouteAuditPrio %d", time.Now().UnixNano()%1_000_000_000)
	var id int
	t.Cleanup(func() {
		if id > 0 {
			_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM ticket_priority WHERE id = ?`), id)
		}
	})

	t.Run("create", func(t *testing.T) {
		w := send(t, adminToken, http.MethodPost, "/api/v1/priorities", map[string]any{"name": name, "color": "#112233"})
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

		var resp struct {
			Success bool `json:"success"`
			Data    struct {
				ID    int    `json:"id"`
				Name  string `json:"name"`
				Color string `json:"color"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.True(t, resp.Success)
		require.Positive(t, resp.Data.ID)
		id = resp.Data.ID
		assert.Equal(t, name, resp.Data.Name)

		var gotName, gotColor string
		var validID int
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
			`SELECT name, color, valid_id FROM ticket_priority WHERE id = ?`), id).Scan(&gotName, &gotColor, &validID))
		assert.Equal(t, name, gotName)
		assert.Equal(t, "#112233", gotColor)
		assert.Equal(t, 1, validID)
	})
	require.Positive(t, id, "create must succeed for the remaining steps")

	t.Run("update", func(t *testing.T) {
		w := send(t, adminToken, http.MethodPut, fmt.Sprintf("/api/v1/priorities/%d", id), map[string]any{"name": name + " v2", "color": "#445566"})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), `"success":true`)

		var gotName, gotColor string
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
			`SELECT name, color FROM ticket_priority WHERE id = ?`), id).Scan(&gotName, &gotColor))
		assert.Equal(t, name+" v2", gotName)
		assert.Equal(t, "#445566", gotColor)
	})

	t.Run("list including invalid shows the row", func(t *testing.T) {
		w := send(t, adminToken, http.MethodGet, "/api/v1/priorities?valid=all", nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), name+" v2")
	})

	t.Run("non-admin agent is rejected", func(t *testing.T) {
		agentToken := testSessionToken(t, GetTestAuthConfig().UserID, "agent@localhost", "agent@localhost", "Agent", false, 0)
		w := send(t, agentToken, http.MethodDelete, fmt.Sprintf("/api/v1/priorities/%d", id), nil)
		require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

		var validID int
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
			`SELECT valid_id FROM ticket_priority WHERE id = ?`), id).Scan(&validID))
		assert.Equal(t, 1, validID)
	})

	t.Run("delete invalidates", func(t *testing.T) {
		w := send(t, adminToken, http.MethodDelete, fmt.Sprintf("/api/v1/priorities/%d", id), nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), `"success":true`)

		var validID int
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
			`SELECT valid_id FROM ticket_priority WHERE id = ?`), id).Scan(&validID))
		assert.Equal(t, 2, validID)
	})
}
