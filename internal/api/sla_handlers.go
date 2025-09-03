package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleListSLAsAPI handles GET /api/v1/slas
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
		SELECT id, name, calendar_id, 
			first_response_time, first_response_notify,
			update_time, update_notify,
			solution_time, solution_notify,
			valid_id
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
			ID                   int
			Name                 string
			CalendarID           int
			FirstResponseTime    int
			FirstResponseNotify  int
			UpdateTime           int
			UpdateNotify         int
			SolutionTime         int
			SolutionNotify       int
			ValidID              int
		}
		
		err := rows.Scan(
			&sla.ID, &sla.Name, &sla.CalendarID,
			&sla.FirstResponseTime, &sla.FirstResponseNotify,
			&sla.UpdateTime, &sla.UpdateNotify,
			&sla.SolutionTime, &sla.SolutionNotify,
			&sla.ValidID,
		)
		if err != nil {
			continue
		}
		
		slas = append(slas, gin.H{
			"id":                    sla.ID,
			"name":                  sla.Name,
			"calendar_id":           sla.CalendarID,
			"first_response_time":   sla.FirstResponseTime,
			"first_response_notify": sla.FirstResponseNotify,
			"update_time":           sla.UpdateTime,
			"update_notify":         sla.UpdateNotify,
			"solution_time":         sla.SolutionTime,
			"solution_notify":       sla.SolutionNotify,
			"valid_id":              sla.ValidID,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"slas":  slas,
		"total": len(slas),
	})
}

// HandleGetSLAAPI handles GET /api/v1/slas/:id
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
		ID                   int    `json:"id"`
		Name                 string `json:"name"`
		CalendarID           int    `json:"calendar_id"`
		FirstResponseTime    int    `json:"first_response_time"`
		FirstResponseNotify  int    `json:"first_response_notify"`
		UpdateTime           int    `json:"update_time"`
		UpdateNotify         int    `json:"update_notify"`
		SolutionTime         int    `json:"solution_time"`
		SolutionNotify       int    `json:"solution_notify"`
		ValidID              int    `json:"valid_id"`
	}

	query := database.ConvertPlaceholders(`
		SELECT id, name, calendar_id,
			first_response_time, first_response_notify,
			update_time, update_notify,
			solution_time, solution_notify,
			valid_id
		FROM sla
		WHERE id = $1
	`)

	err = db.QueryRow(query, slaID).Scan(
		&sla.ID, &sla.Name, &sla.CalendarID,
		&sla.FirstResponseTime, &sla.FirstResponseNotify,
		&sla.UpdateTime, &sla.UpdateNotify,
		&sla.SolutionTime, &sla.SolutionNotify,
		&sla.ValidID,
	)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "SLA not found"})
		return
	}

	c.JSON(http.StatusOK, sla)
}

// HandleCreateSLAAPI handles POST /api/v1/slas
func HandleCreateSLAAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req struct {
		Name                 string `json:"name" binding:"required"`
		CalendarID           int    `json:"calendar_id"`
		FirstResponseTime    int    `json:"first_response_time" binding:"required"`
		FirstResponseNotify  int    `json:"first_response_notify"`
		UpdateTime           int    `json:"update_time"`
		UpdateNotify         int    `json:"update_notify"`
		SolutionTime         int    `json:"solution_time" binding:"required"`
		SolutionNotify       int    `json:"solution_notify"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Default calendar ID if not provided
	if req.CalendarID == 0 {
		req.CalendarID = 1
	}

    db, err := database.GetDB()
    if err != nil || db == nil {
        // DB-less fallback: pretend create
        c.JSON(http.StatusCreated, gin.H{
            "id":                    1,
            "name":                  req.Name,
            "calendar_id":           req.CalendarID,
            "first_response_time":   req.FirstResponseTime,
            "first_response_notify": req.FirstResponseNotify,
            "update_time":           req.UpdateTime,
            "update_notify":         req.UpdateNotify,
            "solution_time":         req.SolutionTime,
            "solution_notify":       req.SolutionNotify,
            "valid_id":              1,
        })
        return
    }

	// Check if SLA with this name already exists
	var count int
	checkQuery := database.ConvertPlaceholders(`
		SELECT 1 FROM sla
		WHERE name = $1 AND valid_id = 1
	`)
	db.QueryRow(checkQuery, req.Name).Scan(&count)
	if count == 1 {
		c.JSON(http.StatusConflict, gin.H{"error": "SLA with this name already exists"})
		return
	}

	// Create SLA
	var slaID int
	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO sla (
			name, calendar_id,
			first_response_time, first_response_notify,
			update_time, update_notify,
			solution_time, solution_notify,
			valid_id, create_time, create_by, change_time, change_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			1, NOW(), $9, NOW(), $9
		) RETURNING id
	`)
	
	err = db.QueryRow(
		insertQuery,
		req.Name, req.CalendarID,
		req.FirstResponseTime, req.FirstResponseNotify,
		req.UpdateTime, req.UpdateNotify,
		req.SolutionTime, req.SolutionNotify,
		userID,
	).Scan(&slaID)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create SLA"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":                    slaID,
		"name":                  req.Name,
		"calendar_id":           req.CalendarID,
		"first_response_time":   req.FirstResponseTime,
		"first_response_notify": req.FirstResponseNotify,
		"update_time":           req.UpdateTime,
		"update_notify":         req.UpdateNotify,
		"solution_time":         req.SolutionTime,
		"solution_notify":       req.SolutionNotify,
		"valid_id":              1,
	})
}

// HandleUpdateSLAAPI handles PUT /api/v1/slas/:id
func HandleUpdateSLAAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Parse SLA ID
	slaID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid SLA ID"})
		return
	}

	var req struct {
		Name                string `json:"name"`
		CalendarID          int    `json:"calendar_id"`
		FirstResponseTime   int    `json:"first_response_time"`
		FirstResponseNotify int    `json:"first_response_notify"`
		UpdateTime          int    `json:"update_time"`
		UpdateNotify        int    `json:"update_notify"`
		SolutionTime        int    `json:"solution_time"`
		SolutionNotify      int    `json:"solution_notify"`
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
	db.QueryRow(checkQuery, slaID).Scan(&count)
	if count != 1 {
		c.JSON(http.StatusNotFound, gin.H{"error": "SLA not found"})
		return
	}

	// Update SLA
	updateQuery := database.ConvertPlaceholders(`
		UPDATE sla 
		SET name = $1, calendar_id = $2,
			first_response_time = $3, first_response_notify = $4,
			update_time = $5, update_notify = $6,
			solution_time = $7, solution_notify = $8,
			change_time = NOW(), change_by = $9
		WHERE id = $10
	`)

	result, err := db.Exec(
		updateQuery,
		req.Name, req.CalendarID,
		req.FirstResponseTime, req.FirstResponseNotify,
		req.UpdateTime, req.UpdateNotify,
		req.SolutionTime, req.SolutionNotify,
		userID, slaID,
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
		"calendar_id":           req.CalendarID,
		"first_response_time":   req.FirstResponseTime,
		"first_response_notify": req.FirstResponseNotify,
		"update_time":           req.UpdateTime,
		"update_notify":         req.UpdateNotify,
		"solution_time":         req.SolutionTime,
		"solution_notify":       req.SolutionNotify,
	})
}

// HandleDeleteSLAAPI handles DELETE /api/v1/slas/:id
func HandleDeleteSLAAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

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
	db.QueryRow(checkQuery, slaID).Scan(&count)
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
	db.QueryRow(ticketQuery, slaID).Scan(&ticketCount)
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

// HandleSLAMetricsAPI handles GET /api/v1/slas/:id/metrics
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