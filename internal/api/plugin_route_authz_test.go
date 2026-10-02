package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/mcp"
	platformmodels "github.com/goatkit/goatflow/internal/platform/models"
	"github.com/goatkit/goatflow/internal/platform/plugin"
	"github.com/goatkit/goatflow/internal/platform/routing"
	"github.com/goatkit/goatflow/internal/platform/shared"
)

// authzEchoPlugin echoes the args it receives so tests can see what the host
// passed. Its routes cover every plugin route middleware form.
type authzEchoPlugin struct{ group string }

func (p *authzEchoPlugin) GKRegister() plugin.GKRegistration {
	return plugin.GKRegistration{
		Name:    "authzecho",
		Version: "1.0.0",
		Routes: []plugin.RouteSpec{
			{Method: "GET", Path: "/authz-echo/agent", Handler: "echo_agent", Middleware: []string{"agent"}},
			{Method: "GET", Path: "/authz-echo/customer", Handler: "echo_customer", Middleware: []string{"customer"}},
			{Method: "GET", Path: "/authz-echo/typo", Handler: "echo_typo", Middleware: []string{"agnet"}},
			{Method: "GET", Path: "/authz-echo/malformed", Handler: "echo_malformed", Middleware: []string{"plugin:authzecho"}},
			{Method: "GET", Path: "/authz-echo/group", Handler: "echo_group", Middleware: []string{"auth", "group:" + p.group}},
			{Method: "POST", Path: "/authz-echo/open", Handler: "echo_open", Middleware: []string{"auth"}},
			{Method: "POST", Path: "/authz-echo/admin", Handler: "echo_admin", Middleware: []string{"admin"}},
		},
	}
}

func (p *authzEchoPlugin) Init(context.Context, plugin.HostAPI) error { return nil }
func (p *authzEchoPlugin) Shutdown(context.Context) error             { return nil }
func (p *authzEchoPlugin) Call(_ context.Context, fn string, args json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(map[string]any{"fn": fn, "args": args})
}

type pluginAuthzFixture struct {
	db        *sql.DB
	group     string
	agentID   int // agent in the plugin's group
	adminID   int // agent in the admin group
	mgr       *plugin.Manager
	agentJWT  string
	adminJWT  string
	custJWT   string // customer token whose id equals agentID
	otherJWT  string // agent outside the group
	otherID   int
	jwtMaker  func(id int, login, role string, isAdmin bool) string
	createdAt time.Time
}

