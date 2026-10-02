package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/routing"
	"github.com/goatkit/goatflow/internal/platform/shared"
)

func createUpdateTestUser(t *testing.T) (int, string) {
	t.Helper()
	login := fmt.Sprintf("upd%d", time.Now().UnixNano())
	t.Cleanup(func() { deleteTestUser(t, login) })
	code, body := doJSON(t, userAPIRouter(), http.MethodPost, "/api/v1/users", map[string]any{
		"login": login, "email": login + "@example.test", "password": "Create-Pass-1",
		"first_name": "First", "last_name": "Last",
	})
	require.Equal(t, http.StatusCreated, code, body)
	return int(body["data"].(map[string]any)["id"].(float64)), login
}

func userEmailPref(t *testing.T, userID int) string {
	t.Helper()
	db, err := database.GetDB()
	require.NoError(t, err)
	var v []byte
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		`SELECT preferences_value FROM user_preferences WHERE user_id = ? AND preferences_key = 'UserEmail'`), userID).Scan(&v))
	return string(v)
}

func userRow(t *testing.T, userID int) (first, last, pw string, validID int) {
	t.Helper()
	db, err := database.GetDB()
	require.NoError(t, err)
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		`SELECT first_name, last_name, pw, valid_id FROM users WHERE id = ?`), userID).Scan(&first, &last, &pw, &validID))
	return first, last, pw, validID
}

func updateRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", uint(1)); c.Next() })
	r.PUT("/api/v1/users/:id", HandleUpdateUserAPI)
	return r
}

// The agent's address is the UserEmail preference (users has no email column).
func TestUserUpdateAPIPersistsEmailAndNames(t *testing.T) {
	userGroupsTestDB(t)
	id, login := createUpdateTestUser(t)

	newEmail := "changed-" + login + "@example.test"
	code, body := doJSON(t, updateRouter(), http.MethodPut, fmt.Sprintf("/api/v1/users/%d", id), map[string]any{
		"email": newEmail, "first_name": "Renamed",
	})
	require.Equal(t, http.StatusOK, code, body)
	assert.Equal(t, newEmail, userEmailPref(t, id))
	first, last, _, _ := userRow(t, id)
	assert.Equal(t, "Renamed", first)
	assert.Equal(t, "Last", last)

	code, body = doJSON(t, updateRouter(), http.MethodPut, fmt.Sprintf("/api/v1/users/%d", id), map[string]any{"last_name": ""})
	assert.Equal(t, http.StatusBadRequest, code, body)
	_, last, _, _ = userRow(t, id)
	assert.Equal(t, "Last", last, "NOT NULL last_name is never blanked")
}

func TestUserUpdateAPIRejectsEmailOfAnotherAgent(t *testing.T) {
	userGroupsTestDB(t)
	id, login := createUpdateTestUser(t)
	_, otherLogin := createUpdateTestUser(t)

	code, body := doJSON(t, updateRouter(), http.MethodPut, fmt.Sprintf("/api/v1/users/%d", id), map[string]any{
		"email": otherLogin + "@example.test",
	})
	assert.Equal(t, http.StatusConflict, code, body)
	assert.Equal(t, login+"@example.test", userEmailPref(t, id))
}

func TestUserUpdateAPIRejectsLoginChange(t *testing.T) {
	userGroupsTestDB(t)
	id, login := createUpdateTestUser(t)

	code, body := doJSON(t, updateRouter(), http.MethodPut, fmt.Sprintf("/api/v1/users/%d", id), map[string]any{
		"login": "hijacked", "first_name": "Changed",
	})
	assert.Equal(t, http.StatusBadRequest, code, body)
	assert.Equal(t, "Login cannot be changed", body["error"])

	db, err := database.GetDB()
	require.NoError(t, err)
	var gotLogin string
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT login FROM users WHERE id = ?`), id).Scan(&gotLogin))
	assert.Equal(t, login, gotLogin)
	first, _, _, _ := userRow(t, id)
	assert.Equal(t, "First", first, "rejected request changes nothing")
}

// userRoutesEngine serves routes/api-v1-global.yaml with the production
// middleware set, so route-level guards are exercised as deployed.
func userRoutesEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	src, err := os.ReadFile("../../../routes/api-v1-global.yaml")
	require.NoError(t, err)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "api-v1-global.yaml"), src, 0o600))
	registry := routing.NewHandlerRegistry()
	registry.Override("HandleCreateUserAPI", HandleCreateUserAPI)
	registry.Override("HandleUpdateUserAPI", HandleUpdateUserAPI)
	registry.Override("HandleDeleteUserAPI", HandleDeleteUserAPI)
	routing.RegisterExistingHandlers(registry)
	r := gin.New()
	require.NoError(t, routing.LoadYAMLRoutes(r, dir, registry))
	return r
}

func bearerJSON(t *testing.T, r *gin.Engine, method, path, token string, body any) int {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

func TestUserMutationRoutesRequireAdmin(t *testing.T) {
	userGroupsTestDB(t)
	id, _ := createUpdateTestUser(t)
	r := userRoutesEngine(t)
	jwt := shared.GetJWTManager()
	token := func(userID uint, role string, admin bool) string {
		tok, err := jwt.GenerateTokenWithAdmin(userID, "authz@example.test", role, admin, 0)
		require.NoError(t, err)
		return tok
	}
	db, err := database.GetDB()
	require.NoError(t, err)

	_, _, pwBefore, _ := userRow(t, id)
	for name, tok := range map[string]string{
		"agent":    token(uint(id), "Agent", false),
		"customer": token(uint(id), "Customer", false),
	} {
		t.Run(name, func(t *testing.T) {
			path := fmt.Sprintf("/api/v1/users/%d", id)
			assert.Equal(t, http.StatusForbidden, bearerJSON(t, r, http.MethodPut, path, tok, map[string]any{"password": "Agent-Set-Pass-9"}))
			assert.Equal(t, http.StatusForbidden, bearerJSON(t, r, http.MethodDelete, path, tok, nil))
			login := fmt.Sprintf("authz%d", time.Now().UnixNano())
			t.Cleanup(func() { deleteTestUser(t, login) })
			assert.Equal(t, http.StatusForbidden, bearerJSON(t, r, http.MethodPost, "/api/v1/users", tok, map[string]any{
				"login": login, "email": login + "@example.test", "password": "Create-Pass-1",
			}))

			var n int
			require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT COUNT(*) FROM users WHERE login = ?`), login).Scan(&n))
			assert.Zero(t, n, "no agent created")
			_, _, pw, validID := userRow(t, id)
			assert.Equal(t, pwBefore, pw, "password unchanged")
			assert.Equal(t, 1, validID, "user not deleted")
		})
	}

	admin := token(1, "Admin", true)
	assert.Equal(t, http.StatusOK, bearerJSON(t, r, http.MethodPut, fmt.Sprintf("/api/v1/users/%d", id), admin, map[string]any{"password": "Admin-Set-Pass-9"}))
	_, _, pw, _ := userRow(t, id)
	assert.NotEqual(t, pwBefore, pw, "admin can reset the password")
	assert.Equal(t, http.StatusNoContent, bearerJSON(t, r, http.MethodDelete, fmt.Sprintf("/api/v1/users/%d", id), admin, nil))
	_, _, _, validID := userRow(t, id)
	assert.Equal(t, 2, validID)
}
