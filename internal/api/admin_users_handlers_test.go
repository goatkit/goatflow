package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// adminUsersJSON serves one request on handler and decodes the JSON body.
func adminUsersJSON(t *testing.T, method, route, path string, handler gin.HandlerFunc, contentType, body string) (int, map[string]interface{}) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := seedAdminRouter()
	router.Handle(method, route, handler)
	req, _ := http.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response), w.Body.String())
	return w.Code, response
}

func TestHandleAdminUserGet(t *testing.T) {
	get := func(t *testing.T, id string) (int, map[string]interface{}) {
		return adminUsersJSON(t, http.MethodGet, "/admin/users/:id", "/admin/users/"+id, HandleAdminUserGet, "", "")
	}

	t.Run("invalid_user_ID", func(t *testing.T) {
		code, response := get(t, "invalid")
		assert.Equal(t, http.StatusBadRequest, code)
		assert.Equal(t, false, response["success"])
		assert.Equal(t, "Invalid user ID", response["error"])
	})

	// Regression: users.title is nullable (the seeded root agent has NULL) and
	// was scanned into a string, so such agents answered 404 "User not found".
	t.Run("agent_with_NULL_title", func(t *testing.T) {
		id, login := createIsolatedAgent(t, "nulltitle")
		db, err := database.GetDB()
		require.NoError(t, err)
		_, err = db.Exec(database.ConvertPlaceholders("UPDATE users SET title = NULL WHERE id = ?"), id)
		require.NoError(t, err)

		code, response := get(t, strconv.Itoa(id))
		require.Equal(t, http.StatusOK, code, response)
		data := response["data"].(map[string]interface{})
		assert.Equal(t, float64(id), data["id"])
		assert.Equal(t, login, data["login"])
		assert.Equal(t, "", data["title"])
		assert.Equal(t, "Isolated", data["first_name"])
		assert.Equal(t, "Agent", data["last_name"])
		assert.Equal(t, float64(1), data["valid_id"])
		assert.Equal(t, []interface{}{}, data["groups"])
	})

	t.Run("unknown_user_is_404", func(t *testing.T) {
		code, response := get(t, "987654321")
		assert.Equal(t, http.StatusNotFound, code)
		assert.Equal(t, false, response["success"])
		assert.Equal(t, "User not found", response["error"])
	})
}

// The users page renders the real agent list (it used to answer a bare
// "<h1>Users</h1>" under APP_ENV=test), including agents with a NULL title.
func TestHandleAdminUsersPage(t *testing.T) {
	SetupTestTemplateRenderer(t)
	id, login := createIsolatedAgent(t, "page")
	db, err := database.GetDB()
	require.NoError(t, err)
	_, err = db.Exec(database.ConvertPlaceholders("UPDATE users SET title = NULL, last_name = 'Pagelisted' WHERE id = ?"), id)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := seedAdminRouter()
	router.GET("/admin/users", HandleAdminUsers)
	req, _ := http.NewRequest(http.MethodGet, "/admin/users", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), login)
	assert.Contains(t, w.Body.String(), "Pagelisted")
}

