package api

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/repository"
)

// LookupItem represents a simple ID/Name lookup option for templates.
type LookupItem struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// loadQueuesForForm loads all valid queues for form dropdowns.
func loadQueuesForForm(ctx context.Context, db *sql.DB) []LookupItem {
	if db == nil {
		return nil
	}
	query := database.ConvertPlaceholders(`
		SELECT id, name FROM queue WHERE valid_id = 1 ORDER BY name`)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var items []LookupItem
	for rows.Next() {
		var item LookupItem
		if err := rows.Scan(&item.ID, &item.Name); err == nil {
			items = append(items, item)
		}
	}
	return items
}

// loadPrioritiesForForm loads all valid priorities for form dropdowns.
func loadPrioritiesForForm(ctx context.Context, db *sql.DB) []LookupItem {
	if db == nil {
		return nil
	}
	query := database.ConvertPlaceholders(`
		SELECT id, name FROM ticket_priority WHERE valid_id = 1 ORDER BY id`)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var items []LookupItem
	for rows.Next() {
		var item LookupItem
		if err := rows.Scan(&item.ID, &item.Name); err == nil {
			items = append(items, item)
		}
	}
	return items
}

// loadStatesForForm loads all valid ticket states for form dropdowns.
func loadStatesForForm(ctx context.Context, db *sql.DB) []LookupItem {
	if db == nil {
		return nil
	}
	query := database.ConvertPlaceholders(`
		SELECT id, name FROM ticket_state WHERE valid_id = 1 ORDER BY name`)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var items []LookupItem
	for rows.Next() {
		var item LookupItem
		if err := rows.Scan(&item.ID, &item.Name); err == nil {
			items = append(items, item)
		}
	}
	return items
}

// loadTypesForForm loads all valid ticket types for form dropdowns.
func loadTypesForForm(ctx context.Context, db *sql.DB) []LookupItem {
	if db == nil {
		return nil
	}
	query := database.ConvertPlaceholders(`
		SELECT id, name FROM ticket_type WHERE valid_id = 1 ORDER BY name`)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var items []LookupItem
	for rows.Next() {
		var item LookupItem
		if err := rows.Scan(&item.ID, &item.Name); err == nil {
			items = append(items, item)
		}
	}
	return items
}

// handleAdminPostmasterFilters renders the postmaster filters management page.
func HandleAdminPostmasterFilters(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		renderPostmasterFiltersFallback(c, nil)
		return
	}

	repo := repository.NewPostmasterFilterRepository(db)
	filters, err := repo.List(c.Request.Context())
	if err != nil {
		renderPostmasterFiltersFallback(c, err)
		return
	}

	if getPongo2Renderer() == nil {
		renderPostmasterFiltersFallback(c, nil)
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

	// Load lookup data for form dropdowns
	db, _ := database.GetDB()
	ctx := c.Request.Context()

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/postmaster_filter_form.pongo2", pongo2.Context{
		"Title":      "New Postmaster Filter",
		"IsNew":      true,
		"Filter":     nil,
		"Queues":     loadQueuesForForm(ctx, db),
		"Priorities": loadPrioritiesForForm(ctx, db),
		"States":     loadStatesForForm(ctx, db),
		"Types":      loadTypesForForm(ctx, db),
		"User":       getUserMapForTemplate(c),
		"ActivePage": "admin",
	})
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

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/postmaster_filter_form.pongo2", pongo2.Context{
		"Title":      "Edit Postmaster Filter",
		"IsNew":      false,
		"Filter":     filter,
		"Queues":     loadQueuesForForm(ctx, db),
		"Priorities": loadPrioritiesForForm(ctx, db),
		"States":     loadStatesForForm(ctx, db),
		"Types":      loadTypesForForm(ctx, db),
		"User":       getUserMapForTemplate(c),
		"ActivePage": "admin",
	})
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
	Name    string                    `json:"name" binding:"required"`
	Stop    bool                      `json:"stop"`
	Matches []repository.FilterMatch  `json:"matches"`
	Sets    []repository.FilterSet    `json:"sets"`
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

// renderPostmasterFiltersFallback renders a basic fallback for when DB is unavailable.
func renderPostmasterFiltersFallback(c *gin.Context, err error) {
	accept := c.GetHeader("Accept")
	if strings.Contains(strings.ToLower(accept), "application/json") {
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		} else {
			c.JSON(http.StatusOK, gin.H{"success": true, "data": []repository.PostmasterFilter{}})
		}
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="utf-8">
	<title>Postmaster Filters</title>
	<link rel="stylesheet" href="/static/css/output.css">
</head>
<body>
	<main class="container mx-auto px-4 py-8">
		<h1 class="text-2xl font-bold mb-4">Postmaster Filters</h1>
		<p class="text-gray-600">Database unavailable. Please try again later.</p>
	</main>
</body>
</html>`)
}
