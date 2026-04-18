package middleware

import (
	"github.com/gin-gonic/gin"
)

// SecurityHeaders adds HTTP security headers to all responses.
//
// CSP Note: GoatFlow uses Alpine.js (requires unsafe-eval for x-data/x-bind
// expression evaluation) and HTMX (injects inline scripts from AJAX responses).
// These frameworks are fundamentally incompatible with strict script-src CSP.
// XSS protection is provided by server-side HTML sanitisation (bluemonday),
// not by CSP script restrictions. CSP still protects against clickjacking,
// content sniffing, form hijacking, and framing attacks.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline' 'unsafe-eval'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: blob: *.giphy.com *.tenor.com media.tenor.com; "+
				"media-src 'self' blob:; "+
				"font-src 'self' data:; "+
				"connect-src 'self'; "+
				"frame-ancestors 'none'; "+
				"base-uri 'self'; "+
				"form-action 'self'")

		c.Header("X-Frame-Options", "DENY")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "interest-cohort=()")

		c.Next()
	}
}
