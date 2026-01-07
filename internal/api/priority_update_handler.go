package api

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleUpdatePriorityAPI handles PUT /api/v1/priorities/:id.
func HandleUpdatePriorityAPI(c *gin.Context) {
	// Check authentication
	userIDRaw, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := normalizeUserID(userIDRaw)

	// Parse priority ID
	priorityID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid priority ID"})
		return
	}

	var req struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Determine target color so response reflects final state
	currentColor := req.Color
	if currentColor == "" {
		fetchColorQuery := database.ConvertPlaceholders(`SELECT color FROM ticket_priority WHERE id = ?`)
		if err := db.QueryRow(fetchColorQuery, priorityID).Scan(&currentColor); err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Priority not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update priority"})
			return
		}
	}

	// Update priority (include color if provided)
	q := database.ConvertPlaceholders(
		`UPDATE ticket_priority SET name = ?, color = COALESCE(NULLIF(?, ''), color), ` +
			`change_time = NOW(), change_by = ? WHERE id = ?`)
	result, err := db.Exec(q, req.Name, req.Color, userID, priorityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update priority"})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update priority"})
		return
	}
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Priority not found"})
		return
	}

	// Return updated priority
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"id":       priorityID,
		"name":     req.Name,
		"color":    currentColor,
		"valid_id": 1,
	}})
}
