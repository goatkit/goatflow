package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleDeleteQueueAPI handles DELETE /api/v1/queues/:id
func HandleDeleteQueueAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Parse queue ID
	queueID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid queue ID"})
		return
	}

	// Protect system queues (IDs 1-3 are typically system queues)
	if queueID <= 3 {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot delete system queue"})
		return
	}

    db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Check if queue exists
	var count int
    checkQuery := database.ConvertPlaceholders(`
        SELECT 1 FROM queue
        WHERE id = $1 AND valid_id = 1
    `)
	db.QueryRow(checkQuery, queueID).Scan(&count)
	if count != 1 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Queue not found"})
		return
	}

	// Check if queue has tickets
	var ticketCount int
    ticketQuery := database.ConvertPlaceholders(`
        SELECT COUNT(*) FROM ticket 
        WHERE queue_id = $1
    `)
	db.QueryRow(ticketQuery, queueID).Scan(&ticketCount)
	if ticketCount > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Queue has tickets",
			"message": "Cannot delete queue with existing tickets. Move or delete tickets first.",
			"ticket_count": ticketCount,
		})
		return
	}

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Remove group associations
    deleteGroupsQuery := database.ConvertPlaceholders(`
        DELETE FROM queue_group WHERE queue_id = $1
    `)
	if _, err := tx.Exec(deleteGroupsQuery, queueID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove group associations"})
		return
	}

	// Soft delete queue (OTRS style - set valid_id = 2)
    deleteQuery := database.ConvertPlaceholders(`
        UPDATE queue 
        SET valid_id = 2, change_time = NOW(), change_by = $1
        WHERE id = $2
    `)
	
	result, err := tx.Exec(deleteQuery, userID, queueID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete queue"})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete queue"})
		return
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

    c.Status(http.StatusNoContent)
}