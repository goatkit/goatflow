package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/middleware"
	"github.com/goatkit/goatflow/internal/platform/plugin"
	"github.com/goatkit/goatflow/internal/platform/pluginui"
	"github.com/goatkit/goatflow/internal/service"
	pkgplugin "github.com/goatkit/goatflow/pkg/plugin"
)

// callerProbe is an in-process plugin that reports what one call looks like
// from the plugin's side: its args, the organisation and language the host
// gives the call, and the gk_org_plugin_access rows a query without any org
// filter returns.
type callerProbe struct {
	name   string
	marker string
	host   plugin.HostAPI
	mu     sync.Mutex
	args   map[string]any
}

func (p *callerProbe) GKRegister() plugin.GKRegistration {
	return plugin.GKRegistration{
		Name: p.name, Version: "1.0.0",
		SetupTasks: []pkgplugin.SetupTaskSpec{{ID: "configure", Title: "Configure", Handler: "probe", Category: "test"}},
		Resources: &plugin.ResourceRequest{Permissions: []plugin.Permission{
			{Type: "db", Access: "read", Scope: []string{"gk_org_plugin_access"}},
		}},
	}
}

func (p *callerProbe) Init(_ context.Context, host plugin.HostAPI) error {
	p.host = host
	return nil
}

func (p *callerProbe) Shutdown(context.Context) error { return nil }

func (p *callerProbe) Call(ctx context.Context, fn string, raw json.RawMessage) (json.RawMessage, error) {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	p.mu.Lock()
	p.args = args
	p.mu.Unlock()
	rows, err := p.host.DBQuery(ctx, "SELECT plugin_name FROM gk_org_plugin_access WHERE plugin_name LIKE ?", p.marker+"%")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(rows))
	for _, r := range rows {
		names = append(names, fmt.Sprint(r["plugin_name"]))
	}
	sort.Strings(names)
	view, err := json.Marshal(probeView{Org: p.host.OrgID(ctx), Text: p.host.Translate(ctx, "common.save"), Rows: names})
	if err != nil {
		return nil, err
	}
	if fn == "ui_home" {
		return json.Marshal(map[string]string{"html": string(view)})
	}
	return view, nil
}

func (p *callerProbe) lastArgs() map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.args
}

type probeView struct {
	Org  int64    `json:"org"`
	Text string   `json:"text"`
	Rows []string `json:"rows"`
}

// callerContextEnv is an admin agent who belongs to organisations A and B,
// each owning one gk_org_plugin_access row named marker-a / marker-b, and the
// probe plugin registered and enabled on a fresh plugin manager.
type callerContextEnv struct {
	db         *sql.DB
	probe      *callerProbe
	token      string
	adminID    int
	orgA, orgB int64
}

