package auth

import (
	"sync"
	"time"
)

// LoginRateLimiter implements fail2ban-style rate limiting for login attempts.
// It tracks failed attempts by IP and username, applying exponential backoff.
type LoginRateLimiter struct {
	mu       sync.RWMutex
	attempts map[string]*attemptRecord

	// Configuration
	maxAttempts   int
	windowSeconds int
	baseBackoff   time.Duration
	maxBackoff    time.Duration
}

type attemptRecord struct {
	failures  int
	lastFail  time.Time
	blockedAt time.Time
}

// DefaultLoginRateLimiter is the global instance for login rate limiting.
var DefaultLoginRateLimiter = NewLoginRateLimiter(5, 300, 2*time.Second, 60*time.Second)

// maxBackoff: maximum backoff duration.
func NewLoginRateLimiter(maxAttempts, windowSeconds int, baseBackoff, maxBackoff time.Duration) *LoginRateLimiter {
	rl := &LoginRateLimiter{
		attempts:      make(map[string]*attemptRecord),
		maxAttempts:   maxAttempts,
		windowSeconds: windowSeconds,
		baseBackoff:   baseBackoff,
		maxBackoff:    maxBackoff,
	}
	go rl.cleanup()
	return rl
}

// key generates a composite key from IP and username.
func (rl *LoginRateLimiter) key(ip, username string) string {
	return ip + ":" + username
}

// IsBlocked checks if an IP+username combination is currently blocked.
func (rl *LoginRateLimiter) IsBlocked(ip, username string) (bool, time.Duration) {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	k := rl.key(ip, username)
	rec, exists := rl.attempts[k]
	if !exists {
		return false, 0
	}

	if rec.blockedAt.IsZero() {
		return false, 0
	}

	backoff := rl.calculateBackoff(rec.failures)
	unblockTime := rec.blockedAt.Add(backoff)

	if time.Now().After(unblockTime) {
		return false, 0
	}

	return true, time.Until(unblockTime)
}

// RecordFailure records a failed login attempt.
func (rl *LoginRateLimiter) RecordFailure(ip, username string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	k := rl.key(ip, username)
	rec, exists := rl.attempts[k]
	if !exists {
		rec = &attemptRecord{}
		rl.attempts[k] = rec
	}

	now := time.Now()
	window := time.Duration(rl.windowSeconds) * time.Second

	// Reset if outside window
	if !rec.lastFail.IsZero() && now.Sub(rec.lastFail) > window {
		rec.failures = 0
		rec.blockedAt = time.Time{}
	}

	rec.failures++
	rec.lastFail = now

	if rec.failures >= rl.maxAttempts {
		rec.blockedAt = now
	}
}

// RecordSuccess clears the failure record for successful login.
func (rl *LoginRateLimiter) RecordSuccess(ip, username string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	k := rl.key(ip, username)
	delete(rl.attempts, k)
}

// calculateBackoff returns exponential backoff duration.
func (rl *LoginRateLimiter) calculateBackoff(failures int) time.Duration {
	if failures <= rl.maxAttempts {
		return rl.baseBackoff
	}

	// Exponential backoff: base * 2^(failures - maxAttempts)
	multiplier := 1 << (failures - rl.maxAttempts)
	backoff := rl.baseBackoff * time.Duration(multiplier)

	if backoff > rl.maxBackoff {
		return rl.maxBackoff
	}
	return backoff
}

// cleanup periodically removes stale entries.
func (rl *LoginRateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		staleThreshold := time.Duration(rl.windowSeconds*2) * time.Second

		for k, rec := range rl.attempts {
			if now.Sub(rec.lastFail) > staleThreshold {
				delete(rl.attempts, k)
			}
		}
		rl.mu.Unlock()
	}
}

// Stats returns current rate limiter statistics (for monitoring).
func (rl *LoginRateLimiter) Stats() map[string]interface{} {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	blocked := 0
	for _, rec := range rl.attempts {
		if !rec.blockedAt.IsZero() {
			backoff := rl.calculateBackoff(rec.failures)
			if time.Now().Before(rec.blockedAt.Add(backoff)) {
				blocked++
			}
		}
	}

	return map[string]interface{}{
		"tracked_keys":  len(rl.attempts),
		"blocked_count": blocked,
		"max_attempts":  rl.maxAttempts,
		"window_sec":    rl.windowSeconds,
	}
}
