package api

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	"github.com/lib/pq"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/shared"
)

// TicketType represents a ticket type.
type TicketType struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	ValidID     int    `json:"valid_id"`
	CreateBy    int    `json:"create_by"`
	ChangeBy    int    `json:"change_by"`
	TicketCount int    `json:"ticket_count"`
}

// handleAdminTypes handles the ticket types management page.
func handleAdminTypes(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		log.Printf("handleAdminTypes: database unavailable: %v", err)
		sendErrorResponse(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	// Get query parameters
	search := c.Query("search")
	sort := c.DefaultQuery("sort", "name")
	order := c.DefaultQuery("order", "asc")

	// Build query
	query := `
		SELECT 
			t.id,
			t.name,
			t.valid_id,
			t.create_by,
			t.change_by,
			COUNT(DISTINCT tk.id) as ticket_count
		FROM ticket_type t
		LEFT JOIN ticket tk ON tk.type_id = t.id
	`

	var args []interface{}

	if search != "" {
		query += " WHERE LOWER(t.name) LIKE LOWER(?)"
		args = append(args, "%"+search+"%")
	}

	query += " GROUP BY t.id, t.name, t.valid_id, t.create_by, t.change_by"

	// Add sorting
	switch sort {
	case "name":
		query += " ORDER BY t.name"
	case "tickets":
		query += " ORDER BY ticket_count"
	default:
		query += " ORDER BY t.name"
	}

	if order == "desc" {
		query += " DESC"
	} else {
		query += " ASC"
	}

	rows, err := db.Query(database.ConvertPlaceholders(query), args...)
	if err != nil {
		log.Printf("handleAdminTypes: query ticket types: %v", err)
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to load ticket types")
		return
	}
	defer rows.Close()

	var types []TicketType
	for rows.Next() {
		var t TicketType
		err := rows.Scan(&t.ID, &t.Name, &t.ValidID, &t.CreateBy, &t.ChangeBy, &t.TicketCount)
		if err != nil {
			continue
		}
		types = append(types, t)
	}
	_ = rows.Err() //nolint:errcheck // Iteration errors don't affect UI

	// Render template or fallback if renderer not initialized
	renderer := getPongo2Renderer()
	if renderer == nil || renderer.TemplateSet() == nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Template renderer unavailable")
		return
	}
	renderer.HTML(c, http.StatusOK, "pages/admin/types.pongo2", pongo2.Context{
		"Title":      "Ticket Type Management",
		"User":       getUserMapForTemplate(c),
		"Types":      types,
		"Search":     search,
		"Sort":       sort,
		"Order":      order,
		"ActivePage": "admin",
	})
}

// handleAdminTypeCreate creates a new ticket type (POST /admin/types/create).
func handleAdminTypeCreate(c *gin.Context) {
	var input struct {
		Name    string `json:"name" form:"name"`
		ValidID int    `json:"valid_id" form:"valid_id"`
	}

	isHX := c.GetHeader("HX-Request") == "true"
	respondError := func(status int, msg string) {
		if isHX {
			shared.SendToastResponse(c, false, msg, "")
		} else {
			c.JSON(status, gin.H{"success": false, "error": msg})
		}
	}
	respondSuccess := func(status int, msg string) {
		if isHX {
			shared.SendToastResponse(c, true, msg, "/admin/types")
		} else {
			c.JSON(status, gin.H{"success": true, "message": msg})
		}
	}

	if err := c.ShouldBind(&input); err != nil {
		respondError(http.StatusBadRequest, "Invalid request body")
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		respondError(http.StatusBadRequest, "Name is required")
		return
	}
	if len(input.Name) > 200 {
		respondError(http.StatusBadRequest, "Name must be less than 200 characters")
		return
	}
	if input.ValidID == 0 {
		input.ValidID = 1
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		respondError(http.StatusInternalServerError, "Database unavailable")
		return
	}

	if taken, err := typeNameTaken(db, input.Name, 0); err != nil {
		respondError(http.StatusInternalServerError, "Failed to create type")
		return
	} else if taken {
		respondError(http.StatusBadRequest, "A type with this name already exists")
		return
	}

	userID := GetUserIDFromCtx(c, 1)
	if _, err := db.Exec(database.ConvertPlaceholders(`
		INSERT INTO ticket_type (name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP, ?)
	`), input.Name, input.ValidID, userID, userID); err != nil {
		if isUniqueViolation(err) {
			respondError(http.StatusBadRequest, "A type with this name already exists")
			return
		}
		respondError(http.StatusInternalServerError, "Failed to create type")
		return
	}

	respondSuccess(http.StatusCreated, "Type created successfully")
}

