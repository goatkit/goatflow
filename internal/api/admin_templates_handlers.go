package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/shared"
)

// StandardTemplate represents a response template in the system.
type StandardTemplate struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	Text         string    `json:"text"`
	ContentType  string    `json:"content_type"`
	TemplateType string    `json:"template_type"` // comma-separated: Answer,Forward,Note
	Comments     string    `json:"comments"`
	ValidID      int       `json:"valid_id"`
	CreateTime   time.Time `json:"create_time"`
	CreateBy     int       `json:"create_by"`
	ChangeTime   time.Time `json:"change_time"`
	ChangeBy     int       `json:"change_by"`
}

// StandardTemplateWithStats includes usage statistics.
type StandardTemplateWithStats struct {
	StandardTemplate
	QueueCount int `json:"queue_count"`
}

// TemplateTypeOption represents a template type for UI selection.
type TemplateTypeOption struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// ValidTemplateTypes returns all available template types matching OTRS.
func ValidTemplateTypes() []TemplateTypeOption {
	return []TemplateTypeOption{
		{Key: "Answer", Label: "Answer"},
		{Key: "Create", Label: "Create"},
		{Key: "Email", Label: "Email"},
		{Key: "Forward", Label: "Forward"},
		{Key: "Note", Label: "Note"},
		{Key: "PhoneCall", Label: "Phone call"},
		{Key: "ProcessManagement", Label: "Process Management"},
		{Key: "Snippet", Label: "Snippet"},
	}
}

