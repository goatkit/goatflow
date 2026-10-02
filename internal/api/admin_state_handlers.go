package api

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// State represents a ticket state.
type State struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	TypeID   *int    `json:"type_id,omitempty"`
	Comments *string `json:"comments,omitempty"`
	ValidID  *int    `json:"valid_id,omitempty"`
}

// StateType represents a ticket state type.
type StateType struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	Comments *string `json:"comments,omitempty"`
}

// StateWithType includes type information.
type StateWithType struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	TypeID      int     `json:"type_id"`
	TypeName    string  `json:"type_name"`
	Comments    *string `json:"comments,omitempty"`
	ValidID     int     `json:"valid_id"`
	TicketCount int     `json:"ticket_count"`
}

// handleAdminStates renders the admin states management page.
func handleAdminStates(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		log.Printf("handleAdminStates: database unavailable: %v", err)
		sendErrorResponse(c, http.StatusInternalServerError, "Database unavailable")
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

	if searchQuery != "" {
		query += " AND (LOWER(s.name) LIKE ? OR LOWER(s.comments) LIKE ?)"
		searchPattern := "%" + strings.ToLower(searchQuery) + "%"
		args = append(args, searchPattern, searchPattern)
	}

	if typeFilter != "" {
		query += " AND s.type_id = ?"
		args = append(args, typeFilter)
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

	rows, err := db.Query(database.ConvertPlaceholders(query), args...)
	if err != nil {
		log.Printf("handleAdminStates: query states: %v", err)
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to load ticket states")
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
	_ = rows.Err() //nolint:errcheck // Iteration complete

	// Get state types for dropdown
	typeRows, err := db.Query(database.ConvertPlaceholders("SELECT id, name, comments FROM ticket_state_type ORDER BY id"))
	if err != nil {
		log.Printf("handleAdminStates: query state types: %v", err)
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to load state types")
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
	_ = typeRows.Err() //nolint:errcheck // Iteration complete
	// Convert typeFilter to int for template comparison
	var typeFilterInt int
	if typeFilter != "" {
		typeFilterInt, _ = strconv.Atoi(typeFilter) //nolint:errcheck // Defaults to 0 on error
	}

	renderer := getPongo2Renderer()
	if renderer == nil || renderer.TemplateSet() == nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Template renderer unavailable")
		return
	}
	renderer.HTML(c, http.StatusOK, "pages/admin/states.pongo2", pongo2.Context{
		"Title":       "Ticket States",
		"States":      states,
		"StateTypes":  stateTypes,
		"SearchQuery": searchQuery,
		"TypeFilter":  typeFilterInt,
		"SortBy":      sortBy,
		"SortOrder":   sortOrder,
		"User":        getUserMapForTemplate(c),
		"ActivePage":  "admin",
	})
}

// handleAdminStateCreate creates a new state (POST /admin/states/create).
func handleAdminStateCreate(c *gin.Context) {
	var input struct {
		Name     string  `json:"name" form:"name"`
		TypeID   int     `json:"type_id" form:"type_id"`
		Comments *string `json:"comments" form:"comments"`
		ValidID  int     `json:"valid_id" form:"valid_id"`
	}
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request body"})
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Name is required"})
		return
	}
	if input.ValidID == 0 {
		input.ValidID = 1
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database unavailable"})
		return
	}

	if ok, err := stateTypeExists(db, input.TypeID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to validate state type"})
		return
	} else if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid state type"})
		return
	}
	if taken, err := stateNameTaken(db, input.Name, 0); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to create state"})
		return
	} else if taken {
		c.JSON(http.StatusConflict, gin.H{"success": false, "error": "A state with this name already exists"})
		return
	}

	userID := GetUserIDFromCtx(c, 1)
	id64, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO ticket_state (name, type_id, comments, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP, ?)
		RETURNING id`), input.Name, input.TypeID, input.Comments, input.ValidID, userID, userID)
	if err != nil {
		if isUniqueViolation(err) {
			c.JSON(http.StatusConflict, gin.H{"success": false, "error": "A state with this name already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to create state"})
		return
	}

	typeID, validID := input.TypeID, input.ValidID
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "State created successfully",
		"data":    State{ID: int(id64), Name: input.Name, TypeID: &typeID, Comments: input.Comments, ValidID: &validID},
	})
}

// handleAdminStateUpdate updates an existing state (PUT /admin/states/:id/update).
func handleAdminStateUpdate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid state ID"})
		return
	}

	var input struct {
		Name     string  `json:"name" form:"name"`
		TypeID   *int    `json:"type_id" form:"type_id"`
		Comments *string `json:"comments" form:"comments"`
		ValidID  *int    `json:"valid_id" form:"valid_id"`
	}
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request body"})
		return
	}
	input.Name = strings.TrimSpace(input.Name)

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database unavailable"})
		return
	}

	if found, err := stateExists(db, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update state"})
		return
	} else if !found {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "State not found"})
		return
	}
	if input.TypeID != nil {
		if ok, err := stateTypeExists(db, *input.TypeID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to validate state type"})
			return
		} else if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid state type"})
			return
		}
	}
	if input.Name != "" {
		if taken, err := stateNameTaken(db, input.Name, id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update state"})
			return
		} else if taken {
			c.JSON(http.StatusConflict, gin.H{"success": false, "error": "A state with this name already exists"})
			return
		}
	}

	query := `UPDATE ticket_state SET change_by = ?, change_time = CURRENT_TIMESTAMP`
	args := []interface{}{GetUserIDFromCtx(c, 1)}
	if input.Name != "" {
		query += ", name = ?"
		args = append(args, input.Name)
	}
	if input.TypeID != nil {
		query += ", type_id = ?"
		args = append(args, *input.TypeID)
	}
	if input.Comments != nil {
		query += ", comments = ?"
		args = append(args, *input.Comments)
	}
	if input.ValidID != nil {
		query += ", valid_id = ?"
		args = append(args, *input.ValidID)
	}
	query += " WHERE id = ?"
	args = append(args, id)

	// Existence was checked above: MySQL reports 0 affected rows when nothing
	// changed, so RowsAffected cannot be used as a not-found signal.
	if _, err := db.Exec(database.ConvertPlaceholders(query), args...); err != nil {
		if isUniqueViolation(err) {
			c.JSON(http.StatusConflict, gin.H{"success": false, "error": "A state with this name already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update state"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "State updated successfully"})
}

// handleAdminStateDelete soft deletes a state (DELETE /admin/states/:id/delete).
func handleAdminStateDelete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid state ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database unavailable"})
		return
	}

	if found, err := stateExists(db, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete state"})
		return
	} else if !found {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "State not found"})
		return
	}

	var ticketCount int
	if err := db.QueryRow(database.ConvertPlaceholders(
		"SELECT COUNT(*) FROM ticket WHERE ticket_state_id = ?"), id).Scan(&ticketCount); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to check state usage"})
		return
	}
	if ticketCount > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Cannot delete state: %d tickets are using this state", ticketCount),
		})
		return
	}

	if _, err := db.Exec(database.ConvertPlaceholders(`
		UPDATE ticket_state
		SET valid_id = 2, change_by = ?, change_time = CURRENT_TIMESTAMP
		WHERE id = ?`), GetUserIDFromCtx(c, 1), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete state"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "State deleted successfully"})
}

func stateExists(db *sql.DB, id int) (bool, error) {
	var n int
	err := db.QueryRow(database.ConvertPlaceholders(`SELECT COUNT(*) FROM ticket_state WHERE id = ?`), id).Scan(&n)
	return n > 0, err
}

func stateTypeExists(db *sql.DB, typeID int) (bool, error) {
	var n int
	err := db.QueryRow(database.ConvertPlaceholders(`SELECT COUNT(*) FROM ticket_state_type WHERE id = ?`), typeID).Scan(&n)
	return n > 0, err
}

func stateNameTaken(db *sql.DB, name string, exceptID int) (bool, error) {
	var n int
	err := db.QueryRow(database.ConvertPlaceholders(
		`SELECT COUNT(*) FROM ticket_state WHERE name = ? AND id <> ?`), name, exceptID).Scan(&n)
	return n > 0, err
}

// handleGetStateTypes returns all state types.
func handleGetStateTypes(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Database unavailable"})
		return
	}
	rows, err := db.Query(database.ConvertPlaceholders("SELECT id, name, comments FROM ticket_state_type ORDER BY id"))
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
	_ = rows.Err() //nolint:errcheck // Iteration errors don't affect UI

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    types,
	})
}
