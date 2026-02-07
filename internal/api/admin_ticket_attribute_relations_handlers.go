package api

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/routing"
	"github.com/goatkit/goatflow/internal/services/ticketattributerelations"
)

func init() {
	routing.RegisterHandler("handleAdminTicketAttributeRelations", handleAdminTicketAttributeRelations)
	routing.RegisterHandler("handleAdminTicketAttributeRelationsNew", handleAdminTicketAttributeRelationsNew)
	routing.RegisterHandler("handleAdminTicketAttributeRelationsCreate", handleAdminTicketAttributeRelationsCreate)
	routing.RegisterHandler("handleAdminTicketAttributeRelationsEdit", handleAdminTicketAttributeRelationsEdit)
	routing.RegisterHandler("handleAdminTicketAttributeRelationsUpdate", handleAdminTicketAttributeRelationsUpdate)
	routing.RegisterHandler("handleAdminTicketAttributeRelationsDelete", handleAdminTicketAttributeRelationsDelete)
	routing.RegisterHandler("handleAdminTicketAttributeRelationsDownload", handleAdminTicketAttributeRelationsDownload)
	routing.RegisterHandler("handleAdminTicketAttributeRelationsReorder", handleAdminTicketAttributeRelationsReorder)
	routing.RegisterHandler("handleAPITicketAttributeRelationsEvaluate", handleAPITicketAttributeRelationsEvaluate)
}

// getTicketAttributeRelationsService creates the service.
func getTicketAttributeRelationsService() (*ticketattributerelations.Service, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	return ticketattributerelations.NewService(db), nil
}

// handleAdminTicketAttributeRelations renders the list page.
func handleAdminTicketAttributeRelations(c *gin.Context) {
	svc, err := getTicketAttributeRelationsService()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database connection failed")
		return
	}

	relations, err := svc.GetAll(c.Request.Context())
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to fetch relations")
		return
	}

	// Check if JSON is requested
	if wantsJSONResponse(c) {
		c.JSON(http.StatusOK, gin.H{
			"success":   true,
			"relations": relations,
			"count":     len(relations),
		})
		return
	}

	if getPongo2Renderer() == nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Template renderer unavailable")
		return
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/ticket_attribute_relations.pongo2", pongo2.Context{
		"Relations":  relations,
		"User":       getUserMapForTemplate(c),
		"ActivePage": "admin",
		"Mode":       "list",
	})
}

// handleAdminTicketAttributeRelationsNew renders the create form.
func handleAdminTicketAttributeRelationsNew(c *gin.Context) {
	svc, err := getTicketAttributeRelationsService()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database connection failed")
		return
	}

	// Get next priority for default
	nextPriority, _ := svc.GetNextPriority(c.Request.Context())

	// Get priority options
	priorityOptions, _ := svc.GetPriorityOptions(c.Request.Context())

	if getPongo2Renderer() == nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Template renderer unavailable")
		return
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/ticket_attribute_relations.pongo2", pongo2.Context{
		"User":            getUserMapForTemplate(c),
		"ActivePage":      "admin",
		"Mode":            "new",
		"NextPriority":    nextPriority,
		"PriorityOptions": priorityOptions,
	})
}

