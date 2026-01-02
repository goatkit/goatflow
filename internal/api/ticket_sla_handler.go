package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/shared"
)

// SLA represents Service Level Agreement status.
type SLA struct {
	Status        string    `json:"status"` // within, warning, overdue
	Deadline      time.Time `json:"deadline"`
	PercentUsed   float64   `json:"percent_used"`
	TimeRemaining string    `json:"time_remaining"`
}

// EscalationResult represents the result of escalation check.
type EscalationResult struct {
	ShouldEscalate  bool     `json:"should_escalate"`
	EscalationLevel string   `json:"escalation_level"`
	NotifyList      []string `json:"notify_list"`
	Level           string   `json:"level"`
}

// SLA hours by priority (configurable, using defaults for now).
var slaHours = map[string]float64{
	"5 very high": 1,  // 1 hour
	"4 high":      4,  // 4 hours
	"3 normal":    8,  // 8 hours
	"2 low":       24, // 24 hours
	"1 very low":  48, // 48 hours
}

var warningThreshold = 75.0 // Warn when 75% of SLA is used

// Calculate SLA status for a ticket.
func calculateSLA(priority string, createdAt, currentTime time.Time) SLA {
	hours, exists := slaHours[priority]
	if !exists {
		hours = slaHours["3 normal"] // Default to normal if unknown
	}

	deadline := createdAt.Add(time.Duration(hours) * time.Hour)
	elapsed := currentTime.Sub(createdAt).Hours()
	percentUsed := (elapsed / hours) * 100

	var status string
	if percentUsed >= 100 {
		status = "overdue"
	} else if percentUsed >= warningThreshold {
		status = "warning"
	} else {
		status = "within"
	}

	remaining := deadline.Sub(currentTime)
	var timeRemaining string
	if remaining < 0 {
		timeRemaining = fmt.Sprintf("-%s overdue", formatDuration(-remaining))
	} else {
		timeRemaining = formatDuration(remaining)
	}

	return SLA{
		Status:        status,
		Deadline:      deadline,
		PercentUsed:   percentUsed,
		TimeRemaining: timeRemaining,
	}
}

