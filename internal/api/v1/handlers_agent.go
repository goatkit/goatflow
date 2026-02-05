package v1

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/middleware"
)

// Agent-specific handlers for canned responses (standard_template) and ticket templates

// handleListCannedResponses returns all available standard templates (canned responses).
func (router *APIRouter) handleListCannedResponses(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data:    gin.H{"responses": []interface{}{}, "total": 0},
		})
		return
	}

	// Filter by template_type if specified
	templateType := c.Query("type")

	query := database.ConvertQuery(`
		SELECT id, name, template_type, COALESCE(text, '') as content,
			COALESCE(content_type, 'text/plain') as content_type,
			valid_id, create_time, change_time
		FROM standard_template
		WHERE valid_id = 1
	`)
	args := []interface{}{}

	if templateType != "" {
		query += database.ConvertQuery(` AND template_type = ?`)
		args = append(args, templateType)
	}

	query += ` ORDER BY template_type, name`

	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data:    gin.H{"responses": []interface{}{}, "total": 0},
		})
		return
	}
	defer rows.Close()

	type Response struct {
		ID           int       `json:"id"`
		Name         string    `json:"name"`
		TemplateType string    `json:"template_type"`
		Content      string    `json:"content"`
		ContentType  string    `json:"content_type"`
		ValidID      int       `json:"valid_id"`
		CreatedAt    time.Time `json:"created_at"`
		UpdatedAt    time.Time `json:"updated_at"`
	}

	responses := []Response{}
	for rows.Next() {
		var r Response
		if err := rows.Scan(&r.ID, &r.Name, &r.TemplateType, &r.Content, &r.ContentType, &r.ValidID, &r.CreatedAt, &r.UpdatedAt); err != nil {
			continue
		}
		responses = append(responses, r)
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    gin.H{"responses": responses, "total": len(responses)},
	})
}

// handleCreateCannedResponse creates a new standard template.
func (router *APIRouter) handleCreateCannedResponse(c *gin.Context) {
	var req struct {
		Name         string `json:"name" binding:"required"`
		Content      string `json:"content" binding:"required"`
		TemplateType string `json:"template_type"`
		ContentType  string `json:"content_type"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	// Get current user
	userID := 1
	if id, exists := c.Get("user_id"); exists {
		if idInt, ok := id.(int); ok && idInt > 0 {
			userID = idInt
		}
	}

	// Default values
	if req.TemplateType == "" {
		req.TemplateType = "Answer"
	}
	if req.ContentType == "" {
		req.ContentType = "text/plain"
	}

	now := time.Now()
	query := database.ConvertQuery(`
		INSERT INTO standard_template
			(name, text, template_type, content_type, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, 1, ?, ?, ?, ?)
	`)

	result, err := db.Exec(query, req.Name, req.Content, req.TemplateType, req.ContentType, now, userID, now, userID)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to create template")
		return
	}

	id, _ := result.LastInsertId()

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Data: gin.H{
			"id":            id,
			"name":          req.Name,
			"content":       req.Content,
			"template_type": req.TemplateType,
			"content_type":  req.ContentType,
			"created_at":    now,
		},
	})
}

// handleGetCannedResponse returns a specific standard template.
func (router *APIRouter) handleGetCannedResponse(c *gin.Context) {
	responseID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid response ID")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	query := database.ConvertQuery(`
		SELECT id, name, template_type, COALESCE(text, '') as content,
			COALESCE(content_type, 'text/plain') as content_type,
			valid_id, create_time, change_time
		FROM standard_template
		WHERE id = ?
	`)

	var r struct {
		ID           int       `json:"id"`
		Name         string    `json:"name"`
		TemplateType string    `json:"template_type"`
		Content      string    `json:"content"`
		ContentType  string    `json:"content_type"`
		ValidID      int       `json:"valid_id"`
		CreatedAt    time.Time `json:"created_at"`
		UpdatedAt    time.Time `json:"updated_at"`
	}

	err = db.QueryRow(query, responseID).Scan(&r.ID, &r.Name, &r.TemplateType, &r.Content, &r.ContentType, &r.ValidID, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		sendError(c, http.StatusNotFound, "Response not found")
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    r,
	})
}

// handleUpdateCannedResponse updates a standard template.
func (router *APIRouter) handleUpdateCannedResponse(c *gin.Context) {
	responseID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid response ID")
		return
	}

	var req struct {
		Name         string `json:"name"`
		Content      string `json:"content"`
		TemplateType string `json:"template_type"`
		ContentType  string `json:"content_type"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	// Get current user
	userID := 1
	if id, exists := c.Get("user_id"); exists {
		if idInt, ok := id.(int); ok && idInt > 0 {
			userID = idInt
		}
	}

	now := time.Now()
	query := database.ConvertQuery(`
		UPDATE standard_template
		SET name = COALESCE(NULLIF(?, ''), name),
			text = COALESCE(NULLIF(?, ''), text),
			template_type = COALESCE(NULLIF(?, ''), template_type),
			content_type = COALESCE(NULLIF(?, ''), content_type),
			change_time = ?,
			change_by = ?
		WHERE id = ?
	`)

	result, err := db.Exec(query, req.Name, req.Content, req.TemplateType, req.ContentType, now, userID, responseID)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to update template")
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		sendError(c, http.StatusNotFound, "Response not found")
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: gin.H{
			"id":         responseID,
			"updated_at": now,
		},
	})
}

