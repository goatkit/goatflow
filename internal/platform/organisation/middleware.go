package organisation

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

// GinContextKey is the key used to store the active org ID in gin context.
const GinContextKey = "active_org_id"

// Middleware resolves the active organisation for each request and sets it
// in both the gin context and the request context. The org is resolved from:
//  1. Session cookie (active_org_id) — set by the org switcher
//  2. User's default org — if no cookie set
//
// In single-org mode (no organisations in DB), this is a no-op.
func Middleware(repo *Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try session cookie first.
		if orgIDStr, err := c.Cookie("active_org_id"); err == nil && orgIDStr != "" {
			var orgID int64
			if _, err := fmt.Sscanf(orgIDStr, "%d", &orgID); err == nil && orgID > 0 {
				setOrgContext(c, orgID)
				c.Next()
				return
			}
		}

		// Fall back to user's default org.
		userID, exists := c.Get("user_id")
		if !exists {
			c.Next()
			return
		}

		uid, ok := toInt(userID)
		if !ok {
			c.Next()
			return
		}

		defaultOrg, err := repo.GetDefaultOrgForUser(uid)
		if err != nil || defaultOrg == nil {
			// No default org — single-org mode.
			c.Next()
			return
		}

		setOrgContext(c, defaultOrg.ID)
		c.Next()
	}
}

// setOrgContext stores the active org in both gin context and request context.
func setOrgContext(c *gin.Context, orgID int64) {
	c.Set(GinContextKey, orgID)
	ctx := WithOrgID(c.Request.Context(), orgID)
	c.Request = c.Request.WithContext(ctx)
}

// ActiveOrgFromGin reads the active org ID from gin context.
// Returns 0 if not set.
func ActiveOrgFromGin(c *gin.Context) int64 {
	if v, exists := c.Get(GinContextKey); exists {
		if id, ok := v.(int64); ok {
			return id
		}
	}
	return 0
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	}
	return 0, false
}