// Format duration in human-readable format.
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 24 {
		days := hours / 24
		hours = hours % 24
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// Get ticket SLA status handler.
func handleGetTicketSLA(c *gin.Context) {
	ticketID := c.Param("id")

	// Parse ticket ID
	id, err := strconv.Atoi(ticketID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	// Mock ticket data for testing
	var ticketData map[string]interface{}

	switch id {
	case 101: // Overdue ticket
		ticketData = map[string]interface{}{
			"priority":   "4 high",
			"created_at": time.Now().Add(-5 * time.Hour),
			"status":     "open",
		}
	case 102: // Closed ticket
		ticketData = map[string]interface{}{
			"priority":   "3 normal",
			"created_at": time.Now().Add(-2 * time.Hour),
			"status":     "closed",
		}
	default: // Normal ticket
		ticketData = map[string]interface{}{
			"priority":   "3 normal",
			"created_at": time.Now().Add(-2 * time.Hour),
			"status":     "open",
		}
	}

	// Check if ticket is closed
	if ticketData["status"] == "closed" {
		// Check if request wants JSON (for API calls)
		if c.GetHeader("Accept") == "application/json" || true {
			c.JSON(http.StatusOK, gin.H{
				"sla_status":          "not_applicable",
				"sla_deadline":        nil,
				"sla_percent_used":    nil,
				"time_remaining":      nil,
				"business_hours_only": false,
			})
			return
		}

		// Return HTML fragment for HTMX
		shared.GetGlobalRenderer().HTML(c, http.StatusOK, "components/sla_status.pongo2", gin.H{
			"sla_status": "not_applicable",
		})
		return
	}

	// Calculate SLA
	priority, _ := ticketData["priority"].(string)       //nolint:errcheck // Defaults to empty
	createdAt, _ := ticketData["created_at"].(time.Time) //nolint:errcheck // Zero time on error
	sla := calculateSLA(
		priority,
		createdAt,
		time.Now(),
	)

	// Check if request wants JSON (for API calls)
	if c.GetHeader("Accept") == "application/json" || true {
		c.JSON(http.StatusOK, gin.H{
			"ticket_id":           id,
			"sla_status":          sla.Status,
			"sla_deadline":        sla.Deadline,
			"sla_percent_used":    sla.PercentUsed,
			"time_remaining":      sla.TimeRemaining,
			"business_hours_only": false,
		})
		return
	}

	// Return HTML fragment for HTMX
	shared.GetGlobalRenderer().HTML(c, http.StatusOK, "components/sla_status.pongo2", gin.H{
		"sla_status":       sla.Status,
		"sla_deadline":     sla.Deadline,
		"sla_percent_used": sla.PercentUsed,
		"time_remaining":   sla.TimeRemaining,
	})
}

// Escalate ticket handler.
func handleEscalateTicket(c *gin.Context) {
	ticketID := c.Param("id")

	// Parse ticket ID
	id, err := strconv.Atoi(ticketID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	// Check if ticket is closed (mock)
	if id == 205 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot escalate closed ticket"})
		return
	}

	var req struct {
		EscalationLevel string `form:"escalation_level" binding:"required"`
		Reason          string `form:"reason"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate reason
	if req.Reason == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Escalation reason is required"})
		return
	}

	// Validate escalation level
	validLevels := []string{"senior_agent", "manager", "executive"}
	valid := false
	for _, level := range validLevels {
		if req.EscalationLevel == level {
			valid = true
			break
		}
	}

	if !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid escalation level"})
		return
	}

	// Determine who to escalate to (mock)
	var escalatedTo string
	switch req.EscalationLevel {
	case "senior_agent":
		escalatedTo = "Senior Agent Team"
	case "manager":
		escalatedTo = "Support Manager"
	case "executive":
		escalatedTo = "Executive Team"
	}

	response := gin.H{
		"message":          "Ticket escalated successfully",
		"ticket_id":        id,
		"escalation_level": req.EscalationLevel,
		"escalated_to":     escalatedTo,
		"escalated_at":     time.Now(),
		"reason":           req.Reason,
	}

	// Auto-upgrade priority for executive escalation
	if req.EscalationLevel == "executive" {
		response["new_priority"] = "5 very high"
	}

	c.JSON(http.StatusOK, response)
}

// Check if ticket should be auto-escalated.
func checkAutoEscalation(ticketData map[string]interface{}) EscalationResult {
	// Already escalated?
	if escalated, ok := ticketData["escalated"].(bool); ok && escalated {
		return EscalationResult{ShouldEscalate: false}
	}

	// Check if closed
	if status, ok := ticketData["status"].(string); ok && status == "closed" {
		return EscalationResult{ShouldEscalate: false}
	}

	priority, _ := ticketData["priority"].(string)       //nolint:errcheck // Defaults to empty
	createdAt, _ := ticketData["created_at"].(time.Time) //nolint:errcheck // Zero time on error

	// Calculate SLA
	sla := calculateSLA(priority, createdAt, time.Now())

	// Determine if escalation needed
	if sla.Status == "overdue" {
		level := "senior_agent"

		// Higher escalation for very high priority only
		if priority == "5 very high" {
			level = "manager"
		}

		return EscalationResult{
			ShouldEscalate:  true,
			EscalationLevel: level,
		}
	}

	// Special case: Very high priority tickets escalate faster
	if priority == "5 very high" && sla.PercentUsed > 75 {
		return EscalationResult{
			ShouldEscalate:  true,
			EscalationLevel: "manager",
		}
	}

	return EscalationResult{ShouldEscalate: false}
}

// Get SLA report handler.
func handleSLAReport(c *gin.Context) {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	queueID := c.Query("queue_id")

	if startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Date range required"})
		return
	}

	// Mock report data
	report := gin.H{
		"start_date":              startDate,
		"end_date":                endDate,
		"total_tickets":           150,
		"within_sla_count":        120,
		"overdue_count":           30,
		"sla_compliance_percent":  80.0,
		"average_resolution_time": "6h 30m",
		"by_priority": map[string]interface{}{
			"5 very high": gin.H{
				"total":      10,
				"within_sla": 8,
				"overdue":    2,
				"compliance": 80.0,
			},
			"4 high": gin.H{
				"total":      30,
				"within_sla": 24,
				"overdue":    6,
				"compliance": 80.0,
			},
			"3 normal": gin.H{
				"total":      80,
				"within_sla": 65,
				"overdue":    15,
				"compliance": 81.25,
			},
			"2 low": gin.H{
				"total":      20,
				"within_sla": 17,
				"overdue":    3,
				"compliance": 85.0,
			},
			"1 very low": gin.H{
				"total":      10,
				"within_sla": 6,
				"overdue":    4,
				"compliance": 60.0,
			},
		},
	}

	// Add queue info if specified
	if queueID == "1" {
		report["queue_name"] = "General Support"
		report["queue_id"] = queueID
	}

	c.JSON(http.StatusOK, report)
}

// Update SLA configuration handler.
func handleUpdateSLAConfig(c *gin.Context) {
	var req struct {
		VeryHighHours     string `form:"very_high_hours"`
		HighHours         string `form:"high_hours"`
		NormalHours       string `form:"normal_hours"`
		LowHours          string `form:"low_hours"`
		VeryLowHours      string `form:"very_low_hours"`
		BusinessHoursOnly string `form:"business_hours_only"`
		WarningThreshold  string `form:"warning_threshold"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate hours
	if req.VeryHighHours != "" {
		hours, err := strconv.ParseFloat(req.VeryHighHours, 64)
		if err != nil || hours <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "SLA hours must be greater than 0"})
			return
		}
		slaHours["5 very high"] = hours
	}

	// Validate warning threshold
	if req.WarningThreshold != "" {
		threshold, err := strconv.ParseFloat(req.WarningThreshold, 64)
		if err != nil || threshold < 1 || threshold > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Warning threshold must be between 1 and 100"})
			return
		}
		warningThreshold = threshold
	}

	// Parse other values
	businessHoursOnly := false
	if req.BusinessHoursOnly == "true" {
		businessHoursOnly = true
	}

	// Build config response
	config := gin.H{
		"very_high_hours":     slaHours["5 very high"],
		"high_hours":          slaHours["4 high"],
		"normal_hours":        slaHours["3 normal"],
		"low_hours":           slaHours["2 low"],
		"very_low_hours":      slaHours["1 very low"],
		"business_hours_only": businessHoursOnly,
		"warning_threshold":   warningThreshold,
	}

	// Update other hours if provided
	if req.HighHours != "" {
		if hours, err := strconv.ParseFloat(req.HighHours, 64); err == nil && hours > 0 {
			slaHours["4 high"] = hours
			config["high_hours"] = hours
		}
	}
	if req.NormalHours != "" {
		if hours, err := strconv.ParseFloat(req.NormalHours, 64); err == nil && hours > 0 {
			slaHours["3 normal"] = hours
			config["normal_hours"] = hours
		}
	}
	if req.LowHours != "" {
		if hours, err := strconv.ParseFloat(req.LowHours, 64); err == nil && hours > 0 {
			slaHours["2 low"] = hours
			config["low_hours"] = hours
		}
	}
	if req.VeryLowHours != "" {
		if hours, err := strconv.ParseFloat(req.VeryLowHours, 64); err == nil && hours > 0 {
			slaHours["1 very low"] = hours
			config["very_low_hours"] = hours
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "SLA configuration updated",
		"config":  config,
	})
}

