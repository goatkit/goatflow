package middleware

import (
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/apierrors"
	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// RateLimiter implements a token bucket rate limiter
type RateLimiter struct {
	mu      sync.RWMutex
	buckets map[string]*bucket
	cleanup time.Duration
}

type bucket struct {
	tokens    float64
	limit     float64   // max tokens (requests per window)
	refillRate float64  // tokens per second
	lastRefill time.Time
}

// Global rate limiter instance
var globalRateLimiter = NewRateLimiter()

// NewRateLimiter creates a new rate limiter
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		buckets: make(map[string]*bucket),
		cleanup: 10 * time.Minute,
	}
	go rl.cleanupLoop()
	return rl
}

// Allow checks if a request is allowed and consumes a token
func (rl *RateLimiter) Allow(key string, limit int) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, exists := rl.buckets[key]
	if !exists {
		// Create new bucket with full tokens
		// Default: limit requests per hour, refill at limit/3600 per second
		b = &bucket{
			tokens:     float64(limit),
			limit:      float64(limit),
			refillRate: float64(limit) / 3600.0, // per hour
			lastRefill: time.Now(),
		}
		rl.buckets[key] = b
	}

	// Refill tokens based on time elapsed
	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * b.refillRate
	if b.tokens > b.limit {
		b.tokens = b.limit
	}
	b.lastRefill = now

	// Check if we can consume a token
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// Remaining returns remaining tokens for a key
func (rl *RateLimiter) Remaining(key string) int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	if b, exists := rl.buckets[key]; exists {
		return int(b.tokens)
	}
	return 0
}

// cleanupLoop removes stale buckets periodically
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanup)
	for range ticker.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-rl.cleanup)
		for key, b := range rl.buckets {
			if b.lastRefill.Before(cutoff) {
				delete(rl.buckets, key)
			}
		}
		rl.mu.Unlock()
	}
}

// RateLimitMiddleware applies rate limiting based on API token or IP
func RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var key string
		var limit int

		// Check if using API token (has custom rate limit)
		if apiToken, exists := c.Get("api_token"); exists {
			token := apiToken.(*models.APIToken)
			key = "token:" + token.Prefix
			limit = token.RateLimit
			if limit <= 0 {
				limit = models.DefaultRateLimit
			}
		} else {
			// Fall back to IP-based limiting
			key = "ip:" + c.ClientIP()
			limit = models.DefaultRateLimit
		}

		if !globalRateLimiter.Allow(key, limit) {
			remaining := globalRateLimiter.Remaining(key)
			c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
			c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
			c.Header("Retry-After", "60")
			apierrors.Error(c, apierrors.CodeRateLimited)
			c.Abort()
			return
		}

		// Add rate limit headers
		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(globalRateLimiter.Remaining(key)))

		c.Next()
	}
}

// RateLimitByIP applies IP-based rate limiting with a custom limit
func RateLimitByIP(requestsPerHour int) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := "ip:" + c.ClientIP()

		if !globalRateLimiter.Allow(key, requestsPerHour) {
			c.Header("Retry-After", "60")
			apierrors.Error(c, apierrors.CodeRateLimited)
			c.Abort()
			return
		}

		c.Next()
	}
}
