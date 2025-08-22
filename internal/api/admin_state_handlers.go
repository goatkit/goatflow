package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/lib/pq"
)

// StateType represents a ticket state type
type StateType struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	Comments *string `json:"comments,omitempty"`
}

// StateWithType includes type information
type StateWithType struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	TypeID       int     `json:"type_id"`
	TypeName     string  `json:"type_name"`
	Comments     *string `json:"comments,omitempty"`
	ValidID      int     `json:"valid_id"`
	TicketCount  int     `json:"ticket_count"`
}

// handleAdminStates renders the admin states management page
func handleAdminStates(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.String(http.StatusInternalServerError, "Database connection failed")
		return
	}

	// Get search and filter parameters
	searchQuery := c.Query("search")
	typeFilter := c.Query("type")
	sortBy := c.DefaultQuery("sort", "id")
	sortOrder := c.DefaultQuery("order", "asc")

	// Build query with filters
	query := `
		SELECT 
			s.id, s.name, s.type_id, st.name as type_name, 
			s.comments, s.valid_id,
			COUNT(DISTINCT t.id) as ticket_count
		FROM ticket_state s
		JOIN ticket_state_type st ON s.type_id = st.id
		LEFT JOIN ticket t ON t.ticket_state_id = s.id
		WHERE 1=1
	`
	
	var args []interface{}
	argCount := 1

	if searchQuery != "" {
		query += fmt.Sprintf(" AND (LOWER(s.name) LIKE $%d OR LOWER(s.comments) LIKE $%d)", argCount, argCount+1)
		searchPattern := "%" + strings.ToLower(searchQuery) + "%"
		args = append(args, searchPattern, searchPattern)
		argCount += 2
	}

	if typeFilter != "" {
		query += fmt.Sprintf(" AND s.type_id = $%d", argCount)
		args = append(args, typeFilter)
		argCount++
	}

	query += " GROUP BY s.id, s.name, s.type_id, st.name, s.comments, s.valid_id"

	// Add sorting
	validSortColumns := map[string]bool{
		"id": true, "name": true, "type_name": true, "ticket_count": true,
	}
	if !validSortColumns[sortBy] {
		sortBy = "id"
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "asc"
	}
	query += fmt.Sprintf(" ORDER BY %s %s", sortBy, sortOrder)

	rows, err := db.Query(query, args...)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to fetch states")
		return
	}
	defer rows.Close()

	var states []StateWithType
	for rows.Next() {
		var s StateWithType
		var comments sql.NullString
		
		err := rows.Scan(&s.ID, &s.Name, &s.TypeID, &s.TypeName, 
			&comments, &s.ValidID, &s.TicketCount)
		if err != nil {
			continue
		}
		
		if comments.Valid {
			s.Comments = &comments.String
		}
		
		states = append(states, s)
	}

	// Get state types for dropdown
	typeRows, err := db.Query("SELECT id, name, comments FROM ticket_state_type ORDER BY id")
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to fetch state types")
		return
	}
	defer typeRows.Close()

	var stateTypes []StateType
	for typeRows.Next() {
		var st StateType
		var comments sql.NullString
		
		err := typeRows.Scan(&st.ID, &st.Name, &comments)
		if err != nil {
			continue
		}
		
		if comments.Valid {
			st.Comments = &comments.String
		}
		
		stateTypes = append(stateTypes, st)
	}

	// Get user from context for navigation
	user := getUserFromContext(c)

	// Convert typeFilter to int for template comparison
	var typeFilterInt int
	if typeFilter != "" {
		typeFilterInt, _ = strconv.Atoi(typeFilter)
	}

	// Render the template
	pongo2Renderer.HTML(c, http.StatusOK, "pages/admin/states.pongo2", pongo2.Context{
		"Title":       "State Management",
		"States":      states,
		"StateTypes":  stateTypes,
		"SearchQuery": searchQuery,
		"TypeFilter":  typeFilterInt,
		"SortBy":      sortBy,
		"SortOrder":   sortOrder,
		"User":        user,
		"ActivePage":  "admin",
	})
}