// handleAdminTypeUpdate updates an existing ticket type (POST /admin/types/:id/update).
func handleAdminTypeUpdate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid type ID"})
		return
	}

	var input struct {
		Name    string `json:"name" form:"name"`
		ValidID *int   `json:"valid_id" form:"valid_id"`
	}

	isHX := c.GetHeader("HX-Request") == "true"
	respondError := func(status int, msg string) {
		if isHX {
			shared.SendToastResponse(c, false, msg, "")
		} else {
			c.JSON(status, gin.H{"success": false, "error": msg})
		}
	}
	respondSuccess := func(msg string) {
		if isHX {
			shared.SendToastResponse(c, true, msg, "/admin/types")
		} else {
			c.JSON(http.StatusOK, gin.H{"success": true, "message": msg})
		}
	}

	if err := c.ShouldBind(&input); err != nil {
		respondError(http.StatusBadRequest, "Invalid request body")
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		respondError(http.StatusBadRequest, "Name cannot be empty")
		return
	}
	if len(input.Name) > 200 {
		respondError(http.StatusBadRequest, "Name must be less than 200 characters")
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		respondError(http.StatusInternalServerError, "Database unavailable")
		return
	}

	if found, err := typeExists(db, id); err != nil {
		respondError(http.StatusInternalServerError, "Failed to update type")
		return
	} else if !found {
		respondError(http.StatusNotFound, "Type not found")
		return
	}
	if taken, err := typeNameTaken(db, input.Name, id); err != nil {
		respondError(http.StatusInternalServerError, "Failed to update type")
		return
	} else if taken {
		respondError(http.StatusBadRequest, "A type with this name already exists")
		return
	}

	query := "UPDATE ticket_type SET change_by = ?, change_time = CURRENT_TIMESTAMP, name = ?"
	args := []interface{}{GetUserIDFromCtx(c, 1), input.Name}
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
			respondError(http.StatusBadRequest, "A type with this name already exists")
			return
		}
		respondError(http.StatusInternalServerError, "Failed to update type")
		return
	}

	respondSuccess("Type updated successfully")
}

// handleAdminTypeDelete soft-deletes a ticket type (POST /admin/types/:id/delete).
func handleAdminTypeDelete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid type ID"})
		return
	}

	isHX := c.GetHeader("HX-Request") == "true"
	respondError := func(status int, msg string) {
		if isHX {
			shared.SendToastResponse(c, false, msg, "")
		} else {
			c.JSON(status, gin.H{"success": false, "error": msg})
		}
	}
	respondSuccess := func(msg string) {
		if isHX {
			shared.SendToastResponse(c, true, msg, "/admin/types")
		} else {
			c.JSON(http.StatusOK, gin.H{"success": true, "message": msg})
		}
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		respondError(http.StatusInternalServerError, "Database unavailable")
		return
	}

	if found, err := typeExists(db, id); err != nil {
		respondError(http.StatusInternalServerError, "Failed to delete type")
		return
	} else if !found {
		respondError(http.StatusNotFound, "Type not found")
		return
	}

	var ticketCount int
	if err := db.QueryRow(database.ConvertPlaceholders(
		`SELECT COUNT(*) FROM ticket WHERE type_id = ?`), id).Scan(&ticketCount); err != nil {
		respondError(http.StatusInternalServerError, "Failed to check type usage")
		return
	}
	if ticketCount > 0 {
		respondError(http.StatusBadRequest, fmt.Sprintf("Cannot delete type: %d tickets are using it", ticketCount))
		return
	}

	if _, err := db.Exec(database.ConvertPlaceholders(`
		UPDATE ticket_type
		SET valid_id = 2, change_by = ?, change_time = CURRENT_TIMESTAMP
		WHERE id = ?
	`), GetUserIDFromCtx(c, 1), id); err != nil {
		respondError(http.StatusInternalServerError, "Failed to delete type")
		return
	}

	respondSuccess("Type deleted successfully")
}

func typeExists(db *sql.DB, id int) (bool, error) {
	var n int
	err := db.QueryRow(database.ConvertPlaceholders(`SELECT COUNT(*) FROM ticket_type WHERE id = ?`), id).Scan(&n)
	return n > 0, err
}

func typeNameTaken(db *sql.DB, name string, exceptID int) (bool, error) {
	var n int
	err := db.QueryRow(database.ConvertPlaceholders(
		`SELECT COUNT(*) FROM ticket_type WHERE name = ? AND id <> ?`), name, exceptID).Scan(&n)
	return n > 0, err
}

// isUniqueViolation reports a unique-constraint violation on either driver.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		return true
	}
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
