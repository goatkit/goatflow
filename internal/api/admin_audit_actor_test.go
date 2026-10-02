package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
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

// seedAdminRouter is a bare router whose requests are authenticated as the
// seeded admin (users.id 1), as the auth middleware would do in production.
// Handlers that write audit columns refuse anonymous requests.
func seedAdminRouter() *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", uint(1))
		c.Next()
	})
	return r
}

// auditActorRouter mounts the admin write handlers behind a stand-in for the
// auth middleware that authenticates actorID (0 = anonymous request).
func auditActorRouter(db *sql.DB, actorID int) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if actorID > 0 {
			c.Set("user_id", uint(actorID))
		}
		c.Set("user_role", "Admin")
		c.Next()
	})
	r.POST("/api/types", handleCreateType)
	r.PUT("/api/types/:id", handleUpdateType)
	r.DELETE("/api/types/:id", handleDeleteType)
	r.POST("/admin/users", HandleAdminUserCreate)
	r.PUT("/admin/users/:id", HandleAdminUserUpdate)
	r.POST("/admin/services", handleAdminServiceCreate)
	r.PUT("/admin/services/:id", handleAdminServiceUpdate)
	r.DELETE("/admin/services/:id", handleAdminServiceDelete)
	r.POST("/admin/roles", handleAdminRoleCreate)
	r.POST("/admin/roles/:id/users", handleAdminRoleUserAdd)
	r.DELETE("/admin/roles/:id", handleAdminRoleDelete)
	r.POST("/admin/groups/:id/users", HandleAdminGroupsAddUser)
	r.POST("/admin/customer/companies", handleAdminCreateCustomerCompany(db))
	r.POST("/admin/customer-users", HandleAdminCustomerUsersCreate)
	r.POST("/admin/sla", handleAdminSLACreate)
	return r
}

func auditJSON(t *testing.T, r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func auditForm(t *testing.T, r *gin.Engine, method, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// auditCols returns create_by and change_by of the single row matched by where.
func auditCols(t *testing.T, db *sql.DB, table, where string, args ...any) (int, int) {
	t.Helper()
	var createBy, changeBy int
	q := fmt.Sprintf("SELECT create_by, change_by FROM %s WHERE %s", table, where) //nolint:gk-sql-sprintf // test-local table/where literals
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(q), args...).Scan(&createBy, &changeBy))
	return createBy, changeBy
}

func cleanupRows(t *testing.T, db *sql.DB, stmts []string, args ...any) {
	t.Cleanup(func() {
		for _, s := range stmts {
			_, _ = db.Exec(database.ConvertPlaceholders(s), args...)
		}
	})
}

