package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/customfields"
	"github.com/goatkit/goatflow/internal/platform/shared"
)

// handleAdminCustomFields renders the custom fields admin list page.
func handleAdminCustomFields(c *gin.Context) {
	renderer := shared.GetGlobalRenderer()

	repo, err := customfields.NewRepository()
	if err != nil {
		renderAdminCFError(c, renderer, "Failed to connect to database")
		return
	}

	entityFilter := c.Query("entity_type")
	ownerFilter := c.Query("owner_type")
	defs, err := repo.ListDefs(entityFilter, ownerFilter, "", false)
	if err != nil {
		renderAdminCFError(c, renderer, "Failed to load custom fields")
		return
	}

	// Group by entity type for display.
	grouped := make(map[string][]customfields.FieldDef)
	for _, et := range customfields.ValidEntityTypes() {
		grouped[et] = nil
	}
	for _, d := range defs {
		grouped[d.EntityType] = append(grouped[d.EntityType], d)
	}

	if renderer == nil {
		c.JSON(http.StatusOK, gin.H{"fields": defs})
		return
	}

	renderer.HTML(c, http.StatusOK, "pages/admin/custom_fields.pongo2", gin.H{
		"FieldsGrouped": grouped,
		"EntityTypes":   customfields.ValidEntityTypes(),
		"FieldTypes":    customfields.ValidFieldTypes(),
		"EntityFilter":  entityFilter,
		"OwnerFilter":   ownerFilter,
		"ActivePage":    "admin",
	})
}

// handleAdminCustomFieldNew renders the new custom field form.
func handleAdminCustomFieldNew(c *gin.Context) {
	renderer := shared.GetGlobalRenderer()
	if renderer == nil {
		c.JSON(http.StatusOK, gin.H{"message": "new custom field form"})
		return
	}

	renderer.HTML(c, http.StatusOK, "pages/admin/custom_field_form.pongo2", gin.H{
		"IsNew":       true,
		"EntityTypes": customfields.ValidEntityTypes(),
		"FieldTypes":  customfields.ValidFieldTypes(),
		"ActivePage":  "admin",
	})
}

// handleAdminCustomFieldEdit renders the edit form for an existing custom field.
func handleAdminCustomFieldEdit(c *gin.Context) {
	renderer := shared.GetGlobalRenderer()

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid field ID"})
		return
	}

	repo, err := customfields.NewRepository()
	if err != nil {
		renderAdminCFError(c, renderer, "Failed to connect to database")
		return
	}

	def, err := repo.GetDef(id)
	if err != nil || def == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Field not found"})
		return
	}

	if renderer == nil {
		c.JSON(http.StatusOK, def)
		return
	}

	renderer.HTML(c, http.StatusOK, "pages/admin/custom_field_form.pongo2", gin.H{
		"Field":       def,
		"IsNew":       false,
		"ReadOnly":    def.OwnerType != customfields.OwnerAdmin,
		"EntityTypes": customfields.ValidEntityTypes(),
		"FieldTypes":  customfields.ValidFieldTypes(),
		"ActivePage":  "admin",
	})
}

// handleCreateCustomField creates a new admin-owned custom field.
func handleCreateCustomField(c *gin.Context) {
	var req struct {
		Name        string           `json:"name" form:"name"`
		Label       string           `json:"label" form:"label"`
		EntityType  string           `json:"entity_type" form:"entity_type"`
		FieldType   string           `json:"field_type" form:"field_type"`
		Section     string           `json:"section" form:"section"`
		FieldOrder  int              `json:"field_order" form:"field_order"`
		Description string           `json:"description" form:"description"`
		Placeholder string           `json:"placeholder" form:"placeholder"`
		Required    bool             `json:"required" form:"required"`
		Config      *json.RawMessage `json:"config,omitempty"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	section := req.Section
	if section == "" {
		section = "custom"
	}

	def := &customfields.FieldDef{
		Name:       req.Name,
		Label:      req.Label,
		EntityType: req.EntityType,
		FieldType:  req.FieldType,
		OwnerType:  customfields.OwnerAdmin,
		Section:    section,
		FieldOrder: req.FieldOrder,
		Required:   req.Required,
		Config:     req.Config,
		ValidID:    1,
	}
	if req.Description != "" {
		def.Description = &req.Description
	}
	if req.Placeholder != "" {
		def.Placeholder = &req.Placeholder
	}

	if err := customfields.ValidateFieldDef(def); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	repo, err := customfields.NewRepository()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	userID := getUserID(c)
	id, err := repo.CreateDef(def, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Support HTMX redirect.
	if c.GetHeader("HX-Request") == "true" {
		c.Header("HX-Redirect", "/admin/custom-fields")
		c.Status(http.StatusOK)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": id})
}

// handleUpdateCustomField updates an existing admin-owned custom field.
func handleUpdateCustomField(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid field ID"})
		return
	}

	repo, err := customfields.NewRepository()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	existing, err := repo.GetDef(id)
	if err != nil || existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Field not found"})
		return
	}
	if existing.OwnerType != customfields.OwnerAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot edit plugin-owned or legacy fields"})
		return
	}

	var req struct {
		Label       string           `json:"label" form:"label"`
		Section     string           `json:"section" form:"section"`
		FieldOrder  int              `json:"field_order" form:"field_order"`
		Description string           `json:"description" form:"description"`
		Placeholder string           `json:"placeholder" form:"placeholder"`
		Required    bool             `json:"required" form:"required"`
		Config      *json.RawMessage `json:"config,omitempty"`
		ValidID     int              `json:"valid_id" form:"valid_id"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	existing.Label = req.Label
	existing.Section = req.Section
	existing.FieldOrder = req.FieldOrder
	existing.Required = req.Required
	existing.Config = req.Config
	if req.Description != "" {
		existing.Description = &req.Description
	}
	if req.Placeholder != "" {
		existing.Placeholder = &req.Placeholder
	}
	if req.ValidID != 0 {
		existing.ValidID = req.ValidID
	}

	userID := getUserID(c)
	if err := repo.UpdateDef(existing, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if c.GetHeader("HX-Request") == "true" {
		c.Header("HX-Redirect", "/admin/custom-fields")
		c.Status(http.StatusOK)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// handleDeleteCustomField soft-deletes a custom field.
func handleDeleteCustomField(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid field ID"})
		return
	}

	repo, err := customfields.NewRepository()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	existing, err := repo.GetDef(id)
	if err != nil || existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Field not found"})
		return
	}
	if existing.OwnerType == customfields.OwnerPlugin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot delete plugin-owned fields"})
		return
	}

	userID := getUserID(c)
	if err := repo.SoftDeleteDef(id, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func renderAdminCFError(c *gin.Context, renderer interface {
	HTML(*gin.Context, int, string, any)
}, msg string) {
	if renderer != nil {
		renderer.HTML(c, http.StatusInternalServerError, "error.html", gin.H{"Error": msg})
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
	}
}

// getUserID is defined in admin_dynamic_fields_handlers.go
