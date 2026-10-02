package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/middleware"
	platformmodels "github.com/goatkit/goatflow/internal/platform/models"
	"github.com/goatkit/goatflow/internal/platform/plugin"
	"github.com/goatkit/goatflow/internal/platform/plugin/core"
	"github.com/goatkit/goatflow/internal/service"
)

// authzClass is who may call a route.
type authzClass string

const (
	authzPublic   authzClass = "public"   // anyone, no login
	authzCustomer authzClass = "customer" // customers only
	authzShared   authzClass = "shared"   // any authenticated principal (handler scopes customers to own data)
	authzAgent    authzClass = "agent"    // agents (incl. admins), never customers
	authzAdmin    authzClass = "admin"    // admins only
)

// authzPublicRoutes are reachable without login. Every other route must
// refuse anonymous callers.
var authzPublicRoutes = map[string]bool{
	"GET /": true, "GET /health": true, "GET /healthz": true,
	"GET /favicon.ico": true, "GET /favicon.svg": true, "GET /manifest.json": true,
	"GET /static/*filepath": true, "GET /sw.js": true, "GET /sw-config.json": true,
	"GET /swagger/*any":  true,
	"GET /api/languages": true, "POST /api/languages": true,
	"GET /api/themes": true, "POST /api/themes": true,
	// agent login, 2FA, passkeys, password reset, SSO
	"GET /login": true, "GET /login/2fa": true, "GET /logout": true,
	"GET /forgot-password": true, "POST /forgot-password": true,
	"GET /reset-password": true, "POST /reset-password": true,
	"POST /api/auth/login":              true,
	"POST /api/auth/2fa/verify":         true,
	"POST /api/auth/2fa/webauthn/begin": true, "POST /api/auth/2fa/webauthn/finish": true,
	"POST /api/auth/passkey/begin": true, "POST /api/auth/passkey/finish": true,
	"POST /api/v1/auth/login": true, "POST /api/v1/auth/refresh": true,
	"GET /auth/:id": true, "GET /auth/:id/callback": true, "GET /auth/:id/saml": true,
	"GET /auth/:id/metadata": true, "POST /auth/:id/acs": true, "GET /auth/customer": true,
	// customer login, 2FA, passkeys, self-service registration and reset
	"GET /customer/login": true, "POST /customer/login": true, "GET /customer/login/2fa": true,
	"GET /customer/logout":          true,
	"POST /api/auth/customer/login": true, "POST /api/auth/customer/2fa/verify": true,
	"POST /api/auth/customer/2fa/webauthn/begin": true, "POST /api/auth/customer/2fa/webauthn/finish": true,
	"POST /api/auth/customer/passkey/begin": true, "POST /api/auth/customer/passkey/finish": true,
	"GET /customer/register": true, "POST /customer/register": true,
	"GET /customer/register/complete": true, "POST /customer/register/complete": true,
	"GET /customer/forgot-password": true, "POST /customer/forgot-password": true,
	"GET /customer/reset-password": true, "POST /customer/reset-password": true,
	// MCP discovery document (the protocol endpoints need a token)
	"GET /api/mcp": true,
	// translation catalogue of the open-source UI (login page language picker)
	"GET /api/v1/i18n/languages": true, "POST /api/v1/i18n/language": true,
	"GET /api/v1/i18n/translations": true, "GET /api/v1/i18n/translations/:lang": true,
	"POST /api/v1/i18n/translate": true, "GET /api/v1/i18n/stats": true,
	"GET /api/v1/i18n/coverage": true, "GET /api/v1/i18n/missing/:lang": true,
	"GET /api/v1/i18n/export/:lang": true, "GET /api/v1/i18n/validate/:lang": true,
}

// authzSharedRoutes are open to every authenticated principal. The handler
// limits customers to their own tickets and customer-visible articles.
var authzSharedRoutes = map[string]bool{
	"GET /api/v1/tickets":                          true,
	"GET /api/v1/tickets/:id":                      true,
	"GET /api/v1/tickets/:id/articles":             true,
	"GET /api/v1/tickets/:id/articles/:article_id": true,
	"POST /api/auth/logout":                        true,
	"GET /api/v1/sse":                              true,
}

// authzAdminResources are /api resources whose writes are system
// configuration (admin only); reads stay open to agents.
var authzAdminResources = []string{
	"/api/v1/queues", "/api/v1/priorities", "/api/v1/users", "/api/v1/custom-fields/definitions",
	"/api/v1/salutations", "/api/v1/signatures", "/api/v1/system-addresses",
	"/api/queues", "/api/types",
}

