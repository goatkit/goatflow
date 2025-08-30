package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleAPIQueueGet handles GET /api/queues/:id
func HandleAPIQueueGet(c *gin.Context) {
	queueID := c.Param("id")
	id, err := strconv.Atoi(queueID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid queue ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	var queue struct {
		ID               int    `json:"id"`
		Name             string `json:"name"`
		GroupID          int    `json:"group_id"`
		SystemAddressID  *int   `json:"system_address_id"`
		Comments         string `json:"comments"`
		UnlockTimeout    int    `json:"unlock_timeout"`
		FollowUpLock     int    `json:"follow_up_lock"`
		ValidID          int    `json:"valid_id"`
	}

	err = db.QueryRow(database.ConvertPlaceholders(`
		SELECT id, name, group_id, system_address_id, comments, 
		       unlock_timeout, follow_up_lock, valid_id
		FROM queue WHERE id = $1
	`), id).Scan(&queue.ID, &queue.Name, &queue.GroupID, &queue.SystemAddressID,
		&queue.Comments, &queue.UnlockTimeout, &queue.FollowUpLock, &queue.ValidID)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Queue not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": queue})
}

// HandleAPIQueueDetails handles GET /api/queues/:id/details
func HandleAPIQueueDetails(c *gin.Context) {
	queueID := c.Param("id")
	id, err := strconv.Atoi(queueID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid queue ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Get queue details with group name
	var queue struct {
		ID               int    `json:"id"`
		Name             string `json:"name"`
		GroupID          int    `json:"group_id"`
		GroupName        string `json:"group_name"`
		SystemAddressID  *int   `json:"system_address_id"`
		Comments         string `json:"comments"`
		UnlockTimeout    int    `json:"unlock_timeout"`
		FollowUpLock     int    `json:"follow_up_lock"`
		ValidID          int    `json:"valid_id"`
		TicketCount      int    `json:"ticket_count"`
	}

	err = db.QueryRow(database.ConvertPlaceholders(`
		SELECT q.id, q.name, q.group_id, g.name as group_name,
		       q.system_address_id, q.comments, 
		       q.unlock_timeout, q.follow_up_lock, q.valid_id,
		       (SELECT COUNT(*) FROM ticket WHERE queue_id = q.id) as ticket_count
		FROM queue q
		LEFT JOIN groups g ON q.group_id = g.id
		WHERE q.id = $1
	`), id).Scan(&queue.ID, &queue.Name, &queue.GroupID, &queue.GroupName,
		&queue.SystemAddressID, &queue.Comments, &queue.UnlockTimeout,
		&queue.FollowUpLock, &queue.ValidID, &queue.TicketCount)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Queue not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": queue})
}

// HandleAPIQueueStatus handles PUT /api/queues/:id/status
func HandleAPIQueueStatus(c *gin.Context) {
	queueID := c.Param("id")
	id, err := strconv.Atoi(queueID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid queue ID"})
		return
	}

	var req struct {
		ValidID int `json:"valid_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Update queue status
	_, err = db.Exec(database.ConvertPlaceholders(`UPDATE queue SET valid_id = $1 WHERE id = $2`), req.ValidID, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update queue status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Queue status updated"})
}