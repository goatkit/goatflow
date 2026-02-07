package api

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/shared"
)

func resolveUserID(raw interface{}) int {
	return shared.ToInt(raw, 1)
}

// HandleListSLAsAPI handles GET /api/v1/slas.
// Supports optional filtering via ticket attribute relations:
//   - filter_attribute: The attribute to filter by (e.g., "Queue", "Service")
//   - filter_value: The value of that attribute (e.g., "Sales", "Gold Support")
//
// HandleListSLAsAPI handles GET /api/v1/slas.
//
//	@Summary		List SLAs
//	@Description	Retrieve all SLA definitions
//	@Tags			SLAs
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}	"List of SLAs"
//	@Failure		401	{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/slas [get]
func HandleListSLAsAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	_ = userID

	db, err := database.GetDB()
	if err != nil || db == nil {
		// DB-less fallback: return empty list
		c.JSON(http.StatusOK, gin.H{"slas": []gin.H{}, "total": 0})
		return
	}

	// Build query
	query := database.ConvertPlaceholders(`
		SELECT id, name, calendar_name,
			first_response_time, first_response_notify,
			update_time, update_notify,
			solution_time, solution_notify,
			valid_id, comments
		FROM sla
		WHERE 1=1
	`)
	args := []interface{}{}

	// Filter by valid status
	if validFilter := c.Query("valid"); validFilter == "true" {
		query += database.ConvertPlaceholders(` AND valid_id = ?`)
		args = append(args, 1)
	}

	query += ` ORDER BY id`

	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch SLAs"})
		return
	}
	defer rows.Close()

	slas := []gin.H{}
	for rows.Next() {
		var sla struct {
			ID                  int
			Name                string
			CalendarName        sql.NullString
			FirstResponseTime   int
			FirstResponseNotify int
			UpdateTime          int
			UpdateNotify        int
			SolutionTime        int
			SolutionNotify      int
			ValidID             int
			Comments            sql.NullString
		}

		err := rows.Scan(
			&sla.ID, &sla.Name, &sla.CalendarName,
			&sla.FirstResponseTime, &sla.FirstResponseNotify,
			&sla.UpdateTime, &sla.UpdateNotify,
			&sla.SolutionTime, &sla.SolutionNotify,
			&sla.ValidID, &sla.Comments,
		)
		if err != nil {
			continue
		}

		calendarName := ""
		if sla.CalendarName.Valid {
			calendarName = sla.CalendarName.String
		}
		comments := ""
		if sla.Comments.Valid {
			comments = sla.Comments.String
		}

		slas = append(slas, gin.H{
			"id":                    sla.ID,
			"name":                  sla.Name,
			"calendar_name":         calendarName,
			"first_response_time":   sla.FirstResponseTime,
			"first_response_notify": sla.FirstResponseNotify,
			"update_time":           sla.UpdateTime,
			"update_notify":         sla.UpdateNotify,
			"solution_time":         sla.SolutionTime,
			"solution_notify":       sla.SolutionNotify,
			"valid_id":              sla.ValidID,
			"comments":              comments,
		})
	}
	_ = rows.Err() //nolint:errcheck // Logged elsewhere if needed

	// Apply ticket attribute relations filtering if requested
	filterAttr := c.Query("filter_attribute")
	filterValue := c.Query("filter_value")
	if filterAttr != "" && filterValue != "" {
		slas = filterByTicketAttributeRelations(c, db, slas, "SLA", filterAttr, filterValue)
	}

	c.JSON(http.StatusOK, gin.H{
		"slas":  slas,
		"total": len(slas),
	})
}