// handleAdminStateCreate creates a new state
func handleAdminStateCreate(c *gin.Context) {
	var input struct {
		Name     string  `json:"name" form:"name" binding:"required"`
		TypeID   int     `json:"type_id" form:"type_id" binding:"required"`
		Comments *string `json:"comments" form:"comments"`
		ValidID  int     `json:"valid_id" form:"valid_id"`
	}

	// Try to bind based on content type
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Name and type are required",
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Validate type_id exists
	var typeExists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM ticket_state_type WHERE id = $1)", input.TypeID).Scan(&typeExists)
	if err != nil || !typeExists {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid state type",
		})
		return
	}

	// Default to valid if not specified
	if input.ValidID == 0 {
		input.ValidID = 1
	}

	// Create the state
	var id int
	query := `
		INSERT INTO ticket_state (name, type_id, comments, valid_id, create_by, change_by) 
		VALUES ($1, $2, $3, $4, 1, 1) 
		RETURNING id
	`

	err = db.QueryRow(query, input.Name, input.TypeID, input.Comments, input.ValidID).Scan(&id)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"error":   "A state with this name already exists",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to create state",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": State{
			ID:       id,
			Name:     input.Name,
			TypeID:   input.TypeID,
			Comments: input.Comments,
			ValidID:  input.ValidID,
		},
	})
}

// handleAdminStateUpdate updates an existing state
func handleAdminStateUpdate(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid state ID",
		})
		return
	}

	var input struct {
		Name     string  `json:"name" form:"name"`
		TypeID   *int    `json:"type_id" form:"type_id"`
		Comments *string `json:"comments" form:"comments"`
		ValidID  *int    `json:"valid_id" form:"valid_id"`
	}

	// Try to bind based on content type
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Validate type_id if provided
	if input.TypeID != nil {
		var typeExists bool
		err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM ticket_state_type WHERE id = $1)", *input.TypeID).Scan(&typeExists)
		if err != nil || !typeExists {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid state type",
			})
			return
		}
	}

	// Build update query dynamically
	query := `UPDATE ticket_state SET change_by = 1, change_time = CURRENT_TIMESTAMP`
	var args []interface{}
	argCount := 1

	if input.Name != "" {
		query += fmt.Sprintf(", name = $%d", argCount)
		args = append(args, input.Name)
		argCount++
	}

	if input.TypeID != nil {
		query += fmt.Sprintf(", type_id = $%d", argCount)
		args = append(args, *input.TypeID)
		argCount++
	}

	if input.Comments != nil {
		query += fmt.Sprintf(", comments = $%d", argCount)
		args = append(args, *input.Comments)
		argCount++
	}

	if input.ValidID != nil {
		query += fmt.Sprintf(", valid_id = $%d", argCount)
		args = append(args, *input.ValidID)
		argCount++
	}

	query += fmt.Sprintf(" WHERE id = $%d", argCount)
	args = append(args, id)

	result, err := db.Exec(query, args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"error":   "A state with this name already exists",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to update state",
		})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "State not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "State updated successfully",
	})
}

// handleAdminStateDelete soft deletes a state
func handleAdminStateDelete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid state ID",
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Check if state is in use
	var ticketCount int
	err = db.QueryRow("SELECT COUNT(*) FROM ticket WHERE ticket_state_id = $1", id).Scan(&ticketCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to check state usage",
		})
		return
	}

	if ticketCount > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Cannot delete state: %d tickets are using this state", ticketCount),
		})
		return
	}

	// Soft delete by setting valid_id = 2
	result, err := db.Exec(`
		UPDATE ticket_state 
		SET valid_id = 2, change_by = 1, change_time = CURRENT_TIMESTAMP 
		WHERE id = $1
	`, id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to delete state",
		})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "State not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "State deleted successfully",
	})
}

// handleGetStateTypes returns all state types
func handleGetStateTypes(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	rows, err := db.Query("SELECT id, name, comments FROM ticket_state_type ORDER BY id")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to fetch state types",
		})
		return
	}
	defer rows.Close()

	var types []StateType
	for rows.Next() {
		var st StateType
		var comments sql.NullString

		err := rows.Scan(&st.ID, &st.Name, &comments)
		if err != nil {
			continue
		}

		if comments.Valid {
			st.Comments = &comments.String
		}

		types = append(types, st)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    types,
	})
}