// Admin writes must record the authenticated admin in create_by/change_by,
// not the seeded user 1.
func TestAdminWritesStampActingUser(t *testing.T) {
	db := getTestDB(t)
	actor, _ := createIsolatedAgent(t, "audit_actor")
	r := auditActorRouter(db, actor)
	sfx := strconv.FormatInt(time.Now().UnixNano(), 10)

	t.Run("ticket type create, update, delete", func(t *testing.T) {
		name := "audit_type_" + sfx
		cleanupRows(t, db, []string{"DELETE FROM ticket_type WHERE name = ?"}, name)
		w := auditJSON(t, r, http.MethodPost, "/api/types", map[string]any{"name": name})
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
		cb, chb := auditCols(t, db, "ticket_type", "name = ?", name)
		assert.Equal(t, actor, cb)
		assert.Equal(t, actor, chb)

		var id int
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders("SELECT id FROM ticket_type WHERE name = ?"), name).Scan(&id))
		require.NoError(t, setChangeBy(db, "ticket_type", id))
		w = auditJSON(t, r, http.MethodPut, "/api/types/"+strconv.Itoa(id), map[string]any{"valid_id": 1})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		_, chb = auditCols(t, db, "ticket_type", "id = ?", id)
		assert.Equal(t, actor, chb)

		require.NoError(t, setChangeBy(db, "ticket_type", id))
		w = auditJSON(t, r, http.MethodDelete, "/api/types/"+strconv.Itoa(id), nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		_, chb = auditCols(t, db, "ticket_type", "id = ?", id)
		assert.Equal(t, actor, chb)
	})

	t.Run("agent create and update with groups", func(t *testing.T) {
		groupID, _ := createIsolatedGroup(t, "audit_grp")
		login := "audit_agent_" + sfx
		cleanupAgentByLogin(t, login)
		w := auditForm(t, r, http.MethodPost, "/admin/users", url.Values{
			"login": {login}, "first_name": {"Audit"}, "last_name": {"Agent"},
			"valid_id": {"1"}, "groups": {strconv.Itoa(groupID)},
		})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		cb, chb := auditCols(t, db, "users", "login = ?", login)
		assert.Equal(t, actor, cb)
		assert.Equal(t, actor, chb)
		cb, chb = auditCols(t, db, "group_user", "group_id = ? AND permission_key = 'rw'", groupID)
		assert.Equal(t, actor, cb)
		assert.Equal(t, actor, chb)

		var id int
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders("SELECT id FROM users WHERE login = ?"), login).Scan(&id))
		require.NoError(t, setChangeBy(db, "users", id))
		_, err := db.Exec(database.ConvertPlaceholders("UPDATE group_user SET create_by = 1, change_by = 1 WHERE user_id = ?"), id)
		require.NoError(t, err)
		w = auditForm(t, r, http.MethodPut, "/admin/users/"+strconv.Itoa(id), url.Values{
			"login": {login}, "first_name": {"Audit"}, "last_name": {"Renamed"},
			"valid_id": {"1"}, "groups": {strconv.Itoa(groupID)},
		})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		_, chb = auditCols(t, db, "users", "id = ?", id)
		assert.Equal(t, actor, chb)
		cb, chb = auditCols(t, db, "group_user", "user_id = ? AND group_id = ?", id, groupID)
		assert.Equal(t, actor, cb)
		assert.Equal(t, actor, chb)
	})

	t.Run("service create, update, delete", func(t *testing.T) {
		name := "audit_service_" + sfx
		cleanupRows(t, db, []string{"DELETE FROM service WHERE name = ?"}, name)
		w := auditJSON(t, r, http.MethodPost, "/admin/services", map[string]any{"name": name})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		cb, chb := auditCols(t, db, "service", "name = ?", name)
		assert.Equal(t, actor, cb)
		assert.Equal(t, actor, chb)

		var id int
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders("SELECT id FROM service WHERE name = ?"), name).Scan(&id))
		require.NoError(t, setChangeBy(db, "service", id))
		w = auditJSON(t, r, http.MethodPut, "/admin/services/"+strconv.Itoa(id), map[string]any{"comments": "changed"})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		_, chb = auditCols(t, db, "service", "id = ?", id)
		assert.Equal(t, actor, chb)

		require.NoError(t, setChangeBy(db, "service", id))
		w = auditJSON(t, r, http.MethodDelete, "/admin/services/"+strconv.Itoa(id), nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		_, chb = auditCols(t, db, "service", "id = ?", id)
		assert.Equal(t, actor, chb)
	})

	t.Run("role create, membership, delete", func(t *testing.T) {
		member, _ := createIsolatedAgent(t, "audit_member")
		name := "audit_role_" + sfx
		cleanupRows(t, db, []string{
			"DELETE FROM role_user WHERE role_id IN (SELECT id FROM roles WHERE name = ?)",
			"DELETE FROM roles WHERE name = ?",
		}, name)
		w := auditJSON(t, r, http.MethodPost, "/admin/roles", map[string]any{"name": name})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		cb, chb := auditCols(t, db, "roles", "name = ?", name)
		assert.Equal(t, actor, cb)
		assert.Equal(t, actor, chb)

		var id int
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders("SELECT id FROM roles WHERE name = ?"), name).Scan(&id))
		w = auditJSON(t, r, http.MethodPost, "/admin/roles/"+strconv.Itoa(id)+"/users", map[string]any{"user_id": member})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		cb, chb = auditCols(t, db, "role_user", "role_id = ? AND user_id = ?", id, member)
		assert.Equal(t, actor, cb)
		assert.Equal(t, actor, chb)

		require.NoError(t, setChangeBy(db, "roles", id))
		w = auditJSON(t, r, http.MethodDelete, "/admin/roles/"+strconv.Itoa(id), nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		_, chb = auditCols(t, db, "roles", "id = ?", id)
		assert.Equal(t, actor, chb)
	})

	t.Run("group membership", func(t *testing.T) {
		groupID, _ := createIsolatedGroup(t, "audit_member_grp")
		member, _ := createIsolatedAgent(t, "audit_grp_member")
		w := auditJSON(t, r, http.MethodPost, "/admin/groups/"+strconv.Itoa(groupID)+"/users", map[string]any{"user_id": member})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		cb, chb := auditCols(t, db, "group_user", "user_id = ? AND group_id = ?", member, groupID)
		assert.Equal(t, actor, cb)
		assert.Equal(t, actor, chb)
	})

	t.Run("customer company create", func(t *testing.T) {
		customerID := "AUDIT" + sfx
		cleanupRows(t, db, []string{"DELETE FROM customer_company WHERE customer_id = ?"}, customerID)
		w := auditForm(t, r, http.MethodPost, "/admin/customer/companies", url.Values{
			"customer_id": {customerID}, "name": {"Audit Co " + sfx},
		})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		cb, chb := auditCols(t, db, "customer_company", "customer_id = ?", customerID)
		assert.Equal(t, actor, cb)
		assert.Equal(t, actor, chb)
	})

	t.Run("customer user create", func(t *testing.T) {
		login := "audit_cust_" + sfx
		cleanupRows(t, db, []string{"DELETE FROM customer_user WHERE login = ?"}, login)
		w := auditJSON(t, r, http.MethodPost, "/admin/customer-users", map[string]any{
			"login": login, "email": login + "@example.test", "customer_id": "AUDIT" + sfx,
			"first_name": "Audit", "last_name": "Customer",
		})
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
		cb, chb := auditCols(t, db, "customer_user", "login = ?", login)
		assert.Equal(t, actor, cb)
		assert.Equal(t, actor, chb)
	})

	t.Run("sla create", func(t *testing.T) {
		name := "audit_sla_" + sfx
		cleanupRows(t, db, []string{"DELETE FROM sla WHERE name = ?"}, name)
		w := auditJSON(t, r, http.MethodPost, "/admin/sla", map[string]any{"name": name})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		cb, chb := auditCols(t, db, "sla", "name = ?", name)
		assert.Equal(t, actor, cb)
		assert.Equal(t, actor, chb)
	})
}

