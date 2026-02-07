package api

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/shared"
)

func handleAdminDynamicFields(c *gin.Context) {
	renderer := shared.GetGlobalRenderer()

	fieldsGrouped, err := GetDynamicFieldsGroupedByObjectType()
	if err != nil {
		if renderer != nil {
			renderer.HTML(c, http.StatusInternalServerError, "error.html", gin.H{
				"Error": "Failed to load dynamic fields",
			})
		} else {
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.String(http.StatusInternalServerError, "<h1>Error</h1><p>Failed to load dynamic fields</p>")
		}
		return
	}

	if renderer == nil {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, "<h1>Dynamic Fields</h1>")
		return
	}

	renderer.HTML(c, http.StatusOK, "pages/admin/dynamic_fields.pongo2", gin.H{
		"Title":             "Dynamic Fields Management",
		"FieldsGrouped":     fieldsGrouped,
		"ObjectTypes":       ValidObjectTypes(),
		"FieldTypes":        ValidFieldTypes(),
		"ScreenDefinitions": GetScreenDefinitions(),
		"ActivePage":        "admin",
	})
}

func handleAdminDynamicFieldNew(c *gin.Context) {
	renderer := shared.GetGlobalRenderer()
	if renderer == nil {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, "<h1>New Dynamic Field</h1>")
		return
	}

	renderer.HTML(c, http.StatusOK, "pages/admin/dynamic_field_form.pongo2", gin.H{
		"Title":       "New Dynamic Field",
		"Field":       &DynamicField{ValidID: 1, FieldOrder: 1},
		"IsNew":       true,
		"ObjectTypes": ValidObjectTypes(),
		"FieldTypes":  ValidFieldTypes(),
		"ActivePage":  "admin",
	})
}

func handleAdminDynamicFieldEdit(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid field ID"})
		return
	}

	field, err := GetDynamicField(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load field"})
		return
	}
	if field == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Field not found"})
		return
	}

	renderer := shared.GetGlobalRenderer()
	if renderer == nil {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, "<h1>Edit Dynamic Field</h1>")
		return
	}

	renderer.HTML(c, http.StatusOK, "pages/admin/dynamic_field_form.pongo2", gin.H{
		"Title":       "Edit Dynamic Field",
		"Field":       field,
		"IsNew":       false,
		"ObjectTypes": ValidObjectTypes(),
		"FieldTypes":  ValidFieldTypes(),
		"ActivePage":  "admin",
	})
}

func handleCreateDynamicField(c *gin.Context) {
	field, err := parseDynamicFieldForm(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := field.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	exists, err := CheckDynamicFieldNameExists(field.Name, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check field name"})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "Field name already exists"})
		return
	}

	userID := getUserID(c)
	fieldID, err := CreateDynamicField(field, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create field"})
		return
	}

	if isHTMXRequest(c) {
		c.Header("HX-Redirect", "/admin/dynamic-fields")
		c.Status(http.StatusOK)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"id":      fieldID,
	})
}

func handleUpdateDynamicField(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid field ID"})
		return
	}

	existing, err := GetDynamicField(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load field"})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Field not found"})
		return
	}

	field, err := parseDynamicFieldForm(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	field.ID = id
	field.InternalField = existing.InternalField

	if err := field.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if field.Name != existing.Name {
		exists, err := CheckDynamicFieldNameExists(field.Name, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check field name"})
			return
		}
		if exists {
			c.JSON(http.StatusConflict, gin.H{"error": "Field name already exists"})
			return
		}
	}

	userID := getUserID(c)
	if err := UpdateDynamicField(field, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update field"})
		return
	}

	if isHTMXRequest(c) {
		c.Header("HX-Redirect", "/admin/dynamic-fields")
		c.Status(http.StatusOK)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func handleDeleteDynamicField(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid field ID"})
		return
	}

	existing, err := GetDynamicField(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load field"})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Field not found"})
		return
	}

	if existing.InternalField == 1 {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot delete internal field"})
		return
	}

	if err := DeleteDynamicField(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete field"})
		return
	}

	if isHTMXRequest(c) {
		c.Header("HX-Trigger", "fieldDeleted")
		c.Status(http.StatusOK)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func handleGetDynamicFields(c *gin.Context) {
	objectType := c.Query("object_type")
	fieldType := c.Query("field_type")

	fields, err := GetDynamicFields(objectType, fieldType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load fields"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    fields,
	})
}

