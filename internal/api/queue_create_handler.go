package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleCreateQueueAPI handles POST /api/v1/queues
func HandleCreateQueueAPI(c *gin.Context) {
	// Check authentication and admin permissions
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	_ = userID // TODO: Check admin permissions

	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		GroupAccess []int  `json:"group_access"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

    db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Check if queue with this name already exists
	var count int
    checkQuery := database.ConvertPlaceholders(`
        SELECT 1 FROM queue
        WHERE name = $1 AND valid_id = 1
    `)
	db.QueryRow(checkQuery, req.Name).Scan(&count)
	if count == 1 {
		c.JSON(http.StatusConflict, gin.H{"error": "Queue with this name already exists"})
		return
	}

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Create queue
	var queueID int
    insertQuery := database.ConvertPlaceholders(`
        INSERT INTO queue (name, comments, valid_id, create_time, create_by, change_time, change_by)
        VALUES ($1, $2, 1, NOW(), $3, NOW(), $3)
        RETURNING id
    `)
	
	err = tx.QueryRow(insertQuery, req.Name, req.Description, userID).Scan(&queueID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create queue"})
		return
	}

	// Add group access if specified
    if len(req.GroupAccess) > 0 {
		for _, groupID := range req.GroupAccess {
            groupInsert := database.ConvertPlaceholders(`
                INSERT INTO queue_group (queue_id, group_id)
                VALUES ($1, $2)
            `)
			if _, err := tx.Exec(groupInsert, queueID, groupID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set group access"})
				return
			}
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

    // Return created queue
    response := gin.H{
		"id":           queueID,
		"name":         req.Name,
		"description":  req.Description,
		"valid_id":     1,
		"group_access": req.GroupAccess,
	}

    c.JSON(http.StatusCreated, gin.H{"success": true, "data": response})
}