// handleAdminTicketAttributeRelationsCreate handles file upload and creation.
func handleAdminTicketAttributeRelationsCreate(c *gin.Context) {
	svc, err := getTicketAttributeRelationsService()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	// Get uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "No file uploaded"})
		return
	}

	// Validate filename extension
	filename := file.Filename
	lowerFilename := strings.ToLower(filename)
	if !strings.HasSuffix(lowerFilename, ".csv") && !strings.HasSuffix(lowerFilename, ".xlsx") {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "File must be CSV or Excel (.xlsx)"})
		return
	}

	// Check filename uniqueness
	exists, err := svc.FilenameExists(c.Request.Context(), filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to check filename"})
		return
	}
	if exists {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "A relation with this filename already exists"})
		return
	}

	// Read file content
	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to open file"})
		return
	}
	defer f.Close()

	content, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to read file"})
		return
	}

	if len(content) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "File is empty"})
		return
	}

	// Parse file to extract attributes and validate
	attr1, attr2, pairs, err := svc.ParseUploadedFile(filename, content)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	if len(pairs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "File contains no data rows"})
		return
	}

	// Get priority
	priority, _ := strconv.ParseInt(c.PostForm("priority"), 10, 64)
	if priority < 1 {
		priority = 1
	}

	// Get user ID
	userID := int64(1)
	if user, exists := c.Get("user_id"); exists {
		if uid, ok := user.(int); ok {
			userID = int64(uid)
		}
	}

	// Prepare data for storage
	aclData := svc.PrepareDataForStorage(filename, content)

	// Create the relation
	relation := &models.TicketAttributeRelation{
		Filename:   filename,
		Attribute1: attr1,
		Attribute2: attr2,
		ACLData:    aclData,
		Priority:   priority,
	}

	id, err := svc.Create(c.Request.Context(), relation, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to create relation"})
		return
	}

	// Handle "add missing dynamic field values" checkbox
	if c.PostForm("dynamic_field_config_update") == "1" {
		// Re-fetch the relation to get the parsed Data field populated
		createdRelation, fetchErr := svc.GetByID(c.Request.Context(), id)
		if fetchErr == nil && createdRelation != nil {
			_, _ = addMissingValuesToAttribute(c.Request.Context(), createdRelation, userID)
		}
	}

	// Handle HTMX redirect
	if c.GetHeader("HX-Request") == "true" {
		c.Header("HX-Redirect", "/admin/ticket-attribute-relations")
		c.Status(http.StatusOK)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"id":      id,
		"message": "Relation created successfully",
	})
}

// handleAdminTicketAttributeRelationsEdit renders the edit form.
func handleAdminTicketAttributeRelationsEdit(c *gin.Context) {
	svc, err := getTicketAttributeRelationsService()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database connection failed")
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		sendErrorResponse(c, http.StatusBadRequest, "Invalid ID")
		return
	}

	relation, err := svc.GetByID(c.Request.Context(), id)
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to fetch relation")
		return
	}
	if relation == nil {
		sendErrorResponse(c, http.StatusNotFound, "Relation not found")
		return
	}

	// Get priority options
	priorityOptions, _ := svc.GetPriorityOptions(c.Request.Context())

	// Check for missing dynamic field values
	missingValues := checkMissingDynamicFieldValues(c.Request.Context(), relation)

	if getPongo2Renderer() == nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Template renderer unavailable")
		return
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/ticket_attribute_relations.pongo2", pongo2.Context{
		"Relation":        relation,
		"User":            getUserMapForTemplate(c),
		"ActivePage":      "admin",
		"Mode":            "edit",
		"PriorityOptions": priorityOptions,
		"MissingValues":   missingValues,
	})
}