func parseDynamicFieldForm(c *gin.Context) (*DynamicField, error) {
	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	label := strings.TrimSpace(c.PostForm("label"))
	if label == "" {
		label = name
	}

	fieldType := c.PostForm("field_type")
	if !isValidFieldType(fieldType) {
		return nil, fmt.Errorf("invalid field type: %s", fieldType)
	}

	objectType := c.PostForm("object_type")
	if objectType == "" {
		objectType = DFObjectTicket
	}
	if !isValidObjectType(objectType) {
		return nil, fmt.Errorf("invalid object type: %s", objectType)
	}

	fieldOrder, _ := strconv.Atoi(c.PostForm("field_order")) //nolint:errcheck // Defaults to 1
	if fieldOrder < 1 {
		fieldOrder = 1
	}

	validID, _ := strconv.Atoi(c.PostForm("valid_id")) //nolint:errcheck // Defaults to 1
	if validID < 1 {
		validID = 1
	}

	// Check if auto-config mode is enabled for supported field types
	autoConfig := c.PostForm("auto_config") == "1"

	var config *DynamicFieldConfig

	if autoConfig && SupportsAutoConfig(fieldType) {
		// Use automatic default configuration
		config = DefaultDynamicFieldConfig(fieldType)
		// Preserve any default value that was set
		if defaultVal := c.PostForm("default_value"); defaultVal != "" {
			config.DefaultValue = defaultVal
		}
	} else {
		// Manual configuration - parse all type-specific fields
		config = &DynamicFieldConfig{
			DefaultValue: c.PostForm("default_value"),
		}

		switch fieldType {
		case DFTypeDropdown, DFTypeMultiselect:
			possibleValues := c.PostForm("possible_values")
			if possibleValues != "" {
				lines := strings.Split(possibleValues, "\n")
				values := make(map[string]string)
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line != "" {
						if strings.Contains(line, "=") {
							parts := strings.SplitN(line, "=", 2)
							values[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
						} else {
							values[line] = line
						}
					}
				}
				config.PossibleValues = values
			}
			if c.PostForm("possible_none") == "1" {
				config.PossibleNone = 1
			}
			if c.PostForm("translatable_values") == "1" {
				config.TranslatableValues = 1
			}

		case DFTypeDate, DFTypeDateTime:
			config.YearsInPast, _ = strconv.Atoi(c.PostForm("years_in_past"))     //nolint:errcheck // Defaults to 0
			config.YearsInFuture, _ = strconv.Atoi(c.PostForm("years_in_future")) //nolint:errcheck // Defaults to 0
			config.DateRestriction = c.PostForm("date_restriction")

		case DFTypeText:
			if regexStr := c.PostForm("regex_list"); regexStr != "" {
				config.RegExList = []RegEx{{Value: regexStr}}
			}
			config.Link = c.PostForm("link")

		case DFTypeTextArea:
			if rows := c.PostForm("rows"); rows != "" {
				config.Rows, _ = strconv.Atoi(rows) //nolint:errcheck // Defaults to 0
			}
			if cols := c.PostForm("cols"); cols != "" {
				config.Cols, _ = strconv.Atoi(cols) //nolint:errcheck // Defaults to 0
			}
		}
	}

	field := &DynamicField{
		Name:       name,
		Label:      label,
		FieldType:  fieldType,
		ObjectType: objectType,
		FieldOrder: fieldOrder,
		ValidID:    validID,
		Config:     config,
	}

	return field, nil
}

func getUserID(c *gin.Context) int {
	if id, ok := c.Get("user_id"); ok {
		switch v := id.(type) {
		case int:
			return v
		case uint:
			return int(v)
		case int64:
			return int(v)
		}
	}
	return 1
}

func isHTMXRequest(c *gin.Context) bool {
	return c.GetHeader("HX-Request") == "true"
}

// RegisterDynamicFieldRoutes registers all dynamic field admin routes.
func RegisterDynamicFieldRoutes(router *gin.RouterGroup, adminRouter *gin.RouterGroup) {
	adminRouter.GET("/dynamic-fields", handleAdminDynamicFields)
	adminRouter.GET("/dynamic-fields/new", handleAdminDynamicFieldNew)
	adminRouter.GET("/dynamic-fields/export", handleAdminDynamicFieldExportPage)
	adminRouter.POST("/dynamic-fields/export", handleAdminDynamicFieldExportAction)
	adminRouter.GET("/dynamic-fields/import", handleAdminDynamicFieldImportPage)
	adminRouter.POST("/dynamic-fields/import", handleAdminDynamicFieldImportAction)
	adminRouter.POST("/dynamic-fields/import/confirm", handleAdminDynamicFieldImportConfirm)
	adminRouter.GET("/dynamic-fields/screens", handleAdminDynamicFieldScreenConfig)
	adminRouter.GET("/dynamic-fields/:id", handleAdminDynamicFieldEdit)

	router.POST("/dynamic-fields", handleCreateDynamicField)
	router.PUT("/dynamic-fields/:id", handleUpdateDynamicField)
	router.DELETE("/dynamic-fields/:id", handleDeleteDynamicField)
	router.GET("/dynamic-fields", handleGetDynamicFields)
	router.PUT("/dynamic-fields/:id/screens", handleAdminDynamicFieldScreenConfigSave)
	router.POST("/dynamic-fields/:id/screen", handleAdminDynamicFieldScreenConfigSingle)
}

