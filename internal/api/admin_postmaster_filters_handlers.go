package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/repository"
)

// LookupItem represents a simple ID/Name lookup option for templates.
type LookupItem struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// formLookupQueries are the dropdown sources shared by the admin filter and notification forms.
var formLookupQueries = map[string]string{
	"Queues":     `SELECT id, name FROM queue WHERE valid_id = 1 ORDER BY name`,
	"Priorities": `SELECT id, name FROM ticket_priority WHERE valid_id = 1 ORDER BY id`,
	"States":     `SELECT id, name FROM ticket_state WHERE valid_id = 1 ORDER BY name`,
	"Types":      `SELECT id, name FROM ticket_type WHERE valid_id = 1 ORDER BY name`,
	"Locks":      `SELECT id, name FROM ticket_lock_type ORDER BY id`,
	"Agents": `SELECT id, CONCAT(first_name, ' ', last_name, ' (', login, ')') AS name
		FROM users WHERE valid_id = 1 ORDER BY login`,
	"Roles":  `SELECT id, name FROM roles WHERE valid_id = 1 ORDER BY name`,
	"Groups": `SELECT id, name FROM groups WHERE valid_id = 1 ORDER BY name`,
}

// loadFormLookups loads the named dropdown lists (keys of formLookupQueries) into ctxOut.
func loadFormLookups(ctx context.Context, db *sql.DB, ctxOut pongo2.Context, keys ...string) error {
	if db == nil {
		return errors.New("database unavailable")
	}
	for _, key := range keys {
		items, err := queryLookupItems(ctx, db, formLookupQueries[key])
		if err != nil {
			return fmt.Errorf("load %s: %w", key, err)
		}
		ctxOut[key] = items
	}
	return nil
}

func queryLookupItems(ctx context.Context, db *sql.DB, query string) ([]LookupItem, error) {
	rows, err := db.QueryContext(ctx, database.ConvertPlaceholders(query))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []LookupItem{}
	for rows.Next() {
		var item LookupItem
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

var postmasterFormLookups = []string{"Queues", "Priorities", "States", "Types"}

// handleAdminPostmasterFilters renders the postmaster filters management page.
func HandleAdminPostmasterFilters(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		log.Printf("HandleAdminPostmasterFilters: database unavailable: %v", err)
		sendErrorResponse(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	repo := repository.NewPostmasterFilterRepository(db)
	filters, err := repo.List(c.Request.Context())
	if err != nil {
		log.Printf("HandleAdminPostmasterFilters: list filters: %v", err)
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to load postmaster filters")
		return
	}

	if getPongo2Renderer() == nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Template renderer unavailable")
		return
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/postmaster_filters.pongo2", pongo2.Context{
		"Title":      "Postmaster Filters",
		"Filters":    filters,
		"User":       getUserMapForTemplate(c),
		"ActivePage": "admin",
	})
}

// handleAdminPostmasterFilterNew renders the new filter creation form.
func HandleAdminPostmasterFilterNew(c *gin.Context) {
	if getPongo2Renderer() == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Template renderer not available"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		log.Printf("HandleAdminPostmasterFilterNew: database unavailable: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not available"})
		return
	}
	tplCtx := pongo2.Context{
		"Title":      "New Postmaster Filter",
		"IsNew":      true,
		"Filter":     nil,
		"User":       getUserMapForTemplate(c),
		"ActivePage": "admin",
	}
	if err := loadFormLookups(c.Request.Context(), db, tplCtx, postmasterFormLookups...); err != nil {
		log.Printf("HandleAdminPostmasterFilterNew: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load form options"})
		return
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/postmaster_filter_form.pongo2", tplCtx)
}