func TestHandleAdminUserCreate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("missing_login", func(t *testing.T) {
		router := seedAdminRouter()
		router.POST("/admin/users", HandleAdminUserCreate)

		formData := url.Values{
			"first_name": {"Test"},
			"last_name":  {"User"},
			"valid_id":   {"1"},
		}

		req, _ := http.NewRequest("POST", "/admin/users",
			strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response["success"].(bool))
		assert.Equal(t, "Login, first name, and last name are required", response["error"].(string))
	})

	t.Run("missing_first_name", func(t *testing.T) {
		router := seedAdminRouter()
		router.POST("/admin/users", HandleAdminUserCreate)

		formData := url.Values{
			"login":     {"test@example.com"},
			"last_name": {"User"},
			"valid_id":  {"1"},
		}

		req, _ := http.NewRequest("POST", "/admin/users",
			strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response["success"].(bool))
		assert.Equal(t, "Login, first name, and last name are required", response["error"].(string))
	})

	t.Run("missing_last_name", func(t *testing.T) {
		router := seedAdminRouter()
		router.POST("/admin/users", HandleAdminUserCreate)

		formData := url.Values{
			"login":      {"test@example.com"},
			"first_name": {"Test"},
			"valid_id":   {"1"},
		}

		req, _ := http.NewRequest("POST", "/admin/users",
			strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response["success"].(bool))
		assert.Equal(t, "Login, first name, and last name are required", response["error"].(string))
	})

	t.Run("all_required_fields_present_returns_200", func(t *testing.T) {
		router := seedAdminRouter()
		router.POST("/admin/users", HandleAdminUserCreate)

		login := "newuser_unique_" + strconv.FormatInt(time.Now().UnixNano(), 10) + "@example.com"
		cleanupAgentByLogin(t, login)
		formData := url.Values{
			"login":      {login},
			"first_name": {"Test"},
			"last_name":  {"User"},
			"valid_id":   {"1"},
		}

		req, _ := http.NewRequest("POST", "/admin/users",
			strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		id := int(response["user_id"].(float64))
		gotLogin, gotFirst, _, validID := adminTestUserRow(t, id)
		assert.Equal(t, login, gotLogin)
		assert.Equal(t, "Test", gotFirst)
		assert.Equal(t, 1, validID)
	})
}

// setAgentPasswordPolicy configures PreferencesGroups###Password settings
// (name suffix -> value) for the duration of the test.
func setAgentPasswordPolicy(t *testing.T, settings map[string]string) {
	t.Helper()
	db, err := database.GetDB()
	require.NoError(t, err)
	for suffix, value := range settings {
		name := "PreferencesGroups###Password::" + suffix
		_, err := db.Exec(database.ConvertPlaceholders(`
			INSERT INTO sysconfig_default (
				name, description, navigation, is_invisible, is_readonly, is_required,
				is_valid, has_configlevel, user_modification_possible, user_modification_active,
				xml_content_raw, xml_content_parsed, xml_filename, effective_value,
				is_dirty, exclusive_lock_guid, create_time, create_by, change_time, change_by
			) VALUES (?, 'test agent password policy', 'Core::Auth', 0, 0, 0,
				1, 0, 0, 0, '', '', 'Test.xml', ?, 0, '', NOW(), 1, NOW(), 1)`), name, value)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM sysconfig_default WHERE name = ?"), name)
		})
	}
}