// ParseTemplateTypes splits a comma-separated type string into a slice.
func ParseTemplateTypes(typeStr string) []string {
	if typeStr == "" {
		return nil
	}
	parts := strings.Split(typeStr, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	sort.Strings(result)
	return result
}

// JoinTemplateTypes joins template types into a sorted comma-separated string.
func JoinTemplateTypes(types []string) string {
	sorted := make([]string, len(types))
	copy(sorted, types)
	sort.Strings(sorted)
	return strings.Join(sorted, ",")
}

// Repository functions

// ListStandardTemplates returns all templates with optional filters.
func ListStandardTemplates(
	search string, validFilter string, typeFilter string, sortBy string, sortOrder string,
) ([]StandardTemplateWithStats, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT
			t.id, t.name, t.text, t.content_type, t.template_type,
			t.comments, t.valid_id, t.create_time, t.create_by,
			t.change_time, t.change_by,
			COUNT(DISTINCT qt.queue_id) as queue_count
		FROM standard_template t
		LEFT JOIN queue_standard_template qt ON qt.standard_template_id = t.id
		WHERE 1=1
	`

	var args []interface{}

	if search != "" {
		query += " AND (LOWER(t.name) LIKE ? OR LOWER(t.comments) LIKE ?)"
		searchPattern := "%" + strings.ToLower(search) + "%"
		args = append(args, searchPattern, searchPattern)
	}

	if validFilter == "valid" {
		query += " AND t.valid_id = ?"
		args = append(args, 1)
	} else if validFilter == "invalid" {
		query += " AND t.valid_id != ?"
		args = append(args, 1)
	}

	if typeFilter != "" && typeFilter != "all" {
		query += " AND t.template_type LIKE ?"
		args = append(args, "%"+typeFilter+"%")
	}

	query += ` GROUP BY t.id, t.name, t.text, t.content_type, t.template_type,
		t.comments, t.valid_id, t.create_time, t.create_by, t.change_time, t.change_by`

	// Sorting
	validSortColumns := map[string]string{
		"id":            "t.id",
		"name":          "t.name",
		"template_type": "t.template_type",
		"valid_id":      "t.valid_id",
		"queue_count":   "queue_count",
		"change_time":   "t.change_time",
	}
	sortCol, ok := validSortColumns[sortBy]
	if !ok {
		sortCol = "t.name"
	}
	if sortOrder != "desc" {
		sortOrder = "asc"
	}
	query += fmt.Sprintf(" ORDER BY %s %s", sortCol, sortOrder)

	rows, err := db.Query(database.ConvertPlaceholders(query), args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var templates []StandardTemplateWithStats
	for rows.Next() {
		var t StandardTemplateWithStats
		var text, contentType, templateType, comments sql.NullString

		err := rows.Scan(
			&t.ID, &t.Name, &text, &contentType, &templateType,
			&comments, &t.ValidID, &t.CreateTime, &t.CreateBy,
			&t.ChangeTime, &t.ChangeBy, &t.QueueCount,
		)
		if err != nil {
			continue
		}

		t.Text = text.String
		t.ContentType = contentType.String
		t.TemplateType = templateType.String
		t.Comments = comments.String

		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating templates: %w", err)
	}

	return templates, nil
}

// GetStandardTemplate retrieves a single template by ID.
func GetStandardTemplate(id int) (*StandardTemplate, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, name, text, content_type, template_type, comments,
			valid_id, create_time, create_by, change_time, change_by
		FROM standard_template
		WHERE id = ?
	`

	var t StandardTemplate
	var text, contentType, templateType, comments sql.NullString

	err = db.QueryRow(database.ConvertPlaceholders(query), id).Scan(
		&t.ID, &t.Name, &text, &contentType, &templateType, &comments,
		&t.ValidID, &t.CreateTime, &t.CreateBy, &t.ChangeTime, &t.ChangeBy,
	)
	if err == sql.ErrNoRows {
		return nil, nil //nolint:nilnil
	}
	if err != nil {
		return nil, err
	}

	t.Text = text.String
	t.ContentType = contentType.String
	t.TemplateType = templateType.String
	t.Comments = comments.String

	return &t, nil
}

// CheckTemplateNameExists checks if a template name already exists (excluding a specific ID).
func CheckTemplateNameExists(name string, excludeID int) (bool, error) {
	db, err := database.GetDB()
	if err != nil {
		return false, err
	}

	query := `SELECT COUNT(*) FROM standard_template WHERE LOWER(name) = LOWER(?)`
	args := []interface{}{name}

	if excludeID > 0 {
		query += ` AND id != ?`
		args = append(args, excludeID)
	}

	var count int
	err = db.QueryRow(database.ConvertPlaceholders(query), args...).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// CreateStandardTemplate creates a new template.
func CreateStandardTemplate(t *StandardTemplate, userID int) (int, error) {
	db, err := database.GetDB()
	if err != nil {
		return 0, err
	}

	now := time.Now()

	// Handle nullable fields
	var text, contentType, templateType, comments interface{}
	if t.Text != "" {
		text = t.Text
	}
	if t.ContentType != "" {
		contentType = t.ContentType
	} else {
		contentType = "text/plain"
	}
	if t.TemplateType != "" {
		templateType = t.TemplateType
	} else {
		templateType = "Answer"
	}
	if t.Comments != "" {
		comments = t.Comments
	}

	query := `
		INSERT INTO standard_template
			(name, text, content_type, template_type, comments, valid_id,
			 create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := db.Exec(database.ConvertPlaceholders(query),
		t.Name, text, contentType, templateType, comments, t.ValidID,
		now, userID, now, userID,
	)
	if err != nil {
		return 0, fmt.Errorf("insert failed: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		// For PostgreSQL, query the ID
		var newID int
		err = db.QueryRow(database.ConvertPlaceholders(
			`SELECT id FROM standard_template WHERE name = ? AND create_by = ? ORDER BY id DESC LIMIT 1`,
		), t.Name, userID).Scan(&newID)
		if err != nil {
			return 0, err
		}
		return newID, nil
	}

	return int(id), nil
}

// UpdateStandardTemplate updates an existing template.
func UpdateStandardTemplate(t *StandardTemplate, userID int) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}

	now := time.Now()

	var text, contentType, templateType, comments interface{}
	if t.Text != "" {
		text = t.Text
	}
	if t.ContentType != "" {
		contentType = t.ContentType
	}
	if t.TemplateType != "" {
		templateType = t.TemplateType
	}
	if t.Comments != "" {
		comments = t.Comments
	}

	query := `
		UPDATE standard_template
		SET name = ?, text = ?, content_type = ?, template_type = ?,
			comments = ?, valid_id = ?, change_time = ?, change_by = ?
		WHERE id = ?
	`

	_, err = db.Exec(database.ConvertPlaceholders(query),
		t.Name, text, contentType, templateType, comments,
		t.ValidID, now, userID, t.ID,
	)
	return err
}

// DeleteStandardTemplate deletes a template and its relationships.
func DeleteStandardTemplate(id int) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}

	// Delete queue relationships
	_, err = db.Exec(database.ConvertPlaceholders(
		`DELETE FROM queue_standard_template WHERE standard_template_id = ?`,
	), id)
	if err != nil {
		return fmt.Errorf("failed to delete queue relationships: %w", err)
	}

	// Delete attachment relationships
	_, err = db.Exec(database.ConvertPlaceholders(
		`DELETE FROM standard_template_attachment WHERE standard_template_id = ?`,
	), id)
	if err != nil {
		return fmt.Errorf("failed to delete attachment relationships: %w", err)
	}

	// Delete the template
	_, err = db.Exec(database.ConvertPlaceholders(
		`DELETE FROM standard_template WHERE id = ?`,
	), id)
	if err != nil {
		return fmt.Errorf("failed to delete template: %w", err)
	}

	return nil
}

// Queue relationship functions

// GetTemplateQueues returns all queues assigned to a template.
func GetTemplateQueues(templateID int) ([]int, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(database.ConvertPlaceholders(
		`SELECT queue_id FROM queue_standard_template WHERE standard_template_id = ?`,
	), templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var queueIDs []int
	for rows.Next() {
		var queueID int
		if err := rows.Scan(&queueID); err != nil {
			continue
		}
		queueIDs = append(queueIDs, queueID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating template queues: %w", err)
	}

	return queueIDs, nil
}

// SetTemplateQueues sets the queue assignments for a template.
func SetTemplateQueues(templateID int, queueIDs []int, userID int) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}

	// Delete existing assignments
	_, err = db.Exec(database.ConvertPlaceholders(
		`DELETE FROM queue_standard_template WHERE standard_template_id = ?`,
	), templateID)
	if err != nil {
		return err
	}

	if len(queueIDs) == 0 {
		return nil
	}

	// Insert new assignments
	now := time.Now()
	for _, queueID := range queueIDs {
		_, err = db.Exec(database.ConvertPlaceholders(`
			INSERT INTO queue_standard_template
				(queue_id, standard_template_id, create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, ?, ?, ?)
		`), queueID, templateID, now, userID, now, userID)
		if err != nil {
			return fmt.Errorf("failed to assign queue %d: %w", queueID, err)
		}
	}

	return nil
}

// Attachment relationship functions

// ListStandardAttachments returns all valid standard attachments.
func ListStandardAttachments() ([]StandardAttachment, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT id, name, content_type, filename, comments, valid_id,
			create_time, create_by, change_time, change_by
		FROM standard_attachment
		WHERE valid_id = 1
		ORDER BY name
	`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attachments []StandardAttachment
	for rows.Next() {
		var a StandardAttachment
		if err := rows.Scan(&a.ID, &a.Name, &a.ContentType, &a.Filename, &a.Comments,
			&a.ValidID, &a.CreateTime, &a.CreateBy, &a.ChangeTime, &a.ChangeBy); err != nil {
			continue
		}
		attachments = append(attachments, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating attachments: %w", err)
	}

	return attachments, nil
}

// GetTemplateAttachments returns all attachment IDs assigned to a template.
func GetTemplateAttachments(templateID int) ([]int, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(database.ConvertPlaceholders(
		`SELECT standard_attachment_id FROM standard_template_attachment WHERE standard_template_id = ?`,
	), templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attachmentIDs []int
	for rows.Next() {
		var attachmentID int
		if err := rows.Scan(&attachmentID); err != nil {
			continue
		}
		attachmentIDs = append(attachmentIDs, attachmentID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating template attachments: %w", err)
	}

	return attachmentIDs, nil
}

// SetTemplateAttachments sets the attachment assignments for a template.
func SetTemplateAttachments(templateID int, attachmentIDs []int, userID int) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}

	// Delete existing assignments
	_, err = db.Exec(database.ConvertPlaceholders(
		`DELETE FROM standard_template_attachment WHERE standard_template_id = ?`,
	), templateID)
	if err != nil {
		return err
	}

	if len(attachmentIDs) == 0 {
		return nil
	}

	// Insert new assignments
	now := time.Now()
	for _, attachmentID := range attachmentIDs {
		_, err = db.Exec(database.ConvertPlaceholders(`
			INSERT INTO standard_template_attachment
				(standard_attachment_id, standard_template_id, create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, ?, ?, ?)
		`), attachmentID, templateID, now, userID, now, userID)
		if err != nil {
			return fmt.Errorf("failed to assign attachment %d: %w", attachmentID, err)
		}
	}

	return nil
}

// TemplateExport represents a single template for YAML export.
type TemplateExport struct {
	Name         string   `yaml:"name"`
	Text         string   `yaml:"text,omitempty"`
	ContentType  string   `yaml:"content_type,omitempty"`
	TemplateType string   `yaml:"template_type,omitempty"`
	Comments     string   `yaml:"comments,omitempty"`
	Valid        bool     `yaml:"valid"`
	Queues       []string `yaml:"queues,omitempty"`
	Attachments  []string `yaml:"attachments,omitempty"`
}

// TemplateExportFile represents the full export file structure.
type TemplateExportFile struct {
	Version    string           `yaml:"version"`
	ExportedAt string           `yaml:"exported_at"`
	Templates  []TemplateExport `yaml:"templates"`
}

// ExportTemplate exports a single template with its relationships.
func ExportTemplate(id int) (*TemplateExport, error) {
	template, err := GetStandardTemplate(id)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, fmt.Errorf("template not found")
	}

	export := &TemplateExport{
		Name:         template.Name,
		Text:         template.Text,
		ContentType:  template.ContentType,
		TemplateType: template.TemplateType,
		Comments:     template.Comments,
		Valid:        template.ValidID == 1,
	}

	// Get queue names
	queueIDs, err := GetTemplateQueues(id)
	if err == nil && len(queueIDs) > 0 {
		db, _ := database.GetDB() //nolint:errcheck // Graceful degradation
		for _, qid := range queueIDs {
			var queueName string
			err := db.QueryRow(database.ConvertPlaceholders(
				`SELECT name FROM queue WHERE id = ?`,
			), qid).Scan(&queueName)
			if err == nil {
				export.Queues = append(export.Queues, queueName)
			}
		}
	}

	// Get attachment names
	attachmentIDs, err := GetTemplateAttachments(id)
	if err == nil && len(attachmentIDs) > 0 {
		db, _ := database.GetDB() //nolint:errcheck // Graceful degradation
		for _, aid := range attachmentIDs {
			var attachmentName string
			err := db.QueryRow(database.ConvertPlaceholders(
				`SELECT name FROM standard_attachment WHERE id = ?`,
			), aid).Scan(&attachmentName)
			if err == nil {
				export.Attachments = append(export.Attachments, attachmentName)
			}
		}
	}

	return export, nil
}

// ExportAllTemplates exports all templates.
func ExportAllTemplates() (*TemplateExportFile, error) {
	templates, err := ListStandardTemplates("", "all", "all", "name", "asc")
	if err != nil {
		return nil, err
	}

	export := &TemplateExportFile{
		Version:    "1.0",
		ExportedAt: time.Now().Format(time.RFC3339),
		Templates:  make([]TemplateExport, 0, len(templates)),
	}

	for _, t := range templates {
		te, err := ExportTemplate(t.ID)
		if err != nil {
			continue
		}
		export.Templates = append(export.Templates, *te)
	}

	return export, nil
}

// ImportTemplates imports templates from YAML data.
func ImportTemplates(data []byte, overwrite bool, userID int) (imported int, skipped int, errors []string) {
	var exportFile TemplateExportFile
	if err := yaml.Unmarshal(data, &exportFile); err != nil {
		errors = append(errors, fmt.Sprintf("Invalid YAML format: %v", err))
		return
	}

	db, err := database.GetDB()
	if err != nil {
		errors = append(errors, fmt.Sprintf("Database error: %v", err))
		return
	}

	for _, te := range exportFile.Templates {
		if te.Name == "" {
			errors = append(errors, "Skipped template with empty name")
			skipped++
			continue
		}

		// Check if template exists
		exists, err := CheckTemplateNameExists(te.Name, 0)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Error checking template '%s': %v", te.Name, err))
			skipped++
			continue
		}

		if exists && !overwrite {
			skipped++
			continue
		}

		validID := 2 // invalid
		if te.Valid {
			validID = 1
		}

		template := &StandardTemplate{
			Name:         te.Name,
			Text:         te.Text,
			ContentType:  te.ContentType,
			TemplateType: te.TemplateType,
			Comments:     te.Comments,
			ValidID:      validID,
		}

		var templateID int
		if exists && overwrite {
			// Get existing ID
			err := db.QueryRow(database.ConvertPlaceholders(
				`SELECT id FROM standard_template WHERE name = ?`,
			), te.Name).Scan(&templateID)
			if err != nil {
				errors = append(errors, fmt.Sprintf("Error finding template '%s': %v", te.Name, err))
				skipped++
				continue
			}
			template.ID = templateID
			if err := UpdateStandardTemplate(template, userID); err != nil {
				errors = append(errors, fmt.Sprintf("Error updating template '%s': %v", te.Name, err))
				skipped++
				continue
			}
		} else {
			newID, err := CreateStandardTemplate(template, userID)
			if err != nil {
				errors = append(errors, fmt.Sprintf("Error creating template '%s': %v", te.Name, err))
				skipped++
				continue
			}
			templateID = newID
		}

		// Assign queues by name
		if len(te.Queues) > 0 {
			var queueIDs []int
			for _, queueName := range te.Queues {
				var qid int
				err := db.QueryRow(database.ConvertPlaceholders(
					`SELECT id FROM queue WHERE name = ?`,
				), queueName).Scan(&qid)
				if err == nil {
					queueIDs = append(queueIDs, qid)
				}
			}
			if len(queueIDs) > 0 {
				_ = SetTemplateQueues(templateID, queueIDs, userID) //nolint:errcheck // Best-effort queue assignment
			}
		}

		// Assign attachments by name
		if len(te.Attachments) > 0 {
			var attachmentIDs []int
			for _, attachmentName := range te.Attachments {
				var aid int
				err := db.QueryRow(database.ConvertPlaceholders(
					`SELECT id FROM standard_attachment WHERE name = ?`,
				), attachmentName).Scan(&aid)
				if err == nil {
					attachmentIDs = append(attachmentIDs, aid)
				}
			}
			if len(attachmentIDs) > 0 {
				_ = SetTemplateAttachments(templateID, attachmentIDs, userID) //nolint:errcheck // Best-effort attachment assignment
			}
		}

		imported++
	}

	return
}

// Handler functions

// handleAdminStandardTemplates renders the admin templates list page.
func handleAdminStandardTemplates(c *gin.Context) {
	search := c.Query("search")
	validFilter := c.DefaultQuery("valid", "all")
	typeFilter := c.DefaultQuery("type", "all")
	sortBy := c.DefaultQuery("sort", "name")
	sortOrder := c.DefaultQuery("order", "asc")

	templates, err := ListStandardTemplates(search, validFilter, typeFilter, sortBy, sortOrder)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load templates: "+err.Error())
		return
	}

	renderer := shared.GetGlobalRenderer()
	if renderer == nil {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, "<h1>Templates</h1>")
		return
	}

	renderer.HTML(c, http.StatusOK, "pages/admin/templates.pongo2", gin.H{
		"Title":         "Response Templates",
		"Templates":     templates,
		"TemplateTypes": ValidTemplateTypes(),
		"Search":        search,
		"ValidFilter":   validFilter,
		"TypeFilter":    typeFilter,
		"SortBy":        sortBy,
		"SortOrder":     sortOrder,
		"ActivePage":    "admin",
	})
}

