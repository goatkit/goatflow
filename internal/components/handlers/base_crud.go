// Package handlers provides base CRUD handlers for component operations.
package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/shared"
)

// CRUDConfig defines the configuration for a CRUD handler.
type CRUDConfig struct {
	EntityName       string // e.g., "priority", "state", "type"
	EntityNamePlural string // e.g., "priorities", "states", "types"
	TableName        string // Database table name
	RoutePrefix      string // e.g., "/admin/priorities"
	TemplatePath     string // Path to the template

	// Field definitions
	Fields []FieldConfig

	// Features
	SoftDelete   bool // Use valid_id instead of DELETE
	Searchable   bool
	ImportExport bool
	HasColor     bool // For entities with color picker
}

// FieldConfig defines a field in the entity.
type FieldConfig struct {
	Name         string
	DBColumn     string
	Type         FieldType
	Required     bool
	Searchable   bool
	Sortable     bool
	DefaultValue interface{}
	Validation   string // Regex or validation rule
}

// FieldType represents the type of field.
type FieldType string

const (
	FieldTypeString   FieldType = "string"
	FieldTypeInt      FieldType = "int"
	FieldTypeFloat    FieldType = "float"
	FieldTypeBool     FieldType = "bool"
	FieldTypeDate     FieldType = "date"
	FieldTypeDateTime FieldType = "datetime"
	FieldTypeColor    FieldType = "color"
	FieldTypeSelect   FieldType = "select"
	FieldTypeText     FieldType = "text"
)

// BaseCRUDHandler provides generic CRUD operations.
type BaseCRUDHandler struct {
	Config   CRUDConfig
	DB       *sql.DB
	Renderer *shared.TemplateRenderer
}

// NewBaseCRUDHandler creates a new base CRUD handler.
func NewBaseCRUDHandler(config CRUDConfig, db *sql.DB, renderer *shared.TemplateRenderer) *BaseCRUDHandler {
	return &BaseCRUDHandler{
		Config:   config,
		DB:       db,
		Renderer: renderer,
	}
}

// RegisterRoutes registers all CRUD routes.
func (h *BaseCRUDHandler) RegisterRoutes(router *gin.RouterGroup) {
	group := router.Group(h.Config.RoutePrefix)
	{
		group.GET("", h.List)
		group.GET("/:id", h.Get)
		group.POST("", h.Create)
		group.PUT("/:id", h.Update)
		group.DELETE("/:id", h.Delete)

		if h.Config.ImportExport {
			group.POST("/import", h.Import)
			group.GET("/export", h.Export)
		}

		if h.Config.Searchable {
			group.GET("/search", h.Search)
		}
	}
}

// List handles GET requests for listing entities.
func (h *BaseCRUDHandler) List(c *gin.Context) {
	query := h.buildListQuery()
	rows, err := h.DB.Query(query)
	if err != nil {
		h.handleError(c, err)
		return
	}
	defer rows.Close()

	entities := h.scanEntities(rows)

	if h.isAPIRequest(c) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    entities,
			"total":   len(entities),
		})
		return
	}

	// Render template
	h.Renderer.HTML(c, http.StatusOK, h.Config.TemplatePath, pongo2.Context{
		"entities":     entities,
		"entityName":   h.Config.EntityName,
		"entityPlural": h.Config.EntityNamePlural,
		"fields":       h.Config.Fields,
		"features": map[string]bool{
			"softDelete":   h.Config.SoftDelete,
			"searchable":   h.Config.Searchable,
			"importExport": h.Config.ImportExport,
			"hasColor":     h.Config.HasColor,
		},
	})
}

// Get handles GET requests for a single entity.
func (h *BaseCRUDHandler) Get(c *gin.Context) {
	id := c.Param("id")

	query := h.buildGetQuery()
	row := h.DB.QueryRow(query, id)

	entity := h.scanEntity(row)
	if entity == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   fmt.Sprintf("%s not found", h.Config.EntityName),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    entity,
	})
}