// authzAdminPrefixes are admin only for every method.
var authzAdminPrefixes = []string{
	"/admin", "/api/v1/admin", "/api/v1/webhooks", "/api/v1/webhook-deliveries", "/api/v1/organisations", "/api/v1/plugin-uis",
	"/api/v1/plugins/logs", "/api/v1/plugins/marketplace", "/api/v1/plugins/upload",
}

func classifyRoute(method, path string) authzClass {
	key := method + " " + path
	if authzPublicRoutes[key] {
		return authzPublic
	}
	if authzSharedRoutes[key] {
		return authzShared
	}
	hasPrefix := func(p string) bool { return path == p || strings.HasPrefix(path, p+"/") }
	for _, p := range authzAdminPrefixes {
		if hasPrefix(p) {
			return authzAdmin
		}
	}
	switch key {
	case "GET /health/detailed", "GET /metrics",
		"POST /api/v1/search/reindex", "POST /api/lookups/cache/invalidate",
		"POST /api/v1/plugins/:name/call/:fn", "POST /api/v1/plugins/:name/enable",
		"POST /api/v1/plugins/:name/disable", "POST /api/v1/plugins/:name/reset-crashloop",
		"DELETE /api/v1/plugins/:name":
		return authzAdmin
	}
	if method != http.MethodGet {
		for _, p := range authzAdminResources {
			if hasPrefix(p) {
				return authzAdmin
			}
		}
	}
	if hasPrefix("/customer") {
		return authzCustomer
	}
	return authzAgent
}

// authzPrincipal is one kind of caller, applied to a request.
type authzPrincipal struct {
	name  string
	apply func(*http.Request)
}

type authzFixture struct {
	db        *sql.DB
	router    *gin.Engine
	ticketID  int
	anon      authzPrincipal
	customer  authzPrincipal // customer JWT
	custToken authzPrincipal // customer API token (no scopes)
	agent     authzPrincipal // non-admin agent JWT
	agentTok  authzPrincipal // non-admin agent's unscoped API token
}

// newProductionRouter assembles the engine the way cmd/goats does: main-engine
// routes (RegisterCoreRoutes) plus the dynamic engine with the YAML routes and
// the built-in plugins' routes.
func newProductionRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	mgr := plugin.NewManager(plugin.NewProdHostAPI())
	require.NoError(t, mgr.Register(context.Background(), core.NewDashboardPlugin()))

	prevMgr, prevDir := pluginManager, dynRouteDir
	dynMu.RLock()
	prevEng := dynEngine
	dynMu.RUnlock()
	t.Cleanup(func() {
		pluginManager, dynRouteDir = prevMgr, prevDir
		dynMu.Lock()
		dynEngine = prevEng
		dynMu.Unlock()
	})
	pluginManager = mgr

	routesDir := "../../routes"
	if _, err := os.Stat(routesDir); err != nil {
		t.Fatalf("routes directory not found at %s: %v", routesDir, err)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.SecurityHeaders())
	RegisterCoreRoutes(r, plugin.NewSSEBroker())
	MountDynamicEngine(r, routesDir)
	return r
}

// allRoutes lists every method+path the production router serves.
func allRoutes(r *gin.Engine) []gin.RouteInfo {
	out := append([]gin.RouteInfo{}, r.Routes()...)
	dynMu.RLock()
	out = append(out, dynEngine.Routes()...)
	dynMu.RUnlock()
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Method < out[j].Method
	})
	return out
}

