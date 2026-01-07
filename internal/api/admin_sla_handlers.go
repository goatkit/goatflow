package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"
	"github.com/lib/pq"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// SLADefinition represents a Service Level Agreement configuration.
type SLADefinition struct {
	ID                  int       `json:"id"`
	Name                string    `json:"name"`
	CalendarName        *string   `json:"calendar_name,omitempty"`
	FirstResponseTime   *int      `json:"first_response_time,omitempty"`
	FirstResponseNotify *int      `json:"first_response_notify,omitempty"`
	UpdateTime          *int      `json:"update_time,omitempty"`
	UpdateNotify        *int      `json:"update_notify,omitempty"`
	SolutionTime        *int      `json:"solution_time,omitempty"`
	SolutionNotify      *int      `json:"solution_notify,omitempty"`
	Comments            *string   `json:"comments,omitempty"`
	ValidID             int       `json:"valid_id"`
	CreateTime          time.Time `json:"create_time"`
	CreateBy            int       `json:"create_by"`
	ChangeTime          time.Time `json:"change_time"`
	ChangeBy            int       `json:"change_by"`
}

// SLAWithStats includes additional statistics.
type SLAWithStats struct {
	SLADefinition
	TicketCount int `json:"ticket_count"`
}

// handleAdminSLA renders the admin SLA management page.
func handleAdminSLA(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.String(http.StatusInternalServerError, "Database connection failed")
		return
	}

	// Get search and filter parameters
	searchQuery := c.Query("search")
	validFilter := c.DefaultQuery("valid", "all")
	sortBy := c.DefaultQuery("sort", "name")
	sortOrder := c.DefaultQuery("order", "asc")

	// Build query with filters
	query := `
		SELECT 
			s.id, s.name, s.calendar_name,
			s.first_response_time, s.first_response_notify,
			s.update_time, s.update_notify,
			s.solution_time, s.solution_notify,
			s.comments, s.valid_id,
			s.create_time, s.create_by, s.change_time, s.change_by,
			COUNT(DISTINCT t.id) as ticket_count
		FROM sla s
		LEFT JOIN ticket t ON t.sla_id = s.id
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

	if validFilter != "all" {
		if validFilter == "valid" {
			query += fmt.Sprintf(" AND s.valid_id = $%d", argCount)
			args = append(args, 1)
		} else if validFilter == "invalid" {
			query += fmt.Sprintf(" AND s.valid_id = $%d", argCount)
			args = append(args, 2)
		}
	}

	query += ` GROUP BY s.id, s.name, s.calendar_name,
		s.first_response_time, s.first_response_notify,
		s.update_time, s.update_notify,
		s.solution_time, s.solution_notify,
		s.comments, s.valid_id,
		s.create_time, s.create_by, s.change_time, s.change_by`

	// Add sorting
	validSortColumns := map[string]bool{
		"id": true, "name": true, "valid_id": true, "ticket_count": true,
		"first_response_time": true, "solution_time": true,
	}
	if !validSortColumns[sortBy] {
		sortBy = "name"
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "asc"
	}
	query += fmt.Sprintf(" ORDER BY %s %s", sortBy, sortOrder)

	rows, err := db.Query(query, args...)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to fetch SLAs")
		return
	}
	defer rows.Close()

	var slas []SLAWithStats
	for rows.Next() {
		var s SLAWithStats
		var calendarName, comments sql.NullString
		var firstResponseTime, firstResponseNotify sql.NullInt32
		var updateTime, updateNotify sql.NullInt32
		var solutionTime, solutionNotify sql.NullInt32

		err := rows.Scan(
			&s.ID, &s.Name, &calendarName,
			&firstResponseTime, &firstResponseNotify,
			&updateTime, &updateNotify,
			&solutionTime, &solutionNotify,
			&comments, &s.ValidID,
			&s.CreateTime, &s.CreateBy, &s.ChangeTime, &s.ChangeBy,
			&s.TicketCount,
		)
		if err != nil {
			continue
		}

		if calendarName.Valid {
			s.CalendarName = &calendarName.String
		}
		if comments.Valid {
			s.Comments = &comments.String
		}
		if firstResponseTime.Valid {
			val := int(firstResponseTime.Int32)
			s.FirstResponseTime = &val
		}
		if firstResponseNotify.Valid {
			val := int(firstResponseNotify.Int32)
			s.FirstResponseNotify = &val
		}
		if updateTime.Valid {
			val := int(updateTime.Int32)
			s.UpdateTime = &val
		}
		if updateNotify.Valid {
			val := int(updateNotify.Int32)
			s.UpdateNotify = &val
		}
		if solutionTime.Valid {
			val := int(solutionTime.Int32)
			s.SolutionTime = &val
		}
		if solutionNotify.Valid {
			val := int(solutionNotify.Int32)
			s.SolutionNotify = &val
		}

		slas = append(slas, s)
	}
	_ = rows.Err() //nolint:errcheck // Iteration complete

	// Get calendars for dropdown (if calendar table exists)
	var calendars []string
	calRows, err := db.Query("SELECT name FROM calendar ORDER BY name")
	if err == nil {
		defer calRows.Close()
		for calRows.Next() {
			var name string
			if err := calRows.Scan(&name); err == nil {
				calendars = append(calendars, name)
			}
		}
		_ = calRows.Err() //nolint:errcheck // Iteration complete
	}

	// Render the template
	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/sla.pongo2", pongo2.Context{
		"Title":       "SLA Management",
		"SLAs":        slas,
		"Calendars":   calendars,
		"SearchQuery": searchQuery,
		"ValidFilter": validFilter,
		"SortBy":      sortBy,
		"SortOrder":   sortOrder,
		"User":        getUserMapForTemplate(c),
		"ActivePage":  "admin",
	})
}

// handleAdminSLACreate creates a new SLA.
func handleAdminSLACreate(c *gin.Context) {
	var input struct {
		Name                string  `json:"name" form:"name" binding:"required"`
		CalendarName        *string `json:"calendar_name" form:"calendar_name"`
		FirstResponseTime   *int    `json:"first_response_time" form:"first_response_time"`
		FirstResponseNotify *int    `json:"first_response_notify" form:"first_response_notify"`
		UpdateTime          *int    `json:"update_time" form:"update_time"`
		UpdateNotify        *int    `json:"update_notify" form:"update_notify"`
		SolutionTime        *int    `json:"solution_time" form:"solution_time"`
		SolutionNotify      *int    `json:"solution_notify" form:"solution_notify"`
		Comments            *string `json:"comments" form:"comments"`
		ValidID             int     `json:"valid_id" form:"valid_id"`
	}

	// Default valid_id to 1 if not provided
	input.ValidID = 1

	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Name is required",
		})
		return
	}

	// Validate name is not empty
	if strings.TrimSpace(input.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Name is required",
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

	// Check for duplicate name
	var exists bool
	err = db.QueryRow(database.ConvertPlaceholders("SELECT EXISTS(SELECT 1 FROM sla WHERE name = ?)"), input.Name).Scan(&exists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to check for duplicate",
		})
		return
	}

	if exists {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "SLA with this name already exists",
		})
		return
	}

	// Convert nil pointers to 0 for NOT NULL columns
	firstResponseTime := 0
	if input.FirstResponseTime != nil {
		firstResponseTime = *input.FirstResponseTime
	}
	updateTime := 0
	if input.UpdateTime != nil {
		updateTime = *input.UpdateTime
	}
	solutionTime := 0
	if input.SolutionTime != nil {
		solutionTime = *input.SolutionTime
	}

	// Insert the new SLA using database adapter for MySQL/PostgreSQL compatibility
	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO sla (
			name, calendar_name, 
			first_response_time, first_response_notify,
			update_time, update_notify,
			solution_time, solution_notify,
			comments, valid_id, 
			create_time, create_by, change_time, change_by
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1
		) RETURNING id
	`)

	adapter := database.GetAdapter()
	id64, err := adapter.InsertWithReturning(db, insertQuery,
		input.Name, input.CalendarName,
		firstResponseTime, input.FirstResponseNotify,
		updateTime, input.UpdateNotify,
		solutionTime, input.SolutionNotify,
		input.Comments, input.ValidID)
	id := int(id64)

	if err != nil {
		// Check for MySQL duplicate key error
		errStr := err.Error()
		if strings.Contains(errStr, "Duplicate entry") || strings.Contains(errStr, "duplicate key") {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "SLA with this name already exists",
			})
			return
		}
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "SLA with this name already exists",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to create SLA: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "SLA created successfully",
		"data": gin.H{
			"id":   id,
			"name": input.Name,
		},
	})
}

