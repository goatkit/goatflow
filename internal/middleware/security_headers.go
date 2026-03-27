package middleware

import (
	"github.com/gin-gonic/gin"
)

// SecurityHeaders adds HTTP security headers to all responses.
// These provide browser-level defence against XSS, clickjacking,
// content sniffing, and other common web attacks.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Content-Security-Policy: block inline scripts and eval.
		// 'self' allows scripts/styles from same origin.
		// 'unsafe-inline' for styles only (needed for Tailwind/theme CSS variables).
		// script-src allows specific CDN-free vendored scripts but blocks inline <script> tags.
		c.Header("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: blob:; "+
				"font-src 'self' data:; "+
				"connect-src 'self'; "+
				"frame-ancestors 'none'; "+
				"base-uri 'self'; "+
				"form-action 'self'")

		// Prevent clickjacking — pages cannot be embedded in iframes.
		c.Header("X-Frame-Options", "DENY")

		// Prevent MIME type sniffing — browser must respect Content-Type.
		c.Header("X-Content-Type-Options", "nosniff")

		// Enable browser XSS filter (legacy, but harmless).
		c.Header("X-XSS-Protection", "1; mode=block")

		// Control referrer information sent with requests.
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Opt out of Google FLoC and Topics API tracking.
		c.Header("Permissions-Policy", "interest-cohort=()")

		c.Next()
	}
}