// handleAdminTicketAttributeRelationsUpdate handles updating a relation.
func handleAdminTicketAttributeRelationsUpdate(c *gin.Context) {
	svc, err := getTicketAttributeRelationsService()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid ID"})
		return
	}

	// Get existing relation
	existing, err := svc.GetByID(c.Request.Context(), id)
	if err != nil || existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Relation not found"})
		return
	}

	// Get user ID early - we need it for both updates and adding missing values
	userID := int64(1)
	if user, exists := c.Get("user_id"); exists {
		if uid, ok := user.(int); ok {
			userID = int64(uid)
		}
	}

	updates := make(map[string]interface{})
	actionTaken := false // Track if any action was performed

	// Check for new file upload
	file, err := c.FormFile("file")
	if err == nil && file != nil {
		// Validate filename extension
		filename := file.Filename
		lowerFilename := strings.ToLower(filename)
		if !strings.HasSuffix(lowerFilename, ".csv") && !strings.HasSuffix(lowerFilename, ".xlsx") {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "File must be CSV or Excel (.xlsx)"})
			return
		}

		// Check filename uniqueness if it changed
		if filename != existing.Filename {
			exists, err := svc.FilenameExists(c.Request.Context(), filename)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to check filename"})
				return
			}
			if exists {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "A relation with this filename already exists"})
				return
			}
		}

		// Read file content
		f, err := file.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to open file"})
			return
		}
		defer f.Close()

		content, err := io.ReadAll(f)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to read file"})
			return
		}

		// Parse file
		attr1, attr2, pairs, err := svc.ParseUploadedFile(filename, content)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}

		if len(pairs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "File contains no data rows"})
			return
		}

		aclData := svc.PrepareDataForStorage(filename, content)

		updates["filename"] = filename
		updates["attribute_1"] = attr1
		updates["attribute_2"] = attr2
		updates["acl_data"] = aclData
	}

	// Check for priority change
	if priorityStr := c.PostForm("priority"); priorityStr != "" {
		priority, err := strconv.ParseInt(priorityStr, 10, 64)
		if err == nil && priority > 0 && priority != existing.Priority {
			updates["priority"] = priority
		}
	}

	// Handle "add missing dynamic field values" checkbox BEFORE checking if updates exist
	// This is an action that should be performed even if no other changes were made
	if c.PostForm("dynamic_field_config_update") == "1" {
		addedCount, addErr := addMissingValuesToAttribute(c.Request.Context(), existing, userID)
		if addErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to add missing values: " + addErr.Error()})
			return
		}
		if addedCount > 0 {
			actionTaken = true
		}
	}

	// Update the relation if there are changes
	if len(updates) > 0 {
		err = svc.Update(c.Request.Context(), id, updates, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update relation"})
			return
		}
		actionTaken = true
	}

	// If no changes and no action taken, redirect back gracefully
	if !actionTaken {
		if c.GetHeader("HX-Request") == "true" {
			if c.PostForm("continue") == "1" {
				c.Header("HX-Redirect", "/admin/ticket-attribute-relations/"+strconv.FormatInt(id, 10))
			} else {
				c.Header("HX-Redirect", "/admin/ticket-attribute-relations")
			}
			c.Status(http.StatusOK)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "No changes needed",
		})
		return
	}

	// Handle HTMX redirect for successful update
	if c.GetHeader("HX-Request") == "true" {
		if c.PostForm("continue") == "1" {
			// Stay on edit page - add ?saved=1 to trigger toast notification
			c.Header("HX-Redirect", "/admin/ticket-attribute-relations/"+strconv.FormatInt(id, 10)+"?saved=1")
		} else {
			// Go back to list page
			c.Header("HX-Redirect", "/admin/ticket-attribute-relations?saved=1")
		}
		c.Status(http.StatusOK)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Relation updated successfully",
	})
}

// handleAdminTicketAttributeRelationsDelete handles deleting a relation.
func handleAdminTicketAttributeRelationsDelete(c *gin.Context) {
	svc, err := getTicketAttributeRelationsService()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid ID"})
		return
	}

	err = svc.Delete(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete relation"})
		return
	}

	// Handle HTMX request
	if c.GetHeader("HX-Request") == "true" {
		c.Header("HX-Redirect", "/admin/ticket-attribute-relations")
		c.Status(http.StatusOK)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Relation deleted successfully",
	})
}

// handleAdminTicketAttributeRelationsDownload handles downloading the original file.
func handleAdminTicketAttributeRelationsDownload(c *gin.Context) {
	svc, err := getTicketAttributeRelationsService()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database connection failed")
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		sendErrorResponse(c, http.StatusBadRequest, "Invalid ID")
		return
	}

	relation, err := svc.GetByID(c.Request.Context(), id)
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to fetch relation")
		return
	}
	if relation == nil {
		sendErrorResponse(c, http.StatusNotFound, "Relation not found")
		return
	}

	data, err := svc.GetRawDataForDownload(relation)
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to prepare download")
		return
	}

	// Determine content type
	contentType := "text/csv; charset=utf-8"
	if strings.HasSuffix(strings.ToLower(relation.Filename), ".xlsx") {
		contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	}

	c.Header("Content-Disposition", "attachment; filename="+relation.Filename)
	c.Data(http.StatusOK, contentType, data)
}

