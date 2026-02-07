package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/shared"

	"github.com/gin-gonic/gin"
)

// TemplateWithAttachmentCount represents a template with its attachment assignment count.
type TemplateWithAttachmentCount struct {
	ID              int
	Name            string
	TemplateType    string
	AttachmentCount int
}

// AttachmentWithTemplateCount represents an attachment with its template assignment count.
type AttachmentWithTemplateCount struct {
	ID            int
	Name          string
	Filename      string
	TemplateCount int
}

// GetTemplatesWithAttachmentCounts returns all valid templates with their attachment assignment counts.
func GetTemplatesWithAttachmentCounts() ([]TemplateWithAttachmentCount, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT st.id, st.name, st.template_type, COUNT(sta.standard_attachment_id) as attachment_count
		FROM standard_template st
		LEFT JOIN standard_template_attachment sta ON st.id = sta.standard_template_id
		WHERE st.valid_id = 1
		GROUP BY st.id, st.name, st.template_type
		ORDER BY st.template_type, st.name
	`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []TemplateWithAttachmentCount
	for rows.Next() {
		var t TemplateWithAttachmentCount
		if err := rows.Scan(&t.ID, &t.Name, &t.TemplateType, &t.AttachmentCount); err != nil {
			continue
		}
		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating templates: %w", err)
	}

	return templates, nil
}

// GetAttachmentsWithTemplateCounts returns all valid attachments with their template assignment counts.
func GetAttachmentsWithTemplateCounts() ([]AttachmentWithTemplateCount, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT sa.id, sa.name, sa.filename, COUNT(sta.standard_template_id) as template_count
		FROM standard_attachment sa
		LEFT JOIN standard_template_attachment sta ON sa.id = sta.standard_attachment_id
		WHERE sa.valid_id = 1
		GROUP BY sa.id, sa.name, sa.filename
		ORDER BY sa.name
	`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attachments []AttachmentWithTemplateCount
	for rows.Next() {
		var a AttachmentWithTemplateCount
		if err := rows.Scan(&a.ID, &a.Name, &a.Filename, &a.TemplateCount); err != nil {
			continue
		}
		attachments = append(attachments, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating attachments: %w", err)
	}

	return attachments, nil
}

// GetAttachmentTemplateIDs returns template IDs assigned to an attachment.
func GetAttachmentTemplateIDs(attachmentID int) ([]int, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(database.ConvertPlaceholders(
		`SELECT standard_template_id FROM standard_template_attachment WHERE standard_attachment_id = ?`,
	), attachmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templateIDs []int
	for rows.Next() {
		var templateID int
		if err := rows.Scan(&templateID); err != nil {
			continue
		}
		templateIDs = append(templateIDs, templateID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating template IDs: %w", err)
	}

	return templateIDs, nil
}

// SetAttachmentTemplates replaces all template assignments for an attachment.
func SetAttachmentTemplates(attachmentID int, templateIDs []int, userID int) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}

	// Delete existing assignments
	_, err = db.Exec(database.ConvertPlaceholders(
		`DELETE FROM standard_template_attachment WHERE standard_attachment_id = ?`,
	), attachmentID)
	if err != nil {
		return err
	}

	if len(templateIDs) == 0 {
		return nil
	}

	// Insert new assignments
	now := time.Now()
	for _, templateID := range templateIDs {
		_, err = db.Exec(database.ConvertPlaceholders(`
			INSERT INTO standard_template_attachment
				(standard_attachment_id, standard_template_id, create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, ?, ?, ?)
		`), attachmentID, templateID, now, userID, now, userID)
		if err != nil {
			return fmt.Errorf("failed to assign template %d: %w", templateID, err)
		}
	}

	return nil
}

// AttachmentBasicInfo represents basic attachment information for the admin UI.
type AttachmentBasicInfo struct {
	ID       int
	Name     string
	Filename string
}

// GetAttachmentBasicInfo returns basic attachment information by ID.
func GetAttachmentBasicInfo(attachmentID int) (*AttachmentBasicInfo, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	var a AttachmentBasicInfo
	err = db.QueryRow(database.ConvertPlaceholders(
		`SELECT id, name, filename FROM standard_attachment WHERE id = ?`,
	), attachmentID).Scan(&a.ID, &a.Name, &a.Filename)
	if err != nil {
		return nil, err
	}

	return &a, nil
}

// handleAdminTemplateAttachments renders the template-attachment relations overview page.
func handleAdminTemplateAttachments(c *gin.Context) {
	templates, err := GetTemplatesWithAttachmentCounts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load templates"})
		return
	}

	attachments, err := GetAttachmentsWithTemplateCounts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load attachments"})
		return
	}

	renderer := shared.GetGlobalRenderer()
	if renderer == nil {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, "<h1>Template Attachments</h1>")
		return
	}

	renderer.HTML(c, http.StatusOK, "pages/admin/template_attachments_overview.pongo2", gin.H{
		"Title":       "Template Attachments",
		"Templates":   templates,
		"Attachments": attachments,
		"ActivePage":  "admin",
	})
}

// handleAdminAttachmentTemplatesEdit renders the attachment->templates edit page.
func handleAdminAttachmentTemplatesEdit(c *gin.Context) {
	idStr := c.Param("id")
	attachmentID, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid attachment ID"})
		return
	}

	attachment, err := GetAttachmentBasicInfo(attachmentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Attachment not found"})
		return
	}

	assignedTemplates, err := GetAttachmentTemplateIDs(attachmentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load template assignments"})
		return
	}

	// Get all valid templates
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	rows, err := db.Query(database.ConvertPlaceholders(
		`SELECT id, name, template_type FROM standard_template WHERE valid_id = 1 ORDER BY template_type, name`,
	))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load templates"})
		return
	}
	defer rows.Close()

	assignedMap := make(map[int]bool)
	for _, tid := range assignedTemplates {
		assignedMap[tid] = true
	}

	var templates []TemplateOption
	for rows.Next() {
		var t TemplateOption
		if err := rows.Scan(&t.ID, &t.Name, &t.TemplateType); err != nil {
			continue
		}
		t.Selected = assignedMap[t.ID]
		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error iterating templates"})
		return
	}

	renderer := shared.GetGlobalRenderer()
	if renderer == nil {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, "<h1>Attachment Templates Edit</h1>")
		return
	}

	renderer.HTML(c, http.StatusOK, "pages/admin/attachment_templates_edit.pongo2", gin.H{
		"Title":      "Assign Templates: " + attachment.Name,
		"Attachment": attachment,
		"Templates":  templates,
		"ActivePage": "admin",
	})
}

// handleUpdateAttachmentTemplates handles PUT to update template assignments for an attachment.
func handleUpdateAttachmentTemplates(c *gin.Context) {
	idStr := c.Param("id")
	attachmentID, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid attachment ID"})
		return
	}

	attachment, err := GetAttachmentBasicInfo(attachmentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Attachment not found"})
		return
	}
	_ = attachment // Verify attachment exists

	var templateIDs []int
	if err := c.Request.ParseForm(); err == nil {
		for _, idStr := range c.Request.Form["template_ids"] {
			if tid, err := strconv.Atoi(idStr); err == nil {
				templateIDs = append(templateIDs, tid)
			}
		}
	}

	userID := getUserID(c)
	if err := SetAttachmentTemplates(attachmentID, templateIDs, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update template assignments"})
		return
	}

	if isHTMXRequest(c) {
		c.Header("HX-Redirect", "/admin/template-attachments")
		c.Status(http.StatusOK)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