// handleAdminStandardTemplateNew renders the new template form.
func handleAdminStandardTemplateNew(c *gin.Context) {
	renderer := shared.GetGlobalRenderer()
	if renderer == nil {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, "<h1>New Template</h1>")
		return
	}

	renderer.HTML(c, http.StatusOK, "pages/admin/template_form.pongo2", gin.H{
		"Title":         "New Response Template",
		"Template":      &StandardTemplate{ValidID: 1, TemplateType: "Answer"},
		"IsNew":         true,
		"TemplateTypes": ValidTemplateTypes(),
		"ActivePage":    "admin",
	})
}

// handleAdminStandardTemplateEdit renders the edit template form.
func handleAdminStandardTemplateEdit(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
		return
	}

	template, err := GetStandardTemplate(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load template"})
		return
	}
	if template == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	renderer := shared.GetGlobalRenderer()
	if renderer == nil {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, "<h1>Edit Template</h1>")
		return
	}

	renderer.HTML(c, http.StatusOK, "pages/admin/template_form.pongo2", gin.H{
		"Title":         "Edit Response Template",
		"Template":      template,
		"IsNew":         false,
		"TemplateTypes": ValidTemplateTypes(),
		"ActivePage":    "admin",
	})
}

// handleCreateStandardTemplate handles POST to create a new template.
func handleCreateStandardTemplate(c *gin.Context) {
	template, err := parseTemplateForm(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if template.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name is required"})
		return
	}

	exists, err := CheckTemplateNameExists(template.Name, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check name"})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "Template name already exists"})
		return
	}

	userID := getUserID(c)
	templateID, err := CreateStandardTemplate(template, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create template: " + err.Error()})
		return
	}

	if isHTMXRequest(c) {
		c.Header("HX-Redirect", "/admin/templates")
		c.Status(http.StatusOK)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"id":      templateID,
	})
}

