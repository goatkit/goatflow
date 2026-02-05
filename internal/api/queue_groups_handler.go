package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleAssignQueueGroupAPI handles POST /api/v1/queues/:id/groups.
//
//	@Summary		Assign group to queue
//	@Description	Assign a group to a queue
//	@Tags			Queues
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int		true	"Queue ID"
//	@Param			group	body		object	true	"Group assignment (group_id, permission)"
//	@Success		200		{object}	map[string]interface{}	"Group assigned"
//	@Failure		400		{object}	map[string]interface{}	"Invalid request"
//	@Failure		401		{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/queues/{id}/groups [post]
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
	row := db.QueryRow(database.ConvertPlaceholders(`SELECT 1 FROM queue WHERE id = ?`), queueID)
	_ = row.Scan(&count) //nolint:errcheck // Defaults to 0
	if count != 1 {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Queue not found"})
		return
	}
	count = 0
	row2 := db.QueryRow(database.ConvertPlaceholders(`SELECT 1 FROM groups WHERE id = ?`), req.GroupID)
	_ = row2.Scan(&count) //nolint:errcheck // Defaults to 0
	if count != 1 {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Group not found"})
		return
	}

	// Ensure mapping exists without using vendor-specific UPSERT
	var existsMap int
	existsQuery := `SELECT 1 FROM queue_group WHERE queue_id = ? AND group_id = ?`
	row3 := db.QueryRow(database.ConvertPlaceholders(existsQuery), queueID, req.GroupID)
	_ = row3.Scan(&existsMap) //nolint:errcheck // Defaults to 0
	if existsMap != 1 {
		insertQuery := `INSERT INTO queue_group (queue_id, group_id) VALUES (?, ?)`
		if _, err := db.Exec(database.ConvertPlaceholders(insertQuery), queueID, req.GroupID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to assign group"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Group assigned",
		"data":    gin.H{"queue_id": queueID, "group_id": req.GroupID, "permissions": req.Permissions},
	})
}

// HandleRemoveQueueGroupAPI handles DELETE /api/v1/queues/:id/groups/:group_id.
//
//	@Summary		Remove group from queue
//	@Description	Remove a group assignment from a queue
//	@Tags			Queues
//	@Accept			json
//	@Produce		json
//	@Param			id			path		int	true	"Queue ID"
//	@Param			group_id	path		int	true	"Group ID"
//	@Success		200			{object}	map[string]interface{}	"Group removed"
//	@Failure		401			{object}	map[string]interface{}	"Unauthorized"
//	@Failure		404			{object}	map[string]interface{}	"Not found"
//	@Security		BearerAuth
//	@Router			/queues/{id}/groups/{group_id} [delete]
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

	result, err := db.Exec(database.ConvertPlaceholders(`DELETE FROM queue_group WHERE queue_id = ? AND group_id = ?`), queueID, groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to remove group"})
		return
	}
	rows, _ := result.RowsAffected() //nolint:errcheck // Defaults to 0
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Association not found"})
		return
	}

	c.Status(http.StatusNoContent)
}
