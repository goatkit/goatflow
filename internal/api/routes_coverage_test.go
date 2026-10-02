package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/shared"
)

// TestLogoutKillsSessionAndClearsAuthCookies drives both logout routes of
// routes/auth.yaml through the real router: GET /logout (the "Sign out" link in
// templates/layouts/base.pongo2) and POST /api/auth/logout. Each must delete
// the server-side session row, expire every agent auth cookie the login set,
// and redirect to the login page.
func TestLogoutKillsSessionAndClearsAuthCookies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := getTestDB(t)
	router := NewSimpleRouter()

	sessions := shared.GetSessionService()
	require.NotNil(t, sessions, "session service must be wired")
	cfg := GetTestAuthConfig()

	authCookies := []string{"access_token", "auth_token", "token", "refresh_token", "session_id", "goatflow_logged_in"}

	sessionRows := func(sessionID string) int {
		var n int
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
			"SELECT COUNT(*) FROM sessions WHERE session_id = ?"), sessionID).Scan(&n))
		return n
	}

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/logout"},
		{http.MethodPost, "/api/auth/logout"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			sessionID, err := sessions.CreateSession(int(cfg.UserID), cfg.Email, "User", "192.0.2.10", "logout-test")
			require.NoError(t, err)
			t.Cleanup(func() { _ = sessions.KillSession(sessionID) })
			require.Positive(t, sessionRows(sessionID), "session row must exist before logout")

			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.AddCookie(&http.Cookie{Name: "access_token", Value: GetTestAuthToken(t)})
			req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
			req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "refresh-token-from-login"})
			req.AddCookie(&http.Cookie{Name: "goatflow_logged_in", Value: "1"})
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusFound, w.Code, w.Body.String())
			assert.Equal(t, "/login", w.Header().Get("Location"))

			expired := map[string]bool{}
			for _, ck := range w.Result().Cookies() {
				if ck.Value == "" && ck.MaxAge < 0 {
					expired[ck.Name] = true
				}
			}
			for _, name := range authCookies {
				assert.True(t, expired[name], "cookie %q must be expired by logout", name)
			}

			assert.Zero(t, sessionRows(sessionID), "logout must delete the session row")
		})
	}
}

// TestStaticFilesServed: handleStaticFiles resolves ./static relative to the
// working directory (the repo root in the container), so run from there and
// check the files' bytes come back.
func TestStaticFilesServed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Chdir(filepath.Join("..", ".."))
	router := NewSimpleRouter()

	for path, file := range map[string]string{
		"/favicon.ico":        "static/favicon.ico",
		"/static/favicon.svg": "static/favicon.svg",
	} {
		t.Run(path, func(t *testing.T) {
			want, err := os.ReadFile(file)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, want, w.Body.Bytes())
		})
	}
}