// handleUpdateStandardTemplate handles PUT to update an existing template.
func handleUpdateStandardTemplate(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
		return
	}

	existing, err := GetStandardTemplate(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load template"})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	template, err := parseTemplateForm(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	template.ID = id

	if template.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name is required"})
		return
	}

	if template.Name != existing.Name {
		exists, err := CheckTemplateNameExists(template.Name, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check name"})
			return
		}
		if exists {
			c.JSON(http.StatusConflict, gin.H{"error": "Template name already exists"})
			return
		}
	}

	userID := getUserID(c)
	if err := UpdateStandardTemplate(template, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update template"})
		return
	}

	if isHTMXRequest(c) {
		c.Header("HX-Redirect", "/admin/templates")
		c.Status(http.StatusOK)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleDeleteStandardTemplate handles DELETE to remove a template.
func handleDeleteStandardTemplate(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
		return
	}

	existing, err := GetStandardTemplate(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load template"})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	if err := DeleteStandardTemplate(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete template"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleAdminStandardTemplateQueues renders the template queue assignment page.
func handleAdminStandardTemplateQueues(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
		return
	}

	template, err := GetStandardTemplate(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load template"})
		return
	}
	if template == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	assignedQueues, err := GetTemplateQueues(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load queue assignments"})
		return
	}

	// Get all queues
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	rows, err := db.Query(database.ConvertPlaceholders(
		`SELECT id, name FROM queue WHERE valid_id = 1 ORDER BY name`,
	))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load queues"})
		return
	}
	defer rows.Close()

	type QueueOption struct {
		ID       int
		Name     string
		Selected bool
	}

	assignedMap := make(map[int]bool)
	for _, qid := range assignedQueues {
		assignedMap[qid] = true
	}

	var queues []QueueOption
	for rows.Next() {
		var q QueueOption
		if err := rows.Scan(&q.ID, &q.Name); err != nil {
			continue
		}
		q.Selected = assignedMap[q.ID]
		queues = append(queues, q)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error iterating queues"})
		return
	}

	renderer := shared.GetGlobalRenderer()
	if renderer == nil {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, "<h1>Template Queues</h1>")
		return
	}

	renderer.HTML(c, http.StatusOK, "pages/admin/template_queues.pongo2", gin.H{
		"Title":      "Queue Assignment: " + template.Name,
		"Template":   template,
		"Queues":     queues,
		"ActivePage": "admin",
	})
}

