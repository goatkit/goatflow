package api

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"
	"github.com/lib/pq"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/shared"
) // Service represents a service in OTRS
type Service struct {
	ID         int       `json:"id"`
	Name       string    `json:"name"`
	Comments   *string   `json:"comments,omitempty"`
	ValidID    int       `json:"valid_id"`
	CreateTime time.Time `json:"create_time"`
	CreateBy   int       `json:"create_by"`
	ChangeTime time.Time `json:"change_time"`
	ChangeBy   int       `json:"change_by"`
}

// ServiceWithStats includes additional statistics.
type ServiceWithStats struct {
	Service
	TicketCount int `json:"ticket_count"`
	SLACount    int `json:"sla_count"`
}

// handleAdminServices renders the admin services management page.
func handleAdminServices(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		log.Printf("handleAdminServices: database unavailable: %v", err)
		sendErrorResponse(c, http.StatusInternalServerError, "Database unavailable")
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
			s.id, s.name, s.comments, s.valid_id,
			s.create_time, s.create_by, s.change_time, s.change_by,
			COUNT(DISTINCT t.id) as ticket_count,
			COUNT(DISTINCT sla.id) as sla_count
		FROM service s
		LEFT JOIN ticket t ON t.service_id = s.id
		LEFT JOIN sla ON sla.id IN (
			SELECT id FROM sla WHERE valid_id = 1
		)
		WHERE 1=1
	`

	var args []interface{}

	if searchQuery != "" {
		query += " AND (LOWER(s.name) LIKE ? OR LOWER(s.comments) LIKE ?)"
		searchPattern := "%" + strings.ToLower(searchQuery) + "%"
		args = append(args, searchPattern, searchPattern)
	}

	if validFilter != "all" {
		if validFilter == "valid" {
			query += " AND s.valid_id = ?"
			args = append(args, 1)
		} else if validFilter == "invalid" {
			query += " AND s.valid_id = ?"
			args = append(args, 2)
		} else if validFilter == "invalid-temporarily" {
			query += " AND s.valid_id = ?"
			args = append(args, 3)
		}
	}

	query += " GROUP BY s.id, s.name, s.comments, s.valid_id, s.create_time, s.create_by, s.change_time, s.change_by"

	// Add sorting
	validSortColumns := map[string]bool{
		"id": true, "name": true, "valid_id": true, "ticket_count": true,
	}
	if !validSortColumns[sortBy] {
		sortBy = "name"
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "asc"
	}
	query += fmt.Sprintf(" ORDER BY %s %s", sortBy, sortOrder)

	rows, err := db.Query(database.ConvertPlaceholders(query), args...)
	if err != nil {
		log.Printf("handleAdminServices: query services: %v", err)
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to fetch services")
		return
	}
	defer rows.Close()

	var services []ServiceWithStats
	for rows.Next() {
		var s ServiceWithStats
		var comments sql.NullString

		err := rows.Scan(
			&s.ID, &s.Name, &comments, &s.ValidID,
			&s.CreateTime, &s.CreateBy, &s.ChangeTime, &s.ChangeBy,
			&s.TicketCount, &s.SLACount,
		)
		if err != nil {
			continue
		}

		if comments.Valid {
			s.Comments = &comments.String
		}

		services = append(services, s)
	}
	_ = rows.Err() //nolint:errcheck // Iteration complete, data already collected

	// Render the template or fallback if renderer not initialized
	if getPongo2Renderer() == nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Template renderer unavailable")
		return
	}
	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/services.pongo2", pongo2.Context{
		"Title":       "Service Management",
		"Services":    services,
		"SearchQuery": searchQuery,
		"ValidFilter": validFilter,
		"SortBy":      sortBy,
		"SortOrder":   sortOrder,
		"User":        getUserMapForTemplate(c),
		"ActivePage":  "admin",
	})
}

// handleAdminServiceCreate creates a new service.
func handleAdminServiceCreate(c *gin.Context) {
	var input struct {
		Name     string  `json:"name" form:"name"`
		Comments *string `json:"comments" form:"comments"`
		ValidID  int     `json:"valid_id" form:"valid_id"`
	}

	// Default valid_id to 1 if not provided
	input.ValidID = 1

	var err error
	if c.ContentType() == "application/json" {
		err = c.ShouldBindJSON(&input)
	} else {
		err = c.ShouldBind(&input)
	}
	if err != nil {
		log.Printf("Service create bind error: %v", err)
		shared.SendToastResponse(c, false, "Name is required", "")
		return
	}

	// Validate name is not empty
	if strings.TrimSpace(input.Name) == "" {
		log.Printf("Service create: name is empty after bind, input=%+v", input)
		shared.SendToastResponse(c, false, "Name is required", "")
		return
	}

	actorID, ok := auditUserID(c)
	if !ok {
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		log.Printf("handleAdminServiceCreate: database unavailable: %v", err)
		shared.SendToastError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	// Check for duplicate name
	var exists bool
	err = db.QueryRow(database.ConvertPlaceholders("SELECT EXISTS(SELECT 1 FROM service WHERE name = ?)"), input.Name).Scan(&exists)
	if err != nil {
		shared.SendToastResponse(c, false, "Failed to check for duplicate", "")
		return
	}

	if exists {
		shared.SendToastResponse(c, false, "Service with this name already exists", "")
		return
	}

	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO service (name, comments, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP, ?)
		RETURNING id
	`)
	id64, err := database.GetAdapter().InsertWithReturning(db, insertQuery, input.Name, input.Comments, input.ValidID, actorID, actorID)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			shared.SendToastResponse(c, false, "Service with this name already exists", "")
			return
		}
		if database.IsMySQL() && strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			shared.SendToastResponse(c, false, "Service with this name already exists", "")
			return
		}
		shared.SendToastResponse(c, false, "Failed to create service", "")
		return
	}
	_ = id64 // ID available if needed

	shared.SendToastResponse(c, true, "Service created successfully", "/admin/services")
}