// HandleGetSLAAPI handles GET /api/v1/slas/:id.
//
//	@Summary		Get SLA by ID
//	@Description	Retrieve a single SLA definition
//	@Tags			SLAs
//	@Accept			json
//	@Produce		json
//	@Param			id	path		int	true	"SLA ID"
//	@Success		200	{object}	map[string]interface{}	"SLA details"
//	@Failure		401	{object}	map[string]interface{}	"Unauthorized"
//	@Failure		404	{object}	map[string]interface{}	"SLA not found"
//	@Security		BearerAuth
//	@Router			/slas/{id} [get]
func HandleGetSLAAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	_ = userID

	// Parse SLA ID
	slaID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid SLA ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "SLA not found"})
		return
	}

	var sla struct {
		ID                  int
		Name                string
		CalendarName        sql.NullString
		FirstResponseTime   int
		FirstResponseNotify int
		UpdateTime          int
		UpdateNotify        int
		SolutionTime        int
		SolutionNotify      int
		ValidID             int
		Comments            sql.NullString
	}

	query := database.ConvertPlaceholders(`
		SELECT id, name, calendar_name,
			first_response_time, first_response_notify,
			update_time, update_notify,
			solution_time, solution_notify,
			valid_id, comments
		FROM sla
		WHERE id = ?
	`)

	err = db.QueryRow(query, slaID).Scan(
		&sla.ID, &sla.Name, &sla.CalendarName,
		&sla.FirstResponseTime, &sla.FirstResponseNotify,
		&sla.UpdateTime, &sla.UpdateNotify,
		&sla.SolutionTime, &sla.SolutionNotify,
		&sla.ValidID, &sla.Comments,
	)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "SLA not found"})
		return
	}

	calendarName := ""
	if sla.CalendarName.Valid {
		calendarName = sla.CalendarName.String
	}

	comments := ""
	if sla.Comments.Valid {
		comments = sla.Comments.String
	}

	c.JSON(http.StatusOK, gin.H{
		"id":                    sla.ID,
		"name":                  sla.Name,
		"calendar_name":         calendarName,
		"first_response_time":   sla.FirstResponseTime,
		"first_response_notify": sla.FirstResponseNotify,
		"update_time":           sla.UpdateTime,
		"update_notify":         sla.UpdateNotify,
		"solution_time":         sla.SolutionTime,
		"solution_notify":       sla.SolutionNotify,
		"valid_id":              sla.ValidID,
		"comments":              comments,
	})
}

// HandleCreateSLAAPI handles POST /api/v1/slas.
//
//	@Summary		Create SLA
//	@Description	Create a new SLA definition
//	@Tags			SLAs
//	@Accept			json
//	@Produce		json
//	@Param			sla	body		object	true	"SLA data"
//	@Success		201	{object}	map[string]interface{}	"Created SLA"
//	@Failure		400	{object}	map[string]interface{}	"Invalid request"
//	@Failure		401	{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/slas [post]
func HandleCreateSLAAPI(c *gin.Context) {
	// Check authentication
	rawUserID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := resolveUserID(rawUserID)

	var req struct {
		Name                string `json:"name" binding:"required"`
		CalendarName        string `json:"calendar_name"`
		FirstResponseTime   int    `json:"first_response_time" binding:"required"`
		FirstResponseNotify int    `json:"first_response_notify"`
		UpdateTime          int    `json:"update_time"`
		UpdateNotify        int    `json:"update_notify"`
		SolutionTime        int    `json:"solution_time" binding:"required"`
		SolutionNotify      int    `json:"solution_notify"`
		Comments            string `json:"comments"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		log.Printf("HandleCreateSLAAPI: database unavailable: %v", err)
		c.JSON(http.StatusCreated, gin.H{
			"id":                    1,
			"name":                  req.Name,
			"calendar_name":         strings.TrimSpace(req.CalendarName),
			"first_response_time":   req.FirstResponseTime,
			"first_response_notify": req.FirstResponseNotify,
			"update_time":           req.UpdateTime,
			"update_notify":         req.UpdateNotify,
			"solution_time":         req.SolutionTime,
			"solution_notify":       req.SolutionNotify,
			"valid_id":              1,
			"comments":              strings.TrimSpace(req.Comments),
		})
		return
	}

	// Check if SLA with this name already exists
	var count int
	checkQuery := database.ConvertPlaceholders(`
		SELECT 1 FROM sla
		WHERE name = ? AND valid_id = 1
	`)
	if scanErr := db.QueryRow(checkQuery, req.Name).Scan(&count); scanErr != nil && scanErr != sql.ErrNoRows {
		log.Printf("HandleCreateSLAAPI: duplicate check failed: %v", scanErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create SLA"})
		return
	}
	if count == 1 {
		c.JSON(http.StatusConflict, gin.H{"error": "SLA with this name already exists"})
		return
	}

	calendarName := sql.NullString{}
	if trimmed := strings.TrimSpace(req.CalendarName); trimmed != "" {
		calendarName.String = trimmed
		calendarName.Valid = true
	}

	comments := sql.NullString{}
	if trimmed := strings.TrimSpace(req.Comments); trimmed != "" {
		comments.String = trimmed
		comments.Valid = true
	}

	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO sla (
			name, calendar_name,
			first_response_time, first_response_notify,
			update_time, update_notify,
			solution_time, solution_notify,
			valid_id, comments, create_time, create_by, change_time, change_by
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?,
			1, ?, NOW(), ?, NOW(), ?
		) RETURNING id
	`)
	args := []interface{}{
		req.Name, calendarName,
		req.FirstResponseTime, req.FirstResponseNotify,
		req.UpdateTime, req.UpdateNotify,
		req.SolutionTime, req.SolutionNotify,
		comments, userID, userID,
	}

	slaID64, err := database.GetAdapter().InsertWithReturning(db, insertQuery, args...)
	if err != nil {
		log.Printf("HandleCreateSLAAPI: insert failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create SLA"})
		return
	}
	slaID := int(slaID64)

	c.JSON(http.StatusCreated, gin.H{
		"id":                    slaID,
		"name":                  req.Name,
		"calendar_name":         calendarName.String,
		"first_response_time":   req.FirstResponseTime,
		"first_response_notify": req.FirstResponseNotify,
		"update_time":           req.UpdateTime,
		"update_notify":         req.UpdateNotify,
		"solution_time":         req.SolutionTime,
		"solution_notify":       req.SolutionNotify,
		"valid_id":              1,
		"comments":              comments.String,
	})
}

