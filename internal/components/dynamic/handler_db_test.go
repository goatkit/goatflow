package dynamic

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/database"
)

// newModulesTestRouter loads the real modules/*.yaml against the test DB.
func newModulesTestRouter(t *testing.T) *gin.Engine {
	return newModulesTestRouterFor(t, "../../../modules")
}

func newModulesTestRouterFor(t *testing.T, modulesDir string) *gin.Engine {
	t.Helper()
	if os.Getenv("GOATFLOW_TEST_DB_READY") == "" {
		t.Skip("integration test: needs the test database (GOATFLOW_TEST_DB_READY)")
	}
	require.NoError(t, database.InitTestDB())
	db, err := database.GetDB()
	require.NoError(t, err)

	h, err := NewDynamicModuleHandler(db, nil, modulesDir)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", uint(1)); c.Next() })
	r.Any("/dynamic/:module", h.ServeModule)
	r.Any("/dynamic/:module/:id", h.ServeModule)
	return r
}

func serveDynamic(t *testing.T, r *gin.Engine, method, path string, form url.Values) (int, map[string]any) {
	t.Helper()
	var req *http.Request
	if form != nil {
		req = httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var out map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out), w.Body.String())
	return w.Code, out
}

func idByColumn(t *testing.T, table, column, value string) int {
	t.Helper()
	db, err := database.GetDB()
	require.NoError(t, err)
	var id int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		"SELECT id FROM "+table+" WHERE "+column+" = ?"), value).Scan(&id)) // sql-schema: table/column are test literals (users, groups, ticket_priority)
	return id
}

func membership(t *testing.T, userID int) []string {
	t.Helper()
	db, err := database.GetDB()
	require.NoError(t, err)
	rows, err := db.Query(database.ConvertPlaceholders(
		`SELECT g.name, gu.permission_key FROM group_user gu JOIN groups g ON g.id = gu.group_id WHERE gu.user_id = ?`), userID)
	require.NoError(t, err)
	defer rows.Close()
	got := []string{}
	for rows.Next() {
		var name, key string
		require.NoError(t, rows.Scan(&name, &key))
		got = append(got, name+":"+key)
	}
	require.NoError(t, rows.Err())
	sort.Strings(got)
	return got
}

// The users module stores membership in group_user: create with groups,
// read them back, replace them on update, filter the list by group, and reject
// unknown groups without touching existing memberships.
func TestDynamicUsersModuleGroupMembership(t *testing.T) {
	r := newModulesTestRouter(t)
	db, err := database.GetDB()
	require.NoError(t, err)
	login := fmt.Sprintf("dyn%d", time.Now().UnixNano())
	t.Cleanup(func() {
		var id int
		if db.QueryRow(database.ConvertPlaceholders(`SELECT id FROM users WHERE login = ?`), login).Scan(&id) == nil {
			_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM group_user WHERE user_id = ?`), id)
			_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM users WHERE id = ?`), id)
		}
	})
	adminID := idByColumn(t, "groups", "name", "admin")
	usersID := idByColumn(t, "groups", "name", "users")

	code, body := serveDynamic(t, r, http.MethodPost, "/dynamic/users", url.Values{
		"login": {login}, "pw": {"Secret123!"}, "first_name": {"Dyn"}, "last_name": {"Agent"},
		"groups": {fmt.Sprintf("%d,%d", usersID, usersID)},
	})
	require.Equal(t, http.StatusCreated, code, body)
	userID := idByColumn(t, "users", "login", login)
	assert.Equal(t, []string{"users:rw"}, membership(t, userID))

	// The password is stored as bcrypt, like every other agent password, and
	// verifies through the login hasher.
	var pw string
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT pw FROM users WHERE id = ?`), userID).Scan(&pw))
	assert.True(t, strings.HasPrefix(pw, "$2"), "expected a bcrypt hash, got %q", pw)
	assert.True(t, auth.NewPasswordHasher().VerifyPassword("Secret123!", pw))
	assert.False(t, auth.NewPasswordHasher().VerifyPassword("wrong", pw))

	code, body = serveDynamic(t, r, http.MethodGet, fmt.Sprintf("/dynamic/users/%d", userID), nil)
	require.Equal(t, http.StatusOK, code, body)
	assert.Equal(t, []any{"users"}, body["data"].(map[string]any)["Groups"])

	code, body = serveDynamic(t, r, http.MethodPut, fmt.Sprintf("/dynamic/users/%d", userID), url.Values{
		"login": {login}, "first_name": {"Dyn"}, "last_name": {"Agent"},
		"groups": {fmt.Sprintf("%d", adminID)},
	})
	require.Equal(t, http.StatusOK, code, body)
	assert.Equal(t, []string{"admin:rw"}, membership(t, userID))

	code, body = serveDynamic(t, r, http.MethodPut, fmt.Sprintf("/dynamic/users/%d", userID), url.Values{
		"login": {login}, "first_name": {"Dyn"}, "last_name": {"Agent"}, "groups": {"999999"},
	})
	assert.Equal(t, http.StatusBadRequest, code, body)
	assert.Equal(t, []string{"admin:rw"}, membership(t, userID), "invalid group must not wipe memberships")

	code, body = serveDynamic(t, r, http.MethodGet, fmt.Sprintf("/dynamic/users?group_id=%d&page_size=100", adminID), nil)
	require.Equal(t, http.StatusOK, code, body)
	found := false
	for _, it := range body["data"].([]any) {
		item := it.(map[string]any)
		if item["login"] == login {
			found = true
			assert.Contains(t, item["group_names"], "admin", "group_names lambda reads group_user")
		}
	}
	assert.True(t, found, "group filter must include the member")

	code, body = serveDynamic(t, r, http.MethodDelete, fmt.Sprintf("/dynamic/users/%d", userID), nil)
	require.Equal(t, http.StatusOK, code, body)
	var valid int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT valid_id FROM users WHERE id = ?`), userID).Scan(&valid))
	assert.Equal(t, 2, valid)
}

