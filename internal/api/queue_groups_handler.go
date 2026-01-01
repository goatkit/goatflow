package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleAssignQueueGroupAPI handles POST /api/v1/queues/:id/groups.
func HandleAssignQueueGroupAPI(c *gin.Context) {
	// Auth
	if _, ok := c.Get("user_id"); !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Authentication required"})
		return
	}

	queueID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid queue ID"})
		return
	}

	var req struct {
		GroupID     int    `json:"group_id" binding:"required"`
		Permissions string `json:"permissions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if req.Permissions == "" {
		req.Permissions = "rw"
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database unavailable"})
		return
	}

	// Verify queue and group exist
	var count int
	db.QueryRow(database.ConvertPlaceholders(`SELECT 1 FROM queue WHERE id = $1`), queueID).Scan(&count)
	if count != 1 {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Queue not found"})
		return
	}
	count = 0
	db.QueryRow(database.ConvertPlaceholders(`SELECT 1 FROM groups WHERE id = $1`), req.GroupID).Scan(&count)
	if count != 1 {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Group not found"})
		return
	}

	// Ensure mapping exists without using vendor-specific UPSERT
	var existsMap int
	db.QueryRow(database.ConvertPlaceholders(`SELECT 1 FROM queue_group WHERE queue_id = $1 AND group_id = $2`), queueID, req.GroupID).Scan(&existsMap)
	if existsMap != 1 {
		if _, err := db.Exec(database.ConvertPlaceholders(`INSERT INTO queue_group (queue_id, group_id) VALUES ($1, $2)`), queueID, req.GroupID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to assign group"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Group assigned", "data": gin.H{"queue_id": queueID, "group_id": req.GroupID, "permissions": req.Permissions}})
}

// HandleRemoveQueueGroupAPI handles DELETE /api/v1/queues/:id/groups/:group_id.
func HandleRemoveQueueGroupAPI(c *gin.Context) {
	// Auth
	if _, ok := c.Get("user_id"); !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Authentication required"})
		return
	}

	queueID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid queue ID"})
		return
	}
	groupID, err := strconv.Atoi(c.Param("group_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid group ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database unavailable"})
		return
	}

	result, err := db.Exec(database.ConvertPlaceholders(`DELETE FROM queue_group WHERE queue_id = $1 AND group_id = $2`), queueID, groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to remove group"})
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Association not found"})
		return
	}

	c.Status(http.StatusNoContent)
}