// handleUpdateTemplateAssociations is a generic handler for updating template associations.
func handleUpdateTemplateAssociations(
	c *gin.Context,
	formKey string,
	setFunc func(templateID int, ids []int, userID int) error,
	entityName string,
) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
		return
	}

	existing, err := GetStandardTemplate(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load template"})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	var ids []int
	if err := c.Request.ParseForm(); err == nil {
		for _, idStr := range c.Request.Form[formKey] {
			if aid, err := strconv.Atoi(idStr); err == nil {
				ids = append(ids, aid)
			}
		}
	}

	userID := getUserID(c)
	if err := setFunc(id, ids, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update " + entityName + " assignments"})
		return
	}

	if isHTMXRequest(c) {
		// Check referer to determine where to redirect back to
		referer := c.GetHeader("Referer")
		redirectURL := "/admin/templates"
		if strings.Contains(referer, "/admin/queue-templates") || strings.Contains(referer, "/queues/") {
			redirectURL = "/admin/queue-templates"
		} else if strings.Contains(referer, "/admin/template-attachments") || strings.Contains(referer, "/attachments/") {
			redirectURL = "/admin/template-attachments"
		}
		c.Header("HX-Redirect", redirectURL)
		c.Status(http.StatusOK)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleUpdateStandardTemplateQueues handles PUT to update queue assignments.
func handleUpdateStandardTemplateQueues(c *gin.Context) {
	handleUpdateTemplateAssociations(c, "queue_ids", SetTemplateQueues, "queue")
}

// handleAdminStandardTemplateAttachments renders the template attachment assignment page.
func handleAdminStandardTemplateAttachments(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
		return
	}

	template, err := GetStandardTemplate(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load template"})
		return
	}
	if template == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	assignedAttachments, err := GetTemplateAttachments(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load attachment assignments"})
		return
	}

	// Get all valid attachments
	allAttachments, err := ListStandardAttachments()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load attachments"})
		return
	}

	type AttachmentOption struct {
		ID          int
		Name        string
		Filename    string
		ContentType string
		Comments    string
		Selected    bool
	}

	assignedMap := make(map[int]bool)
	for _, aid := range assignedAttachments {
		assignedMap[aid] = true
	}

	attachments := make([]AttachmentOption, 0, len(allAttachments))
	for _, a := range allAttachments {
		comments := ""
		if a.Comments != nil {
			comments = *a.Comments
		}
		attachments = append(attachments, AttachmentOption{
			ID:          a.ID,
			Name:        a.Name,
			Filename:    a.Filename,
			ContentType: a.ContentType,
			Comments:    comments,
			Selected:    assignedMap[a.ID],
		})
	}

	renderer := shared.GetGlobalRenderer()
	if renderer == nil {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, "<h1>Template Attachments</h1>")
		return
	}

	renderer.HTML(c, http.StatusOK, "pages/admin/template_attachments.pongo2", gin.H{
		"Title":       "Attachment Assignment: " + template.Name,
		"Template":    template,
		"Attachments": attachments,
		"ActivePage":  "admin",
	})
}

