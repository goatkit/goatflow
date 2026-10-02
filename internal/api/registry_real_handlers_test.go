package api

import (
	"encoding/json"
	"fmt"
	"html"
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
	"github.com/goatkit/goatflow/internal/platform/shared"
)

// yamlHandlerRouter mounts the handler that routes/*.yaml resolve for name,
// authenticated as actor.
func yamlHandlerRouter(t *testing.T, actor int, method, path, name string) *gin.Engine {
	t.Helper()
	h, err := NewRoutingHandlerResolver().Get(name)
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", uint(actor))
		c.Next()
	})
	r.Handle(method, path, h)
	return r
}

// The YAML group-user endpoints resolve to the real handlers: they list,
// add and remove real group_user rows instead of answering a canned success.
func TestYAMLGroupUserHandlersAreReal(t *testing.T) {
	if !dbAvailable() {
		t.Skip("DB not available")
	}
	db := isolatedDB(t)
	actorID, _ := createIsolatedAgent(t, "grpusr_actor")
	memberID, memberLogin := createIsolatedAgent(t, "grpusr_member")
	groupID, _ := createIsolatedGroup(t, "grpusr_group")

	add := yamlHandlerRouter(t, actorID, http.MethodPost, "/admin/groups/:id/users", "HandleAdminGroupsAddUser")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/groups/%d/users", groupID),
		strings.NewReader(url.Values{"user_id": {fmt.Sprint(memberID)}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	add.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var createBy int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		`SELECT create_by FROM group_user WHERE user_id = ? AND group_id = ? AND permission_key = 'rw'`),
		memberID, groupID).Scan(&createBy))
	assert.Equal(t, actorID, createBy)

	list := yamlHandlerRouter(t, actorID, http.MethodGet, "/admin/groups/:id/users", "HandleAdminGroupsUsers")
	w = httptest.NewRecorder()
	list.ServeHTTP(w, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/groups/%d/users", groupID), nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var listed struct {
		Users []struct {
			ID    int    `json:"id"`
			Login string `json:"login"`
		} `json:"users"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listed))
	require.Len(t, listed.Users, 1, w.Body.String())
	assert.Equal(t, memberID, listed.Users[0].ID)
	assert.Equal(t, memberLogin, listed.Users[0].Login)

	remove := yamlHandlerRouter(t, actorID, http.MethodDelete, "/admin/groups/:id/users/:userId", "HandleAdminGroupsRemoveUser")
	w = httptest.NewRecorder()
	remove.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/admin/groups/%d/users/%d", groupID, memberID), nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Zero(t, articleCountRows(t, db, `SELECT COUNT(*) FROM group_user WHERE user_id = ? AND group_id = ?`, memberID, groupID))
}

// GET /admin/password-policy answers the configured policy in the shape the
// users page reads (data.policy), not a hard-coded placeholder.
func TestYAMLPasswordPolicyHandlerIsReal(t *testing.T) {
	if !dbAvailable() {
		t.Skip("DB not available")
	}
	r := yamlHandlerRouter(t, 1, http.MethodGet, "/admin/password-policy", "handleAdminPasswordPolicy")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/password-policy", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var resp struct {
		Success bool           `json:"success"`
		Policy  map[string]any `json:"policy"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.NotEmpty(t, resp.Policy, w.Body.String())
}

// Pages whose template renderer is missing answer 500 instead of a
// hand-built stand-in page.
func TestPagesWithoutRendererAnswer500(t *testing.T) {
	if !dbAvailable() {
		t.Skip("DB not available")
	}
	db := isolatedDB(t)
	queueID, queueName := createIsolatedQueue(t, "norenderer")

	prev := shared.GetGlobalRenderer()
	shared.SetGlobalRenderer(nil)
	t.Cleanup(func() { shared.SetGlobalRenderer(prev) })

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", uint(1)); c.Next() })
	r.GET("/admin/customer/companies", handleAdminCustomerCompanies(db))
	r.GET("/admin/customer-user-services", handleAdminCustomerUserServices(db))
	r.GET("/queues/:id", handleQueueDetail)

	for _, tc := range []struct {
		path string
		hx   bool
	}{
		{"/admin/customer/companies", false},
		{"/admin/customer-user-services", false},
		{fmt.Sprintf("/queues/%d", queueID), false},
		{fmt.Sprintf("/queues/%d", queueID), true},
	} {
		t.Run(fmt.Sprintf("%s hx=%v", tc.path, tc.hx), func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			if tc.hx {
				req.Header.Set("HX-Request", "true")
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
			assert.NotContains(t, w.Body.String(), queueName)
			assert.NotContains(t, w.Body.String(), "<h1>")
		})
	}
}

// Customer user search is bound as a parameter: quote characters are
// searched for literally and cannot break or extend the SQL.
func TestCustomerUserServicesSearchIsParameterized(t *testing.T) {
	if !dbAvailable() {
		t.Skip("DB not available")
	}
	SetupTestTemplateRenderer(t)
	db := isolatedDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", uint(1)); c.Next() })
	r.GET("/admin/customer-user-services", handleAdminCustomerUserServices(db))

	login := fmt.Sprintf("o'brien_%d", time.Now().UnixNano())
	createTestCustomerUser(t, db, login)
	t.Cleanup(func() { cleanupTestCustomerUser(t, db, login) })

	// A quote in the term used to end the SQL string literal: the query
	// failed and the page silently listed nobody.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet,
		"/admin/customer-user-services?search="+url.QueryEscape("o'brien_"), nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), html.EscapeString(login))

	// An injected OR clause is searched literally and matches nobody.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet,
		"/admin/customer-user-services?search="+url.QueryEscape("x') OR 1=1 --"), nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.NotContains(t, w.Body.String(), html.EscapeString(login))
}
