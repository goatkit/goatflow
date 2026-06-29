package api

import (
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/convert"
)

// GetUserIDFromCtx extracts the user ID from the gin context with proper type handling.
func GetUserIDFromCtx(c *gin.Context, fallback int) int {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		return fallback
	}
	return convert.ToInt(userIDVal, fallback)
}
