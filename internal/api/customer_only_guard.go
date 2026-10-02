package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// customerOnlyExact are single public paths a customer-only instance serves.
// /login stays reachable because handleLoginPage redirects it to the customer
// login there; /health is the liveness probe (not /health/detailed).
var customerOnlyExact = map[string]struct{}{
	"/":              {},
	"/favicon.ico":   {},
	"/login":         {},
	"/auth/customer": {},
	"/health":        {},
	"/healthz":       {},
	"/customer":      {},
	"/api/languages": {},
	"/api/themes":    {},
}

// customerOnlyTrees are path trees (matched as "<tree>/...") a customer-only
// instance serves. Each entry ends in "/" so "/customers" never matches
// "/customer" and "/api/auth/login" never matches "/api/auth/customer/".
var customerOnlyTrees = []string{
	"/customer/",
	"/api/auth/customer/",
	"/api/languages/",
	"/api/themes/",
	"/static/",
	"/assets/",
	"/runtime/",
}

// customerOnlyAllowed reports whether path may be served on a CUSTOMER_FE_ONLY
// instance. Agent login, agent 2FA, agent passkey and admin routes are not.
func customerOnlyAllowed(path string) bool {
	if _, ok := customerOnlyExact[path]; ok {
		return true
	}
	for _, p := range customerOnlyTrees {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// CustomerOnlyGuard blocks non-customer paths when enabled, to keep admin/auth UIs off the customer FE.
func CustomerOnlyGuard(enabled bool) gin.HandlerFunc {
	if !enabled {
		return func(c *gin.Context) {}
	}
	return func(c *gin.Context) {
		if customerOnlyAllowed(c.Request.URL.Path) {
			c.Next()
			return
		}
		c.AbortWithStatus(http.StatusNotFound)
	}
}