// handleUpdateStandardTemplateAttachments handles PUT to update attachment assignments.
func handleUpdateStandardTemplateAttachments(c *gin.Context) {
	handleUpdateTemplateAssociations(c, "attachment_ids", SetTemplateAttachments, "attachment")
}

// parseTemplateForm extracts template data from form submission.
func parseTemplateForm(c *gin.Context) (*StandardTemplate, error) {
	if err := c.Request.ParseForm(); err != nil {
		return nil, fmt.Errorf("failed to parse form: %w", err)
	}

	t := &StandardTemplate{
		Name:        strings.TrimSpace(c.Request.FormValue("name")),
		Text:        c.Request.FormValue("text"),
		ContentType: c.Request.FormValue("content_type"),
		Comments:    strings.TrimSpace(c.Request.FormValue("comments")),
	}

	// Handle template types (can be multi-select)
	templateTypes := c.Request.Form["template_type"]
	if len(templateTypes) > 0 {
		t.TemplateType = JoinTemplateTypes(templateTypes)
	} else {
		// Single value fallback
		if tt := c.Request.FormValue("template_type"); tt != "" {
			t.TemplateType = tt
		}
	}

	// Default content type based on content
	if t.ContentType == "" {
		if strings.Contains(t.Text, "<") && strings.Contains(t.Text, ">") {
			t.ContentType = "text/html"
		} else {
			t.ContentType = "text/plain"
		}
	}

	// Parse valid_id
	if validStr := c.Request.FormValue("valid_id"); validStr != "" {
		if v, err := strconv.Atoi(validStr); err == nil {
			t.ValidID = v
		} else {
			t.ValidID = 1
		}
	} else {
		t.ValidID = 1
	}

	return t, nil
}

