package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleUpdateQueueAPI handles PUT /api/v1/queues/:id
func HandleUpdateQueueAPI(c *gin.Context) {
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

	var req struct {
		Name        string `json:"name"`
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

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Update queue if name or description provided
	if req.Name != "" || req.Description != "" {
        updateQuery := database.ConvertPlaceholders(`
            UPDATE queue 
            SET change_time = NOW(), change_by = $1
        `)
		args := []interface{}{userID}
		paramCount := 1

		if req.Name != "" {
			paramCount++
			updateQuery += database.ConvertPlaceholders(`, name = $` + strconv.Itoa(paramCount))
			args = append(args, req.Name)
		}
		if req.Description != "" {
			paramCount++
            updateQuery += database.ConvertPlaceholders(`, comments = $` + strconv.Itoa(paramCount))
			args = append(args, req.Description)
		}

		paramCount++
		updateQuery += database.ConvertPlaceholders(` WHERE id = $` + strconv.Itoa(paramCount))
		args = append(args, queueID)

		if _, err := tx.Exec(updateQuery, args...); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update queue"})
			return
		}
	}

	// Update group access if provided
	if req.GroupAccess != nil {
		// Remove existing group access
        deleteQuery := database.ConvertPlaceholders(`
            DELETE FROM queue_group WHERE queue_id = $1
        `)
		if _, err := tx.Exec(deleteQuery, queueID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update group access"})
			return
		}

		// Add new group access
		for _, groupID := range req.GroupAccess {
            insertQuery := database.ConvertPlaceholders(`
                INSERT INTO queue_group (queue_id, group_id)
                VALUES ($1, $2)
            `)
			if _, err := tx.Exec(insertQuery, queueID, groupID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update group access"})
				return
			}
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	// Get updated queue data
	var queue struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		ValidID     int    `json:"valid_id"`
		GroupAccess []int  `json:"group_access"`
	}

    query := database.ConvertPlaceholders(`
        SELECT id, name, comments, valid_id
        FROM queue
        WHERE id = $1
    `)
	err = db.QueryRow(query, queueID).Scan(&queue.ID, &queue.Name, &queue.Description, &queue.ValidID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve updated queue"})
		return
	}

	// Get group access
	groupQuery := database.ConvertPlaceholders(`
		SELECT group_id FROM queue_groups 
		WHERE queue_id = $1
	`)
	
	rows, err := db.Query(groupQuery, queueID)
	if err == nil {
		defer rows.Close()
		queue.GroupAccess = []int{}
		for rows.Next() {
			var groupID int
			if err := rows.Scan(&groupID); err == nil {
				queue.GroupAccess = append(queue.GroupAccess, groupID)
			}
		}
	}

    c.JSON(http.StatusOK, gin.H{"success": true, "data": queue})
}