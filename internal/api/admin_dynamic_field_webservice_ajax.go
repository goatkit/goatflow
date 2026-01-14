package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/service/genericinterface"
)

var wsFieldService *genericinterface.WebserviceFieldService

// getWSFieldService returns the webservice field service, initializing if needed.
func getWSFieldService() *genericinterface.WebserviceFieldService {
	if wsFieldService == nil {
		db, err := database.GetDB()
		if err != nil {
			return nil
		}
		wsFieldService = genericinterface.NewWebserviceFieldService(db)
	}
	return wsFieldService
}

// handleDynamicFieldAutocomplete handles autocomplete requests for webservice-backed dynamic fields.
// GET /admin/api/dynamic-fields/:id/autocomplete?term=xxx
func handleDynamicFieldAutocomplete(c *gin.Context) {
	fieldIDStr := c.Param("id")
	fieldID, err := strconv.Atoi(fieldIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid field ID",
		})
		return
	}

	term := c.Query("term")
	if term == "" {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}

	// Get the field configuration
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	field, err := getDynamicFieldByID(db, fieldID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "Field not found",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to fetch field",
			})
		}
		return
	}

	// Verify this is a webservice field type
	if field.FieldType != DFTypeWebserviceDropdown && field.FieldType != DFTypeWebserviceMultiselect {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Field is not a webservice field type",
		})
		return
	}

	// Parse field config
	if err := field.ParseConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to parse field configuration",
		})
		return
	}

	// Build field config for service
	fieldConfig := genericinterface.FieldConfig{
		Webservice:               field.Config.Webservice,
		InvokerSearch:            field.Config.InvokerSearch,
		InvokerGet:               field.Config.InvokerGet,
		StoredValue:              field.Config.StoredValue,
		DisplayedValuesSeparator: field.Config.DisplayedValuesSeparator,
		AutocompleteMinLength:    field.Config.AutocompleteMinLength,
		Limit:                    field.Config.Limit,
		CacheTTL:                 field.Config.CacheTTL,
	}

	// Parse displayed values
	if field.Config.DisplayedValues != "" {
		fieldConfig.DisplayedValues = strings.Split(field.Config.DisplayedValues, ",")
		for i := range fieldConfig.DisplayedValues {
			fieldConfig.DisplayedValues[i] = strings.TrimSpace(fieldConfig.DisplayedValues[i])
		}
	}

	// Set defaults
	if fieldConfig.AutocompleteMinLength == 0 {
		fieldConfig.AutocompleteMinLength = 3
	}
	if fieldConfig.Limit == 0 {
		fieldConfig.Limit = 20
	}
	if fieldConfig.CacheTTL == 0 {
		fieldConfig.CacheTTL = 60
	}
	if fieldConfig.DisplayedValuesSeparator == "" {
		fieldConfig.DisplayedValuesSeparator = " - "
	}

	// Get the service and perform search
	svc := getWSFieldService()
	if svc == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Service unavailable",
		})
		return
	}

	results, err := svc.Search(c.Request.Context(), fieldConfig, term)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Search failed: " + err.Error(),
		})
		return
	}

	// Return results in format expected by autocomplete UI
	c.JSON(http.StatusOK, results)
}

// handleDynamicFieldWebserviceTest tests the webservice configuration for a dynamic field.
// POST /admin/api/dynamic-fields/:id/webservice-test
func handleDynamicFieldWebserviceTest(c *gin.Context) {
	fieldIDStr := c.Param("id")
	fieldID, err := strconv.Atoi(fieldIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid field ID",
		})
		return
	}

	// Get the field configuration
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	field, err := getDynamicFieldByID(db, fieldID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "Field not found",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to fetch field",
			})
		}
		return
	}

	// Verify this is a webservice field type
	if field.FieldType != DFTypeWebserviceDropdown && field.FieldType != DFTypeWebserviceMultiselect {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Field is not a webservice field type",
		})
		return
	}

	// Parse field config
	if err := field.ParseConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to parse field configuration",
		})
		return
	}

	// Build field config for service
	fieldConfig := genericinterface.FieldConfig{
		Webservice:               field.Config.Webservice,
		InvokerSearch:            field.Config.InvokerSearch,
		InvokerGet:               field.Config.InvokerGet,
		StoredValue:              field.Config.StoredValue,
		DisplayedValuesSeparator: field.Config.DisplayedValuesSeparator,
		AutocompleteMinLength:    1, // Use 1 for testing
		Limit:                    5, // Limit results for test
		CacheTTL:                 0, // No caching for test
	}

	// Parse displayed values
	if field.Config.DisplayedValues != "" {
		fieldConfig.DisplayedValues = strings.Split(field.Config.DisplayedValues, ",")
		for i := range fieldConfig.DisplayedValues {
			fieldConfig.DisplayedValues[i] = strings.TrimSpace(fieldConfig.DisplayedValues[i])
		}
	}

	if fieldConfig.DisplayedValuesSeparator == "" {
		fieldConfig.DisplayedValuesSeparator = " - "
	}

	// Get the service and perform test search
	svc := getWSFieldService()
	if svc == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Service unavailable",
		})
		return
	}

	// Try to search with a test term
	results, err := svc.Search(c.Request.Context(), fieldConfig, "test")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   "Webservice test failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"message":      "Webservice configuration is valid",
		"sample_count": len(results),
		"sample_data":  results,
	})
}

// getDynamicFieldByID retrieves a dynamic field by ID.
func getDynamicFieldByID(db *sql.DB, id int) (*DynamicField, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, internal_field, name, label, field_order, field_type, object_type,
		       config, valid_id, create_time, create_by, change_time, change_by
		FROM dynamic_field
		WHERE id = ?
	`)

	field := &DynamicField{}
	err := db.QueryRow(query, id).Scan(
		&field.ID, &field.InternalField, &field.Name, &field.Label,
		&field.FieldOrder, &field.FieldType, &field.ObjectType,
		&field.ConfigRaw, &field.ValidID, &field.CreateTime, &field.CreateBy,
		&field.ChangeTime, &field.ChangeBy,
	)
	if err != nil {
		return nil, err
	}

	return field, nil
}