// handleAdminPostmasterFilterEdit renders the filter edit form.
func HandleAdminPostmasterFilterEdit(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Filter name is required"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not available"})
		return
	}

	ctx := c.Request.Context()

	repo := repository.NewPostmasterFilterRepository(db)
	filter, err := repo.Get(ctx, name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Filter not found"})
		return
	}

	if getPongo2Renderer() == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Template renderer not available"})
		return
	}

	tplCtx := pongo2.Context{
		"Title":      "Edit Postmaster Filter",
		"IsNew":      false,
		"Filter":     filter,
		"User":       getUserMapForTemplate(c),
		"ActivePage": "admin",
	}
	if err := loadFormLookups(ctx, db, tplCtx, postmasterFormLookups...); err != nil {
		log.Printf("HandleAdminPostmasterFilterEdit: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load form options"})
		return
	}
	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/postmaster_filter_form.pongo2", tplCtx)
}

// handleAdminPostmasterFilterGet returns a filter's details as JSON.
func HandleAdminPostmasterFilterGet(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Filter name is required"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database not available"})
		return
	}

	repo := repository.NewPostmasterFilterRepository(db)
	filter, err := repo.Get(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Filter not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": filter})
}

// PostmasterFilterInput represents the JSON input for creating/updating filters.
type PostmasterFilterInput struct {
	Name    string                   `json:"name" binding:"required"`
	Stop    bool                     `json:"stop"`
	Matches []repository.FilterMatch `json:"matches"`
	Sets    []repository.FilterSet   `json:"sets"`
}

// handleCreatePostmasterFilter creates a new postmaster filter.
func HandleCreatePostmasterFilter(c *gin.Context) {
	var input PostmasterFilterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request: " + err.Error()})
		return
	}

	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Filter name is required"})
		return
	}

	if len(input.Matches) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "At least one match condition is required"})
		return
	}

	if len(input.Sets) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "At least one set action is required"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database not available"})
		return
	}

	repo := repository.NewPostmasterFilterRepository(db)

	// Check if filter already exists
	existing, _ := repo.Get(c.Request.Context(), input.Name)
	if existing != nil {
		c.JSON(http.StatusConflict, gin.H{"success": false, "error": "A filter with this name already exists"})
		return
	}

	filter := &repository.PostmasterFilter{
		Name:    input.Name,
		Stop:    input.Stop,
		Matches: input.Matches,
		Sets:    input.Sets,
	}

	if err := repo.Create(c.Request.Context(), filter); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to create filter: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "message": "Filter created successfully", "data": filter})
}

// handleUpdatePostmasterFilter updates an existing postmaster filter.
func HandleUpdatePostmasterFilter(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Filter name is required"})
		return
	}

	var input PostmasterFilterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request: " + err.Error()})
		return
	}

	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		input.Name = name // Keep original name if not provided
	}

	if len(input.Matches) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "At least one match condition is required"})
		return
	}

	if len(input.Sets) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "At least one set action is required"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database not available"})
		return
	}

	repo := repository.NewPostmasterFilterRepository(db)

	// Check if filter exists
	existing, err := repo.Get(c.Request.Context(), name)
	if err != nil || existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Filter not found"})
		return
	}

	// If renaming, check that new name doesn't conflict
	if input.Name != name {
		conflict, _ := repo.Get(c.Request.Context(), input.Name)
		if conflict != nil {
			c.JSON(http.StatusConflict, gin.H{"success": false, "error": "A filter with this name already exists"})
			return
		}
	}

	filter := &repository.PostmasterFilter{
		Name:    input.Name,
		Stop:    input.Stop,
		Matches: input.Matches,
		Sets:    input.Sets,
	}

	if err := repo.Update(c.Request.Context(), name, filter); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update filter: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Filter updated successfully", "data": filter})
}

// handleDeletePostmasterFilter deletes a postmaster filter.
func HandleDeletePostmasterFilter(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Filter name is required"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database not available"})
		return
	}

	repo := repository.NewPostmasterFilterRepository(db)

	if err := repo.Delete(c.Request.Context(), name); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Filter not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Filter deleted successfully"})
}
