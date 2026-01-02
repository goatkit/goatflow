package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleDeletePriorityAPI handles DELETE /api/v1/priorities/:id.
func HandleDeletePriorityAPI(c *gin.Context) {
	// Check authentication
	userIDRaw, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized"})
		return
	}
	userID := normalizeUserID(userIDRaw)

	// Parse priority ID
	priorityID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid priority ID"})
		return
	}

	if priorityID == 1 {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Cannot delete system priority"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	// Soft delete priority (OTRS style - set valid_id = 2)
	adapter := database.GetAdapter()
	result, err := adapter.Exec(db,
		`UPDATE ticket_priority SET valid_id = 2, change_time = NOW(), change_by = $2 WHERE id = $1`,
		priorityID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete priority"})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete priority"})
		return
	}
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Priority not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Priority deleted successfully"})
}
