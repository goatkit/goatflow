// Package api provides HTTP handlers for the GoatFlow application.
package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// HandleDeleteQueueAPI handles DELETE /api/v1/queues/:id.
//
//	@Summary		Delete queue
//	@Description	Delete a queue (soft delete)
//	@Tags			Queues
//	@Accept			json
//	@Produce		json
//	@Param			id	path		int	true	"Queue ID"
//	@Success		200	{object}	map[string]interface{}	"Queue deleted"
//	@Failure		401	{object}	map[string]interface{}	"Unauthorized"
//	@Failure		404	{object}	map[string]interface{}	"Queue not found"
//	@Security		BearerAuth
//	@Router			/queues/{id} [delete]
func HandleDeleteQueueAPI(c *gin.Context) {
	if _, exists := c.Get("user_id"); !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized"})
		return
	}

	queueID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid queue ID"})
		return
	}

	if queueID <= 3 {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Cannot delete system queue"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	var ticketCount int
	ticketQuery := database.ConvertPlaceholders(`SELECT COUNT(*) FROM ticket WHERE queue_id = ?`)
	if err := db.QueryRow(ticketQuery, queueID).Scan(&ticketCount); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to check queue tickets"})
		return
	}

	if ticketCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"success": false, "error": "Cannot delete queue with existing tickets"})
		return
	}

	// Soft delete: the queue keeps its group (queue.group_id) so it can be
	// restored as it was.
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
	if _, err := db.Exec(database.ConvertPlaceholders(`
		UPDATE queue
		SET valid_id = 2, change_time = NOW(), change_by = ?
		WHERE id = ?
	`), GetUserIDFromCtx(c, 1), queueID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete queue"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Queue deleted successfully"})
}