// handleExportStandardTemplate exports a single template as YAML.
func handleExportStandardTemplate(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
		return
	}

	template, err := ExportTemplate(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to export template: " + err.Error()})
		return
	}

	exportFile := &TemplateExportFile{
		Version:    "1.0",
		ExportedAt: time.Now().Format(time.RFC3339),
		Templates:  []TemplateExport{*template},
	}

	data, err := yaml.Marshal(exportFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate YAML"})
		return
	}

	filename := fmt.Sprintf("template_%s.yaml", strings.ReplaceAll(strings.ToLower(template.Name), " ", "_"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/x-yaml")
	c.Data(http.StatusOK, "application/x-yaml", data)
}

// handleExportAllStandardTemplates exports all templates as YAML.
func handleExportAllStandardTemplates(c *gin.Context) {
	exportFile, err := ExportAllTemplates()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to export templates: " + err.Error()})
		return
	}

	data, err := yaml.Marshal(exportFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate YAML"})
		return
	}

	filename := fmt.Sprintf("templates_export_%s.yaml", time.Now().Format("20060102_150405"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/x-yaml")
	c.Data(http.StatusOK, "application/x-yaml", data)
}

// handleAdminStandardTemplateImport renders the import page.
func handleAdminStandardTemplateImport(c *gin.Context) {
	renderer := shared.GetGlobalRenderer()
	if renderer == nil {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, "<h1>Import Templates</h1>")
		return
	}

	renderer.HTML(c, http.StatusOK, "pages/admin/template_import.pongo2", gin.H{
		"Title":      "Import Templates",
		"ActivePage": "admin",
	})
}

// handleImportStandardTemplates handles the import form submission.
func handleImportStandardTemplates(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
		return
	}
	defer f.Close()

	data := make([]byte, file.Size)
	_, err = f.Read(data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file content"})
		return
	}

	overwrite := c.Request.FormValue("overwrite") == "1" || c.Request.FormValue("overwrite") == "true"
	userID := getUserID(c)

	imported, skipped, errors := ImportTemplates(data, overwrite, userID)

	if isHTMXRequest(c) {
		if len(errors) > 0 {
			c.Header("HX-Trigger", `{"showToast": {"message": "Import completed with errors", "type": "warning"}}`)
		} else {
			c.Header("HX-Trigger", fmt.Sprintf(
				`{"showToast": {"message": "Imported %d templates, skipped %d", "type": "success"}}`,
				imported, skipped))
		}
		c.Header("HX-Redirect", "/admin/templates")
		c.Status(http.StatusOK)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  len(errors) == 0,
		"imported": imported,
		"skipped":  skipped,
		"errors":   errors,
	})
}