// Admin-set agent passwords follow the agent password policy (the one agents'
// own password change enforces) and the confirmation field.
func TestAdminSetPasswordPolicy(t *testing.T) {
	setAgentPasswordPolicy(t, map[string]string{
		"PasswordMinSize":   "10",
		"PasswordNeedDigit": "1",
	})
	const form = "application/x-www-form-urlencoded"

	t.Run("create_rejects_short_password_and_creates_nothing", func(t *testing.T) {
		login := "pwpol_create_" + strconv.FormatInt(time.Now().UnixNano(), 10)
		cleanupAgentByLogin(t, login)
		code, response := adminUsersJSON(t, http.MethodPost, "/admin/users", "/admin/users", HandleAdminUserCreate, form,
			url.Values{"login": {login}, "first_name": {"P"}, "last_name": {"P"}, "valid_id": {"1"},
				"password": {"Short1"}, "confirm_password": {"Short1"}}.Encode())
		assert.Equal(t, http.StatusBadRequest, code)
		assert.Equal(t, "min_size", response["code"])
		assert.Equal(t, "Minimum 10 characters", response["error"])
		db, err := database.GetDB()
		require.NoError(t, err)
		var n int
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders("SELECT COUNT(*) FROM users WHERE login = ?"), login).Scan(&n))
		assert.Equal(t, 0, n)
	})

	t.Run("create_rejects_mismatched_confirmation", func(t *testing.T) {
		login := "pwpol_confirm_" + strconv.FormatInt(time.Now().UnixNano(), 10)
		cleanupAgentByLogin(t, login)
		code, response := adminUsersJSON(t, http.MethodPost, "/admin/users", "/admin/users", HandleAdminUserCreate, form,
			url.Values{"login": {login}, "first_name": {"P"}, "last_name": {"P"}, "valid_id": {"1"},
				"password": {"Longenough123"}, "confirm_password": {"Longenough124"}}.Encode())
		assert.Equal(t, http.StatusBadRequest, code)
		assert.Equal(t, "Passwords do not match", response["error"])
	})

	t.Run("create_accepts_compliant_password", func(t *testing.T) {
		login := "pwpol_ok_" + strconv.FormatInt(time.Now().UnixNano(), 10)
		cleanupAgentByLogin(t, login)
		code, response := adminUsersJSON(t, http.MethodPost, "/admin/users", "/admin/users", HandleAdminUserCreate, form,
			url.Values{"login": {login}, "first_name": {"P"}, "last_name": {"P"}, "valid_id": {"1"},
				"password": {"Longenough123"}, "confirm_password": {"Longenough123"}}.Encode())
		require.Equal(t, http.StatusOK, code, response)
		_, _, pw, _ := adminTestUserRow(t, int(response["user_id"].(float64)))
		assert.NotEqual(t, "", pw)
	})

	t.Run("update_rejects_password_without_digit_and_keeps_hash", func(t *testing.T) {
		id, login := createIsolatedAgent(t, "pwpol_upd")
		code, response := adminUsersJSON(t, http.MethodPut, "/admin/users/:id", "/admin/users/"+strconv.Itoa(id), HandleAdminUserUpdate, form,
			url.Values{"login": {login}, "first_name": {"P"}, "last_name": {"P"}, "valid_id": {"1"},
				"password": {"nodigitsatall"}}.Encode())
		assert.Equal(t, http.StatusBadRequest, code)
		assert.Equal(t, "need_digit", response["code"])
		assert.Equal(t, "At least 1 number", response["error"])
		_, first, pw, _ := adminTestUserRow(t, id)
		assert.Equal(t, "x", pw, "rejected update must not change the hash")
		assert.Equal(t, "Isolated", first, "rejected update must not change other fields")
	})

	t.Run("reset_rejects_short_password", func(t *testing.T) {
		id, _ := createIsolatedAgent(t, "pwpol_reset")
		code, response := adminUsersJSON(t, http.MethodPost, "/admin/users/:id/reset-password", "/admin/users/"+strconv.Itoa(id)+"/reset-password",
			HandleAdminUserResetPassword, "application/json", `{"password":"abc1"}`)
		assert.Equal(t, http.StatusBadRequest, code)
		assert.Equal(t, "min_size", response["code"])
		_, _, pw, _ := adminTestUserRow(t, id)
		assert.Equal(t, "x", pw)
	})

	t.Run("reset_generated_password_satisfies_policy", func(t *testing.T) {
		id, _ := createIsolatedAgent(t, "pwpol_gen")
		code, response := adminUsersJSON(t, http.MethodPost, "/admin/users/:id/reset-password", "/admin/users/"+strconv.Itoa(id)+"/reset-password",
			HandleAdminUserResetPassword, "application/json", `{}`)
		require.Equal(t, http.StatusOK, code, response)
		generated := response["generatedPassword"].(string)
		assert.GreaterOrEqual(t, len(generated), 10)
		assert.Regexp(t, `[0-9]`, generated)
		_, _, pw, _ := adminTestUserRow(t, id)
		assert.NotEqual(t, "x", pw)
	})

	t.Run("reset_unknown_user_is_404", func(t *testing.T) {
		code, _ := adminUsersJSON(t, http.MethodPost, "/admin/users/:id/reset-password", "/admin/users/987654321/reset-password",
			HandleAdminUserResetPassword, "application/json", `{"password":"Longenough123"}`)
		assert.Equal(t, http.StatusNotFound, code)
	})

	t.Run("policy_endpoint_reports_configured_policy", func(t *testing.T) {
		code, response := adminUsersJSON(t, http.MethodGet, "/admin/password-policy", "/admin/password-policy", HandlePasswordPolicy, "", "")
		require.Equal(t, http.StatusOK, code)
		policy := response["policy"].(map[string]interface{})
		assert.Equal(t, float64(10), policy["password_min_size"])
		assert.Equal(t, true, policy["password_need_digit"])
		assert.Equal(t, false, policy["password_min_2_lower_2_upper_characters"])
	})
}

