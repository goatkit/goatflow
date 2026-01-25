package api

import "github.com/gin-gonic/gin"

// GetUserIDFromCtx extracts the user ID from the gin context with proper type handling.
// This is a local copy to avoid circular imports with internal/api.
func GetUserIDFromCtx(c *gin.Context, fallback int) int {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		return fallback
	}

	switch v := userIDVal.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case uint:
		return int(v)
	case uint64:
		return int(v)
	case float64:
		return int(v)
	case string:
		// Don't try to parse strings as user IDs
		return fallback
	default:
		return fallback
	}
}