func newAuthzFixture(t *testing.T) *authzFixture {
	t.Helper()
	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)
	f := &authzFixture{db: db, router: newProductionRouter(t)}

	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	custLogin := "authz-cust-" + suffix + "@example.com"
	custID := custAttInsertCustomer(t, db, custLogin)
	// The fixture ticket belongs to another customer: no test principal may
	// reach it through an agent or customer route it is not entitled to.
	f.ticketID = custAttInsertTicket(t, db, "AZ"+suffix, "authz-other-"+suffix+"@example.com")

	agentLogin := "authz-agent-" + suffix
	agentID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 'x', 'Authz', 'Agent', 1, NOW(), 1, NOW(), 1) RETURNING id`), agentLogin)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM user_api_tokens WHERE user_id = ? AND user_type = 'agent'`), agentID)
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM users WHERE id = ?`), agentID)
	})

	custJWT := testSessionToken(t, uint(custID), custLogin, custLogin, "Customer", false, 0)
	agentJWT := testSessionToken(t, uint(agentID), agentLogin, agentLogin, "Agent", false, 0)

	svc := service.NewAPITokenService(db)
	middleware.SetAPITokenVerifier(svc)
	ctx := context.Background()
	custTok, err := svc.GenerateToken(ctx, &platformmodels.APITokenCreateRequest{Name: "authz", ExpiresIn: "30d"},
		int(custID), platformmodels.APITokenUserCustomer, int(custID))
	require.NoError(t, err)
	agentTok, err := svc.GenerateToken(ctx, &platformmodels.APITokenCreateRequest{Name: "authz", ExpiresIn: "30d"},
		int(agentID), platformmodels.APITokenUserAgent, int(agentID))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM user_api_tokens WHERE user_id = ? AND user_type = 'customer'`), custID)
	})

	bearer := func(name, tok string) authzPrincipal {
		return authzPrincipal{name: name, apply: func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+tok) }}
	}
	f.anon = authzPrincipal{name: "anonymous", apply: func(*http.Request) {}}
	f.customer = bearer("customer JWT", custJWT)
	f.custToken = bearer("customer API token", custTok.Token)
	f.agent = bearer("agent JWT", agentJWT)
	f.agentTok = bearer("agent API token", agentTok.Token)
	return f
}

// concretePath fills route parameters: ticket ids get the fixture ticket,
// everything else a value that names nothing.
func (f *authzFixture) concretePath(path string) string {
	parts := strings.Split(path, "/")
	ticketPath := strings.Contains(path, "ticket")
	for i, p := range parts {
		switch {
		case strings.HasPrefix(p, "*"):
			parts[i] = "authz"
		case p == ":id" && ticketPath && i > 0 && strings.Contains(parts[i-1], "ticket"),
			p == ":ticket_id":
			parts[i] = strconv.Itoa(f.ticketID)
		case strings.HasPrefix(p, ":"):
			parts[i] = "987654321"
		}
	}
	return strings.Join(parts, "/")
}

// denied reports whether the response refuses the caller: 401/403, or a
// redirect to a login page.
func denied(w *httptest.ResponseRecorder) bool {
	switch w.Code {
	case http.StatusUnauthorized, http.StatusForbidden:
		return true
	case http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect:
		loc := w.Header().Get("Location")
		return strings.HasPrefix(loc, "/login") || strings.HasPrefix(loc, "/customer/login")
	}
	return false
}

func (f *authzFixture) call(method, path string, p authzPrincipal) *httptest.ResponseRecorder {
	var body *strings.Reader
	if method == http.MethodGet || method == http.MethodHead || method == http.MethodDelete {
		body = strings.NewReader("")
	} else {
		body = strings.NewReader("{}")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req := httptest.NewRequest(method, f.concretePath(path), body).WithContext(ctx)
	req.Header.Set("Accept", "application/json")
	if body.Len() > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	p.apply(req)
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	return w
}

// TestRouteAuthorizationMatrix sends every production route a request from
// each kind of principal and checks that whoever the route's class excludes
// is refused before the handler acts. A new route without the right
// middleware fails here: anonymous access needs an entry in
// authzPublicRoutes, customer access needs authzSharedRoutes or a /customer
// path.
func TestRouteAuthorizationMatrix(t *testing.T) {
	f := newAuthzFixture(t)
	routes := allRoutes(f.router)
	require.Greater(t, len(routes), 700, "production route table looks incomplete")

	excluded := map[authzClass][]authzPrincipal{
		authzPublic:   nil,
		authzShared:   {f.anon},
		authzCustomer: {f.anon, f.agent, f.agentTok},
		authzAgent:    {f.anon, f.customer, f.custToken},
		authzAdmin:    {f.anon, f.customer, f.custToken, f.agent, f.agentTok},
	}

	counts := map[authzClass]int{}
	served := map[string]bool{}
	for _, rt := range routes {
		served[rt.Method+" "+rt.Path] = true
		class := classifyRoute(rt.Method, rt.Path)
		counts[class]++
		for _, p := range excluded[class] {
			w := f.call(rt.Method, rt.Path, p)
			if !denied(w) {
				t.Errorf("%s %s (%s): %s got %d, want 401/403/login redirect; body=%.200s",
					rt.Method, rt.Path, class, p.name, w.Code, w.Body.String())
			}
		}
	}
	// A stale allowlist entry would silently open a route re-added later.
	for _, list := range []map[string]bool{authzPublicRoutes, authzSharedRoutes} {
		for key := range list {
			if !served[key] {
				t.Errorf("allowlisted route %q is not served any more: remove it from the list", key)
			}
		}
	}
	t.Logf("route classes: %s", fmt.Sprint(counts))
}