func newCallerContextEnv(t *testing.T) *callerContextEnv {
	t.Helper()
	db, err := database.GetDB()
	require.NoError(t, err)
	sfx := strconv.FormatInt(time.Now().UnixNano(), 10)
	env := &callerContextEnv{db: db}
	env.adminID, env.token = selfAuthzAgent(t, db, "callerctx-admin-"+sfx, true)

	marker := "callerctx-" + sfx
	for _, side := range []struct {
		letter string
		id     *int64
	}{{"a", &env.orgA}, {"b", &env.orgB}} {
		slug := marker + "-" + side.letter
		*side.id = selfAuthzOrg(t, db, slug, "")
		_, err := db.Exec(database.ConvertPlaceholders(`
			INSERT INTO gk_org_plugin_access (org_id, plugin_name, group_id, create_time, create_by)
			VALUES (?, ?, 1, NOW(), 1)`), *side.id, slug)
		require.NoError(t, err)
		_, err = db.Exec(database.ConvertPlaceholders(`
			INSERT INTO gk_user_organisation (org_id, user_id, role, is_default, create_time, create_by)
			VALUES (?, ?, 'member', FALSE, NOW(), 1)`), *side.id, env.adminID)
		require.NoError(t, err)
		orgID := *side.id
		t.Cleanup(func() {
			_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM gk_org_plugin_access WHERE org_id = ?`), orgID)
			_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM gk_user_organisation WHERE org_id = ?`), orgID)
		})
	}

	env.probe = &callerProbe{name: "callerprobe-" + sfx, marker: marker}
	mgr := plugin.NewManager(plugin.NewProdHostAPI(plugin.WithDB("default", db)))
	require.NoError(t, mgr.Register(context.Background(), env.probe))
	require.NoError(t, mgr.Enable(env.probe.name))
	prevMgr := GetPluginManager()
	SetPluginManager(mgr)
	t.Cleanup(func() {
		SetPluginManager(prevMgr)
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM sysconfig_modified WHERE name = ?`), "Plugin::"+env.probe.name+"::Enabled")
	})
	return env
}

func (e *callerContextEnv) rows(letters ...string) []string {
	out := make([]string, 0, len(letters))
	for _, l := range letters {
		out = append(out, e.probe.marker+"-"+l)
	}
	return out
}

// do sends the request as the admin agent with the given active org and UI
// language cookies.
func (e *callerContextEnv) do(t *testing.T, router http.Handler, method, path, body string, orgID int64) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+e.token)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "lang", Value: "de"})
	req.AddCookie(&http.Cookie{Name: "active_org_id", Value: strconv.FormatInt(orgID, 10)})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func decodeView(t *testing.T, w *httptest.ResponseRecorder) probeView {
	t.Helper()
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var v probeView
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &v), w.Body.String())
	return v
}

// TestPluginSetupTaskEnvelope: both setup-task routes send the plugin the
// host envelope, not the client's. A body naming another user, role,
// organisation or language used to reach the plugin verbatim and became the
// acting user and organisation of the call.
func TestPluginSetupTaskEnvelope(t *testing.T) {
	env := newCallerContextEnv(t)
	db := env.db
	getSetupAssistantService()
	prevSvc := setupAssistantSvc
	setupAssistantSvc = service.NewSetupAssistantService(db, GetPluginManager())
	t.Cleanup(func() { setupAssistantSvc = prevSvc })

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.NewI18nMiddleware().Handle())
	router.POST("/api/v1/admin/setup/tasks/:plugin/:task_id", SessionOrJWTAuth(), RequireAdmin(), HandleAPISetupTask)
	router.POST("/admin/setup/task/:plugin/:task_id", SessionOrJWTAuth(), RequireAdmin(), handleAdminSetupTask)

	forged := fmt.Sprintf(`{"_user_id":1,"_user_role":"Customer","_customer_login":"x@example.com","_org_id":%d,"_lang":"fr","name":"kept"}`, env.orgB)
	for _, path := range []string{"/api/v1/admin/setup/tasks/", "/admin/setup/task/"} {
		t.Run(path, func(t *testing.T) {
			view := decodeView(t, env.do(t, router, http.MethodPost, path+env.probe.name+"/configure", forged, env.orgA))
			assert.Equal(t, env.orgA, view.Org, "call runs in the caller's active org")
			assert.Equal(t, "Speichern", view.Text, "call runs in the caller's language")
			assert.Equal(t, env.rows("a"), view.Rows, "org B rows must stay invisible")

			args := env.probe.lastArgs()
			assert.EqualValues(t, env.adminID, args["_user_id"])
			assert.Equal(t, "Admin", args["_user_role"])
			assert.NotContains(t, args, "_customer_login")
			assert.EqualValues(t, env.orgA, args["_org_id"])
			assert.Equal(t, "de", args["_lang"])
			assert.Equal(t, "kept", args["name"], "non-envelope body keys are forwarded")
		})
	}

	w := env.do(t, router, http.MethodPost, "/api/v1/admin/setup/tasks/"+env.probe.name+"/configure", `[1,2]`, env.orgA)
	assert.Equal(t, http.StatusBadRequest, w.Code, "a non-object body is refused")
}

// TestPluginCallsRunInCallerOrgAndLanguage: API plugin calls and plugin UI
// pages run in the caller's active organisation (the sandbox scopes org-owned
// tables to it) and language (HostAPI Translate). UI pages used to read the
// org only from a context key no middleware sets and passed no language.
func TestPluginCallsRunInCallerOrgAndLanguage(t *testing.T) {
	env := newCallerContextEnv(t)

	cfg, err := json.Marshal(pluginui.UIConfig{Routes: []pluginui.UIRouteConfig{{Path: "/", Handler: "ui_home"}}})
	require.NoError(t, err)
	raw := json.RawMessage(cfg)
	repo := pluginui.NewRepositoryWithDB(env.db)
	ui := &pluginui.PluginUI{
		PluginName: env.probe.name, UIID: "app", FullID: pluginui.BuildFullID(env.probe.name, "app"), Name: "Probe",
		UIType: pluginui.TypeAgentApp, Shell: pluginui.ShellNone, Config: &raw, Enabled: true, ValidID: 1,
	}
	uiID, err := repo.Create(ui, 1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = repo.Delete(uiID) })

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.NewI18nMiddleware().Handle())
	RegisterPluginAPIRoutes(router.Group("/api/v1"))
	require.NoError(t, pluginui.RegisterUIRoutes(router, repo, GetPluginManager(), nil, pluginUIAuth(), slog.Default()))

	uiPath := "/ui/" + ui.FullID + "/"
	callPath := "/api/v1/plugins/" + env.probe.name + "/call/probe"
	for _, tc := range []struct {
		name, method, path string
	}{{"ui page", http.MethodGet, uiPath}, {"api call", http.MethodPost, callPath}} {
		t.Run(tc.name, func(t *testing.T) {
			view := decodeView(t, env.do(t, router, tc.method, tc.path, "", env.orgA))
			assert.Equal(t, env.orgA, view.Org)
			assert.Equal(t, "Speichern", view.Text)
			assert.Equal(t, env.rows("a"), view.Rows, "org A call must not read org B rows")

			view = decodeView(t, env.do(t, router, tc.method, tc.path, "", env.orgB))
			assert.Equal(t, env.orgB, view.Org)
			assert.Equal(t, env.rows("b"), view.Rows, "org B call must not read org A rows")
		})
	}
}