// setChangeBy resets change_by to the seeded admin so a later assertion proves
// the handler under test wrote it.
func setChangeBy(db *sql.DB, table string, id int) error {
	q := fmt.Sprintf("UPDATE %s SET change_by = 1 WHERE id = ?", table) //nolint:gk-sql-sprintf // test-local table literal
	_, err := db.Exec(database.ConvertPlaceholders(q), id)
	return err
}

// Without an authenticated user there is nobody to attribute the write to:
// the handler refuses instead of recording user 1.
func TestAdminWritesRejectAnonymousActor(t *testing.T) {
	db := getTestDB(t)
	r := auditActorRouter(db, 0)
	name := "audit_anon_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	cleanupRows(t, db, []string{"DELETE FROM ticket_type WHERE name = ?", "DELETE FROM service WHERE name = ?"}, name)

	w := auditJSON(t, r, http.MethodPost, "/api/types", map[string]any{"name": name})
	assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
	w = auditJSON(t, r, http.MethodPost, "/admin/services", map[string]any{"name": name})
	assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())

	var n int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		"SELECT (SELECT COUNT(*) FROM ticket_type WHERE name = ?) + (SELECT COUNT(*) FROM service WHERE name = ?)"), name, name).Scan(&n))
	assert.Zero(t, n)
}