func newPluginAuthzFixture(t *testing.T) *pluginAuthzFixture {
	t.Helper()
	db := getTestDB(t)
	f := &pluginAuthzFixture{db: db, createdAt: time.Now()}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	f.group = "authz-echo-" + suffix
	adapter := database.GetAdapter()

	insertUser := func(login string) int {
		id, err := adapter.InsertWithReturning(db, database.ConvertPlaceholders(`INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, 'x', 'Authz', 'Echo', 1, ?, 1, ?, 1) RETURNING id`), login, f.createdAt, f.createdAt)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM group_user WHERE user_id = ?`), id)
			_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM users WHERE id = ?`), id)
		})
		return int(id)
	}
	addToGroup := func(userID int, groupID int64) {
		_, err := db.Exec(database.ConvertPlaceholders(`INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
			VALUES (?, ?, 'rw', ?, 1, ?, 1)`), userID, groupID, f.createdAt, f.createdAt)
		require.NoError(t, err)
	}

	groupID, err := adapter.InsertWithReturning(db, database.ConvertPlaceholders(`INSERT INTO groups (name, comments, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 'authz test', 1, ?, 1, ?, 1) RETURNING id`), f.group, f.createdAt, f.createdAt)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM group_user WHERE group_id = ?`), groupID)
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM groups WHERE id = ?`), groupID)
	})
	var adminGroupID int64
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT id FROM groups WHERE name = 'admin'`)).Scan(&adminGroupID))

	f.agentID = insertUser("authz-agent-" + suffix)
	addToGroup(f.agentID, groupID)
	f.adminID = insertUser("authz-admin-" + suffix)
	addToGroup(f.adminID, adminGroupID)
	f.otherID = insertUser("authz-other-" + suffix)

	jwtMgr := shared.GetJWTManager()
	f.jwtMaker = func(id int, login, role string, isAdmin bool) string {
		tok, err := jwtMgr.GenerateTokenWithLogin(uint(id), login, login, role, isAdmin, 0)
		require.NoError(t, err)
		return tok
	}
	f.agentJWT = f.jwtMaker(f.agentID, "authz-agent-"+suffix, "Agent", false)
	f.otherJWT = f.jwtMaker(f.otherID, "authz-other-"+suffix, "Agent", false)
	f.adminJWT = f.jwtMaker(f.adminID, "authz-admin-"+suffix, "Admin", true)
	f.custJWT = f.jwtMaker(f.agentID, "authz-cust-"+suffix, "Customer", false)

	host := plugin.NewProdHostAPI(plugin.WithDB("default", db))
	f.mgr = plugin.NewManager(host)
	require.NoError(t, f.mgr.Register(context.Background(), &authzEchoPlugin{group: f.group}))
	require.NoError(t, f.mgr.Enable("authzecho"))
	prev := GetPluginManager()
	SetPluginManager(f.mgr)
	t.Cleanup(func() { SetPluginManager(prev) })
	return f
}

func doAuthzRequest(h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func echoedArgs(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var resp struct {
		Args map[string]any `json:"args"`
	}
	require.NoError(t, json.Unmarshal(body, &resp), string(body))
	return resp.Args
}

// Plugin route middleware is enforced for every supported name, and an
// unknown or malformed name leaves the route unregistered. Before, only
// auth/admin/group:/plugin:/webhook were applied: "agent", "customer" and
// typos registered the route with no auth, a malformed plugin: spec was
// skipped, and group: admitted a customer whose customer_user id matched a
// group member's users id.
func TestPluginRouteMiddlewareGates(t *testing.T) {
	f := newPluginAuthzFixture(t)
	RebuildDynamicEngine()
	dynMu.RLock()
	eng := dynEngine
	dynMu.RUnlock()

	cases := []struct {
		name, path, token string
		want              int
	}{
		{"agent route anonymous", "/authz-echo/agent", "", http.StatusUnauthorized},
		{"agent route customer", "/authz-echo/agent", f.custJWT, http.StatusForbidden},
		{"agent route agent", "/authz-echo/agent", f.agentJWT, http.StatusOK},
		{"customer route agent", "/authz-echo/customer", f.agentJWT, http.StatusForbidden},
		{"customer route customer", "/authz-echo/customer", f.custJWT, http.StatusOK},
		{"unknown middleware not registered", "/authz-echo/typo", f.adminJWT, http.StatusNotFound},
		{"malformed plugin: not registered", "/authz-echo/malformed", f.adminJWT, http.StatusNotFound},
		{"group member", "/authz-echo/group", f.agentJWT, http.StatusOK},
		{"group non-member", "/authz-echo/group", f.otherJWT, http.StatusForbidden},
		{"customer with a member's id", "/authz-echo/group", f.custJWT, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doAuthzRequest(eng, "GET", tc.path, tc.token, "")
			require.Equal(t, tc.want, w.Code, w.Body.String())
			if tc.want == http.StatusOK {
				args := echoedArgs(t, w.Body.Bytes())
				require.Equal(t, float64(f.agentID), args["_user_id"])
			}
		})
	}
}

// Client-supplied values under the host's envelope keys never reach the
// plugin: before, a body {"_customer_login": ...} passed through for agents
// and {"_is_admin": true} passed through whenever the auth middleware had not
// set isInAdminGroup (session and API-token callers).
func TestPluginRouteArgsEnvelope(t *testing.T) {
	f := newPluginAuthzFixture(t)
	RebuildDynamicEngine()
	dynMu.RLock()
	eng := dynEngine
	dynMu.RUnlock()

	w := doAuthzRequest(eng, "POST", "/authz-echo/open", f.agentJWT,
		`{"_is_admin":true,"_customer_login":"victim","_user_id":1,"x":"y"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	args := echoedArgs(t, w.Body.Bytes())
	require.Equal(t, float64(f.agentID), args["_user_id"])
	require.Equal(t, false, args["_is_admin"])
	require.NotContains(t, args, "_customer_login")
	require.Equal(t, "y", args["x"])

	// Session-authenticated caller (no isInAdminGroup key).
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/authz-echo/open", bytes.NewBufferString(`{"_is_admin":true}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", f.otherID)
	c.Set("user_role", "Agent")
	var m map[string]any
	require.NoError(t, json.Unmarshal(buildPluginArgs(c, "authzecho"), &m))
	require.Equal(t, false, m["_is_admin"])
}

// The generic plugin API: listing, health, widgets and SSE channels refuse
// customers; direct function calls (which skip the plugin's route ACLs) are
// admin-only and carry the caller's identity, not body-supplied values.
func TestPluginAPIRoutesPrincipalGates(t *testing.T) {
	f := newPluginAuthzFixture(t)
	r := gin.New()
	RegisterPluginAPIRoutes(r.Group("/api/v1"))

	for _, path := range []string{"/api/v1/plugins", "/api/v1/plugins/health", "/api/v1/plugins/widgets", "/api/v1/plugins/authzecho/widgets/x", "/api/v1/plugins/authzecho/events/ch"} {
		w := doAuthzRequest(r, "GET", path, f.custJWT, "")
		require.Equal(t, http.StatusForbidden, w.Code, "customer on %s: %s", path, w.Body.String())
	}
	w := doAuthzRequest(r, "GET", "/api/v1/plugins", f.agentJWT, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = doAuthzRequest(r, "POST", "/api/v1/plugins/authzecho/call/echo_admin", f.agentJWT, `{}`)
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	w = doAuthzRequest(r, "POST", "/api/v1/plugins/authzecho/call/echo_admin", f.custJWT, `{}`)
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	w = doAuthzRequest(r, "POST", "/api/v1/plugins/authzecho/call/echo_admin", f.adminJWT, `{"_user_id":999,"q":1}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	args := echoedArgs(t, w.Body.Bytes())
	require.Equal(t, float64(f.adminID), args["_user_id"])
	require.Equal(t, true, args["_is_admin"])
	require.Equal(t, float64(1), args["q"])
}

// mcpCall posts one tools/call to HandleMCP as the principal set by auth.
func mcpCall(t *testing.T, auth gin.HandlerFunc, tool string, args map[string]any) (text string, isError bool) {
	t.Helper()
	r := gin.New()
	r.POST("/api/mcp", auth, HandleMCP)
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": args}})
	req := httptest.NewRequest("POST", "/api/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var resp struct {
		Result mcp.ToolCallResult `json:"result"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp), w.Body.String())
	require.NotEmpty(t, resp.Result.Content, w.Body.String())
	return resp.Result.Content[0].Text, resp.Result.IsError
}

