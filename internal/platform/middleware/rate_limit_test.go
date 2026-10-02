package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	platformmodels "github.com/goatkit/goatflow/internal/platform/models"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// =============================================================================
// RATE LIMITER CORE TESTS
// =============================================================================

func TestRateLimiter_AllowsRequestsWithinLimit(t *testing.T) {
	rl := NewRateLimiter()
	key := "test:within-limit"
	limit := 10

	// Should allow 'limit' requests
	for i := 0; i < limit; i++ {
		allowed := rl.Allow(key, limit)
		assert.True(t, allowed, "request %d should be allowed", i+1)
	}
}

func TestRateLimiter_BlocksRequestsOverLimit(t *testing.T) {
	rl := NewRateLimiter()
	key := "test:over-limit"
	limit := 5

	// Exhaust the limit
	for i := 0; i < limit; i++ {
		rl.Allow(key, limit)
	}

	// Next request should be blocked
	allowed := rl.Allow(key, limit)
	assert.False(t, allowed, "request over limit should be blocked")
}

func TestRateLimiter_DifferentKeysHaveSeparateLimits(t *testing.T) {
	rl := NewRateLimiter()
	limit := 3

	// Exhaust key1
	for i := 0; i < limit; i++ {
		rl.Allow("key1", limit)
	}

	// key1 should be blocked
	assert.False(t, rl.Allow("key1", limit), "key1 should be blocked")

	// key2 should still work
	assert.True(t, rl.Allow("key2", limit), "key2 should be allowed")
}

func TestRateLimiter_RemainingReturnsCorrectCount(t *testing.T) {
	rl := NewRateLimiter()
	key := "test:remaining"
	limit := 10

	// Use 3 tokens
	for i := 0; i < 3; i++ {
		rl.Allow(key, limit)
	}

	remaining := rl.Remaining(key)
	// Should have 7 remaining (10 - 3)
	assert.Equal(t, 7, remaining, "should have 7 tokens remaining")
}

func TestRateLimiter_RemainingReturnsZeroForUnknownKey(t *testing.T) {
	rl := NewRateLimiter()
	remaining := rl.Remaining("unknown:key")
	assert.Equal(t, 0, remaining, "unknown key should return 0 remaining")
}

// =============================================================================
// PER-TOKEN LIMIT (user_api_tokens.rate_limit)
// =============================================================================

func tokenLimitRouter(token *platformmodels.APIToken) *gin.Engine {
	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		if !allowAPITokenRequest(c, token) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	})
	return router
}

func serveTokenLimit(router *gin.Engine) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/test", nil))
	return w
}

func TestAllowAPITokenRequest_EnforcesTokenLimit(t *testing.T) {
	old := apiTokenRateLimiter
	apiTokenRateLimiter = NewRateLimiter()
	defer func() { apiTokenRateLimiter = old }()

	router := tokenLimitRouter(&platformmodels.APIToken{ID: 101, RateLimit: 3})
	for i := range 3 {
		w := serveTokenLimit(router)
		assert.Equal(t, http.StatusOK, w.Code, "request %d", i+1)
		assert.Equal(t, "3", w.Header().Get("X-RateLimit-Limit"))
		assert.Equal(t, strconv.Itoa(2-i), w.Header().Get("X-RateLimit-Remaining"))
	}

	w := serveTokenLimit(router)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "1200", w.Header().Get("Retry-After"), "3/hour refills one request every 1200s")
	assert.Equal(t, "0", w.Header().Get("X-RateLimit-Remaining"))

	other := tokenLimitRouter(&platformmodels.APIToken{ID: 102, RateLimit: 3})
	assert.Equal(t, http.StatusOK, serveTokenLimit(other).Code, "another token has its own budget")
}

func TestAllowAPITokenRequest_UnsetLimitUsesDefault(t *testing.T) {
	old := apiTokenRateLimiter
	apiTokenRateLimiter = NewRateLimiter()
	defer func() { apiTokenRateLimiter = old }()

	w := serveTokenLimit(tokenLimitRouter(&platformmodels.APIToken{ID: 201, RateLimit: 0}))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, strconv.Itoa(platformmodels.DefaultRateLimit), w.Header().Get("X-RateLimit-Limit"))
}
