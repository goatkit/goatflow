package api

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

func resolveUserID(raw interface{}) int {
	switch v := raw.(type) {
	case int:
		return v
	case int8:
		return int(v)
	case int16:
		return int(v)
	case int32:
		return int(v)
	case int64:
		return int(v)
	case uint:
		return int(v)
	case uint8:
		return int(v)
	case uint16:
		return int(v)
	case uint32:
		return int(v)
	case uint64:
		return int(v)
	case float32:
		return int(v)
	case float64:
		return int(v)
	case string:
		if id, err := strconv.Atoi(v); err == nil {
			return id
		}
	}
	return 1
}

// HandleListSLAsAPI handles GET /api/v1/slas.
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
	paramCount := 0

	// Filter by valid status
	if validFilter := c.Query("valid"); validFilter == "true" {
		paramCount++
		query += database.ConvertPlaceholders(` AND valid_id = $` + strconv.Itoa(paramCount))
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

	c.JSON(http.StatusOK, gin.H{
		"slas":  slas,
		"total": len(slas),
	})
}

// HandleGetSLAAPI handles GET /api/v1/slas/:id.
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
		WHERE id = $1
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
		WHERE name = $1 AND valid_id = 1
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
			$1, $2, $3, $4, $5, $6, $7, $8,
			1, $9, NOW(), $10, NOW(), $11
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
		WHERE id = $1 AND valid_id = 1
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
		SET name = $1, calendar_name = $2,
			first_response_time = $3, first_response_notify = $4,
			update_time = $5, update_notify = $6,
			solution_time = $7, solution_notify = $8,
			comments = $9,
			change_time = NOW(), change_by = $10
		WHERE id = $11
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
		WHERE id = $1 AND valid_id = 1
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
		WHERE sla_id = $1
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
		SET valid_id = 2, change_time = NOW(), change_by = $1
		WHERE id = $2
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
		WHERE id = $1 AND valid_id = 1
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
		WHERE sla_id = $1
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