// TestHandleAdminUserCreate_PersistsSelectedGroups is a regression test: the
// create path's group insert used 4 placeholders but passed only 2 args, so the
// INSERT silently failed (driver returned "Incorrect arguments to EXECUTE") and
// selected groups were never persisted (the error was swallowed by errcheck).
func TestHandleAdminUserCreate_PersistsSelectedGroups(t *testing.T) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skip("test database not available")
	}

	// Find a real active group to reference by name.
	groupName := "admin"
	var groupID int
	err = db.QueryRow(database.ConvertPlaceholders(
		"SELECT id FROM `groups` WHERE name = ? AND valid_id = 1 LIMIT 1"), groupName).Scan(&groupID)
	if err != nil {
		t.Skipf("group %q not found; skipping", groupName)
	}

	login := "grp_created_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	router := seedAdminRouter()
	router.POST("/admin/users", HandleAdminUserCreate)

	formData := url.Values{
		"login":      {login},
		"first_name": {"Group"},
		"last_name":  {"Persist"},
		"valid_id":   {"1"},
		"password":   {"Passw0rd!string"},
		"groups":     {groupName},
	}
	req, _ := http.NewRequest("POST", "/admin/users",
		strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(
			"DELETE FROM group_user WHERE user_id = (SELECT id FROM users WHERE login = ?)"), login)
		_, _ = db.Exec(database.ConvertPlaceholders(
			"DELETE FROM users WHERE login = ?"), login)
	})

	var uid int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		"SELECT id FROM users WHERE login = ?"), login).Scan(&uid))

	var n int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		"SELECT COUNT(*) FROM group_user WHERE user_id = ? AND group_id = ? AND permission_key = 'rw'"),
		uid, groupID).Scan(&n))
	assert.Equal(t, 1, n, "selected group should be persisted in group_user")
}

// TestHandleAdminUserGet_DedupesMultiPermissionGroups is a regression test for
// the SELECT DISTINCT fix: a user granted one group under several permission
// keys (e.g. rw + ro + owner rows) used to surface that group once per
// permission row in user.Groups. The group must now appear exactly once.
func TestHandleAdminUserGet_DedupesMultiPermissionGroups(t *testing.T) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skip("test database not available")
	}

	// Find a real active group to reference by name.
	groupName := "admin"
	var groupID int
	err = db.QueryRow(database.ConvertPlaceholders(
		"SELECT id FROM `groups` WHERE name = ? AND valid_id = 1 LIMIT 1"), groupName).Scan(&groupID)
	if err != nil {
		t.Skipf("group %q not found; skipping", groupName)
	}

	// Create the user {and} one rw membership through the real handler, then add
	// further permission rows for the same group directly (as a user who holds
	// several permission keys would have).
	login := "grp_distinct_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	router := seedAdminRouter()
	router.POST("/admin/users", HandleAdminUserCreate)

	formData := url.Values{
		"login":      {login},
		"first_name": {"Distinct"},
		"last_name":  {"Groups"},
		"valid_id":   {"1"},
		"password":   {"Passw0rd!string"},
		"groups":     {groupName},
	}
	req, _ := http.NewRequest("POST", "/admin/users",
		strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var uid int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		"SELECT id FROM users WHERE login = ?"), login).Scan(&uid))

	// Additional permission keys for the SAME (user, group) membership.
	for _, perm := range []string{"ro", "owner", "move_into"} {
		_, err = db.Exec(database.ConvertPlaceholders(
			"INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by) VALUES (?, ?, ?, NOW(), ?, NOW(), ?)"),
			uid, groupID, perm, uid, uid)
		require.NoError(t, err)
	}

	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(
			"DELETE FROM group_user WHERE user_id = ?"), uid)
		_, _ = db.Exec(database.ConvertPlaceholders(
			"DELETE FROM users WHERE id = ?"), uid)
	})

	// GET the user and confirm the group is reported exactly once.
	gin.SetMode(gin.TestMode)
	router = seedAdminRouter()
	router.GET("/admin/users/:id", HandleAdminUserGet)
	req, _ = http.NewRequest("GET", "/admin/users/"+strconv.Itoa(uid), nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.True(t, response["success"].(bool))

	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok)
	groups, ok := data["groups"].([]interface{})
	require.True(t, ok, "groups should be a list")

	count := 0
	for _, g := range groups {
		if g == groupName {
			count++
		}
	}
	assert.Equal(t, 1, count,
		"group should appear exactly once despite multiple permission rows")
}

