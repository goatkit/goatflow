package v1

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// handleListPriorities returns all ticket priorities.
func (router *APIRouter) handleListPriorities(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data:    gin.H{"priorities": []interface{}{}, "total": 0},
		})
		return
	}

	query := database.ConvertQuery(`
		SELECT id, name, valid_id, create_time, change_time
		FROM ticket_priority
		WHERE valid_id = 1
		ORDER BY id
	`)

	rows, err := db.Query(query)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data:    gin.H{"priorities": []interface{}{}, "total": 0},
		})
		return
	}
	defer rows.Close()

	type Priority struct {
		ID        int       `json:"id"`
		Name      string    `json:"name"`
		ValidID   int       `json:"valid_id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	priorities := []Priority{}
	for rows.Next() {
		var p Priority
		if err := rows.Scan(&p.ID, &p.Name, &p.ValidID, &p.CreatedAt, &p.UpdatedAt); err != nil {
			continue
		}
		priorities = append(priorities, p)
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    gin.H{"priorities": priorities, "total": len(priorities)},
	})
}

// handleCreatePriority creates a new ticket priority.
func (router *APIRouter) handleCreatePriority(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	userID := 1
	if id, exists := c.Get("user_id"); exists {
		if idInt, ok := id.(int); ok && idInt > 0 {
			userID = idInt
		}
	}

	now := time.Now()
	query := database.ConvertQuery(`
		INSERT INTO ticket_priority (name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 1, ?, ?, ?, ?)
	`)

	result, err := db.Exec(query, req.Name, now, userID, now, userID)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to create priority")
		return
	}

	id, _ := result.LastInsertId()

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Data: gin.H{
			"id":         id,
			"name":       req.Name,
			"created_at": now,
		},
	})
}

// handleGetPriority returns a specific priority.
func (router *APIRouter) handleGetPriority(c *gin.Context) {
	priorityID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid priority ID")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	query := database.ConvertQuery(`
		SELECT id, name, valid_id, create_time, change_time
		FROM ticket_priority
		WHERE id = ?
	`)

	var priority struct {
		ID        int       `json:"id"`
		Name      string    `json:"name"`
		ValidID   int       `json:"valid_id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	err = db.QueryRow(query, priorityID).Scan(&priority.ID, &priority.Name, &priority.ValidID, &priority.CreatedAt, &priority.UpdatedAt)
	if err != nil {
		sendError(c, http.StatusNotFound, "Priority not found")
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    priority,
	})
}

// handleUpdatePriority updates a priority.
func (router *APIRouter) handleUpdatePriority(c *gin.Context) {
	priorityID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid priority ID")
		return
	}

	var req struct {
		Name    string `json:"name"`
		ValidID *int   `json:"valid_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	userID := 1
	if id, exists := c.Get("user_id"); exists {
		if idInt, ok := id.(int); ok && idInt > 0 {
			userID = idInt
		}
	}

	now := time.Now()
	query := database.ConvertQuery(`
		UPDATE ticket_priority SET
			name = COALESCE(NULLIF(?, ''), name),
			valid_id = COALESCE(?, valid_id),
			change_time = ?,
			change_by = ?
		WHERE id = ?
	`)

	result, err := db.Exec(query, req.Name, req.ValidID, now, userID, priorityID)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to update priority")
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		sendError(c, http.StatusNotFound, "Priority not found")
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: gin.H{
			"id":         priorityID,
			"updated_at": now,
		},
	})
}

// handleDeletePriority soft-deletes a priority.
func (router *APIRouter) handleDeletePriority(c *gin.Context) {
	priorityID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid priority ID")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	userID := 1
	if id, exists := c.Get("user_id"); exists {
		if idInt, ok := id.(int); ok && idInt > 0 {
			userID = idInt
		}
	}

	query := database.ConvertQuery(`
		UPDATE ticket_priority SET valid_id = 2, change_time = ?, change_by = ?
		WHERE id = ?
	`)

	result, err := db.Exec(query, time.Now(), userID, priorityID)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to delete priority")
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		sendError(c, http.StatusNotFound, "Priority not found")
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    gin.H{"message": "Priority deleted"},
	})
}
