package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/auth"
)

// twoFactorLimiterKey is the DefaultLoginRateLimiter key for second-factor code
// checks. It is separate from the password key so a correct password (which
// clears the password counter) never resets the code-guessing counter.
func twoFactorLimiterKey(isCustomer bool, login string) string {
	if isCustomer {
		return "2fa:customer:" + login
	}
	return "2fa:agent:" + login
}

// tooManyAttemptsJSON writes the 429 body shared by every login endpoint.
func tooManyAttemptsJSON(c *gin.Context, remaining time.Duration) {
	secs := int(remaining.Seconds())
	if secs < 1 {
		secs = 1
	}
	c.Header("Retry-After", fmt.Sprintf("%d", secs))
	c.JSON(http.StatusTooManyRequests, gin.H{
		"success":         false,
		"error":           fmt.Sprintf("too many failed attempts, try again in %d seconds", secs),
		"retry_after_sec": secs,
	})
}

// rejectIfLoginBlocked answers 429 and returns true when ip+key is locked out
// by auth.DefaultLoginRateLimiter.
func rejectIfLoginBlocked(c *gin.Context, key string) bool {
	blocked, remaining := auth.DefaultLoginRateLimiter.IsBlocked(c.ClientIP(), key)
	if blocked {
		tooManyAttemptsJSON(c, remaining)
	}
	return blocked
}

// passwordRecheckKey is the DefaultLoginRateLimiter key for password
// re-checks by a signed-in account (password change, 2FA setup/disable,
// recovery codes, passkeys). The limiter counts it per client IP, so a stolen
// session cannot be used to guess the account password at full speed.
func passwordRecheckKey(account mfaAccount) string {
	return "recheck:" + account.userType + ":" + account.webAuthnKey()
}

// countPasswordRecheck records the outcome of a password re-check under key
// and returns ok. Call rejectIfLoginBlocked(c, key) before checking.
func countPasswordRecheck(c *gin.Context, key string, ok bool) bool {
	if ok {
		auth.DefaultLoginRateLimiter.RecordSuccess(c.ClientIP(), key)
	} else {
		auth.DefaultLoginRateLimiter.RecordFailure(c.ClientIP(), key)
	}
	return ok
}