// A lookup-style module (priority → ticket_priority) round-trips on both drivers.
func TestDynamicPriorityModuleCRUD(t *testing.T) {
	r := newModulesTestRouter(t)
	db, err := database.GetDB()
	require.NoError(t, err)
	name := fmt.Sprintf("dynprio%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM ticket_priority WHERE name LIKE ?`), name+"%")
	})

	code, body := serveDynamic(t, r, http.MethodPost, "/dynamic/priority", url.Values{"name": {name}, "color": {"#112233"}})
	require.Equal(t, http.StatusCreated, code, body)
	id := idByColumn(t, "ticket_priority", "name", name)

	code, body = serveDynamic(t, r, http.MethodGet, fmt.Sprintf("/dynamic/priority/%d", id), nil)
	require.Equal(t, http.StatusOK, code, body)
	assert.Equal(t, name, body["data"].(map[string]any)["name"])

	code, body = serveDynamic(t, r, http.MethodPut, fmt.Sprintf("/dynamic/priority/%d", id), url.Values{"name": {name + "x"}, "color": {"#445566"}})
	require.Equal(t, http.StatusOK, code, body)
	var got, color string
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT name, color FROM ticket_priority WHERE id = ?`), id).Scan(&got, &color))
	assert.Equal(t, name+"x", got)
	assert.Equal(t, "#445566", color)

	code, body = serveDynamic(t, r, http.MethodGet, "/dynamic/priority?page_size=100", nil)
	require.Equal(t, http.StatusOK, code, body)
	listed := false
	for _, it := range body["data"].([]any) {
		if it.(map[string]any)["name"] == name+"x" {
			listed = true
		}
	}
	assert.True(t, listed)

	// lambda_demo (same table) runs db.queryRow with a `?` placeholder.
	code, body = serveDynamic(t, r, http.MethodGet, "/dynamic/lambda_demo?page_size=100", nil)
	require.Equal(t, http.StatusOK, code, body)
	for _, it := range body["data"].([]any) {
		item := it.(map[string]any)
		if item["name"] == name+"x" {
			assert.Contains(t, item["usage_stats"], "Not used", "new priority has no tickets")
		}
	}

	code, body = serveDynamic(t, r, http.MethodDelete, fmt.Sprintf("/dynamic/priority/%d", id), nil)
	require.Equal(t, http.StatusOK, code, body)
	var valid int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT valid_id FROM ticket_priority WHERE id = ?`), id).Scan(&valid))
	assert.Equal(t, 2, valid)
}

// A computed-field lambda whose SQL the database layer rejects must fail the
// request with a message. It used to panic inside the lambda goroutine, which
// gin cannot recover, killing the process.
func TestDynamicLambdaInvalidSQLReportsError(t *testing.T) {
	dir := t.TempDir()
	module := `module:
  name: badsql
  singular: Bad
  plural: Bads
  table: ticket_priority
fields:
  - name: id
    type: int
    db_column: id
    show_in_list: true
  - name: name
    type: string
    db_column: name
    show_in_list: true
computed_fields:
  - name: dollar
    show_in_list: true
    lambda: |
      return db.query("SELECT name FROM ticket_priority WHERE id = $1", item.id); // sql-ok: deliberate $N, the test asserts it is rejected
`
	require.NoError(t, os.WriteFile(dir+"/badsql.yaml", []byte(module), 0o600))
	r := newModulesTestRouterFor(t, dir)

	code, body := serveDynamic(t, r, http.MethodGet, "/dynamic/badsql", nil)
	assert.Equal(t, http.StatusInternalServerError, code, body)
	assert.Contains(t, body["error"], `computed field "dollar"`)
	assert.Contains(t, body["error"], "$N placeholders are not allowed")

	// The handler (and process) is still alive for the next request.
	code, body = serveDynamic(t, r, http.MethodGet, "/dynamic/badsql/1", nil)
	assert.Equal(t, http.StatusOK, code, body)
}