// GetDynamicFieldsForScreen retrieves active fields for a specific screen.
func GetDynamicFieldsForScreen(objectType, screenKey string) ([]DynamicField, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	query := database.ConvertPlaceholders(`
		SELECT df.id, df.internal_field, df.name, df.label, df.field_order,
		       df.field_type, df.object_type, df.config, df.valid_id,
		       df.create_time, df.create_by, df.change_time, df.change_by,
		       COALESCE(sc.config_value, 0) as screen_config
		FROM dynamic_field df
		LEFT JOIN dynamic_field_screen_config sc
		  ON df.id = sc.field_id AND sc.screen_key = ?
		WHERE df.object_type = ? AND df.valid_id = 1
		  AND COALESCE(sc.config_value, 0) > 0
		ORDER BY df.field_order, df.name
	`)

	rows, err := db.Query(query, screenKey, objectType)
	if err != nil {
		return nil, fmt.Errorf("failed to query screen fields: %w", err)
	}
	defer rows.Close()

	var fields []DynamicField
	for rows.Next() {
		var f DynamicField
		var screenConfig int
		err := rows.Scan(
			&f.ID, &f.InternalField, &f.Name, &f.Label, &f.FieldOrder,
			&f.FieldType, &f.ObjectType, &f.ConfigRaw, &f.ValidID,
			&f.CreateTime, &f.CreateBy, &f.ChangeTime, &f.ChangeBy,
			&screenConfig,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan field: %w", err)
		}
		if err := f.ParseConfig(); err != nil {
			return nil, err
		}
		fields = append(fields, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating fields: %w", err)
	}

	return fields, nil
}

// handleAdminDynamicFieldScreenConfig shows the screen configuration page.
func handleAdminDynamicFieldScreenConfig(c *gin.Context) {
	objectType := c.DefaultQuery("object_type", "Ticket")

	matrix, err := GetScreenConfigMatrix(objectType)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Error": "Failed to load screen configuration"})
		return
	}

	filteredScreens := []ScreenDefinition{}
	for _, s := range matrix.Screens {
		if s.ObjectType == objectType {
			filteredScreens = append(filteredScreens, s)
		}
	}
	matrix.Screens = filteredScreens

	// Build template-friendly row data
	type ScreenCell struct {
		ScreenKey        string
		ConfigValue      int
		SupportsRequired bool
		IsDisplayOnly    bool
	}
	type FieldRow struct {
		Field DynamicField
		Cells []ScreenCell
	}
	rows := make([]FieldRow, 0, len(matrix.Fields))
	for _, f := range matrix.Fields {
		row := FieldRow{Field: f, Cells: []ScreenCell{}}
		fieldConfigs := matrix.ConfigMap[f.ID]
		for _, s := range matrix.Screens {
			val := 0
			if fieldConfigs != nil {
				val = fieldConfigs[s.Key]
			}
			row.Cells = append(row.Cells, ScreenCell{
				ScreenKey:        s.Key,
				ConfigValue:      val,
				SupportsRequired: s.SupportsRequired,
				IsDisplayOnly:    s.IsDisplayOnly,
			})
		}
		rows = append(rows, row)
	}

	renderer := shared.GetGlobalRenderer()
	if renderer == nil {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, "<h1>Screen Configuration</h1>")
		return
	}

	renderer.HTML(c, http.StatusOK, "pages/admin/dynamic_field_screens.pongo2", gin.H{
		"Title":            "Dynamic Field Screen Configuration",
		"Matrix":           matrix,
		"Rows":             rows,
		"ObjectTypes":      ValidObjectTypes(),
		"ActiveObjectType": objectType,
		"ActivePage":       "admin",
	})
}