// handleAdminServiceUpdate updates an existing service.
func handleAdminServiceUpdate(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		shared.SendToastResponse(c, false, "Invalid service ID", "")
		return
	}

	var input struct {
		Name     string  `json:"name" form:"name"`
		Comments *string `json:"comments" form:"comments"`
		ValidID  *int    `json:"valid_id" form:"valid_id"`
	}

	if err := c.ShouldBind(&input); err != nil {
		shared.SendToastResponse(c, false, "Invalid input", "")
		return
	}

	actorID, ok := auditUserID(c)
	if !ok {
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		log.Printf("handleAdminServiceUpdate: database unavailable: %v", err)
		shared.SendToastError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	// Build update query dynamically
	updates := []string{"change_time = CURRENT_TIMESTAMP", "change_by = ?"}
	args := []interface{}{actorID}

	if input.Name != "" {
		updates = append(updates, "name = ?")
		args = append(args, input.Name)
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
	query := fmt.Sprintf("UPDATE service SET %s WHERE id = ?", strings.Join(updates, ", ")) //nolint:gk-sql-sprintf // hardcoded column fragments; user values bound via ?

	result, err := db.Exec(database.ConvertPlaceholders(query), args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			shared.SendToastResponse(c, false, "Service with this name already exists", "")
			return
		}
		shared.SendToastResponse(c, false, "Failed to update service", "")
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		rowsAffected = 0
	}
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Service not found"})
		return
	}

	shared.SendToastResponse(c, true, "Service updated successfully", "")
}

// handleAdminServiceDelete soft deletes a service (sets valid_id = 2).
func handleAdminServiceDelete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		shared.SendToastResponse(c, false, "Invalid service ID", "")
		return
	}

	actorID, ok := auditUserID(c)
	if !ok {
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		log.Printf("handleAdminServiceDelete: database unavailable: %v", err)
		shared.SendToastError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	// Check if service has associated tickets
	var ticketCount int
	err = db.QueryRow(database.ConvertPlaceholders("SELECT COUNT(*) FROM ticket WHERE service_id = ?"), id).Scan(&ticketCount)
	if err != nil {
		shared.SendToastResponse(c, false, "Failed to check ticket dependencies", "")
		return
	}

	// In OTRS, services are typically soft-deleted (marked invalid) rather than hard deleted
	// This preserves referential integrity with existing tickets
	result, err := db.Exec(database.ConvertPlaceholders(`
		UPDATE service
		SET valid_id = 2, change_time = CURRENT_TIMESTAMP, change_by = ?
		WHERE id = ?
	`), actorID, id)

	if err != nil {
		shared.SendToastResponse(c, false, "Failed to delete service", "")
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		rowsAffected = 0
	}
	if rowsAffected == 0 {
		shared.SendToastResponse(c, false, "Service not found", "")
		return
	}

	shared.SendToastResponse(c, true, "Service deleted successfully", "")
}
