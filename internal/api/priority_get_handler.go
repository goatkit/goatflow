package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
)

// HandleGetPriorityAPI handles GET /api/v1/priorities/:id.
//
//	@Summary		Get priority by ID
//	@Description	Retrieve a single priority
//	@Tags			Priorities
//	@Accept			json
//	@Produce		json
//	@Param			id	path		int	true	"Priority ID"
//	@Success		200	{object}	map[string]interface{}	"Priority details"
//	@Failure		401	{object}	map[string]interface{}	"Unauthorized"
//	@Failure		404	{object}	map[string]interface{}	"Priority not found"
//	@Security		BearerAuth
//	@Router			/priorities/{id} [get]
func HandleGetPriorityAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	_ = userID // Will use for permission checks later

	// Parse priority ID
	priorityID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid priority ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Get priority details
	var priority struct {
		ID      int    `json:"id"`
		Name    string `json:"name"`
		Color   string `json:"color"`
		ValidID int    `json:"valid_id"`
	}

	query := database.ConvertPlaceholders(`
		SELECT id, name, color, valid_id
		FROM ticket_priority
		WHERE id = ?
	`)

	err = db.QueryRow(query, priorityID).Scan(&priority.ID, &priority.Name, &priority.Color, &priority.ValidID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Priority not found"})
		return
	}

	c.JSON(http.StatusOK, priority)
}
