// Package middleware provides HTTP middleware for GoatFlow.
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/goatkit/goatflow/internal/platform/config"
)

// demoModeEnabled reports whether app.demo_mode is on, returning false if
// config has not been loaded yet (e.g. in test harnesses that wire handlers
// without initialising config).
func demoModeEnabled() bool {
	cfg := config.Get()
	return cfg != nil && cfg.App.DemoMode
}

// DemoMode sets is_demo=true on every request when app.demo_mode is enabled.
// This allows templates and handlers to check for demo mode globally.
func DemoMode() gin.HandlerFunc {
	return func(c *gin.Context) {
		if demoModeEnabled() {
			c.Set("is_demo", true)
		}
		c.Next()
	}
}

// DemoGuard blocks non-admin users from modifying account security settings
// (password, MFA) when demo mode is active. Returns 403 with a friendly message.
func DemoGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !demoModeEnabled() {
			c.Next()
			return
		}

		// Admins can always make changes
		if isAdmin(c) {
			c.Next()
			return
		}

		blockDemoChange(c)
	}
}

// blockDemoChange answers a blocked request: JSON clients get a 403 with a
// friendly message, browsers go back to the page they came from (the site
// root when there is no Referer, so a guarded page never redirects to
// itself).
func blockDemoChange(c *gin.Context) {
	if wantsJSON(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "This action is disabled in demo mode",
			"message": "Password and MFA changes are not available on the demo instance. Feel free to explore everything else!",
		})
	} else {
		back := c.Request.Referer()
		if back == "" {
			back = "/"
		}
		c.Redirect(http.StatusSeeOther, back)
	}
	c.Abort()
}

// isAdmin reports whether the auth middleware marked the user as a member of
// the admin group (group_user) or gave them the Admin role.
func isAdmin(c *gin.Context) bool {
	if v, exists := c.Get("isInAdminGroup"); exists {
		if b, ok := v.(bool); ok && b {
			return true
		}
	}
	if role, exists := c.Get("user_role"); exists {
		if r, ok := role.(string); ok && strings.EqualFold(r, "admin") {
			return true
		}
	}
	return false
}

// wantsJSON returns true if the request expects a JSON response.
func wantsJSON(c *gin.Context) bool {
	return strings.HasPrefix(c.GetHeader("Accept"), "application/json") ||
		c.GetHeader("X-Requested-With") == "XMLHttpRequest" ||
		c.ContentType() == "application/json"
}
