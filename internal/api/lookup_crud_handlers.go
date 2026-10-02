package api

import (
	"database/sql"
	"errors"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// Helper for optional string pointers.
func valueOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Type CRUD handlers.
func handleCreateType(c *gin.Context) {
	if !checkAdminPermission(c) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Admin access required"})
		return
	}

	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	name, _ := body["name"].(string)         //nolint:errcheck // Defaults to empty
	comments, _ := body["comments"].(string) //nolint:errcheck // Defaults to empty
	validID := 1
	if v, ok := body["valid_id"].(float64); ok {
		validID = int(v)
	}
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Name is required"})
		return
	}
	actorID, ok := auditUserID(c)
	if !ok {
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		log.Printf("handleCreateType: database unavailable: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database unavailable"})
		return
	}
	// Note: ticket_type table doesn't have a comments column
	query := database.ConvertPlaceholders(`INSERT INTO ticket_type (name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, NOW(), ?, NOW(), ?) RETURNING id`)
	newID, err := database.GetAdapter().InsertWithReturning(db, query, name, validID, actorID, actorID)
	if err != nil {
		log.Printf("handleCreateType: insert: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to create type"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"id":       newID,
			"name":     name,
			"comments": comments, // Return for API compatibility even though not stored
			"valid_id": validID,
		},
	})
}

func handleUpdateType(c *gin.Context) {
	if !checkAdminPermission(c) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Admin access required"})
		return
	}

	idStr := strings.TrimSpace(c.Param("id"))
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid type ID"})
		return
	}

	// Pointer fields detect presence: omitted fields keep their stored value.
	var body struct {
		Name     *string `json:"name"`
		Comments *string `json:"comments"`
		ValidID  *int    `json:"valid_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Name != nil && strings.TrimSpace(*body.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Name is required"})
		return
	}
	actorID, ok := auditUserID(c)
	if !ok {
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		log.Printf("handleUpdateType: database unavailable: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database unavailable"})
		return
	}
	if !fitsSmallint(id) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Type not found"})
		return
	}
	// Note: ticket_type table doesn't have a comments column.
	_, execErr := db.Exec(database.ConvertPlaceholders(`
        UPDATE ticket_type
        SET valid_id = COALESCE(?, valid_id), name = COALESCE(?, name), change_time = NOW(), change_by = ?
        WHERE id = ?
    `), body.ValidID, body.Name, actorID, id)
	if execErr != nil {
		log.Printf("handleUpdateType: update: %v", execErr)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update type"})
		return
	}
	var name string
	var validID int
	err = db.QueryRow(database.ConvertPlaceholders(
		"SELECT name, valid_id FROM ticket_type WHERE id = ?"), id).Scan(&name, &validID)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Type not found"})
		return
	}
	if err != nil {
		log.Printf("handleUpdateType: reload: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update type"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": map[string]interface{}{
		"id":       id,
		"name":     name,
		"comments": valueOrEmpty(body.Comments), // Return for API compatibility
		"valid_id": validID,
	}})
}

func handleDeleteType(c *gin.Context) {
	if !checkAdminPermission(c) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Admin access required"})
		return
	}

	idStr := strings.TrimSpace(c.Param("id"))
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid type ID"})
		return
	}
	actorID, ok := auditUserID(c)
	if !ok {
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		log.Printf("handleDeleteType: database unavailable: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database unavailable"})
		return
	}
	if !fitsSmallint(id) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Type not found"})
		return
	}
	res, execErr := db.Exec(database.ConvertPlaceholders(`
        UPDATE ticket_type
        SET valid_id = 2, change_time = CURRENT_TIMESTAMP, change_by = ?
        WHERE id = ?
    `), actorID, id)
	if execErr != nil {
		log.Printf("handleDeleteType: update: %v", execErr)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete type"})
		return
	}
	rows, _ := res.RowsAffected() //nolint:errcheck // Error unlikely and rows default to 0
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Type not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Type deleted successfully"})
}

// Helper functions.
func checkAdminPermission(c *gin.Context) bool {
	// In production, check actual user permissions from JWT/session
	userRole := c.GetString("user_role")
	return userRole == "Admin"
}

// fitsSmallint reports whether id can name a row in a table whose id column
// is SMALLINT (ticket_priority, ticket_state, ticket_type). An id outside that
// range matches no row; handlers answer not-found without querying, because
// PostgreSQL rejects such a parameter with an error where MySQL finds nothing.
func fitsSmallint(id int) bool {
	return id >= math.MinInt16 && id <= math.MaxInt16
}
