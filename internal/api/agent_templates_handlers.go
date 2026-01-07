package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// TemplateForAgent represents a template available to agents.
type TemplateForAgent struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Text         string `json:"text"`
	ContentType  string `json:"content_type"`
	TemplateType string `json:"template_type"`
}

// GetTemplatesForQueue returns templates available for a specific queue and optional type filter.
func GetTemplatesForQueue(queueID int, templateType string) ([]TemplateForAgent, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT DISTINCT t.id, t.name, t.text, t.content_type, t.template_type
		FROM standard_template t
		INNER JOIN queue_standard_template qt ON qt.standard_template_id = t.id
		WHERE qt.queue_id = ?
		  AND t.valid_id = 1
	`
	args := []interface{}{queueID}
	argCount := 2

	if templateType != "" {
		query += fmt.Sprintf(" AND t.template_type LIKE $%d", argCount)
		args = append(args, "%"+templateType+"%")
	}

	query += " ORDER BY t.name ASC"

	rows, err := db.Query(database.ConvertPlaceholders(query), args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var templates []TemplateForAgent
	for rows.Next() {
		var t TemplateForAgent
		var text, contentType, templateType sql.NullString

		err := rows.Scan(&t.ID, &t.Name, &text, &contentType, &templateType)
		if err != nil {
			continue
		}

		t.Text = text.String
		t.ContentType = contentType.String
		t.TemplateType = templateType.String
		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating templates: %w", err)
	}

	return templates, nil
}

// SubstituteTemplateVariables replaces template variables with actual values.
func SubstituteTemplateVariables(text string, vars map[string]string) string {
	result := text
	for key, value := range vars {
		// Support both <OTRS_*> and <GOTRS_*> style variables
		result = strings.ReplaceAll(result, "<OTRS_"+key+">", value)
		result = strings.ReplaceAll(result, "<GOTRS_"+key+">", value)
	}
	return result
}

// GET /agent/api/templates?queue_id=X&type=Answer.
func handleGetAgentTemplates(c *gin.Context) {
	queueIDStr := c.Query("queue_id")
	templateType := c.Query("type")

	if queueIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "queue_id is required",
		})
		return
	}

	queueID, err := strconv.Atoi(queueIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "invalid queue_id",
		})
		return
	}

	templates, err := GetTemplatesForQueue(queueID, templateType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "failed to fetch templates",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    templates,
	})
}

// GET /agent/api/templates/:id?ticket_id=X.
func handleGetAgentTemplate(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "invalid template id",
		})
		return
	}

	template, err := GetStandardTemplate(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "failed to fetch template",
		})
		return
	}

	if template == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "template not found",
		})
		return
	}

	// Get ticket context for variable substitution if provided
	ticketIDStr := c.Query("ticket_id")
	if ticketIDStr != "" {
		ticketID, err := strconv.Atoi(ticketIDStr)
		if err == nil {
			vars := GetTicketTemplateVariables(ticketID)
			template.Text = SubstituteTemplateVariables(template.Text, vars)
		}
	}

	// Get template attachments
	attachmentIDs, _ := GetTemplateAttachments(id) //nolint:errcheck // Empty array on error
	attachments := GetAttachmentsByIDs(attachmentIDs)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":           template.ID,
			"name":         template.Name,
			"text":         template.Text,
			"content_type": template.ContentType,
			"attachments":  attachments,
		},
	})
}

// GetTicketTemplateVariables returns template variables for a ticket.
func GetTicketTemplateVariables(ticketID int) map[string]string {
	vars := make(map[string]string)

	db, err := database.GetDB()
	if err != nil {
		return vars
	}

	// Get ticket data
	query := `
		SELECT t.tn, t.title, t.customer_id, t.customer_user_id,
			   q.name as queue_name, u.login as owner_login
		FROM ticket t
		LEFT JOIN queue q ON q.id = t.queue_id
		LEFT JOIN users u ON u.id = t.user_id
		WHERE t.id = ?
	`

	var tn, title, customerID, customerUserID sql.NullString
	var queueName, ownerLogin sql.NullString

	err = db.QueryRow(database.ConvertPlaceholders(query), ticketID).Scan(
		&tn, &title, &customerID, &customerUserID,
		&queueName, &ownerLogin,
	)
	if err != nil {
		return vars
	}

	vars["TICKET_TicketNumber"] = tn.String
	vars["TICKET_Title"] = title.String
	vars["TICKET_CustomerID"] = customerID.String
	vars["TICKET_CustomerUserID"] = customerUserID.String
	vars["TICKET_Queue"] = queueName.String
	vars["TICKET_Owner"] = ownerLogin.String

	// Get customer user data if available
	if customerUserID.String != "" {
		cuQuery := `
			SELECT first_name, last_name, email
			FROM customer_user
			WHERE login = ?
		`
		var firstName, lastName, email sql.NullString
		err = db.QueryRow(database.ConvertPlaceholders(cuQuery), customerUserID.String).Scan(
			&firstName, &lastName, &email,
		)
		if err == nil {
			vars["CUSTOMER_UserFirstname"] = firstName.String
			vars["CUSTOMER_UserLastname"] = lastName.String
			vars["CUSTOMER_UserFullname"] = strings.TrimSpace(firstName.String + " " + lastName.String)
			vars["CUSTOMER_UserEmail"] = email.String
		}
	}

	// Get current user (agent) data from context
	vars["CURRENT_UserFirstname"] = ""
	vars["CURRENT_UserLastname"] = ""
	vars["CURRENT_UserFullname"] = ""

	return vars
}

// AttachmentInfo represents minimal attachment info for agent API.
type AttachmentInfo struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	ContentSize int64  `json:"content_size"`
}

// GetAttachmentsByIDs returns attachment details for given IDs.
func GetAttachmentsByIDs(ids []int) []AttachmentInfo {
	if len(ids) == 0 {
		return []AttachmentInfo{}
	}

	db, err := database.GetDB()
	if err != nil {
		return []AttachmentInfo{}
	}

	result := []AttachmentInfo{}
	for _, id := range ids {
		var a AttachmentInfo
		err := db.QueryRow(database.ConvertPlaceholders(`
			SELECT id, name, filename, content_type, LENGTH(content) as content_size
			FROM standard_attachment
			WHERE id = ? AND valid_id = 1
		`), id).Scan(&a.ID, &a.Name, &a.Filename, &a.ContentType, &a.ContentSize)
		if err == nil {
			result = append(result, a)
		}
	}

	return result
}
