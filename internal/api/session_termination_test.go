package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/shared"
)

// sessionRowCount returns the number of sessions rows (one per session) for the
// account; the table is a key/value store, so count distinct ids.
func sessionRowCount(t *testing.T, userID int64) int {
	t.Helper()
	db, err := database.GetDB()
	require.NoError(t, err)
	var n int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		`SELECT COUNT(DISTINCT session_id) FROM sessions WHERE data_key = ? AND data_value = ?`),
		models.SessionKeyUserID, fmt.Sprint(userID)).Scan(&n))
	return n
}

// killSessionRow deletes the sessions row exactly as admin "Kill session" does.
func killSessionRow(t *testing.T, sessionID string) {
	t.Helper()
	db, err := database.GetDB()
	require.NoError(t, err)
	_, err = db.Exec(database.ConvertPlaceholders(`DELETE FROM sessions WHERE session_id = ?`), sessionID)
	require.NoError(t, err)
}

// Killing a session (admin "Kill session", logout) must revoke the access
// token everywhere, not only on UI routes and not only when the client keeps
// sending the session_id cookie (F5).
func TestSessionTerminationRevokesTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.GetDB()
	require.NoError(t, err)
	router := NewSimpleRouter()
	jwtManager := shared.GetJWTManager()

	sfx := fmt.Sprintf("%d", time.Now().UnixNano())
	now := time.Now().UTC().Truncate(time.Second)
	password := "Kill-Test-" + sfx
	hash, err := auth.NewPasswordHasher().HashPassword(password)
	require.NoError(t, err)

	agentLogin, customerLogin := "kill_agent_"+sfx, "kill_cust_"+sfx
	agentID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'Kill', 'Agent', 1, ?, 1, ?, 1) RETURNING id`), agentLogin, hash, now, now)
	require.NoError(t, err)
	customerID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO customer_user (login, email, customer_id, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'kill-co', ?, 'Kill', 'Customer', 1, ?, 1, ?, 1) RETURNING id`),
		customerLogin, customerLogin+"@example.test", hash, now, now)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM users WHERE id = ?"), agentID)
		_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM customer_user WHERE id = ?"), customerID)
		_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM sessions WHERE data_key = ? AND data_value IN (?, ?)"),
			models.SessionKeyUserLogin, agentLogin, customerLogin)
	})

	// send performs a request with the given bearer token and cookies.
	send := func(method, path, token string, cookies []*http.Cookie, accept string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("Accept", accept)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		for _, c := range cookies {
			req.AddCookie(c)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	postJSON := func(path string, body interface{}) *httptest.ResponseRecorder {
		return searchRequest(t, router, http.MethodPost, path, "", body)
	}
	// webLogin signs the agent in through the browser login and returns the
	// access token and the cookies the browser would keep.
	webLogin := func(t *testing.T) (string, []*http.Cookie) {
		t.Helper()
		w := postJSON("/api/auth/login", gin.H{"login": agentLogin, "password": password})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var body struct {
			AccessToken string `json:"access_token"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		require.NotEmpty(t, body.AccessToken)
		return body.AccessToken, w.Result().Cookies()
	}
	sidOf := func(t *testing.T, token string) string {
		t.Helper()
		claims, err := jwtManager.ValidateToken(token)
		require.NoError(t, err)
		require.NotEmpty(t, claims.SessionID, "access token names its session")
		return claims.SessionID
	}
	cookiesOnly := func(cookies []*http.Cookie, names ...string) []*http.Cookie {
		var out []*http.Cookie
		for _, c := range cookies {
			for _, n := range names {
				if c.Name == n && c.Value != "" {
					out = append(out, c)
				}
			}
		}
		return out
	}

	t.Run("web login: killed session is refused on the API with bearer, with cookies, and on the UI", func(t *testing.T) {
		token, cookies := webLogin(t)
		sid := sidOf(t, token)
		require.NotEmpty(t, cookiesOnly(cookies, "session_id"), "login sets the session_id cookie")

		// (f) a live session works on every surface
		assert.Equal(t, http.StatusOK, send(http.MethodGet, "/api/v1/users/me", token, nil, "application/json").Code)
		assert.Equal(t, http.StatusOK, send(http.MethodGet, "/api/v1/users/me", "", cookiesOnly(cookies, "auth_token"), "application/json").Code)
		assert.Equal(t, http.StatusOK, send(http.MethodGet, "/api/v1/users/me", "", cookiesOnly(cookies, "auth_token", "session_id"), "application/json").Code)

		killSessionRow(t, sid)

		// (a) bearer token only
		w := send(http.MethodGet, "/api/v1/users/me", token, nil, "application/json")
		assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
		// (b) cookies only, with and without session_id
		w = send(http.MethodGet, "/api/v1/users/me", "", cookiesOnly(cookies, "auth_token"), "application/json")
		assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
		w = send(http.MethodGet, "/api/v1/users/me", "", cookiesOnly(cookies, "auth_token", "session_id"), "application/json")
		assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
		// UI route, auth_token cookie only (no session_id): redirected to login
		w = send(http.MethodGet, "/dashboard", "", cookiesOnly(cookies, "auth_token"), "text/html")
		assert.Equal(t, http.StatusSeeOther, w.Code, w.Body.String())
		assert.Equal(t, "/login", w.Header().Get("Location"))
	})

	t.Run("logout kills the session named by the token even without a session_id cookie", func(t *testing.T) {
		token, cookies := webLogin(t)
		w := send(http.MethodPost, "/api/auth/logout", "", cookiesOnly(cookies, "auth_token"), "text/html")
		assert.Equal(t, http.StatusFound, w.Code)
		w = send(http.MethodGet, "/api/v1/users/me", token, nil, "application/json")
		assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
	})

	t.Run("customer login: killed session is refused on the portal and the customer API", func(t *testing.T) {
		w := postJSON("/customer/login", gin.H{"login": customerLogin, "password": password})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var body struct {
			AccessToken string `json:"access_token"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		token := body.AccessToken
		sid := sidOf(t, token)
		cookies := cookiesOnly(w.Result().Cookies(), "customer_auth_token")
		require.NotEmpty(t, cookiesOnly(w.Result().Cookies(), "customer_session_id"), "customer login sets its session cookie")
		assert.Equal(t, 1, sessionRowCount(t, customerID), "customer login creates a session row")

		// (f) live
		assert.Equal(t, http.StatusOK, send(http.MethodGet, "/customer/tickets", "", cookies, "text/html").Code)
		assert.Equal(t, http.StatusOK, send(http.MethodGet, "/api/v1/tickets", token, nil, "application/json").Code)

		killSessionRow(t, sid)

		// (c) portal page redirects to login, API refuses, with cookies and with bearer
		w = send(http.MethodGet, "/customer/tickets", "", cookies, "text/html")
		assert.Equal(t, http.StatusFound, w.Code, w.Body.String())
		assert.Equal(t, "/customer/login", w.Header().Get("Location"))
		w = send(http.MethodGet, "/api/v1/tickets", token, nil, "application/json")
		assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
		w = send(http.MethodGet, "/api/v1/tickets", "", cookies, "application/json")
		assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
		// the login page no longer bounces the dead token to the landing page
		w = send(http.MethodGet, "/customer/login", "", cookies, "text/html")
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("API login creates a session row and refresh stops once it is killed", func(t *testing.T) {
		before := sessionRowCount(t, agentID)
		w := postJSON("/api/v1/auth/login", gin.H{"login": agentLogin, "password": password})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		pair := decodeTokenPair(t, w.Body.Bytes())
		sid := sidOf(t, pair.AccessToken)
		// (d) the API login is listed and killable like a web login
		assert.Equal(t, before+1, sessionRowCount(t, agentID))

		// refresh keeps the session: the new access token names the same row
		w = postJSON("/api/v1/auth/refresh", gin.H{"refresh_token": pair.RefreshToken})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		next := decodeTokenPair(t, w.Body.Bytes())
		assert.Equal(t, sid, sidOf(t, next.AccessToken))
		assert.Equal(t, before+1, sessionRowCount(t, agentID), "refresh does not create a second session")

		killSessionRow(t, sid)

		// (e) refresh after kill
		w = postJSON("/api/v1/auth/refresh", gin.H{"refresh_token": next.RefreshToken})
		assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
		w = send(http.MethodGet, "/api/v1/users/me", next.AccessToken, nil, "application/json")
		assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
	})

	t.Run("SSO login creates a session row and a token bound to it", func(t *testing.T) {
		before := sessionRowCount(t, agentID)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/auth/oidc/callback", nil)
		createSession(c, &models.User{ID: uint(agentID), Login: agentLogin, Email: agentLogin, Role: "Agent"}) // #nosec G115 -- test id

		assert.Equal(t, http.StatusFound, w.Code)
		assert.Equal(t, "/dashboard", w.Header().Get("Location"))
		assert.Equal(t, before+1, sessionRowCount(t, agentID))
		cookies := w.Result().Cookies()
		authCookie := cookiesOnly(cookies, "auth_token")
		require.Len(t, authCookie, 1)
		sid := sidOf(t, authCookie[0].Value)
		sessionCookie := cookiesOnly(cookies, "session_id")
		require.Len(t, sessionCookie, 1)
		assert.Equal(t, sid, sessionCookie[0].Value)

		assert.Equal(t, http.StatusOK, send(http.MethodGet, "/api/v1/users/me", authCookie[0].Value, nil, "application/json").Code)
		killSessionRow(t, sid)
		assert.Equal(t, http.StatusUnauthorized, send(http.MethodGet, "/api/v1/users/me", authCookie[0].Value, nil, "application/json").Code)
	})
}
