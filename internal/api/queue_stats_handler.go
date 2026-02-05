package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleGetQueueStatsAPI handles GET /api/v1/queues/:id/stats.
//
//	@Summary		Get queue statistics
//	@Description	Get ticket statistics for a queue
//	@Tags			Queues
//	@Accept			json
//	@Produce		json
//	@Param			id	path		int	true	"Queue ID"
//	@Success		200	{object}	map[string]interface{}	"Queue statistics"
//	@Failure		401	{object}	map[string]interface{}	"Unauthorized"
//	@Failure		404	{object}	map[string]interface{}	"Queue not found"
//	@Security		BearerAuth
//	@Router			/queues/{id}/stats [get]
func HandleGetQueueStatsAPI(c *gin.Context) {
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

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database unavailable"})
		return
	}

	// Verify queue exists
	var exists int
	row := db.QueryRow(database.ConvertPlaceholders(`SELECT 1 FROM queue WHERE id = ?`), queueID)
	_ = row.Scan(&exists) //nolint:errcheck // Defaults to 0
	if exists != 1 {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Queue not found"})
		return
	}

	// Compute stats from ticket table using OTRS semantics
	// Map of state categories; adjust IDs per actual seed data if needed
	var total, openCount, closedCount, pendingCount int
	statsQuery := database.ConvertPlaceholders(`
		SELECT 
			COUNT(*) as total,
			COUNT(CASE WHEN ticket_state_id IN (1,4) THEN 1 END) as open_count,
			COUNT(CASE WHEN ticket_state_id IN (2,3) THEN 1 END) as closed_count,
			COUNT(CASE WHEN ticket_state_id IN (5,6) THEN 1 END) as pending_count
		FROM ticket
		WHERE queue_id = ?
	`)
	if err := db.QueryRow(statsQuery, queueID).Scan(&total, &openCount, &closedCount, &pendingCount); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to compute stats"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"total_tickets":   total,
			"open_tickets":    openCount,
			"closed_tickets":  closedCount,
			"pending_tickets": pendingCount,
		},
	})
}