func jwtPrincipal(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Header.Set("Authorization", "Bearer "+token)
		SessionOrJWTAuth()(c)
	}
}

// MCP tools run with the caller's real permissions. Before: plugin tools ran
// with no check of the plugin route's middleware and passed client-supplied
// _is_admin through; core tools saw an agent API token as role "Admin" when
// its owner was an admin, ignoring that the token lacked the admin:* scope.
func TestMCPToolsEnforceCallerPermissions(t *testing.T) {
	f := newPluginAuthzFixture(t)

	reg := routing.NewHandlerRegistry()
	routing.RegisterExistingHandlers(reg)
	prevReg := routing.SetGlobalRegistryForTest(reg)
	t.Cleanup(func() { routing.SetGlobalRegistryForTest(prevReg) })
	routing.GlobalHandlerMap["mcpAuthzAdminProbe"] = func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"probe": "ok"}) }
	t.Cleanup(func() { delete(routing.GlobalHandlerMap, "mcpAuthzAdminProbe") })

	mcpInitOnce.Do(func() {})
	prevBridge := mcpBridge
	mcpBridge = mcp.NewAPIBridge()
	mcpBridge.SetPluginGate(pluginMCPGate{})
	mcpBridge.SetPluginCaller(f.mgr)
	t.Cleanup(func() { mcpBridge = prevBridge })
	mcp.RefreshDynamicTools([]mcp.RouteInput{{
		GroupName: "api-v1-mcpauthz", Prefix: "/api/v1", GroupMiddleware: []string{"unified_auth"},
		Path: "/mcpauthz/probe", Method: "GET", HandlerName: "mcpAuthzAdminProbe", Middleware: []string{"admin"},
	}})
	refreshPluginMCPTools(f.mgr)
	adminTool := ""
	for name, tool := range mcp.GetDynamicToolsMap() {
		if tool.HandlerName == "mcpAuthzAdminProbe" {
			adminTool = name
		}
	}
	require.NotEmpty(t, adminTool)

	t.Run("plugin admin route refuses agent", func(t *testing.T) {
		text, isErr := mcpCall(t, jwtPrincipal(f.agentJWT), "authzecho_echo_admin", nil)
		require.True(t, isErr, text)
		require.Contains(t, text, "403")
	})
	t.Run("plugin group route refuses customer with member id", func(t *testing.T) {
		text, isErr := mcpCall(t, jwtPrincipal(f.custJWT), "authzecho_echo_group", nil)
		require.True(t, isErr, text)
		require.Contains(t, text, "403")
	})
	t.Run("plugin group route admits member", func(t *testing.T) {
		text, isErr := mcpCall(t, jwtPrincipal(f.agentJWT), "authzecho_echo_group", nil)
		require.False(t, isErr, text)
	})
	t.Run("plugin args envelope", func(t *testing.T) {
		text, isErr := mcpCall(t, jwtPrincipal(f.agentJWT), "authzecho_echo_open", map[string]any{"_is_admin": true, "_user_id": 1})
		require.False(t, isErr, text)
		args := echoedArgs(t, []byte(text))
		require.Equal(t, false, args["_is_admin"])
		require.Equal(t, float64(f.agentID), args["_user_id"])
	})

	// What the API token middleware sets for an agent token of an admin owner
	// without the admin:* scope: role "User", no isInAdminGroup.
	unscopedToken := func(c *gin.Context) {
		c.Set("user_id", f.adminID)
		c.Set("user_role", "User")
		c.Set("api_token", &platformmodels.APIToken{ID: 1, UserID: f.adminID, UserType: platformmodels.APITokenUserAgent, Scopes: []string{"tickets:read"}})
		c.Next()
	}
	t.Run("core admin tool refuses token without admin scope", func(t *testing.T) {
		text, isErr := mcpCall(t, unscopedToken, adminTool, nil)
		require.True(t, isErr, text)
		require.Contains(t, text, "403")
	})
	t.Run("core admin tool admits admin JWT", func(t *testing.T) {
		text, isErr := mcpCall(t, jwtPrincipal(f.adminJWT), adminTool, nil)
		require.False(t, isErr, text)
		require.Contains(t, text, `"probe":"ok"`)
	})
}
