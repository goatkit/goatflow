package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/auth"
	"github.com/goatkit/goatflow/internal/database"
)

func TestPasskeyLoginBeginSetsHttpOnlyPendingCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	database.SetDB(db)
	t.Cleanup(database.ResetDB)
	auth.DefaultLoginRateLimiter.RecordSuccess("203.0.113.21", passkeyLimiterKey("agent"))
	t.Cleanup(func() {
		auth.DefaultLoginRateLimiter.RecordSuccess("203.0.113.21", passkeyLimiterKey("agent"))
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "https://example.com/api/auth/passkey/begin", nil)
	req.RemoteAddr = "203.0.113.21:443"
	c.Request = req

	handlePasskeyLoginBegin(c)

	require.Equal(t, http.StatusOK, w.Code)
	cookie := findCookie(w.Result().Cookies(), "passkey_login_pending")
	require.NotNil(t, cookie)
	require.NotEmpty(t, cookie.Value)
	require.True(t, cookie.HttpOnly)
	require.Equal(t, "/", cookie.Path)
	require.Equal(t, 300, cookie.MaxAge)

	var body struct {
		Success bool                   `json:"success"`
		Options map[string]interface{} `json:"options"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Contains(t, body.Options, "publicKey")
}

func TestPasskeyLoginFinishRejectsMissingPendingCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth.DefaultLoginRateLimiter.RecordSuccess("203.0.113.22", passkeyLimiterKey("agent"))
	t.Cleanup(func() {
		auth.DefaultLoginRateLimiter.RecordSuccess("203.0.113.22", passkeyLimiterKey("agent"))
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "https://example.com/api/auth/passkey/finish", nil)
	req.RemoteAddr = "203.0.113.22:443"
	c.Request = req

	handlePasskeyLoginFinish(c)

	require.Equal(t, http.StatusUnauthorized, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, false, body["success"])
	require.Equal(t, "passkey login challenge expired", body["error"])
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}