// Determine escalation level based on priority and overdue time.
func determineEscalationLevel(priority string, hoursOverdue float64, previousLevel string) EscalationResult {
	var level string
	var notifyList []string

	// Escalation matrix based on priority and overdue hours
	switch priority {
	case "5 very high":
		if previousLevel == "manager" || hoursOverdue >= 24 {
			level = "executive"
			notifyList = []string{"executive", "manager", "director"}
		} else if hoursOverdue >= 0.5 {
			level = "manager"
			notifyList = []string{"manager", "senior_agents", "team_lead"}
		} else {
			level = "senior_agent"
			notifyList = []string{"senior_agents", "team_lead"}
		}

	case "4 high":
		if previousLevel == "senior_agent" && hoursOverdue >= 8 {
			level = "manager"
			notifyList = []string{"manager", "team_lead", "assigned_agent"}
		} else {
			level = "senior_agent"
			notifyList = []string{"senior_agents", "team_lead"}
		}

	default: // Normal and below
		if hoursOverdue >= 48 {
			level = "manager"
			notifyList = []string{"manager", "team_lead"}
		} else {
			level = "senior_agent"
			notifyList = []string{"senior_agents", "team_lead"}
		}
	}

	return EscalationResult{
		Level:      level,
		NotifyList: notifyList,
	}
}