// Create handles POST requests to create a new entity.
func (h *BaseCRUDHandler) Create(c *gin.Context) {
	data := h.parseFormData(c)

	if err := h.validateData(data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	query := h.buildInsertQuery()
	args := h.buildInsertArgs(data)

	result, err := h.DB.Exec(query, args...)
	if err != nil {
		h.handleError(c, err)
		return
	}

	id, _ := result.LastInsertId()

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"id":      id,
		"message": fmt.Sprintf("%s created successfully", h.Config.EntityName),
	})
}

// Update handles PUT requests to update an entity.
func (h *BaseCRUDHandler) Update(c *gin.Context) {
	id := c.Param("id")
	data := h.parseFormData(c)

	if err := h.validateData(data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	query := h.buildUpdateQuery()
	args := h.buildUpdateArgs(data, id)

	_, err := h.DB.Exec(query, args...)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("%s updated successfully", h.Config.EntityName),
	})
}

// Delete handles DELETE requests (soft delete if configured).
func (h *BaseCRUDHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	var query string
	if h.Config.SoftDelete {
		query = fmt.Sprintf("UPDATE %s SET valid_id = 2 WHERE id = $1", h.Config.TableName)
	} else {
		query = fmt.Sprintf("DELETE FROM %s WHERE id = $1", h.Config.TableName)
	}

	_, err := h.DB.Exec(query, id)
	if err != nil {
		h.handleError(c, err)
		return
	}

	action := "deleted"
	if h.Config.SoftDelete {
		action = "deactivated"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("%s %s successfully", h.Config.EntityName, action),
	})
}

// Import handles CSV import.
func (h *BaseCRUDHandler) Import(c *gin.Context) {
	// Implementation depends on specific requirements
	c.JSON(http.StatusNotImplemented, gin.H{
		"success": false,
		"error":   "Import not yet implemented",
	})
}

// Export handles CSV export.
func (h *BaseCRUDHandler) Export(c *gin.Context) {
	// Implementation depends on specific requirements
	c.JSON(http.StatusNotImplemented, gin.H{
		"success": false,
		"error":   "Export not yet implemented",
	})
}

// Search handles search requests.
func (h *BaseCRUDHandler) Search(c *gin.Context) {
	searchTerm := c.Query("q")
	if searchTerm == "" {
		h.List(c)
		return
	}

	query := h.buildSearchQuery(searchTerm)
	rows, err := h.DB.Query(query, "%"+searchTerm+"%")
	if err != nil {
		h.handleError(c, err)
		return
	}
	defer rows.Close()

	entities := h.scanEntities(rows)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    entities,
		"total":   len(entities),
	})
}

// Helper methods

func (h *BaseCRUDHandler) buildListQuery() string {
	columns := h.getSelectColumns()
	query := fmt.Sprintf("SELECT %s FROM %s", columns, h.Config.TableName)

	if h.Config.SoftDelete {
		query += " WHERE valid_id = 1"
	}

	query += " ORDER BY id"
	return query
}

func (h *BaseCRUDHandler) buildGetQuery() string {
	columns := h.getSelectColumns()
	return fmt.Sprintf("SELECT %s FROM %s WHERE id = $1", columns, h.Config.TableName)
}

func (h *BaseCRUDHandler) buildInsertQuery() string {
	// Build based on field configuration
	columns := []string{}
	placeholders := []string{}

	for i, field := range h.Config.Fields {
		if field.DBColumn != "id" {
			columns = append(columns, field.DBColumn)
			placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
		}
	}

	if h.Config.SoftDelete {
		columns = append(columns, "valid_id")
		placeholders = append(placeholders, "1")
	}

	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		h.Config.TableName,
		joinStrings(columns, ", "),
		joinStrings(placeholders, ", "))
}

func (h *BaseCRUDHandler) buildUpdateQuery() string {
	sets := []string{}
	for i, field := range h.Config.Fields {
		if field.DBColumn != "id" {
			sets = append(sets, fmt.Sprintf("%s = $%d", field.DBColumn, i+1))
		}
	}

	return fmt.Sprintf("UPDATE %s SET %s WHERE id = $%d",
		h.Config.TableName,
		joinStrings(sets, ", "),
		len(h.Config.Fields))
}

