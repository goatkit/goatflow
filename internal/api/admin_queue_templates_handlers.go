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

// QueueWithTemplateCount represents a queue with its template assignment count.
type QueueWithTemplateCount struct {
	ID            int
	Name          string
	TemplateCount int
}

// TemplateWithQueueCount represents a template with its queue assignment count.
type TemplateWithQueueCount struct {
	ID           int
	Name         string
	TemplateType string
	QueueCount   int
}

// GetQueuesWithTemplateCounts returns all valid queues with their template assignment counts.
func GetQueuesWithTemplateCounts() ([]QueueWithTemplateCount, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT q.id, q.name, COUNT(qst.standard_template_id) as template_count
		FROM queue q
		LEFT JOIN queue_standard_template qst ON q.id = qst.queue_id
		WHERE q.valid_id = 1
		GROUP BY q.id, q.name
		ORDER BY q.name
	`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var queues []QueueWithTemplateCount
	for rows.Next() {
		var q QueueWithTemplateCount
		if err := rows.Scan(&q.ID, &q.Name, &q.TemplateCount); err != nil {
			continue
		}
		queues = append(queues, q)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating queues: %w", err)
	}

	return queues, nil
}

// GetTemplatesWithQueueCounts returns all valid templates with their queue assignment counts.
func GetTemplatesWithQueueCounts() ([]TemplateWithQueueCount, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT st.id, st.name, st.template_type, COUNT(qst.queue_id) as queue_count
		FROM standard_template st
		LEFT JOIN queue_standard_template qst ON st.id = qst.standard_template_id
		WHERE st.valid_id = 1
		GROUP BY st.id, st.name, st.template_type
		ORDER BY st.template_type, st.name
	`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []TemplateWithQueueCount
	for rows.Next() {
		var t TemplateWithQueueCount
		if err := rows.Scan(&t.ID, &t.Name, &t.TemplateType, &t.QueueCount); err != nil {
			continue
		}
		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating templates: %w", err)
	}

	return templates, nil
}

// GetQueueTemplateIDs returns template IDs assigned to a queue.
func GetQueueTemplateIDs(queueID int) ([]int, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(database.ConvertPlaceholders(
		`SELECT standard_template_id FROM queue_standard_template WHERE queue_id = ?`,
	), queueID)
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

// SetQueueTemplates replaces all template assignments for a queue.
func SetQueueTemplates(queueID int, templateIDs []int, userID int) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}

	// Delete existing assignments
	_, err = db.Exec(database.ConvertPlaceholders(
		`DELETE FROM queue_standard_template WHERE queue_id = ?`,
	), queueID)
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
			INSERT INTO queue_standard_template
				(queue_id, standard_template_id, create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, ?, ?, ?)
		`), queueID, templateID, now, userID, now, userID)
		if err != nil {
			return fmt.Errorf("failed to assign template %d: %w", templateID, err)
		}
	}

	return nil
}

// QueueInfo represents basic queue information.
type QueueInfo struct {
	ID   int
	Name string
}

// GetQueueInfo returns basic queue information by ID.
func GetQueueInfo(queueID int) (*QueueInfo, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	var q QueueInfo
	err = db.QueryRow(database.ConvertPlaceholders(
		`SELECT id, name FROM queue WHERE id = ?`,
	), queueID).Scan(&q.ID, &q.Name)
	if err != nil {
		return nil, err
	}

	return &q, nil
}

// TemplateOption represents a template option for checkbox selection.
type TemplateOption struct {
	ID           int
	Name         string
	TemplateType string
	Selected     bool
}

// handleAdminQueueTemplates renders the queue-template relations overview page.
func handleAdminQueueTemplates(c *gin.Context) {
	queues, err := GetQueuesWithTemplateCounts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load queues"})
		return
	}

	templates, err := GetTemplatesWithQueueCounts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load templates"})
		return
	}

	renderer := shared.GetGlobalRenderer()
	if renderer == nil {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, "<h1>Queue Templates</h1>")
		return
	}

	renderer.HTML(c, http.StatusOK, "pages/admin/queue_templates.pongo2", gin.H{
		"Title":      "Queue Templates",
		"Queues":     queues,
		"Templates":  templates,
		"ActivePage": "admin",
	})
}

// handleAdminQueueTemplatesEdit renders the queue->templates edit page.
func handleAdminQueueTemplatesEdit(c *gin.Context) {
	idStr := c.Param("id")
	queueID, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid queue ID"})
		return
	}

	queue, err := GetQueueInfo(queueID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Queue not found"})
		return
	}

	assignedTemplates, err := GetQueueTemplateIDs(queueID)
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
		c.String(http.StatusOK, "<h1>Queue Templates Edit</h1>")
		return
	}

	renderer.HTML(c, http.StatusOK, "pages/admin/queue_templates_edit.pongo2", gin.H{
		"Title":      "Assign Templates: " + queue.Name,
		"Queue":      queue,
		"Templates":  templates,
		"ActivePage": "admin",
	})
}

// handleUpdateQueueTemplates handles PUT to update template assignments for a queue.
func handleUpdateQueueTemplates(c *gin.Context) {
	idStr := c.Param("id")
	queueID, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid queue ID"})
		return
	}

	queue, err := GetQueueInfo(queueID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Queue not found"})
		return
	}
	_ = queue // Verify queue exists

	var templateIDs []int
	if err := c.Request.ParseForm(); err == nil {
		for _, idStr := range c.Request.Form["template_ids"] {
			if tid, err := strconv.Atoi(idStr); err == nil {
				templateIDs = append(templateIDs, tid)
			}
		}
	}

	userID := getUserID(c)
	if err := SetQueueTemplates(queueID, templateIDs, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update template assignments"})
		return
	}

	if isHTMXRequest(c) {
		c.Header("HX-Redirect", "/admin/queue-templates")
		c.Status(http.StatusOK)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
