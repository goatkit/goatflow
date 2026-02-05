package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/gotrs-io/gotrs-ce/internal/models"
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
// MIDDLEWARE INTEGRATION TESTS
// =============================================================================

func setupTestRouter() *gin.Engine {
	router := gin.New()
	router.Use(RateLimitMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})
	return router
}

func TestRateLimitMiddleware_AddsHeaders(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Header().Get("X-RateLimit-Limit"), "should have X-RateLimit-Limit header")
	assert.NotEmpty(t, w.Header().Get("X-RateLimit-Remaining"), "should have X-RateLimit-Remaining header")
}

func TestRateLimitMiddleware_BlocksAfterLimitExceeded(t *testing.T) {
	// Use a fresh rate limiter for this test
	oldLimiter := globalRateLimiter
	globalRateLimiter = NewRateLimiter()
	defer func() { globalRateLimiter = oldLimiter }()

	router := gin.New()
	router.Use(RateLimitByIP(5)) // Very low limit for testing
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	// Make 5 requests (should all succeed)
	for i := 0; i < 5; i++ {
		req, _ := http.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.1:12345"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "request %d should succeed", i+1)
	}

	// 6th request should be rate limited
	req, _ := http.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code, "should return 429 when rate limited")
	assert.NotEmpty(t, w.Header().Get("Retry-After"), "should have Retry-After header")
}

func TestRateLimitMiddleware_DifferentIPsHaveSeparateLimits(t *testing.T) {
	oldLimiter := globalRateLimiter
	globalRateLimiter = NewRateLimiter()
	defer func() { globalRateLimiter = oldLimiter }()

	router := gin.New()
	router.Use(RateLimitByIP(2))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	// Exhaust IP1's limit
	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.1:12345"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}

	// IP1 should be blocked
	req1, _ := http.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "10.0.0.1:12345"
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusTooManyRequests, w1.Code, "IP1 should be rate limited")

	// IP2 should still work
	req2, _ := http.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "10.0.0.2:12345"
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code, "IP2 should not be rate limited")
}

func TestRateLimitMiddleware_UsesAPITokenLimit(t *testing.T) {
	oldLimiter := globalRateLimiter
	globalRateLimiter = NewRateLimiter()
	defer func() { globalRateLimiter = oldLimiter }()

	router := gin.New()
	// Inject a mock API token before rate limit middleware
	router.Use(func(c *gin.Context) {
		c.Set("api_token", &models.APIToken{
			Prefix:    "test_token_123",
			RateLimit: 3, // Custom low limit
		})
		c.Next()
	})
	router.Use(RateLimitMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	// Make 3 requests (should all succeed)
	for i := 0; i < 3; i++ {
		req, _ := http.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "request %d should succeed", i+1)
	}

	// 4th request should be rate limited
	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code, "should be rate limited after token limit exceeded")
}

func TestRateLimitMiddleware_DefaultsToDefaultRateLimitWhenTokenHasZero(t *testing.T) {
	oldLimiter := globalRateLimiter
	globalRateLimiter = NewRateLimiter()
	defer func() { globalRateLimiter = oldLimiter }()

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("api_token", &models.APIToken{
			Prefix:    "test_zero_limit",
			RateLimit: 0, // Zero should default to DefaultRateLimit
		})
		c.Next()
	})
	router.Use(RateLimitMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Should use DefaultRateLimit (1000), not 0
	limitHeader := w.Header().Get("X-RateLimit-Limit")
	assert.Equal(t, "1000", limitHeader, "should use default rate limit when token has 0")
}
