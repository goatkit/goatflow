package api

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// CustomerOnlyGuard blocks non-customer paths when enabled, to keep admin/auth UIs off the customer FE.
func CustomerOnlyGuard(enabled bool) gin.HandlerFunc {
	if !enabled {
		return func(c *gin.Context) {}
	}

	allowedPrefixes := []string{
		"/customer",
		"/auth/customer",
		"/login",
		"/api/auth",
		"/api/languages", // Public language selector API
		"/api/themes",    // Public theme selector API
		"/health",
		"/healthz",
		"/health/detailed",
		"/metrics",
		"/static",
		"/assets",
		"/runtime",
	}

	allowedExact := map[string]struct{}{
		"/":            {},
		"/favicon.ico": {},
	}

	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if _, ok := allowedExact[path]; ok {
			c.Next()
			return
		}
		for _, p := range allowedPrefixes {
			if strings.HasPrefix(path, p) {
				c.Next()
				return
			}
		}
		c.AbortWithStatus(404)
	}
}