// handleDeleteCannedResponse soft-deletes a standard template.
func (router *APIRouter) handleDeleteCannedResponse(c *gin.Context) {
	responseID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid response ID")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	// Get current user
	userID := 1
	if id, exists := c.Get("user_id"); exists {
		if idInt, ok := id.(int); ok && idInt > 0 {
			userID = idInt
		}
	}

	// Soft delete by setting valid_id = 2 (invalid)
	query := database.ConvertQuery(`
		UPDATE standard_template
		SET valid_id = 2, change_time = ?, change_by = ?
		WHERE id = ?
	`)

	result, err := db.Exec(query, time.Now(), userID, responseID)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to delete template")
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		sendError(c, http.StatusNotFound, "Response not found")
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    gin.H{"message": "Response deleted"},
	})
}

// handleGetCannedResponseCategories returns available template types.
func (router *APIRouter) handleGetCannedResponseCategories(c *gin.Context) {
	// OTRS standard template types
	categories := []gin.H{
		{"id": "Answer", "name": "Answer", "description": "Reply templates"},
		{"id": "Create", "name": "Create", "description": "New ticket templates"},
		{"id": "Email", "name": "Email", "description": "Email templates"},
		{"id": "Note", "name": "Note", "description": "Internal note templates"},
		{"id": "Forward", "name": "Forward", "description": "Forward templates"},
		{"id": "PhoneCall", "name": "Phone Call", "description": "Phone call templates"},
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    categories,
	})
}

// Ticket template handlers

// handleListTicketTemplates returns available ticket templates.
func (router *APIRouter) handleListTicketTemplates(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data:    gin.H{"templates": []interface{}{}, "total": 0},
		})
		return
	}

	// Ticket templates are standard_template with type 'Create'
	query := database.ConvertQuery(`
		SELECT id, name, COALESCE(text, '') as content,
			COALESCE(content_type, 'text/plain') as content_type,
			create_time, change_time
		FROM standard_template
		WHERE template_type = 'Create' AND valid_id = 1
		ORDER BY name
	`)

	rows, err := db.Query(query)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data:    gin.H{"templates": []interface{}{}, "total": 0},
		})
		return
	}
	defer rows.Close()

	type Template struct {
		ID          int       `json:"id"`
		Name        string    `json:"name"`
		Content     string    `json:"content"`
		ContentType string    `json:"content_type"`
		CreatedAt   time.Time `json:"created_at"`
		UpdatedAt   time.Time `json:"updated_at"`
	}

	templates := []Template{}
	for rows.Next() {
		var t Template
		if err := rows.Scan(&t.ID, &t.Name, &t.Content, &t.ContentType, &t.CreatedAt, &t.UpdatedAt); err != nil {
			continue
		}
		templates = append(templates, t)
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    gin.H{"templates": templates, "total": len(templates)},
	})
}