// adminTestUserRow reads the columns HandleAdminUserUpdate writes.
func adminTestUserRow(t *testing.T, id int) (login, firstName, pw string, validID int) {
	t.Helper()
	db, err := database.GetDB()
	require.NoError(t, err)
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		"SELECT login, first_name, pw, valid_id FROM users WHERE id = ?"), id).Scan(&login, &firstName, &pw, &validID))
	return login, firstName, pw, validID
}

// adminTestUserGroups returns the names of the groups an agent holds rw on.
func adminTestUserGroups(t *testing.T, id int) []string {
	t.Helper()
	db, err := database.GetDB()
	require.NoError(t, err)
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT g.name FROM group_user gu JOIN `+"`groups`"+` g ON g.id = gu.group_id
		WHERE gu.user_id = ? AND gu.permission_key = 'rw' ORDER BY g.name`), id)
	require.NoError(t, err)
	defer rows.Close()
	names := []string{}
	for rows.Next() {
		var n string
		require.NoError(t, rows.Scan(&n))
		names = append(names, n)
	}
	require.NoError(t, rows.Err())
	return names
}

// TestAdminUserDuplicateLogin: a login that belongs to another agent is
// rejected with 409 and a JSON error the users page can show, on create and on
// update, without touching the stored rows.
func TestAdminUserDuplicateLogin(t *testing.T) {
	existingID, existingLogin := createIsolatedAgent(t, "dupa")
	otherID, otherLogin := createIsolatedAgent(t, "dupb")
	form := func(login string) string {
		return url.Values{"login": {login}, "first_name": {"Dup"}, "last_name": {"User"}, "valid_id": {"1"}}.Encode()
	}

	t.Run("create", func(t *testing.T) {
		status, body := adminUsersJSON(t, http.MethodPost, "/admin/users", "/admin/users",
			HandleAdminUserCreate, "application/x-www-form-urlencoded", form(existingLogin))
		assert.Equal(t, http.StatusConflict, status)
		assert.Equal(t, false, body["success"])
		assert.Equal(t, "User already exists", body["error"])
		_, first, _, _ := adminTestUserRow(t, existingID)
		assert.NotEqual(t, "Dup", first, "the existing agent must not change")
	})

	t.Run("update", func(t *testing.T) {
		status, body := adminUsersJSON(t, http.MethodPut, "/admin/users/:id", "/admin/users/"+strconv.Itoa(otherID),
			HandleAdminUserUpdate, "application/x-www-form-urlencoded", form(existingLogin))
		assert.Equal(t, http.StatusConflict, status)
		assert.Equal(t, false, body["success"])
		assert.Equal(t, "User already exists", body["error"])
		login, first, _, _ := adminTestUserRow(t, otherID)
		assert.Equal(t, otherLogin, login, "the login must not change")
		assert.NotEqual(t, "Dup", first)
	})

	t.Run("update keeping own login", func(t *testing.T) {
		status, body := adminUsersJSON(t, http.MethodPut, "/admin/users/:id", "/admin/users/"+strconv.Itoa(otherID),
			HandleAdminUserUpdate, "application/x-www-form-urlencoded", form(otherLogin))
		assert.Equal(t, http.StatusOK, status, "%v", body)
		_, first, _, _ := adminTestUserRow(t, otherID)
		assert.Equal(t, "Dup", first)
	})
}

func TestHandleAdminUserUpdate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	put := func(t *testing.T, userID string, form url.Values) (*httptest.ResponseRecorder, map[string]interface{}) {
		t.Helper()
		router := seedAdminRouter()
		router.PUT("/admin/users/:id", HandleAdminUserUpdate)
		req, _ := http.NewRequest("PUT", "/admin/users/"+userID, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		return w, response
	}

	t.Run("invalid_user_ID", func(t *testing.T) {
		w, response := put(t, "invalid", url.Values{
			"login": {"test@example.com"}, "first_name": {"Test"}, "last_name": {"User"}, "valid_id": {"1"},
		})
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.False(t, response["success"].(bool))
		assert.Equal(t, "Invalid user ID", response["error"].(string))
	})

	t.Run("successful_update", func(t *testing.T) {
		id, login := createIsolatedAgent(t, "upd")
		newLogin := "updated_" + login
		w, response := put(t, strconv.Itoa(id), url.Values{
			"login": {newLogin}, "first_name": {"Updated"}, "last_name": {"User"}, "valid_id": {"1"},
		})
		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, response["success"].(bool))
		gotLogin, gotFirst, _, _ := adminTestUserRow(t, id)
		assert.Equal(t, newLogin, gotLogin)
		assert.Equal(t, "Updated", gotFirst)
	})

	t.Run("update_with_password", func(t *testing.T) {
		id, login := createIsolatedAgent(t, "updpw")
		_, _, oldPW, _ := adminTestUserRow(t, id)
		w, response := put(t, strconv.Itoa(id), url.Values{
			"login": {login}, "first_name": {"Test"}, "last_name": {"User"},
			"password": {"Newpassword123"}, "valid_id": {"1"},
		})
		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, response["success"].(bool))
		_, _, newPW, _ := adminTestUserRow(t, id)
		assert.NotEqual(t, oldPW, newPW, "password hash should be replaced")
	})

	t.Run("update_with_groups", func(t *testing.T) {
		id, login := createIsolatedAgent(t, "updgrp")
		w, response := put(t, strconv.Itoa(id), url.Values{
			"login": {login}, "first_name": {"Test"}, "last_name": {"User"}, "valid_id": {"1"},
			"groups": {"admin", "users"},
		})
		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, response["success"].(bool))
		assert.Equal(t, []string{"admin", "users"}, adminTestUserGroups(t, id))
	})

	// Regression: under APP_ENV=test an unknown id used to be inserted as a
	// new agent and the update reported success.
	t.Run("unknown_user_is_404_and_not_created", func(t *testing.T) {
		login := "ghost_" + strconv.FormatInt(time.Now().UnixNano(), 10)
		cleanupAgentByLogin(t, login)
		w, response := put(t, "987654321", url.Values{
			"login": {login}, "first_name": {"Ghost"}, "last_name": {"User"}, "valid_id": {"1"},
		})
		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Equal(t, "User not found", response["error"])
		db, err := database.GetDB()
		require.NoError(t, err)
		var n int
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
			"SELECT COUNT(*) FROM users WHERE id = 987654321 OR login = ?"), login).Scan(&n))
		assert.Equal(t, 0, n)
	})

	t.Run("unknown_group_is_400_and_nothing_changes", func(t *testing.T) {
		id, login := createIsolatedAgent(t, "updbadgrp")
		w, response := put(t, strconv.Itoa(id), url.Values{
			"login": {login}, "first_name": {"Changed"}, "last_name": {"User"}, "valid_id": {"1"},
			"groups": {"admin", "no_such_group_xyz"},
		})
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, response["error"], "no_such_group_xyz")
		_, first, _, _ := adminTestUserRow(t, id)
		assert.Equal(t, "Isolated", first)
		assert.Empty(t, adminTestUserGroups(t, id))
	})
}

func TestHandleAdminUserSoftDeleteAndStatusUnknown(t *testing.T) {
	t.Run("delete_sets_valid_id_2", func(t *testing.T) {
		id, _ := createIsolatedAgent(t, "softdel")
		code, response := adminUsersJSON(t, http.MethodDelete, "/admin/users/:id", "/admin/users/"+strconv.Itoa(id), HandleAdminUserDelete, "", "")
		require.Equal(t, http.StatusOK, code, response)
		_, _, _, validID := adminTestUserRow(t, id)
		assert.Equal(t, 2, validID)
	})
	t.Run("delete_unknown_user_is_404", func(t *testing.T) {
		code, _ := adminUsersJSON(t, http.MethodDelete, "/admin/users/:id", "/admin/users/987654321", HandleAdminUserDelete, "", "")
		assert.Equal(t, http.StatusNotFound, code)
	})
	t.Run("status_unknown_user_is_404", func(t *testing.T) {
		code, _ := adminUsersJSON(t, http.MethodPut, "/admin/users/:id/status", "/admin/users/987654321/status",
			HandleAdminUsersStatus, "application/json", `{"valid_id": 2}`)
		assert.Equal(t, http.StatusNotFound, code)
	})
}

func TestHandleAdminUserDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("invalid_user_ID", func(t *testing.T) {
		router := seedAdminRouter()
		router.DELETE("/admin/users/:id", HandleAdminUserDelete)

		req, _ := http.NewRequest("DELETE", "/admin/users/invalid", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response["success"].(bool))
		assert.Equal(t, "Invalid user ID", response["error"].(string))
	})
}

func TestHandleAdminUserGroups(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("invalid_user_ID", func(t *testing.T) {
		router := seedAdminRouter()
		router.GET("/admin/users/:id/groups", HandleAdminUserGroups)

		req, _ := http.NewRequest("GET", "/admin/users/invalid/groups", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response["success"].(bool))
		assert.Equal(t, "Invalid user ID", response["error"].(string))
	})
}

func TestHandleAdminUsersStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		userID         string
		formData       url.Values
		expectedStatus int
		expectSuccess  bool
		expectedError  string
	}{
		{
			name:           "invalid_user_ID",
			userID:         "invalid",
			formData:       url.Values{"valid_id": {"1"}},
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
			expectedError:  "Invalid user ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := seedAdminRouter()
			router.PUT("/admin/users/:id/status", HandleAdminUsersStatus)

			req, _ := http.NewRequest("PUT", "/admin/users/"+tt.userID+"/status",
				strings.NewReader(tt.formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.expectedError != "" {
				assert.False(t, response["success"].(bool))
				assert.Equal(t, tt.expectedError, response["error"].(string))
			}
		})
	}
}

func TestHandleAdminUserCreateJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("create_user_with_JSON_returns_200", func(t *testing.T) {
		router := seedAdminRouter()
		router.POST("/admin/users", HandleAdminUserCreate)

		login := "jsonuser_" + strconv.FormatInt(time.Now().UnixNano(), 10) + "@example.com"
		cleanupAgentByLogin(t, login)
		jsonBody := `{
			"login": "` + login + `",
			"first_name": "JSON",
			"last_name": "User",
			"valid_id": 1,
			"groups": ["admin"]
		}`

		req, _ := http.NewRequest("POST", "/admin/users",
			strings.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		assert.Equal(t, []string{"admin"}, adminTestUserGroups(t, int(response["user_id"].(float64))))
	})
}

func TestHandleAdminUserUpdateJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("update_user_with_JSON", func(t *testing.T) {
		router := seedAdminRouter()
		router.PUT("/admin/users/:id", HandleAdminUserUpdate)

		id, login := createIsolatedAgent(t, "updjson")
		newLogin := "updated_" + login
		jsonBody := `{
			"login": "` + newLogin + `",
			"first_name": "Updated",
			"last_name": "User",
			"valid_id": 1,
			"groups": ["admin", "users"]
		}`

		req, _ := http.NewRequest("PUT", "/admin/users/"+strconv.Itoa(id),
			strings.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		gotLogin, _, _, _ := adminTestUserRow(t, id)
		assert.Equal(t, newLogin, gotLogin)
		assert.Equal(t, []string{"admin", "users"}, adminTestUserGroups(t, id))
	})

	t.Run("update_user_with_empty_groups_clears_memberships", func(t *testing.T) {
		router := seedAdminRouter()
		router.PUT("/admin/users/:id", HandleAdminUserUpdate)

		id, login := createIsolatedAgent(t, "updclear")
		db, err := database.GetDB()
		require.NoError(t, err)
		_, err = db.Exec(database.ConvertPlaceholders(`
			INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
			VALUES (?, 1, 'rw', CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)`), id)
		require.NoError(t, err)

		// Simulate form submission with groups_submitted flag but no groups
		formData := url.Values{
			"login":            {login},
			"first_name":       {"Test"},
			"last_name":        {"User"},
			"valid_id":         {"1"},
			"groups_submitted": {"1"},
		}

		req, _ := http.NewRequest("PUT", "/admin/users/"+strconv.Itoa(id),
			strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		assert.Empty(t, adminTestUserGroups(t, id))
	})
}

func TestHandleAdminUsersStatusJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("toggle_status_with_JSON", func(t *testing.T) {
		router := seedAdminRouter()
		router.PUT("/admin/users/:id/status", HandleAdminUsersStatus)

		id, _ := createIsolatedAgent(t, "status")
		jsonBody := `{"valid_id": 2}`

		req, _ := http.NewRequest("PUT", "/admin/users/"+strconv.Itoa(id)+"/status",
			strings.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		_, _, _, validID := adminTestUserRow(t, id)
		assert.Equal(t, 2, validID)
	})
}

func TestAdminUsersFormEncoding(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("handles_groups_array_bracket_notation_returns_200", func(t *testing.T) {
		router := seedAdminRouter()
		router.POST("/admin/users", HandleAdminUserCreate)

		// Some frontend frameworks use groups[] notation
		login := "test_bracket_" + strconv.FormatInt(time.Now().UnixNano(), 10) + "@example.com"
		cleanupAgentByLogin(t, login)
		formData := "login=" + login + "&first_name=Test&last_name=User&valid_id=1&groups[]=admin&groups[]=users"

		req, _ := http.NewRequest("POST", "/admin/users",
			strings.NewReader(formData))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		assert.Equal(t, []string{"admin", "users"}, adminTestUserGroups(t, int(response["user_id"].(float64))))
	})

	t.Run("handles_multiple_groups_values", func(t *testing.T) {
		login := "test_multi_" + strconv.FormatInt(time.Now().UnixNano(), 10) + "@example.com"
		cleanupAgentByLogin(t, login)
		formData := url.Values{"login": {login}, "first_name": {"Test"}, "last_name": {"User"}, "valid_id": {"1"}}
		formData.Add("groups", "admin")
		formData.Add("groups", "users")
		code, response := adminUsersJSON(t, http.MethodPost, "/admin/users", "/admin/users", HandleAdminUserCreate,
			"application/x-www-form-urlencoded", formData.Encode())
		require.Equal(t, http.StatusOK, code, response)
		assert.Equal(t, []string{"admin", "users"}, adminTestUserGroups(t, int(response["user_id"].(float64))))
	})

	// Unknown groups used to be created on the fly ("test-friendly").
	t.Run("unknown_group_is_400_and_creates_nothing", func(t *testing.T) {
		login := "test_badgrp_" + strconv.FormatInt(time.Now().UnixNano(), 10) + "@example.com"
		group := "no_such_group_" + strconv.FormatInt(time.Now().UnixNano(), 10)
		cleanupAgentByLogin(t, login)
		cleanupGroupByNameAtEnd(t, group)
		formData := url.Values{"login": {login}, "first_name": {"Test"}, "last_name": {"User"}, "valid_id": {"1"}}
		formData.Add("groups", "admin")
		formData.Add("groups", group)
		code, response := adminUsersJSON(t, http.MethodPost, "/admin/users", "/admin/users", HandleAdminUserCreate,
			"application/x-www-form-urlencoded", formData.Encode())
		assert.Equal(t, http.StatusBadRequest, code)
		assert.Contains(t, response["error"], group)
		db, err := database.GetDB()
		require.NoError(t, err)
		var users, groups int
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders("SELECT COUNT(*) FROM users WHERE login = ?"), login).Scan(&users))
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders("SELECT COUNT(*) FROM `groups` WHERE name = ?"), group).Scan(&groups))
		assert.Equal(t, 0, users)
		assert.Equal(t, 0, groups)
	})
}