func (h *BaseCRUDHandler) buildSearchQuery(searchTerm string) string {
	searchableFields := []string{}
	for _, field := range h.Config.Fields {
		if field.Searchable {
			searchableFields = append(searchableFields,
				fmt.Sprintf("%s ILIKE $1", field.DBColumn))
		}
	}

	columns := h.getSelectColumns()
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s",
		columns,
		h.Config.TableName,
		joinStrings(searchableFields, " OR "))

	if h.Config.SoftDelete {
		query += " AND valid_id = 1"
	}

	return query
}

func (h *BaseCRUDHandler) getSelectColumns() string {
	columns := []string{}
	for _, field := range h.Config.Fields {
		columns = append(columns, field.DBColumn)
	}
	return joinStrings(columns, ", ")
}

func (h *BaseCRUDHandler) parseFormData(c *gin.Context) map[string]interface{} {
	data := make(map[string]interface{})

	if c.ContentType() == "application/json" {
		c.ShouldBindJSON(&data)
	} else {
		// Parse form data
		for _, field := range h.Config.Fields {
			value := c.PostForm(field.Name)
			if value != "" {
				data[field.Name] = h.convertValue(value, field.Type)
			}
		}
	}

	return data
}

func (h *BaseCRUDHandler) convertValue(value string, fieldType FieldType) interface{} {
	switch fieldType {
	case FieldTypeInt:
		i, _ := strconv.Atoi(value)
		return i
	case FieldTypeFloat:
		f, _ := strconv.ParseFloat(value, 64)
		return f
	case FieldTypeBool:
		return value == "true" || value == "1"
	default:
		return value
	}
}

func (h *BaseCRUDHandler) validateData(data map[string]interface{}) error {
	for _, field := range h.Config.Fields {
		if field.Required {
			if _, exists := data[field.Name]; !exists {
				return fmt.Errorf("%s is required", field.Name)
			}
		}
	}
	return nil
}

func (h *BaseCRUDHandler) buildInsertArgs(data map[string]interface{}) []interface{} {
	args := []interface{}{}
	for _, field := range h.Config.Fields {
		if field.DBColumn != "id" {
			if value, exists := data[field.Name]; exists {
				args = append(args, value)
			} else {
				args = append(args, field.DefaultValue)
			}
		}
	}
	return args
}

func (h *BaseCRUDHandler) buildUpdateArgs(data map[string]interface{}, id string) []interface{} {
	args := h.buildInsertArgs(data)
	args = append(args, id)
	return args
}

func (h *BaseCRUDHandler) scanEntities(rows *sql.Rows) []map[string]interface{} {
	entities := []map[string]interface{}{}

	for rows.Next() {
		entity := h.createEntityScanners()
		if err := rows.Scan(entity...); err == nil {
			entities = append(entities, h.scannersToMap(entity))
		}
	}

	return entities
}

func (h *BaseCRUDHandler) scanEntity(row *sql.Row) map[string]interface{} {
	entity := h.createEntityScanners()
	if err := row.Scan(entity...); err != nil {
		return nil
	}
	return h.scannersToMap(entity)
}

func (h *BaseCRUDHandler) createEntityScanners() []interface{} {
	scanners := []interface{}{}
	for range h.Config.Fields {
		var value interface{}
		scanners = append(scanners, &value)
	}
	return scanners
}

func (h *BaseCRUDHandler) scannersToMap(scanners []interface{}) map[string]interface{} {
	entity := make(map[string]interface{})
	for i, field := range h.Config.Fields {
		if ptr, ok := scanners[i].(*interface{}); ok && ptr != nil {
			entity[field.Name] = *ptr
		}
	}
	return entity
}

func (h *BaseCRUDHandler) handleError(c *gin.Context, err error) {
	c.JSON(http.StatusInternalServerError, gin.H{
		"success": false,
		"error":   err.Error(),
	})
}

func (h *BaseCRUDHandler) isAPIRequest(c *gin.Context) bool {
	return c.GetHeader("X-Requested-With") == "XMLHttpRequest" ||
		c.GetHeader("Accept") == "application/json"
}

// Utility function.
func joinStrings(strs []string, sep string) string {
	result := ""
	for i, str := range strs {
		if i > 0 {
			result += sep
		}
		result += str
	}
	return result
}