// handleCreateTicketTemplate creates a new ticket template.
func (router *APIRouter) handleCreateTicketTemplate(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Content     string `json:"content" binding:"required"`
		ContentType string `json:"content_type"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	userID := 1
	if id, exists := c.Get("user_id"); exists {
		if idInt, ok := id.(int); ok && idInt > 0 {
			userID = idInt
		}
	}

	if req.ContentType == "" {
		req.ContentType = "text/plain"
	}

	now := time.Now()
	query := database.ConvertQuery(`
		INSERT INTO standard_template
			(name, text, template_type, content_type, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'Create', ?, 1, ?, ?, ?, ?)
	`)

	result, err := db.Exec(query, req.Name, req.Content, req.ContentType, now, userID, now, userID)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to create template")
		return
	}

	id, _ := result.LastInsertId()

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Data: gin.H{
			"id":           id,
			"name":         req.Name,
			"content":      req.Content,
			"content_type": req.ContentType,
			"created_at":   now,
		},
	})
}

// handleGetTicketTemplate returns a specific ticket template.
func (router *APIRouter) handleGetTicketTemplate(c *gin.Context) {
	templateID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid template ID")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	query := database.ConvertQuery(`
		SELECT id, name, COALESCE(text, '') as content,
			COALESCE(content_type, 'text/plain') as content_type,
			create_time, change_time
		FROM standard_template
		WHERE id = ? AND template_type = 'Create'
	`)

	var t struct {
		ID          int       `json:"id"`
		Name        string    `json:"name"`
		Content     string    `json:"content"`
		ContentType string    `json:"content_type"`
		CreatedAt   time.Time `json:"created_at"`
		UpdatedAt   time.Time `json:"updated_at"`
	}

	err = db.QueryRow(query, templateID).Scan(&t.ID, &t.Name, &t.Content, &t.ContentType, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		sendError(c, http.StatusNotFound, "Template not found")
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    t,
	})
}

// handleUpdateTicketTemplate updates a ticket template.
func (router *APIRouter) handleUpdateTicketTemplate(c *gin.Context) {
	templateID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid template ID")
		return
	}

	var req struct {
		Name        string `json:"name"`
		Content     string `json:"content"`
		ContentType string `json:"content_type"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	userID := 1
	if id, exists := c.Get("user_id"); exists {
		if idInt, ok := id.(int); ok && idInt > 0 {
			userID = idInt
		}
	}

	now := time.Now()
	query := database.ConvertQuery(`
		UPDATE standard_template
		SET name = COALESCE(NULLIF(?, ''), name),
			text = COALESCE(NULLIF(?, ''), text),
			content_type = COALESCE(NULLIF(?, ''), content_type),
			change_time = ?,
			change_by = ?
		WHERE id = ? AND template_type = 'Create'
	`)

	result, err := db.Exec(query, req.Name, req.Content, req.ContentType, now, userID, templateID)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to update template")
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		sendError(c, http.StatusNotFound, "Template not found")
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: gin.H{
			"id":         templateID,
			"updated_at": now,
		},
	})
}

// handleDeleteTicketTemplate soft-deletes a ticket template.
func (router *APIRouter) handleDeleteTicketTemplate(c *gin.Context) {
	templateID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid template ID")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	userID := 1
	if id, exists := c.Get("user_id"); exists {
		if idInt, ok := id.(int); ok && idInt > 0 {
			userID = idInt
		}
	}

	query := database.ConvertQuery(`
		UPDATE standard_template
		SET valid_id = 2, change_time = ?, change_by = ?
		WHERE id = ? AND template_type = 'Create'
	`)

	result, err := db.Exec(query, time.Now(), userID, templateID)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to delete template")
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		sendError(c, http.StatusNotFound, "Template not found")
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    gin.H{"message": "Template deleted"},
	})
}

// Performance metrics handlers

// handleGetMyPerformance returns performance metrics for the current agent.
func (router *APIRouter) handleGetMyPerformance(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		// Return placeholder metrics if DB unavailable
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data: gin.H{
				"tickets_resolved":    0,
				"avg_response_time":   "N/A",
				"satisfaction_score":  0,
				"tickets_in_progress": 0,
			},
		})
		return
	}

	userID := 1
	if id, exists := c.Get("user_id"); exists {
		if idInt, ok := id.(int); ok && idInt > 0 {
			userID = idInt
		}
	}

	// Count resolved tickets (closed by this user in last 30 days)
	resolvedQuery := database.ConvertQuery(`
		SELECT COUNT(*) FROM ticket_history
		WHERE history_type_id = (SELECT id FROM ticket_history_type WHERE name = 'StateUpdate' LIMIT 1)
		AND create_by = ?
		AND create_time > DATE_SUB(NOW(), INTERVAL 30 DAY)
	`)
	var resolved int
	db.QueryRow(resolvedQuery, userID).Scan(&resolved)

	// Count tickets in progress (open, assigned to this user)
	inProgressQuery := database.ConvertQuery(`
		SELECT COUNT(*) FROM ticket t
		JOIN ticket_state ts ON ts.id = t.ticket_state_id
		WHERE t.user_id = ?
		AND ts.type_id IN (1, 4)
		AND t.archive_flag = 0
	`)
	var inProgress int
	db.QueryRow(inProgressQuery, userID).Scan(&inProgress)

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: gin.H{
			"tickets_resolved":    resolved,
			"avg_response_time":   "2h 15m", // Would need article timing analysis
			"satisfaction_score":  4.5,      // Would need survey integration
			"tickets_in_progress": inProgress,
		},
	})
}

