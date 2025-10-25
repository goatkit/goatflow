package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleCreatePriorityAPI handles POST /api/v1/priorities
func HandleCreatePriorityAPI(c *gin.Context) {
	// Check authentication and admin permissions
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req struct {
		Name  string `json:"name" binding:"required"`
		Color string `json:"color"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Name is required"})
		return
	}

	if req.Color == "" {
		req.Color = "#cdcdcd"
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Check if priority with this name already exists
	var count int
	checkQuery := database.ConvertPlaceholders(`
		SELECT 1 FROM ticket_priority
		WHERE name = $1 AND valid_id = 1
	`)
	db.QueryRow(checkQuery, req.Name).Scan(&count)
	if count == 1 {
		c.JSON(http.StatusConflict, gin.H{"success": false, "error": "Priority with this name already exists"})
		return
	}

	// Create priority
	var priorityID int
	insertQuery := database.ConvertPlaceholders(`
        INSERT INTO ticket_priority (name, color, valid_id, create_time, create_by, change_time, change_by)
        VALUES ($1, $2, $3, NOW(), $4, NOW(), $5)
        RETURNING id
    `)
	err = db.QueryRow(insertQuery, req.Name, req.Color, 1, userID, userID).Scan(&priorityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to create priority"})
		return
	}

	// Return created priority
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": gin.H{
			"id":       priorityID,
			"name":     req.Name,
			"color":    req.Color,
			"valid_id": 1,
		},
	})
}
