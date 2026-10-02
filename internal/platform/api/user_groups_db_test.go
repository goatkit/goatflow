package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
)

func userGroupsTestDB(t *testing.T) {
	t.Helper()
	if os.Getenv("GOATFLOW_TEST_DB_READY") == "" {
		t.Skip("integration test: needs the test database (GOATFLOW_TEST_DB_READY)")
	}
	require.NoError(t, database.InitTestDB())
}

func groupIDByName(t *testing.T, name string) int {
	t.Helper()
	db, err := database.GetDB()
	require.NoError(t, err)
	var id int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT id FROM groups WHERE name = ?`), name).Scan(&id))
	return id
}

func userAPIRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", uint(1)); c.Next() })
	r.POST("/api/v1/users", HandleCreateUserAPI)
	r.GET("/api/v1/users", HandleListUsersAPI)
	r.GET("/api/v1/users/:id", HandleGetUserAPI)
	return r
}

func deleteTestUser(t *testing.T, login string) {
	db, err := database.GetDB()
	require.NoError(t, err)
	var id int
	if err := db.QueryRow(database.ConvertPlaceholders(`SELECT id FROM users WHERE login = ?`), login).Scan(&id); err != nil {
		return
	}
	for _, q := range []string{
		`DELETE FROM group_user WHERE user_id = ?`,
		`DELETE FROM user_preferences WHERE user_id = ?`,
		`DELETE FROM users WHERE id = ?`,
	} {
		_, err := db.Exec(database.ConvertPlaceholders(q), id)
		require.NoError(t, err)
	}
}

func doJSON(t *testing.T, r *gin.Engine, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var out map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out), w.Body.String())
	return w.Code, out
}

// Creating an agent with groups writes group_user rows ('rw') and the
// UserEmail preference; GET and the group-filtered list read them back.
func TestUserAPIGroupMembershipRoundTrip(t *testing.T) {
	userGroupsTestDB(t)
	r := userAPIRouter()
	login := fmt.Sprintf("ugtest%d", time.Now().UnixNano())
	t.Cleanup(func() { deleteTestUser(t, login) })

	adminID := groupIDByName(t, "admin")
	usersID := groupIDByName(t, "users")

	code, body := doJSON(t, r, http.MethodPost, "/api/v1/users", map[string]any{
		"login": login, "email": login + "@example.test", "password": "SecurePass123!",
		"first_name": "Group", "last_name": "Tester", "groups": []int{usersID, adminID, usersID},
	})
	require.Equal(t, http.StatusCreated, code, body)
	newID := int(body["data"].(map[string]any)["id"].(float64))

	db, err := database.GetDB()
	require.NoError(t, err)
	var n int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		`SELECT COUNT(*) FROM group_user WHERE user_id = ? AND permission_key = 'rw'`), newID).Scan(&n))
	assert.Equal(t, 2, n, "duplicate group id must not create a second row")

	code, body = doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/v1/users/%d", newID), nil)
	require.Equal(t, http.StatusOK, code, body)
	data := body["data"].(map[string]any)
	assert.Equal(t, login+"@example.test", data["email"])
	assert.Equal(t, []any{
		map[string]any{"id": float64(adminID), "name": "admin", "permissions": []any{"rw"}},
		map[string]any{"id": float64(usersID), "name": "users", "permissions": []any{"rw"}},
	}, data["groups"])

	code, body = doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/v1/users?group_id=%d&search=%s", adminID, login), nil)
	require.Equal(t, http.StatusOK, code, body)
	list := body["data"].([]any)
	require.Len(t, list, 1)
	assert.Equal(t, float64(newID), list[0].(map[string]any)["id"])
	assert.Len(t, list[0].(map[string]any)["groups"], 2)
}

func TestUserAPICreateRejectsUnknownGroup(t *testing.T) {
	userGroupsTestDB(t)
	r := userAPIRouter()
	login := fmt.Sprintf("ugbad%d", time.Now().UnixNano())
	t.Cleanup(func() { deleteTestUser(t, login) })

	code, body := doJSON(t, r, http.MethodPost, "/api/v1/users", map[string]any{
		"login": login, "email": login + "@example.test", "password": "SecurePass123!",
		"groups": []int{999999},
	})
	assert.Equal(t, http.StatusBadRequest, code, body)

	db, err := database.GetDB()
	require.NoError(t, err)
	var n int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT COUNT(*) FROM users WHERE login = ?`), login).Scan(&n))
	assert.Zero(t, n, "no user may be created when a group is invalid")
}

func TestUserAPIListRejectsNonNumericGroup(t *testing.T) {
	userGroupsTestDB(t)
	code, body := doJSON(t, userAPIRouter(), http.MethodGet, "/api/v1/users?group_id=abc", nil)
	assert.Equal(t, http.StatusBadRequest, code, body)
}

// GET /users/:id must not leak credentials kept in user_preferences.
func TestUserAPIGetDoesNotExposeSecretPreferences(t *testing.T) {
	userGroupsTestDB(t)
	r := userAPIRouter()
	login := fmt.Sprintf("ugsec%d", time.Now().UnixNano())
	t.Cleanup(func() { deleteTestUser(t, login) })

	code, body := doJSON(t, r, http.MethodPost, "/api/v1/users", map[string]any{
		"login": login, "email": login + "@example.test", "password": "SecurePass123!",
	})
	require.Equal(t, http.StatusCreated, code, body)
	id := int(body["data"].(map[string]any)["id"].(float64))

	db, err := database.GetDB()
	require.NoError(t, err)
	secrets := map[string]string{
		"UserTOTPSecret":        "JBSWY3DPEHPK3PXPLEAKME",
		"UserTOTPRecoveryCodes": `["rc-leak-1","rc-leak-2"]`,
		"UserTOTPPendingSecret": "PENDINGLEAKSECRET",
		"Language":              "de",
	}
	for k, v := range secrets {
		_, err := db.Exec(database.ConvertPlaceholders(
			`INSERT INTO user_preferences (user_id, preferences_key, preferences_value) VALUES (?, ?, ?)`), id, k, []byte(v))
		require.NoError(t, err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/users/%d", id), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	raw := w.Body.String()
	for _, leak := range []string{"JBSWY3DPEHPK3PXPLEAKME", "rc-leak-1", "PENDINGLEAKSECRET", "UserTOTP"} {
		assert.NotContains(t, raw, leak)
	}

	var out map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	data := out["data"].(map[string]any)
	assert.Equal(t, map[string]any{"Language": "de", "UserEmail": login + "@example.test"}, data["preferences"])
	assert.Equal(t, login+"@example.test", data["email"])
}
