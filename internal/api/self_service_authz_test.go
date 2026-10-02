package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/middleware"
	"github.com/goatkit/goatflow/internal/platform/shared"
	"github.com/goatkit/goatflow/internal/service"
)

// selfAuthzAgent inserts a valid agent and returns its id and a JWT for it.
func selfAuthzAgent(t *testing.T, db *sql.DB, login string, isAdmin bool) (int, string) {
	t.Helper()
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 'x', 'Self', 'Authz', 1, NOW(), 1, NOW(), 1) RETURNING id`), login)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM user_api_tokens WHERE user_id = ? AND user_type = 'agent'`), id)
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM gk_push_subscription WHERE user_id = ?`), id)
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM users WHERE id = ?`), id)
	})
	role := "Agent"
	if isAdmin {
		role = "Admin"
	}
	tok, err := shared.GetJWTManager().GenerateTokenWithLogin(uint(id), login, login, role, isAdmin, 0)
	require.NoError(t, err)
	return int(id), tok
}

func selfAuthzDo(t *testing.T, r *gin.Engine, method, url, bearer string, body any) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, url, &buf)
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	out := map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func selfAuthzTokenCount(t *testing.T, db *sql.DB, userID int, name string) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		`SELECT COUNT(*) FROM user_api_tokens WHERE user_id = ? AND user_type = 'agent' AND name = ?`),
		userID, name).Scan(&n))
	return n
}

const selfAuthzTokensURL = "/api/v1/tokens"

// Token creation must not hand out more than the caller holds: admin scopes
// need an admin, and a scoped API token cannot mint a broader one.
func TestTokenCreateScopeCeiling(t *testing.T) {
	db, err := database.GetDB()
	require.NoError(t, err)
	svc := service.NewAPITokenService(db)
	SetAPITokenService(svc)
	middleware.SetAPITokenVerifier(svc)
	t.Cleanup(func() { SetAPITokenService(nil) })
	r := newCustAttRouter(t)

	sfx := strconv.FormatInt(time.Now().UnixNano(), 36)
	agentID, agentJWT := selfAuthzAgent(t, db, "selfauthz-agent-"+sfx, false)
	adminID, adminJWT := selfAuthzAgent(t, db, "selfauthz-admin-"+sfx, true)

	for _, scopes := range [][]string{{"admin:*"}, {"tickets:read", "admin:*"}} {
		name := fmt.Sprintf("agent-admin-%v", scopes)
		status, body := selfAuthzDo(t, r, http.MethodPost, selfAuthzTokensURL, agentJWT,
			map[string]any{"name": name, "scopes": scopes})
		assert.Equal(t, http.StatusForbidden, status, "%v", body)
		assert.Equal(t, 0, selfAuthzTokenCount(t, db, agentID, name), "no token stored for %v", scopes)
	}

	status, body := selfAuthzDo(t, r, http.MethodPost, selfAuthzTokensURL, adminJWT,
		map[string]any{"name": "admin-admin", "scopes": []string{"admin:*"}})
	require.Equal(t, http.StatusCreated, status, "%v", body)
	assert.Equal(t, 1, selfAuthzTokenCount(t, db, adminID, "admin-admin"))

	// A tickets:read token of the agent.
	status, body = selfAuthzDo(t, r, http.MethodPost, selfAuthzTokensURL, agentJWT,
		map[string]any{"name": "narrow", "scopes": []string{"tickets:read"}})
	require.Equal(t, http.StatusCreated, status, "%v", body)
	narrow, _ := body["token"].(string)
	require.NotEmpty(t, narrow)

	for name, scopes := range map[string][]string{
		"via-narrow-write": {"tickets:write"},
		"via-narrow-full":  {},
	} {
		status, body = selfAuthzDo(t, r, http.MethodPost, selfAuthzTokensURL, narrow,
			map[string]any{"name": name, "scopes": scopes})
		assert.Equal(t, http.StatusForbidden, status, "%s: %v", name, body)
		assert.Equal(t, 0, selfAuthzTokenCount(t, db, agentID, name), name)
	}
	status, body = selfAuthzDo(t, r, http.MethodPost, selfAuthzTokensURL, narrow,
		map[string]any{"name": "via-narrow-same", "scopes": []string{"tickets:read"}})
	require.Equal(t, http.StatusCreated, status, "%v", body)
	assert.Equal(t, 1, selfAuthzTokenCount(t, db, agentID, "via-narrow-same"))
}