// handleAdminSLAUpdate updates an existing SLA.
func handleAdminSLAUpdate(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid SLA ID",
		})
		return
	}

	var input struct {
		Name                *string `json:"name" form:"name"`
		CalendarName        *string `json:"calendar_name" form:"calendar_name"`
		FirstResponseTime   *int    `json:"first_response_time" form:"first_response_time"`
		FirstResponseNotify *int    `json:"first_response_notify" form:"first_response_notify"`
		UpdateTime          *int    `json:"update_time" form:"update_time"`
		UpdateNotify        *int    `json:"update_notify" form:"update_notify"`
		SolutionTime        *int    `json:"solution_time" form:"solution_time"`
		SolutionNotify      *int    `json:"solution_notify" form:"solution_notify"`
		Comments            *string `json:"comments" form:"comments"`
		ValidID             *int    `json:"valid_id" form:"valid_id"`
	}

	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid input",
		})
		return
	}

	// Build update query dynamically using ? placeholders
	updates := []string{"change_time = CURRENT_TIMESTAMP", "change_by = 1"}
	args := []interface{}{}

	if input.Name != nil && *input.Name != "" {
		updates = append(updates, "name = ?")
		args = append(args, *input.Name)
	}

	if input.CalendarName != nil {
		updates = append(updates, "calendar_name = ?")
		args = append(args, *input.CalendarName)
	}

	if input.FirstResponseTime != nil {
		updates = append(updates, "first_response_time = ?")
		args = append(args, *input.FirstResponseTime)
	}

	if input.FirstResponseNotify != nil {
		updates = append(updates, "first_response_notify = ?")
		args = append(args, *input.FirstResponseNotify)
	}

	if input.UpdateTime != nil {
		updates = append(updates, "update_time = ?")
		args = append(args, *input.UpdateTime)
	}

	if input.UpdateNotify != nil {
		updates = append(updates, "update_notify = ?")
		args = append(args, *input.UpdateNotify)
	}

	if input.SolutionTime != nil {
		updates = append(updates, "solution_time = ?")
		args = append(args, *input.SolutionTime)
	}

	if input.SolutionNotify != nil {
		updates = append(updates, "solution_notify = ?")
		args = append(args, *input.SolutionNotify)
	}

	if input.Comments != nil {
		updates = append(updates, "comments = ?")
		args = append(args, *input.Comments)
	}

	if input.ValidID != nil {
		updates = append(updates, "valid_id = ?")
		args = append(args, *input.ValidID)
	}

	args = append(args, id)

	// Use QueryBuilder for proper placeholder rebinding
	qb, err := database.GetQueryBuilder()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	query := qb.Rebind("UPDATE sla SET " + strings.Join(updates, ", ") + " WHERE id = ?")
	result, err := qb.Exec(query, args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "SLA with this name already exists",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to update SLA",
		})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		rowsAffected = 0
	}
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "SLA not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "SLA updated successfully",
	})
}

// handleAdminSLADelete soft deletes an SLA (sets valid_id = 2).
func handleAdminSLADelete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid SLA ID",
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

	// Check if SLA has associated tickets
	var ticketCount int
	err = db.QueryRow(database.ConvertPlaceholders("SELECT COUNT(*) FROM ticket WHERE sla_id = ?"), id).Scan(&ticketCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to check ticket dependencies",
		})
		return
	}

	// Soft delete (mark as invalid)
	result, err := db.Exec(database.ConvertPlaceholders(`
		UPDATE sla 
		SET valid_id = 2, change_time = CURRENT_TIMESTAMP, change_by = 1 
		WHERE id = ?
	`), id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to delete SLA",
		})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		rowsAffected = 0
	}
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "SLA not found",
		})
		return
	}

	message := "SLA deleted successfully"
	if ticketCount > 0 {
		message = fmt.Sprintf("SLA deactivated successfully (has %d associated tickets)", ticketCount)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": message,
	})
}