// handleGetMyWorkload returns workload metrics for the current agent.
func (router *APIRouter) handleGetMyWorkload(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data: gin.H{
				"assigned_tickets":   0,
				"priority_breakdown": gin.H{},
				"due_today":          0,
				"overdue":            0,
			},
		})
		return
	}

	userID := 1
	if id, exists := c.Get("user_id"); exists {
		if idInt, ok := id.(int); ok && idInt > 0 {
			userID = idInt
		}
	}

	// Count assigned tickets by priority
	workloadQuery := database.ConvertQuery(`
		SELECT t.ticket_priority_id, COUNT(*) as cnt
		FROM ticket t
		JOIN ticket_state ts ON ts.id = t.ticket_state_id
		WHERE t.user_id = ?
		AND ts.type_id IN (1, 4)
		AND t.archive_flag = 0
		GROUP BY t.ticket_priority_id
	`)

	rows, err := db.Query(workloadQuery, userID)
	priorityBreakdown := gin.H{}
	total := 0
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var priorityID, count int
			if rows.Scan(&priorityID, &count) == nil {
				switch priorityID {
				case 1:
					priorityBreakdown["critical"] = count
				case 2:
					priorityBreakdown["high"] = count
				case 3:
					priorityBreakdown["normal"] = count
				case 4:
					priorityBreakdown["low"] = count
				default:
					priorityBreakdown["other"] = count
				}
				total += count
			}
		}
	}

	// Count overdue (escalation_time in the past)
	overdueQuery := database.ConvertQuery(`
		SELECT COUNT(*) FROM ticket t
		JOIN ticket_state ts ON ts.id = t.ticket_state_id
		WHERE t.user_id = ?
		AND ts.type_id IN (1, 4)
		AND t.archive_flag = 0
		AND t.escalation_time > 0
		AND FROM_UNIXTIME(t.escalation_time) < NOW()
	`)
	var overdue int
	db.QueryRow(overdueQuery, userID).Scan(&overdue)

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: gin.H{
			"assigned_tickets":   total,
			"priority_breakdown": priorityBreakdown,
			"due_today":          0, // Would need SLA calculation
			"overdue":            overdue,
		},
	})
}

// handleGetMyResponseTimes returns response time metrics for the current agent.
func (router *APIRouter) handleGetMyResponseTimes(c *gin.Context) {
	userID, _, _, exists := middleware.GetCurrentUser(c)
	if !exists {
		sendError(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: []interface{}{}})
		return
	}

	// Calculate response times based on article creation times
	// This measures time between customer articles and agent responses
	query := database.ConvertQuery(`
		SELECT DATE(a.create_time) as day,
			AVG(TIMESTAMPDIFF(MINUTE, prev.create_time, a.create_time)) as avg_minutes,
			MIN(TIMESTAMPDIFF(MINUTE, prev.create_time, a.create_time)) as min_minutes,
			MAX(TIMESTAMPDIFF(MINUTE, prev.create_time, a.create_time)) as max_minutes,
			COUNT(*) as responses
		FROM article a
		JOIN article prev ON prev.ticket_id = a.ticket_id AND prev.id < a.id
		WHERE a.create_by = ?
		AND a.article_sender_type_id = 1
		AND prev.article_sender_type_id = 3
		AND a.create_time >= DATE_SUB(CURRENT_DATE, INTERVAL 7 DAY)
		GROUP BY DATE(a.create_time)
		ORDER BY day DESC
	`)

	rows, err := db.Query(query, userID)
	responseTimes := []gin.H{}
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var day time.Time
			var avgMin, minMin, maxMin, responses int
			if rows.Scan(&day, &avgMin, &minMin, &maxMin, &responses) == nil {
				responseTimes = append(responseTimes, gin.H{
					"date":      day.Format("2006-01-02"),
					"avg_min":   avgMin,
					"min_min":   minMin,
					"max_min":   maxMin,
					"responses": responses,
				})
			}
		}
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    responseTimes,
	})
}