// An agent can only remove its own push subscription, even when it knows
// another agent's endpoint.
func TestPushUnsubscribeOwnOnly(t *testing.T) {
	db, err := database.GetDB()
	require.NoError(t, err)
	r := newCustAttRouter(t)

	sfx := strconv.FormatInt(time.Now().UnixNano(), 36)
	aID, aJWT := selfAuthzAgent(t, db, "selfauthz-push-a-"+sfx, false)
	_, bJWT := selfAuthzAgent(t, db, "selfauthz-push-b-"+sfx, false)
	endpoint := "https://push.example.test/" + sfx

	status, body := selfAuthzDo(t, r, http.MethodPost, "/api/push/subscribe", aJWT, map[string]any{
		"endpoint": endpoint, "keys": map[string]string{"p256dh": "p", "auth": "a"},
	})
	require.Equal(t, http.StatusOK, status, "%v", body)

	owner := func() (int, string) {
		var uid int
		var utype string
		err := db.QueryRow(database.ConvertPlaceholders(
			`SELECT user_id, user_type FROM gk_push_subscription WHERE endpoint = ?`), endpoint).Scan(&uid, &utype)
		if err == sql.ErrNoRows {
			return 0, ""
		}
		require.NoError(t, err)
		return uid, utype
	}
	uid, utype := owner()
	require.Equal(t, aID, uid)
	require.Equal(t, string(models.APITokenUserAgent), utype)

	status, _ = selfAuthzDo(t, r, http.MethodDelete, "/api/push/unsubscribe", bJWT, map[string]any{"endpoint": endpoint})
	require.Equal(t, http.StatusOK, status)
	uid, _ = owner()
	assert.Equal(t, aID, uid, "agent B must not remove agent A's subscription")

	status, _ = selfAuthzDo(t, r, http.MethodDelete, "/api/push/unsubscribe", aJWT, map[string]any{"endpoint": endpoint})
	require.Equal(t, http.StatusOK, status)
	uid, _ = owner()
	assert.Equal(t, 0, uid, "owner removes own subscription")
}

func selfAuthzOrg(t *testing.T, db *sql.DB, slug, companyID string) int64 {
	t.Helper()
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO gk_organisation (name, slug, status, customer_company_id, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'active', ?, 1, NOW(), 1, NOW(), 1) RETURNING id`), slug, slug, companyID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM gk_organisation WHERE id = ?`), id)
	})
	return id
}

// The active_org_id cookie is client-controlled: a caller that forges another
// organisation's id must not act in that organisation.
func TestActiveOrgCookieRequiresMembership(t *testing.T) {
	db, err := database.GetDB()
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)

	sfx := strconv.FormatInt(time.Now().UnixNano(), 36)
	orgA := selfAuthzOrg(t, db, "selfauthz-a-"+sfx, "selfauthz-coa-"+sfx)
	orgB := selfAuthzOrg(t, db, "selfauthz-b-"+sfx, "selfauthz-cob-"+sfx)

	// Agent A is a member of org A only.
	agentID, _ := selfAuthzAgent(t, db, "selfauthz-org-agent-"+sfx, false)
	_, err = db.Exec(database.ConvertPlaceholders(`
		INSERT INTO gk_user_organisation (org_id, user_id, role, is_default, create_time, create_by)
		VALUES (?, ?, 'member', TRUE, NOW(), 1)`), orgA, agentID)
	require.NoError(t, err)

	// Customer A belongs to org A through its company; org B grants plugin
	// "selfauthz" to group 1, which customer A is also in.
	custLogin := "selfauthz-cust-" + sfx
	_, err = database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO customer_user (login, email, customer_id, first_name, last_name, valid_id,
			create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, 'Self', 'Authz', 1, NOW(), 1, NOW(), 1) RETURNING id`), custLogin, custLogin, "selfauthz-coa-"+sfx)
	require.NoError(t, err)
	_, err = db.Exec(database.ConvertPlaceholders(`
		INSERT INTO group_customer_user (user_id, group_id, permission_key, permission_value, create_time, create_by, change_time, change_by)
		VALUES (?, 1, 'rw', 1, NOW(), 1, NOW(), 1)`), custLogin)
	require.NoError(t, err)
	_, err = db.Exec(database.ConvertPlaceholders(`
		INSERT INTO gk_org_plugin_access (org_id, plugin_name, group_id, create_time, create_by)
		VALUES (?, 'selfauthz', 1, NOW(), 1)`), orgB)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM group_customer_user WHERE user_id = ?`), custLogin)
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM customer_user WHERE login = ?`), custLogin)
	})

	ctxWithCookie := func(orgID int64, set func(c *gin.Context)) *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		c.Request.AddCookie(&http.Cookie{Name: "active_org_id", Value: strconv.FormatInt(orgID, 10)})
		set(c)
		return c
	}
	asAgent := func(c *gin.Context) { c.Set("user_id", agentID); c.Set("user_role", "Agent") }
	asCustomer := func(c *gin.Context) { c.Set("user_role", "Customer"); c.Set("username", custLogin) }

	assert.Equal(t, orgA, orgIDFromContext(ctxWithCookie(orgA, asAgent)), "member org is honoured")
	assert.Equal(t, int64(0), orgIDFromContext(ctxWithCookie(orgB, asAgent)), "foreign org cookie ignored for agent")
	assert.Equal(t, orgA, orgIDFromContext(ctxWithCookie(orgA, asCustomer)), "company org is honoured")
	assert.Equal(t, int64(0), orgIDFromContext(ctxWithCookie(orgB, asCustomer)), "foreign org cookie ignored for customer")
	assert.False(t, HasPluginAccess(ctxWithCookie(orgB, asCustomer), "selfauthz"),
		"customer must not gain org B's plugin by forging its cookie")
}
