package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/routing"
	"github.com/goatkit/goatflow/internal/platform/shared"
)

// cannedRoutesEngine serves routes/api-canned-responses.yaml exactly as
// production loads it: YAML handler names plus the unified_auth middleware.
func cannedRoutesEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	src, err := os.ReadFile("../../routes/api-canned-responses.yaml")
	require.NoError(t, err)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "api-canned-responses.yaml"), src, 0o600))
	r := gin.New()
	require.NoError(t, routing.LoadYAMLRoutes(r, dir, NewRoutingHandlerResolver()))
	return r
}

func cannedToken(t *testing.T, userID uint, role string, isAdmin bool) string {
	t.Helper()
	tok, err := shared.GetJWTManager().GenerateTokenWithAdmin(userID, "cr-test@example.com", role, isAdmin, 0)
	require.NoError(t, err)
	return tok
}

func cannedRequest(t *testing.T, r *gin.Engine, method, path, token string, body any) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	out := map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &out) //nolint:errcheck // non-JSON bodies are asserted via status
	return w.Code, out
}

func cannedNames(t *testing.T, body map[string]any) []string {
	t.Helper()
	raw, ok := body["responses"].([]any)
	require.True(t, ok, "responses array missing: %v", body)
	names := make([]string, 0, len(raw))
	for _, item := range raw {
		names = append(names, item.(map[string]any)["name"].(string))
	}
	return names
}

func insertCannedResponse(t *testing.T, db *sql.DB, name, category, scope string, ownerID, usage int) int {
	t.Helper()
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO canned_response
		(name, category, content, content_type, tags, scope, owner_id, placeholders, usage_count, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, 'text', '[]', ?, ?, '[]', ?, 1, NOW(), 1, NOW(), 1)
		RETURNING id`), name, category, "Body of "+name, scope, ownerID, usage)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM canned_response WHERE id = ?`), id) //nolint:errcheck // cleanup
	})
	return int(id)
}

