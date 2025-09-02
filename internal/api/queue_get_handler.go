package api

import (
    "net/http"
    "strconv"

    "github.com/gin-gonic/gin"
)

// HandleGetQueueAPI handles GET /api/v1/queues/:id
func HandleGetQueueAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	_ = userID // Will use for permission checks later

	// Parse queue ID
    if _, err := strconv.Atoi(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid queue ID"})
		return
	}

    // Delegate to unified OTRS-based handler for consistency
    // Delegate to unified OTRS-based handler for consistency.
    // Preserve API v1 response shape {success, data}
    // We call the underlying handler and adapt its response if needed.
    HandleAPIQueueGet(c)
}