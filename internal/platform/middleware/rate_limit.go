package middleware

import (
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/apierrors"
	platformmodels "github.com/goatkit/goatflow/internal/platform/models"
)

// RateLimiter implements a token bucket rate limiter
type RateLimiter struct {
	mu      sync.RWMutex
	buckets map[string]*bucket
	cleanup time.Duration
}

type bucket struct {
	tokens     float64
	limit      float64 // max tokens (requests per window)
	refillRate float64 // tokens per second
	lastRefill time.Time
}

// Global rate limiter instance
var globalRateLimiter = NewRateLimiter()

// GlobalRateLimiter returns the shared rate limiter instance.
func GlobalRateLimiter() *RateLimiter {
	return globalRateLimiter
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		buckets: make(map[string]*bucket),
		cleanup: 10 * time.Minute,
	}
	go rl.cleanupLoop()
	return rl
}

// Allow checks if a request is allowed and consumes a token from a bucket of
// limit requests per hour.
func (rl *RateLimiter) Allow(key string, limit int) bool {
	return rl.AllowPer(key, limit, time.Hour)
}

// AllowPer checks if a request is allowed and consumes a token from a bucket
// of limit requests per window. A bucket whose limit or window changed (e.g.
// a reconfigured plugin UI) adopts the new values.
func (rl *RateLimiter) AllowPer(key string, limit int, window time.Duration) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	refillRate := float64(limit) / window.Seconds()
	b, exists := rl.buckets[key]
	if !exists {
		b = &bucket{
			tokens:     float64(limit),
			limit:      float64(limit),
			refillRate: refillRate,
			lastRefill: time.Now(),
		}
		rl.buckets[key] = b
	} else if b.limit != float64(limit) || b.refillRate != refillRate {
		b.limit = float64(limit)
		b.refillRate = refillRate
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

// apiTokenRateLimiter holds the per-token buckets. It is separate from the
// global limiter so other callers' keys can never collide with token keys.
var apiTokenRateLimiter = NewRateLimiter()

// allowAPITokenRequest enforces user_api_tokens.rate_limit (requests per hour,
// DefaultRateLimit when unset) for a request authenticated by apiToken. It
// sets the X-RateLimit-* headers and, when the budget is spent, answers 429
// with Retry-After, aborts the request and returns false.
func allowAPITokenRequest(c *gin.Context, apiToken *platformmodels.APIToken) bool {
	limit := apiToken.RateLimit
	if limit <= 0 {
		limit = platformmodels.DefaultRateLimit
	}
	key := "token:" + strconv.FormatInt(apiToken.ID, 10)
	allowed := apiTokenRateLimiter.Allow(key, limit)

	c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
	c.Header("X-RateLimit-Remaining", strconv.Itoa(apiTokenRateLimiter.Remaining(key)))
	if allowed {
		return true
	}
	// One token refills every hour/limit; round up to whole seconds.
	retryAfter := (3600 + limit - 1) / limit
	c.Header("Retry-After", strconv.Itoa(retryAfter))
	apierrors.Error(c, apierrors.CodeRateLimited)
	c.Abort()
	return false
}
