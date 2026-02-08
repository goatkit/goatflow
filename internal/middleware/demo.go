// Package middleware provides HTTP middleware for GoatFlow.
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/goatkit/goatflow/internal/config"
)

// DemoMode sets is_demo=true on every request when app.demo_mode is enabled.
// This allows templates and handlers to check for demo mode globally.
func DemoMode() gin.HandlerFunc {
	return func(c *gin.Context) {
		if config.Get().App.DemoMode {
			c.Set("is_demo", true)
		}
		c.Next()
	}
}

// DemoGuard blocks non-admin users from modifying account security settings
// (password, MFA) when demo mode is active. Returns 403 with a friendly message.
func DemoGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !config.Get().App.DemoMode {
			c.Next()
			return
		}

		// Admins can always make changes
		if isAdmin(c) {
			c.Next()
			return
		}

		// Block the request
		if wantsJSON(c) {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "This action is disabled in demo mode",
				"message": "Password and MFA changes are not available on the demo instance. Feel free to explore everything else!",
			})
		} else {
			c.Redirect(http.StatusSeeOther, c.Request.Referer())
		}
		c.Abort()
	}
}

// isAdmin checks if the current user belongs to the admin group.
func isAdmin(c *gin.Context) bool {
	// Check for admin group membership (set by auth middleware)
	if groups, exists := c.Get("user_groups"); exists {
		if groupList, ok := groups.([]string); ok {
			for _, g := range groupList {
				if g == "admin" {
					return true
				}
			}
		}
	}
	// Check role-based admin flag
	if role, exists := c.Get("user_role"); exists {
		if r, ok := role.(string); ok && r == "admin" {
			return true
		}
	}
	return false
}

// wantsJSON returns true if the request expects a JSON response.
func wantsJSON(c *gin.Context) bool {
	accept := c.GetHeader("Accept")
	return accept == "application/json" ||
		c.GetHeader("X-Requested-With") == "XMLHttpRequest" ||
		c.ContentType() == "application/json" ||
		len(accept) > 0 && accept[:16] == "application/json"
}