// handleAPITicketAttributeRelationsEvaluate returns filtered values for AJAX.
func handleAPITicketAttributeRelationsEvaluate(c *gin.Context) {
	svc, err := getTicketAttributeRelationsService()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	attr := c.Query("attribute")
	value := c.Query("value")

	if attr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Attribute parameter required"})
		return
	}

	result, err := svc.EvaluateRelations(c.Request.Context(), attr, value)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to evaluate relations"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"allowed_values": result,
	})
}

// checkMissingDynamicFieldValues checks if any values in the relation data are missing
// from the corresponding dynamic field's PossibleValues configuration.
func checkMissingDynamicFieldValues(ctx context.Context, relation *models.TicketAttributeRelation) map[string][]string {
	missing := make(map[string][]string)

	// Check both attributes if they are dynamic fields
	for _, attr := range []string{relation.Attribute1, relation.Attribute2} {
		if !models.IsDynamicFieldAttribute(attr) {
			continue
		}

		fieldName := models.GetDynamicFieldName(attr)
		possibleValues := getDynamicFieldPossibleValues(ctx, fieldName)
		if possibleValues == nil {
			continue
		}

		// Build set of possible values
		possibleSet := make(map[string]bool)
		for _, v := range possibleValues {
			possibleSet[v] = true
		}

		// Check which values in relation data are missing
		var missingVals []string
		seen := make(map[string]bool)

		for _, pair := range relation.Data {
			var val string
			if attr == relation.Attribute1 {
				val = pair.Attribute1Value
			} else {
				val = pair.Attribute2Value
			}

			if val != "" && !possibleSet[val] && !seen[val] {
				missingVals = append(missingVals, val)
				seen[val] = true
			}
		}

		if len(missingVals) > 0 {
			missing[attr] = missingVals
		}
	}

	return missing
}

// getDynamicFieldPossibleValues returns the PossibleValues for a dynamic field.
func getDynamicFieldPossibleValues(ctx context.Context, fieldName string) []string {
	db, err := database.GetDB()
	if err != nil {
		return nil
	}

	query := database.ConvertPlaceholders(`
		SELECT config FROM dynamic_field WHERE name = ?
	`)

	var config string
	err = db.QueryRowContext(ctx, query, fieldName).Scan(&config)
	if err != nil {
		return nil
	}

	// Parse config to extract PossibleValues
	// The config is stored as YAML, but for simplicity we'll do basic parsing
	// TODO: Implement proper YAML parsing for dynamic field config
	return nil
}

// addMissingValuesToAttribute adds any values from the relation's CSV data that don't exist
// in the corresponding attribute's value set. This works for:
// - Service: adds missing service names to the service table
// - DynamicField_*: adds missing values to dynamic field's PossibleValues (for Dropdown/Multiselect)
// Returns the count of values added.
func addMissingValuesToAttribute(ctx context.Context, relation *models.TicketAttributeRelation, userID int64) (int, error) {
	db, err := database.GetDB()
	if err != nil {
		return 0, err
	}

	totalAdded := 0

	// Process both attributes
	for _, attrInfo := range []struct {
		attr   string
		values []string
	}{
		{relation.Attribute1, relation.GetUniqueAttribute1Values()},
		{relation.Attribute2, relation.GetUniqueAttribute2Values()},
	} {
		attr := attrInfo.attr
		values := attrInfo.values

		switch attr {
		case "Service":
			added, err := addMissingServices(ctx, db, values, userID)
			if err != nil {
				return totalAdded, err
			}
			totalAdded += added

		case "Queue", "State", "Priority", "Type", "SLA":
			// These standard attributes typically shouldn't be auto-created
			// as they require additional configuration (system states, etc.)
			// Skip silently
			continue

		default:
			if models.IsDynamicFieldAttribute(attr) {
				fieldName := models.GetDynamicFieldName(attr)
				added, err := addMissingDynamicFieldValues(ctx, db, fieldName, values, userID)
				if err != nil {
					return totalAdded, err
				}
				totalAdded += added
			}
		}
	}

	return totalAdded, nil
}

