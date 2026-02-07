// Package api provides HTTP handlers for the GoatFlow application.
package api

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
)

// deleteQueueInTransaction performs the queue deletion within a transaction.
func deleteQueueInTransaction(tx *sql.Tx, queueID int, userID interface{}) error {
	deleteGroupsQuery := database.ConvertPlaceholders(`DELETE FROM queue_group WHERE queue_id = ?`)
	if _, err := tx.Exec(deleteGroupsQuery, queueID); err != nil {
		return err
	}

	deleteQuery := database.ConvertPlaceholders(`
		UPDATE queue
		SET valid_id = 2, change_time = NOW(), change_by = ?
		WHERE id = ?
	`)

	// Args order matches query: change_by=?, id=?
	result, err := tx.Exec(deleteQuery, userID, queueID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return tx.Commit()
}

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
	userID, exists := c.Get("user_id")
	if !exists {
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

	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("queue_delete_handler: tx.Rollback failed: %v", err)
		}
	}()

	if err := deleteQueueInTransaction(tx, queueID, userID); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Queue not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete queue"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Queue deleted successfully"})
}