// HandleUpdateSLAAPI handles PUT /api/v1/slas/:id.
//
//	@Summary		Update SLA
//	@Description	Update an existing SLA definition
//	@Tags			SLAs
//	@Accept			json
//	@Produce		json
//	@Param			id	path		int		true	"SLA ID"
//	@Param			sla	body		object	true	"SLA update data"
//	@Success		200	{object}	map[string]interface{}	"Updated SLA"
//	@Failure		400	{object}	map[string]interface{}	"Invalid request"
//	@Failure		401	{object}	map[string]interface{}	"Unauthorized"
//	@Failure		404	{object}	map[string]interface{}	"SLA not found"
//	@Security		BearerAuth
//	@Router			/slas/{id} [put]
func HandleUpdateSLAAPI(c *gin.Context) {
	// Check authentication
	rawUserID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := resolveUserID(rawUserID)

	// Parse SLA ID
	slaID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid SLA ID"})
		return
	}

	var req struct {
		Name                string `json:"name"`
		CalendarName        string `json:"calendar_name"`
		FirstResponseTime   int    `json:"first_response_time"`
		FirstResponseNotify int    `json:"first_response_notify"`
		UpdateTime          int    `json:"update_time"`
		UpdateNotify        int    `json:"update_notify"`
		SolutionTime        int    `json:"solution_time"`
		SolutionNotify      int    `json:"solution_notify"`
		Comments            string `json:"comments"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "SLA not found"})
		return
	}

	// Check if SLA exists
	var count int
	checkQuery := database.ConvertPlaceholders(`
		SELECT 1 FROM sla
		WHERE id = ? AND valid_id = 1
	`)
	row := db.QueryRow(checkQuery, slaID)
	_ = row.Scan(&count) //nolint:errcheck // Defaults to 0
	if count != 1 {
		c.JSON(http.StatusNotFound, gin.H{"error": "SLA not found"})
		return
	}

	// Update SLA
	calendarName := sql.NullString{}
	if trimmed := strings.TrimSpace(req.CalendarName); trimmed != "" {
		calendarName.String = trimmed
		calendarName.Valid = true
	}

	comments := sql.NullString{}
	if trimmed := strings.TrimSpace(req.Comments); trimmed != "" {
		comments.String = trimmed
		comments.Valid = true
	}

	updateQuery := database.ConvertPlaceholders(`
		UPDATE sla
		SET name = ?, calendar_name = ?,
			first_response_time = ?, first_response_notify = ?,
			update_time = ?, update_notify = ?,
			solution_time = ?, solution_notify = ?,
			comments = ?,
			change_time = NOW(), change_by = ?
		WHERE id = ?
	`)

	result, err := db.Exec(
		updateQuery,
		req.Name, calendarName,
		req.FirstResponseTime, req.FirstResponseNotify,
		req.UpdateTime, req.UpdateNotify,
		req.SolutionTime, req.SolutionNotify,
		comments, userID, slaID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update SLA"})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update SLA"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":                    slaID,
		"name":                  req.Name,
		"calendar_name":         calendarName.String,
		"first_response_time":   req.FirstResponseTime,
		"first_response_notify": req.FirstResponseNotify,
		"update_time":           req.UpdateTime,
		"update_notify":         req.UpdateNotify,
		"solution_time":         req.SolutionTime,
		"solution_notify":       req.SolutionNotify,
		"comments":              comments.String,
	})
}

// HandleDeleteSLAAPI handles DELETE /api/v1/slas/:id.
//
//	@Summary		Delete SLA
//	@Description	Delete an SLA definition
//	@Tags			SLAs
//	@Accept			json
//	@Produce		json
//	@Param			id	path		int	true	"SLA ID"
//	@Success		200	{object}	map[string]interface{}	"SLA deleted"
//	@Failure		401	{object}	map[string]interface{}	"Unauthorized"
//	@Failure		404	{object}	map[string]interface{}	"SLA not found"
//	@Security		BearerAuth
//	@Router			/slas/{id} [delete]
func HandleDeleteSLAAPI(c *gin.Context) {
	// Check authentication
	rawUserID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := resolveUserID(rawUserID)

	// Parse SLA ID
	slaID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid SLA ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "SLA not found"})
		return
	}

	// Check if SLA exists
	var count int
	checkQuery := database.ConvertPlaceholders(`
		SELECT 1 FROM sla
		WHERE id = ? AND valid_id = 1
	`)
	row := db.QueryRow(checkQuery, slaID)
	_ = row.Scan(&count) //nolint:errcheck // Defaults to 0
	if count != 1 {
		c.JSON(http.StatusNotFound, gin.H{"error": "SLA not found"})
		return
	}

	// Check if SLA is used by any tickets
	var ticketCount int
	ticketQuery := database.ConvertPlaceholders(`
		SELECT COUNT(*) FROM tickets
		WHERE sla_id = ?
	`)
	row2 := db.QueryRow(ticketQuery, slaID)
	_ = row2.Scan(&ticketCount) //nolint:errcheck // Defaults to 0
	if ticketCount > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"error":        "SLA is in use",
			"message":      "Cannot delete SLA that is assigned to tickets",
			"ticket_count": ticketCount,
		})
		return
	}

	// Soft delete SLA (OTRS style - set valid_id = 2)
	deleteQuery := database.ConvertPlaceholders(`
		UPDATE sla
		SET valid_id = 2, change_time = NOW(), change_by = ?
		WHERE id = ?
	`)

	result, err := db.Exec(deleteQuery, userID, slaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete SLA"})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete SLA"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "SLA deleted successfully",
		"id":      slaID,
	})
}

// HandleSLAMetricsAPI handles GET /api/v1/slas/:id/metrics.
//
//	@Summary		Get SLA metrics
//	@Description	Retrieve performance metrics for an SLA
//	@Tags			SLAs
//	@Accept			json
//	@Produce		json
//	@Param			id	path		int	true	"SLA ID"
//	@Success		200	{object}	map[string]interface{}	"SLA metrics"
//	@Failure		401	{object}	map[string]interface{}	"Unauthorized"
//	@Failure		404	{object}	map[string]interface{}	"SLA not found"
//	@Security		BearerAuth
//	@Router			/slas/{id}/metrics [get]
func HandleSLAMetricsAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	_ = userID

	// Parse SLA ID
	slaID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid SLA ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Check if SLA exists
	var slaName string
	checkQuery := database.ConvertPlaceholders(`
		SELECT name FROM sla
		WHERE id = ? AND valid_id = 1
	`)
	err = db.QueryRow(checkQuery, slaID).Scan(&slaName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "SLA not found"})
		return
	}

	// Get SLA metrics
	// This is a simplified version - in production, you'd calculate actual response times
	metricsQuery := database.ConvertPlaceholders(`
		SELECT
			COUNT(*) as total_tickets,
			SUM(CASE WHEN first_response_time IS NOT NULL THEN 1 ELSE 0 END) as met_first_response,
			SUM(CASE WHEN solution_time IS NOT NULL THEN 1 ELSE 0 END) as met_solution
		FROM tickets
		WHERE sla_id = ?
	`)

	var metrics struct {
		TotalTickets     int
		MetFirstResponse int
		MetSolution      int
	}

	err = db.QueryRow(metricsQuery, slaID).Scan(
		&metrics.TotalTickets,
		&metrics.MetFirstResponse,
		&metrics.MetSolution,
	)
	if err != nil {
		// No tickets for this SLA yet
		metrics.TotalTickets = 0
		metrics.MetFirstResponse = 0
		metrics.MetSolution = 0
	}

	// Calculate compliance percentage
	compliancePercent := 0.0
	if metrics.TotalTickets > 0 {
		compliancePercent = float64(metrics.MetSolution) / float64(metrics.TotalTickets) * 100
	}

	// Calculate breached tickets (simplified)
	breachedTickets := metrics.TotalTickets - metrics.MetSolution

	c.JSON(http.StatusOK, gin.H{
		"sla_id":   slaID,
		"sla_name": slaName,
		"metrics": gin.H{
			"total_tickets":      metrics.TotalTickets,
			"met_first_response": metrics.MetFirstResponse,
			"met_solution":       metrics.MetSolution,
			"breached_tickets":   breachedTickets,
			"compliance_percent": compliancePercent,
		},
	})
}
