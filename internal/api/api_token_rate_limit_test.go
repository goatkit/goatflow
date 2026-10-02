package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/middleware"
	"github.com/goatkit/goatflow/internal/platform/shared"
	"github.com/goatkit/goatflow/internal/service"
)

// A gf_* token's user_api_tokens.rate_limit is enforced on every route that
// accepts API tokens (unified_auth).
func TestAPITokenRateLimitEnforcedOnUnifiedAuth(t *testing.T) {
	WithCleanDB(t)
	db, err := database.GetDB()
	require.NoError(t, err)

	svc := service.NewAPITokenService(db)
	middleware.SetAPITokenVerifier(svc)
	t.Cleanup(func() { middleware.SetAPITokenVerifier(nil) })

	ctx := context.Background()
	limited, err := svc.GenerateToken(ctx, &models.APITokenCreateRequest{Name: "rate-limited", Scopes: []string{"tickets:read"}}, 1, models.APITokenUserAgent, 1)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM user_api_tokens WHERE id = ?"), limited.ID)
	})
	_, err = db.Exec(database.ConvertPlaceholders("UPDATE user_api_tokens SET rate_limit = ? WHERE id = ?"), 2, limited.ID)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/probe", middleware.UnifiedAuthMiddleware(shared.GetJWTManager()), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})
	call := func(token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/probe", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	for i := range 2 {
		w := call(limited.Token)
		require.Equal(t, http.StatusOK, w.Code, "request %d: %s", i+1, w.Body.String())
		require.Equal(t, "2", w.Header().Get("X-RateLimit-Limit"))
	}
	w := call(limited.Token)
	require.Equal(t, http.StatusTooManyRequests, w.Code, w.Body.String())
	require.Equal(t, "1800", w.Header().Get("Retry-After"))
}