// handleAdminDynamicFieldScreenConfigSave saves screen configuration for a field.
func handleAdminDynamicFieldScreenConfigSave(c *gin.Context) {
	idStr := c.Param("id")
	fieldID, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid field ID"})
		return
	}

	var input struct {
		Configs map[string]int `json:"configs"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}

	userID := getUserID(c)

	if err := BulkSetScreenConfigForField(fieldID, input.Configs, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleAdminDynamicFieldScreenConfigSingle saves a single screen config.
func handleAdminDynamicFieldScreenConfigSingle(c *gin.Context) {
	idStr := c.Param("id")
	fieldID, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid field ID"})
		return
	}

	var input struct {
		ScreenKey   string `json:"screen_key"`
		ConfigValue int    `json:"config_value"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}

	userID := getUserID(c)

	if err := SetScreenConfig(fieldID, input.ScreenKey, input.ConfigValue, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "config_value": input.ConfigValue})
}

// handleAdminDynamicFieldExportPage shows the export page with field checkboxes.
func handleAdminDynamicFieldExportPage(c *gin.Context) {
	renderer := shared.GetGlobalRenderer()

	fieldsGrouped, err := GetDynamicFieldsGroupedByObjectType()
	if err != nil {
		if renderer != nil {
			renderer.HTML(c, http.StatusInternalServerError, "error.html", gin.H{
				"Error": "Failed to load dynamic fields",
			})
		} else {
			c.String(http.StatusInternalServerError, "Failed to load dynamic fields")
		}
		return
	}

	if renderer == nil {
		c.String(http.StatusOK, "<h1>Export Dynamic Fields</h1>")
		return
	}

	renderer.HTML(c, http.StatusOK, "pages/admin/dynamic_field_export.pongo2", gin.H{
		"Title":         "Export Dynamic Field Configuration",
		"FieldsGrouped": fieldsGrouped,
		"ObjectTypes":   ValidObjectTypes(),
		"ActivePage":    "admin",
	})
}

// handleAdminDynamicFieldExportAction generates and returns a YAML export file.
func handleAdminDynamicFieldExportAction(c *gin.Context) {
	// Get selected fields from form
	fieldNames := c.PostFormArray("field_config")
	screenNames := c.PostFormArray("screen_config")

	if len(fieldNames) == 0 && len(screenNames) == 0 {
		c.Redirect(http.StatusFound, "/admin/dynamic-fields/export?error=no_selection")
		return
	}

	// Combine unique field names
	allFields := make(map[string]bool)
	for _, name := range fieldNames {
		allFields[name] = true
	}
	for _, name := range screenNames {
		allFields[name] = true
	}

	var exportFields []string
	for name := range allFields {
		exportFields = append(exportFields, name)
	}

	// Export with screens only if screen configs were selected
	includeScreens := len(screenNames) > 0

	yamlData, err := ExportDynamicFieldsYAML(exportFields, includeScreens)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to export: " + err.Error()})
		return
	}

	// Generate filename with timestamp
	filename := fmt.Sprintf("Export_DynamicFields_%s.yml", time.Now().Format("2006-01-02_15-04-05"))

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/x-yaml")
	c.Data(http.StatusOK, "application/x-yaml", yamlData)
}

// handleAdminDynamicFieldImportPage shows the import page.
func handleAdminDynamicFieldImportPage(c *gin.Context) {
	renderer := shared.GetGlobalRenderer()

	if renderer == nil {
		c.String(http.StatusOK, "<h1>Import Dynamic Fields</h1>")
		return
	}

	renderer.HTML(c, http.StatusOK, "pages/admin/dynamic_field_import.pongo2", gin.H{
		"Title":      "Import Dynamic Field Configuration",
		"ActivePage": "admin",
		"ShowUpload": true,
	})
}