func insertCannedTestAgent(t *testing.T, db *sql.DB, login string) int {
	t.Helper()
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 'x', 'Canned', 'Tester', 1, NOW(), 1, NOW(), 1)
		RETURNING id`), login)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM canned_response WHERE owner_id = ? OR create_by = ? OR change_by = ?`), id, id, id) //nolint:errcheck // cleanup
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM users WHERE id = ?`), id)                                                           //nolint:errcheck // cleanup
	})
	return int(id)
}

func TestCannedResponsesAPI_ReadsPersistedRows(t *testing.T) {
	db := getTestDB(t)
	sfx := strconv.FormatInt(time.Now().UnixNano(), 36)
	other := insertCannedTestAgent(t, db, "cr-other-"+sfx)

	mine := "CR mine " + sfx
	global := "CR global " + sfx
	foreign := "CR foreign " + sfx
	popular := "CR popular " + sfx
	mineID := insertCannedResponse(t, db, mine, "Billing", "personal", 1, 1)
	insertCannedResponse(t, db, global, "Technical", "global", 1, 0)
	foreignID := insertCannedResponse(t, db, foreign, "Billing", "personal", other, 0)
	insertCannedResponse(t, db, popular, "Technical", "global", 1, 1000000)

	r := cannedRoutesEngine(t)
	tok := cannedToken(t, 1, "Agent", false)

	status, body := cannedRequest(t, r, http.MethodGet, "/api/canned-responses?search="+sfx, tok, nil)
	require.Equal(t, http.StatusOK, status, "%v", body)
	names := cannedNames(t, body)
	assert.ElementsMatch(t, []string{mine, global, popular}, names, "own personal + global rows, never another agent's personal row")

	status, body = cannedRequest(t, r, http.MethodGet, "/api/canned-responses/category/Billing?search="+sfx, tok, nil)
	require.Equal(t, http.StatusOK, status, "%v", body)
	assert.Equal(t, []string{mine}, cannedNames(t, body))

	status, body = cannedRequest(t, r, http.MethodGet, "/api/canned-responses/search?q="+sfx, tok, nil)
	require.Equal(t, http.StatusOK, status, "%v", body)
	assert.ElementsMatch(t, []string{mine, global, popular}, cannedNames(t, body))

	status, body = cannedRequest(t, r, http.MethodGet, "/api/canned-responses/popular?limit=1", tok, nil)
	require.Equal(t, http.StatusOK, status, "%v", body)
	assert.Equal(t, []string{popular}, cannedNames(t, body), "popular sorts by usage_count desc and honours limit")

	status, body = cannedRequest(t, r, http.MethodGet, fmt.Sprintf("/api/canned-responses/%d", mineID), tok, nil)
	require.Equal(t, http.StatusOK, status, "%v", body)
	assert.Equal(t, mine, body["response"].(map[string]any)["name"])

	status, _ = cannedRequest(t, r, http.MethodGet, fmt.Sprintf("/api/canned-responses/%d", foreignID), tok, nil)
	assert.Equal(t, http.StatusForbidden, status, "another agent's personal response is not readable by id")

	status, body = cannedRequest(t, r, http.MethodGet, "/api/canned-responses/categories", tok, nil)
	require.Equal(t, http.StatusOK, status, "%v", body)
	assert.Contains(t, body["categories"], "Billing", "categories come from canned_response_category")
}

func TestCannedResponsesAPI_WritesPersist(t *testing.T) {
	db := getTestDB(t)
	sfx := strconv.FormatInt(time.Now().UnixNano(), 36)
	agentID := insertCannedTestAgent(t, db, "cr-agent-"+sfx)
	r := cannedRoutesEngine(t)
	agentTok := cannedToken(t, uint(agentID), "Agent", false)
	adminTok := cannedToken(t, 1, "Admin", true)

	name := "CR created " + sfx
	status, body := cannedRequest(t, r, http.MethodPost, "/api/canned-responses", agentTok, map[string]any{
		"name": name, "category": "General", "content": "Hello {{customer_name}}",
	})
	require.Equal(t, http.StatusCreated, status, "%v", body)
	id := int(body["id"].(float64))

	var owner int
	var scope, content string
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		`SELECT owner_id, scope, content FROM canned_response WHERE id = ?`), id).Scan(&owner, &scope, &content))
	assert.Equal(t, agentID, owner)
	assert.Equal(t, "personal", scope)
	assert.Equal(t, "Hello {{customer_name}}", content)

	status, body = cannedRequest(t, r, http.MethodPut, fmt.Sprintf("/api/canned-responses/%d", id), agentTok, map[string]any{"content": "Updated"})
	require.Equal(t, http.StatusOK, status, "%v", body)
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT content FROM canned_response WHERE id = ?`), id).Scan(&content))
	assert.Equal(t, "Updated", content)

	status, body = cannedRequest(t, r, http.MethodPost, fmt.Sprintf("/api/canned-responses/%d/use", id), agentTok, map[string]any{})
	require.Equal(t, http.StatusOK, status, "%v", body)
	var usage int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT usage_count FROM canned_response WHERE id = ?`), id).Scan(&usage))
	assert.Equal(t, 1, usage)

	status, _ = cannedRequest(t, r, http.MethodPost, "/api/canned-responses", agentTok, map[string]any{
		"name": "CR global by agent " + sfx, "content": "x", "scope": "global",
	})
	assert.Equal(t, http.StatusForbidden, status, "non-admin agents cannot create global responses")

	globalName := "CR global by admin " + sfx
	status, body = cannedRequest(t, r, http.MethodPost, "/api/canned-responses", adminTok, map[string]any{
		"name": globalName, "content": "x", "scope": "global",
	})
	require.Equal(t, http.StatusCreated, status, "%v", body)
	globalID := int(body["id"].(float64))
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM canned_response WHERE id = ?`), globalID) //nolint:errcheck // cleanup
	})

	status, _ = cannedRequest(t, r, http.MethodDelete, fmt.Sprintf("/api/canned-responses/%d", globalID), agentTok, nil)
	assert.Equal(t, http.StatusForbidden, status, "agents cannot delete global responses")

	status, body = cannedRequest(t, r, http.MethodDelete, fmt.Sprintf("/api/canned-responses/%d", id), agentTok, nil)
	require.Equal(t, http.StatusOK, status, "%v", body)
	var validID int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT valid_id FROM canned_response WHERE id = ?`), id).Scan(&validID))
	assert.Equal(t, 2, validID, "delete invalidates the row")

	status, _ = cannedRequest(t, r, http.MethodGet, fmt.Sprintf("/api/canned-responses/%d", id), agentTok, nil)
	assert.Equal(t, http.StatusNotFound, status, "deleted responses are gone")
}

func TestCannedResponsesAPI_RejectsCustomers(t *testing.T) {
	r := cannedRoutesEngine(t)
	status, _ := cannedRequest(t, r, http.MethodGet, "/api/canned-responses", cannedToken(t, 1, "Customer", false), nil)
	assert.Equal(t, http.StatusForbidden, status)
}
