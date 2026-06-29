package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/customfields"
	"github.com/goatkit/goatflow/internal/routing"
)

func init() {
	routing.RegisterHandler("handleAPIListCustomFieldDefs", handleAPIListCustomFieldDefs)
	routing.RegisterHandler("handleAPIGetCustomFieldDef", handleAPIGetCustomFieldDef)
	routing.RegisterHandler("handleAPIGetCustomFieldValues", handleAPIGetCustomFieldValues)
	routing.RegisterHandler("handleAPISetCustomFieldValues", handleAPISetCustomFieldValues)
	routing.RegisterHandler("handleAPIQueryCustomFields", handleAPIQueryCustomFields)
}

// --- REST API v1: /api/v1/custom-fields/* ---

// handleAPIListCustomFieldDefs lists custom field definitions.
// GET /api/v1/custom-fields/definitions?entity_type=contact&owner_type=admin
func handleAPIListCustomFieldDefs(c *gin.Context) {
	repo, err := customfields.NewRepository()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	defs, err := repo.ListDefs(
		c.Query("entity_type"),
		c.Query("owner_type"),
		"",
		c.Query("active_only") == "true",
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"definitions": defs})
}

// handleAPIGetCustomFieldDef gets a single custom field definition.
// GET /api/v1/custom-fields/definitions/:id
func handleAPIGetCustomFieldDef(c *gin.Context) {
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

	def, err := repo.GetDef(id)
	if err != nil || def == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Field not found"})
		return
	}

	c.JSON(http.StatusOK, def)
}

// handleAPIGetCustomFieldValues gets custom field values for an entity.
// GET /api/v1/:entity_type/:id/custom-fields
func handleAPIGetCustomFieldValues(c *gin.Context) {
	entityType := c.Param("entity_type")
	objectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid entity ID"})
		return
	}

	if !customfields.IsValidEntityType(entityType) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid entity type"})
		return
	}

	repo, err := customfields.NewRepository()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	values, err := repo.GetValues(entityType, objectID, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"entity_type": entityType, "object_id": objectID, "values": values})
}

// handleAPISetCustomFieldValues sets custom field values for an entity.
// PUT /api/v1/:entity_type/:id/custom-fields
func handleAPISetCustomFieldValues(c *gin.Context) {
	entityType := c.Param("entity_type")
	objectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid entity ID"})
		return
	}

	if !customfields.IsValidEntityType(entityType) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid entity type"})
		return
	}

	var req struct {
		Values map[string]any `json:"values"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	repo, err := customfields.NewRepository()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Load defs for validation.
	defs, err := repo.ListDefs(entityType, "", "", true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defMap := make(map[string]*customfields.FieldDef, len(defs))
	for i := range defs {
		defMap[defs[i].Name] = &defs[i]
	}

	if errs := customfields.ValidateValues(defMap, req.Values); errs != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Validation failed", "details": errs})
		return
	}

	if err := repo.SetValues(entityType, objectID, req.Values); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// handleAPIQueryCustomFields queries entities by custom field values.
// POST /api/v1/custom-fields/query
func handleAPIQueryCustomFields(c *gin.Context) {
	var req struct {
		EntityType string                     `json:"entity_type"`
		Filters    []customfields.FieldFilter `json:"filters"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !customfields.IsValidEntityType(req.EntityType) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid entity type"})
		return
	}

	repo, err := customfields.NewRepository()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	ids, err := repo.QueryByFields(req.EntityType, req.Filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"entity_type": req.EntityType, "object_ids": ids})
}
