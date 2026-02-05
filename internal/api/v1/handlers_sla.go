package v1

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
)

// handleGetTicketSLA returns the SLA status for a specific ticket.
func (router *APIRouter) handleGetTicketSLA(c *gin.Context) {
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid ticket ID",
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Database unavailable",
		})
		return
	}

	ticketRepo := repository.NewTicketRepository(db)

	// Verify ticket exists
	ticket, err := ticketRepo.GetByID(uint(ticketID))
	if err != nil || ticket == nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "Ticket not found",
		})
		return
	}

	// Check if ticket is closed - SLA not applicable
	if ticket.TicketStateID == 2 || ticket.TicketStateID == 3 { // Closed states
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data: gin.H{
				"ticket_id":      ticketID,
				"sla_status":     "not_applicable",
				"reason":         "Ticket is closed",
				"sla_active":     false,
			},
		})
		return
	}

	// Calculate SLA based on priority and creation time
	// Default SLA times by priority (in hours)
	slaHours := map[int]int{
		1: 1,   // Critical: 1 hour
		2: 4,   // High: 4 hours
		3: 8,   // Normal: 8 hours
		4: 24,  // Low: 24 hours
		5: 72,  // Very low: 72 hours
	}

	targetHours, ok := slaHours[ticket.TicketPriorityID]
	if !ok {
		targetHours = 8 // Default to 8 hours
	}

	deadline := ticket.CreateTime.Add(time.Duration(targetHours) * time.Hour)
	now := time.Now()
	timeRemaining := deadline.Sub(now)
	
	// Calculate percentage used
	totalDuration := time.Duration(targetHours) * time.Hour
	elapsed := now.Sub(ticket.CreateTime)
	percentUsed := float64(elapsed) / float64(totalDuration) * 100
	if percentUsed > 100 {
		percentUsed = 100
	}

	// Determine status
	status := "ok"
	if timeRemaining <= 0 {
		status = "breached"
	} else if percentUsed >= 75 {
		status = "warning"
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: gin.H{
			"ticket_id":           ticketID,
			"sla_status":          status,
			"sla_active":          true,
			"deadline":            deadline,
			"time_remaining_secs": int(timeRemaining.Seconds()),
			"percent_used":        int(percentUsed),
			"priority_id":         ticket.TicketPriorityID,
			"target_hours":        targetHours,
		},
	})
}

// handleListSLAs returns all SLA definitions.
func (router *APIRouter) handleListSLAs(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data:    gin.H{"slas": []interface{}{}, "total": 0},
		})
		return
	}

	// Query SLA definitions
	query := database.ConvertQuery(`
		SELECT id, name, calendar_name,
			first_response_time, first_response_notify,
			update_time, update_notify,
			solution_time, solution_notify,
			valid_id, comments
		FROM sla
		WHERE valid_id = 1
		ORDER BY name
	`)

	rows, err := db.Query(query)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data:    gin.H{"slas": []interface{}{}, "total": 0},
		})
		return
	}
	defer rows.Close()

	type SLADef struct {
		ID                  int     `json:"id"`
		Name                string  `json:"name"`
		CalendarName        *string `json:"calendar_name"`
		FirstResponseTime   *int    `json:"first_response_time"`
		FirstResponseNotify *int    `json:"first_response_notify"`
		UpdateTime          *int    `json:"update_time"`
		UpdateNotify        *int    `json:"update_notify"`
		SolutionTime        *int    `json:"solution_time"`
		SolutionNotify      *int    `json:"solution_notify"`
		ValidID             int     `json:"valid_id"`
		Comments            *string `json:"comments"`
	}

	slas := []SLADef{}
	for rows.Next() {
		var sla SLADef
		if err := rows.Scan(
			&sla.ID, &sla.Name, &sla.CalendarName,
			&sla.FirstResponseTime, &sla.FirstResponseNotify,
			&sla.UpdateTime, &sla.UpdateNotify,
			&sla.SolutionTime, &sla.SolutionNotify,
			&sla.ValidID, &sla.Comments,
		); err != nil {
			continue
		}
		slas = append(slas, sla)
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    gin.H{"slas": slas, "total": len(slas)},
	})
}

// handleGetSLA returns a specific SLA definition.
func (router *APIRouter) handleGetSLA(c *gin.Context) {
	slaID, err := strconv.ParseUint(c.Param("sla_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid SLA ID",
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Database unavailable",
		})
		return
	}

	query := database.ConvertQuery(`
		SELECT id, name, calendar_name,
			first_response_time, first_response_notify,
			update_time, update_notify,
			solution_time, solution_notify,
			valid_id, comments
		FROM sla
		WHERE id = ?
	`)

	var sla struct {
		ID                  int     `json:"id"`
		Name                string  `json:"name"`
		CalendarName        *string `json:"calendar_name"`
		FirstResponseTime   *int    `json:"first_response_time"`
		FirstResponseNotify *int    `json:"first_response_notify"`
		UpdateTime          *int    `json:"update_time"`
		UpdateNotify        *int    `json:"update_notify"`
		SolutionTime        *int    `json:"solution_time"`
		SolutionNotify      *int    `json:"solution_notify"`
		ValidID             int     `json:"valid_id"`
		Comments            *string `json:"comments"`
	}

	err = db.QueryRow(query, slaID).Scan(
		&sla.ID, &sla.Name, &sla.CalendarName,
		&sla.FirstResponseTime, &sla.FirstResponseNotify,
		&sla.UpdateTime, &sla.UpdateNotify,
		&sla.SolutionTime, &sla.SolutionNotify,
		&sla.ValidID, &sla.Comments,
	)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "SLA not found",
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    sla,
	})
}
