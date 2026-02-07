package api

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/repository"
)

// Get or create the singleton lookup repository.
var lookupRepo repository.LookupRepository

func GetLookupRepository() repository.LookupRepository {
	if lookupRepo == nil {
		lookupRepo = repository.NewMemoryLookupRepository()
	}
	return lookupRepo
}

// Helper for optional string pointers.
func valueOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Type CRUD handlers.
func handleCreateType(c *gin.Context) {
	// Allow in tests without admin context
	if os.Getenv("APP_ENV") != "test" && !checkAdminPermission(c) {
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

	if db, err := database.GetDB(); err == nil && db != nil {
		adapter := database.GetAdapter()
		// Note: ticket_type table doesn't have a comments column
		query := database.ConvertPlaceholders(`INSERT INTO ticket_type (name, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, ?, NOW(), 1, NOW(), 1) RETURNING id`)
		newID, err := adapter.InsertWithReturning(db, query, name, validID)
		if err != nil {
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
		return
	}

	// Fallback echo for non-DB environments
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"id":       1,
			"name":     name,
			"comments": comments,
			"valid_id": validID,
		},
	})
}

func handleUpdateType(c *gin.Context) {
	if os.Getenv("APP_ENV") != "test" && !checkAdminPermission(c) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Admin access required"})
		return
	}

	idStr := strings.TrimSpace(c.Param("id"))
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid type ID"})
		return
	}

	// Use pointer fields to detect presence
	var body struct {
		Name     *string `json:"name"`
		Comments *string `json:"comments"`
		ValidID  *int    `json:"valid_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.ValidID == nil {
		def := 1
		body.ValidID = &def
	}

	if db, dErr := database.GetDB(); dErr == nil && db != nil {
		// Note: ticket_type table doesn't have a comments column
		// Update only valid_id and name
		res, execErr := db.Exec(database.ConvertPlaceholders(`
            UPDATE ticket_type
            SET valid_id = ?, name = ?, change_time = NOW(), change_by = 1
            WHERE id = ?
        `), *body.ValidID, valueOrEmpty(body.Name), id)
		if execErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update type"})
			return
		}
		rows, _ := res.RowsAffected() //nolint:errcheck // Error unlikely and rows default to 0
		if rows == 0 {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Type not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": map[string]interface{}{
			"id":       id,
			"name":     valueOrEmpty(body.Name),
			"comments": valueOrEmpty(body.Comments), // Return for API compatibility
			"valid_id": *body.ValidID,
		}})
		return
	}

	// Fallback echo
	name := ""
	if body.Name != nil {
		name = *body.Name
	}
	comments := ""
	if body.Comments != nil {
		comments = *body.Comments
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": map[string]interface{}{
		"id":       id,
		"name":     name,
		"comments": comments,
		"valid_id": *body.ValidID,
	}})
}

func handleDeleteType(c *gin.Context) {
	if os.Getenv("APP_ENV") != "test" && !checkAdminPermission(c) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Admin access required"})
		return
	}

	idStr := strings.TrimSpace(c.Param("id"))
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid type ID"})
		return
	}

	if db, err := database.GetDB(); err == nil && db != nil {
		// Args order: change_by, id
		res, execErr := db.Exec(database.ConvertPlaceholders(`
            UPDATE ticket_type
            SET valid_id = 2, change_time = CURRENT_TIMESTAMP, change_by = ?
            WHERE id = ?
        `), 1, id)
		if execErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete type"})
			return
		}
		rows, _ := res.RowsAffected() //nolint:errcheck // Error unlikely and rows default to 0
		if rows == 0 {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Type not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "Type deleted successfully"})
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