// addMissingServices adds any service names that don't exist in the service table.
func addMissingServices(ctx context.Context, db *sql.DB, serviceNames []string, userID int64) (int, error) {
	if len(serviceNames) == 0 {
		return 0, nil
	}

	// Get existing services
	existingQuery := database.ConvertPlaceholders(`SELECT name FROM service WHERE valid_id = 1`)
	rows, err := db.QueryContext(ctx, existingQuery)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	existing := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return 0, err
		}
		existing[name] = true
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	// Find missing services
	var missing []string
	for _, name := range serviceNames {
		name = strings.TrimSpace(name)
		if name == "" || name == "-" {
			continue
		}
		if !existing[name] {
			missing = append(missing, name)
		}
	}

	if len(missing) == 0 {
		return 0, nil
	}

	// Insert missing services
	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO service (name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 1, NOW(), ?, NOW(), ?)
	`)

	added := 0
	for _, name := range missing {
		_, err := db.ExecContext(ctx, insertQuery, name, userID, userID)
		if err != nil {
			// Skip duplicates (race condition or case sensitivity)
			if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "Duplicate") {
				continue
			}
			return added, err
		}
		added++
	}

	return added, nil
}

// addMissingDynamicFieldValues adds missing values to a dynamic field's PossibleValues config.
// This only works for Dropdown and Multiselect field types.
func addMissingDynamicFieldValues(ctx context.Context, db *sql.DB, fieldName string, values []string, userID int64) (int, error) {
	if len(values) == 0 {
		return 0, nil
	}

	// Get the dynamic field by name
	field, err := GetDynamicFieldByName(fieldName)
	if err != nil {
		return 0, err
	}
	if field == nil {
		// Field doesn't exist - can't add values
		return 0, nil
	}

	// Only Dropdown and Multiselect fields have PossibleValues
	if field.FieldType != "Dropdown" && field.FieldType != "Multiselect" {
		return 0, nil
	}

	// Parse the config if not already parsed
	if field.Config == nil {
		if err := field.ParseConfig(); err != nil {
			return 0, err
		}
	}

	// Initialize PossibleValues if nil
	if field.Config.PossibleValues == nil {
		field.Config.PossibleValues = make(map[string]string)
	}

	// Find missing values
	added := 0
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || value == "-" {
			continue
		}

		// Check if value already exists (as either key or value)
		exists := false
		for k, v := range field.Config.PossibleValues {
			if k == value || v == value {
				exists = true
				break
			}
		}

		if !exists {
			// Add value - use value as both key and display text
			field.Config.PossibleValues[value] = value
			added++
		}
	}

	if added == 0 {
		return 0, nil
	}

	// Serialize config back to YAML and update
	if err := field.SerializeConfig(); err != nil {
		return 0, err
	}

	// Update the dynamic field in the database
	if err := UpdateDynamicField(field, int(userID)); err != nil {
		return 0, err
	}

	return added, nil
}

// handleAdminTicketAttributeRelationsReorder handles reordering relations via drag-and-drop.
// It accepts a JSON array of relation IDs in the new order.
func handleAdminTicketAttributeRelationsReorder(c *gin.Context) {
	svc, err := getTicketAttributeRelationsService()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	// Parse the ordered IDs from JSON body
	var req struct {
		OrderedIDs []int64 `json:"ordered_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request body"})
		return
	}

	if len(req.OrderedIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "No IDs provided"})
		return
	}

	// Get user ID
	userID := int64(1)
	if user, exists := c.Get("user_id"); exists {
		if uid, ok := user.(int); ok {
			userID = int64(uid)
		}
	}

	// Reorder by updating priorities to match the new order
	if err := svc.ReorderPriorities(c.Request.Context(), req.OrderedIDs, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to reorder relations"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Relations reordered successfully",
	})
}
