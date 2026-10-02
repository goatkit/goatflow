package api

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// A queue belongs to exactly one permission group: queue.group_id. Every
// queue permission check (group_user / group_role) keys on that column, so it
// is the only queue-to-group relation GoatFlow stores.

// errQueueGroupNotFound / errQueueGroupInvalid describe a group_id that cannot
// own a queue.
var (
	errQueueGroupNotFound = errors.New("group not found")
	errQueueGroupInvalid  = errors.New("group is not valid")
)

// lookupQueueGroup returns the name of a group that may own a queue: it must
// exist and be valid (valid_id = 1). For an invalid group the name is returned
// together with errQueueGroupInvalid.
func lookupQueueGroup(db *sql.DB, groupID int) (string, error) {
	var name string
	var validID int
	err := db.QueryRow(database.ConvertPlaceholders(
		`SELECT name, valid_id FROM groups WHERE id = ?`), groupID).Scan(&name, &validID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errQueueGroupNotFound
	}
	if err != nil {
		return "", err
	}
	if validID != 1 {
		return name, errQueueGroupInvalid
	}
	return name, nil
}

// respondQueueGroupError writes the HTTP error for a lookupQueueGroup failure.
func respondQueueGroupError(c *gin.Context, groupID int, err error) {
	switch {
	case errors.Is(err, errQueueGroupNotFound):
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": fmt.Sprintf("Group %d not found", groupID)})
	case errors.Is(err, errQueueGroupInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": fmt.Sprintf("Group %d is not valid", groupID)})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to look up group"})
	}
}

// queueGroupList renders a queue's single group in the list shape the queue
// endpoints expose as "groups".
func queueGroupList(groupID int, groupName string) []gin.H {
	return []gin.H{{"id": groupID, "name": groupName}}
}

// HandleAssignQueueGroupAPI handles POST /api/v1/queues/:id/groups.
//
//	@Summary		Set queue group
//	@Description	Make the given group the queue's permission group (queue.group_id), replacing the current one. A queue has exactly one group.
//	@Tags			Queues
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int		true	"Queue ID"
//	@Param			group	body		object	true	"Group assignment (group_id)"
//	@Success		200		{object}	map[string]interface{}	"Group assigned"
//	@Failure		400		{object}	map[string]interface{}	"Invalid request or group"
//	@Failure		401		{object}	map[string]interface{}	"Unauthorized"
//	@Failure		404		{object}	map[string]interface{}	"Queue not found"
//	@Security		BearerAuth
//	@Router			/queues/{id}/groups [post]
func HandleAssignQueueGroupAPI(c *gin.Context) {
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
		GroupID int `json:"group_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.GroupID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "group_id is required"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database unavailable"})
		return
	}

	// Existence is checked up front: MySQL reports 0 affected rows for an
	// UPDATE that changes nothing, so RowsAffected cannot detect a missing queue.
	var exists int
	err = db.QueryRow(database.ConvertPlaceholders(`SELECT 1 FROM queue WHERE id = ?`), queueID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Queue not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to load queue"})
		return
	}

	groupName, err := lookupQueueGroup(db, req.GroupID)
	if err != nil {
		respondQueueGroupError(c, req.GroupID, err)
		return
	}

	if _, err := db.Exec(database.ConvertPlaceholders(
		`UPDATE queue SET group_id = ?, change_time = NOW(), change_by = ? WHERE id = ?`),
		req.GroupID, GetUserIDFromCtx(c, 1), queueID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to assign group"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Group assigned",
		"data": gin.H{
			"queue_id": queueID,
			"group_id": req.GroupID,
			"groups":   queueGroupList(req.GroupID, groupName),
		},
	})
}

// HandleRemoveQueueGroupAPI handles DELETE /api/v1/queues/:id/groups/:group_id.
// A queue must always have a group, so removing its group is rejected; assign
// a different group (POST /queues/:id/groups) to move the queue instead.
//
//	@Summary		Remove group from queue (always rejected)
//	@Description	A queue has exactly one group, which cannot be removed. Returns 409 for the queue's group and 404 for any other group. Use POST /queues/{id}/groups to change it.
//	@Tags			Queues
//	@Accept			json
//	@Produce		json
//	@Param			id			path		int	true	"Queue ID"
//	@Param			group_id	path		int	true	"Group ID"
//	@Failure		401			{object}	map[string]interface{}	"Unauthorized"
//	@Failure		404			{object}	map[string]interface{}	"Queue not found or group not assigned"
//	@Failure		409			{object}	map[string]interface{}	"Queue must keep its group"
//	@Security		BearerAuth
//	@Router			/queues/{id}/groups/{group_id} [delete]
func HandleRemoveQueueGroupAPI(c *gin.Context) {
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

	var currentGroupID int
	err = db.QueryRow(database.ConvertPlaceholders(`SELECT group_id FROM queue WHERE id = ?`), queueID).Scan(&currentGroupID)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Queue not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to load queue"})
		return
	}
	if groupID != currentGroupID {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": fmt.Sprintf("Group %d is not assigned to this queue", groupID)})
		return
	}

	c.JSON(http.StatusConflict, gin.H{
		"success": false,
		"error":   "A queue must have a group; assign a different group to the queue instead of removing this one",
	})
}