// handleAdminDynamicFieldImportAction processes the uploaded YAML file and shows preview.
func handleAdminDynamicFieldImportAction(c *gin.Context) {
	renderer := shared.GetGlobalRenderer()

	file, err := c.FormFile("yaml_file")
	if err != nil {
		if renderer != nil {
			renderer.HTML(c, http.StatusBadRequest, "pages/admin/dynamic_field_import.pongo2", gin.H{
				"Title":      "Import Dynamic Field Configuration",
				"ActivePage": "admin",
				"ShowUpload": true,
				"Error":      "Please select a file to upload",
			})
		} else {
			c.String(http.StatusBadRequest, "Please select a file to upload")
		}
		return
	}

	// Open and read the file
	f, err := file.Open()
	if err != nil {
		if renderer != nil {
			renderer.HTML(c, http.StatusBadRequest, "pages/admin/dynamic_field_import.pongo2", gin.H{
				"Title":      "Import Dynamic Field Configuration",
				"ActivePage": "admin",
				"ShowUpload": true,
				"Error":      "Failed to read uploaded file",
			})
		} else {
			c.String(http.StatusBadRequest, "Failed to read uploaded file")
		}
		return
	}
	defer f.Close()

	data := make([]byte, file.Size)
	_, err = f.Read(data)
	if err != nil {
		if renderer != nil {
			renderer.HTML(c, http.StatusBadRequest, "pages/admin/dynamic_field_import.pongo2", gin.H{
				"Title":      "Import Dynamic Field Configuration",
				"ActivePage": "admin",
				"ShowUpload": true,
				"Error":      "Failed to read file content",
			})
		} else {
			c.String(http.StatusBadRequest, "Failed to read file content")
		}
		return
	}

	// Parse the YAML
	export, err := ParseDynamicFieldsYAML(data)
	if err != nil {
		if renderer != nil {
			renderer.HTML(c, http.StatusBadRequest, "pages/admin/dynamic_field_import.pongo2", gin.H{
				"Title":      "Import Dynamic Field Configuration",
				"ActivePage": "admin",
				"ShowUpload": true,
				"Error":      "Invalid YAML file: " + err.Error(),
			})
		} else {
			c.String(http.StatusBadRequest, "Invalid YAML file: "+err.Error())
		}
		return
	}

	// Get import preview
	preview, err := GetImportPreview(export)
	if err != nil {
		if renderer != nil {
			renderer.HTML(c, http.StatusInternalServerError, "pages/admin/dynamic_field_import.pongo2", gin.H{
				"Title":      "Import Dynamic Field Configuration",
				"ActivePage": "admin",
				"ShowUpload": true,
				"Error":      "Failed to generate preview: " + err.Error(),
			})
		} else {
			c.String(http.StatusInternalServerError, "Failed to generate preview")
		}
		return
	}

	// Store the YAML data in session/cache for the confirm step
	// For simplicity, we'll encode it in a hidden field
	yamlBase64 := base64.StdEncoding.EncodeToString(data)

	if renderer == nil {
		c.String(http.StatusOK, "<h1>Import Preview</h1>")
		return
	}

	renderer.HTML(c, http.StatusOK, "pages/admin/dynamic_field_import.pongo2", gin.H{
		"Title":           "Import Dynamic Field Configuration",
		"ActivePage":      "admin",
		"ShowUpload":      false,
		"ShowPreview":     true,
		"Preview":         preview,
		"YAMLData":        yamlBase64,
		"HasScreenConfig": export.DynamicFieldScreens != nil && len(export.DynamicFieldScreens) > 0,
	})
}

// handleAdminDynamicFieldImportConfirm performs the actual import.
func handleAdminDynamicFieldImportConfirm(c *gin.Context) {
	renderer := shared.GetGlobalRenderer()

	// Get the base64-encoded YAML data
	yamlBase64 := c.PostForm("yaml_data")
	if yamlBase64 == "" {
		c.Redirect(http.StatusFound, "/admin/dynamic-fields/import?error=missing_data")
		return
	}

	yamlData, err := base64.StdEncoding.DecodeString(yamlBase64)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/dynamic-fields/import?error=invalid_data")
		return
	}

	// Parse the YAML again
	export, err := ParseDynamicFieldsYAML(yamlData)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/dynamic-fields/import?error=invalid_yaml")
		return
	}

	// Get selected fields
	selectedFields := c.PostFormArray("field_config")
	selectedScreens := c.PostFormArray("screen_config")
	overwrite := c.PostForm("overwrite") == "1"
	userID := getUserID(c)

	// Perform import
	result, err := ImportDynamicFields(export, selectedFields, selectedScreens, overwrite, userID)
	if err != nil {
		if renderer != nil {
			renderer.HTML(c, http.StatusInternalServerError, "pages/admin/dynamic_field_import.pongo2", gin.H{
				"Title":      "Import Dynamic Field Configuration",
				"ActivePage": "admin",
				"ShowUpload": true,
				"Error":      "Import failed: " + err.Error(),
			})
		} else {
			c.String(http.StatusInternalServerError, "Import failed: "+err.Error())
		}
		return
	}

	// Show results
	if renderer == nil {
		c.String(http.StatusOK, fmt.Sprintf("Created: %d, Updated: %d, Skipped: %d",
			len(result.Created), len(result.Updated), len(result.Skipped)))
		return
	}

	renderer.HTML(c, http.StatusOK, "pages/admin/dynamic_field_import.pongo2", gin.H{
		"Title":       "Import Dynamic Field Configuration",
		"ActivePage":  "admin",
		"ShowUpload":  false,
		"ShowResults": true,
		"Result":      result,
	})
}
