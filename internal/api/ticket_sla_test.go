package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test-Driven Development for SLA and Escalation Feature
// Writing tests first, then implementing the functionality

func TestSLACalculation(t *testing.T) {
	tests := []struct {
		name         string
		priority     string
		createdAt    time.Time
		currentTime  time.Time
		wantStatus   string
		wantDeadline time.Time
		wantPercent  float64
	}{
		{
			name:         "Very High priority - 1 hour SLA",
			priority:     "5 very high",
			createdAt:    time.Now().Add(-30 * time.Minute),
			currentTime:  time.Now(),
			wantStatus:   "within",
			wantDeadline: time.Now().Add(30 * time.Minute),
			wantPercent:  50.0, // 30 minutes of 1 hour used
		},
		{
			name:         "High priority - 4 hour SLA",
			priority:     "4 high",
			createdAt:    time.Now().Add(-3 * time.Hour),
			currentTime:  time.Now(),
			wantStatus:   "warning", // 75% of SLA used
			wantDeadline: time.Now().Add(1 * time.Hour),
			wantPercent:  75.0,
		},
		{
			name:         "Normal priority - 8 hour SLA",
			priority:     "3 normal",
			createdAt:    time.Now().Add(-2 * time.Hour),
			currentTime:  time.Now(),
			wantStatus:   "within",
			wantDeadline: time.Now().Add(6 * time.Hour),
			wantPercent:  25.0,
		},
		{
			name:         "Low priority - 24 hour SLA",
			priority:     "2 low",
			createdAt:    time.Now().Add(-20 * time.Hour),
			currentTime:  time.Now(),
			wantStatus:   "warning", // Over 80% of SLA used
			wantDeadline: time.Now().Add(4 * time.Hour),
			wantPercent:  83.33,
		},
		{
			name:         "Very Low priority - 48 hour SLA",
			priority:     "1 very low",
			createdAt:    time.Now().Add(-50 * time.Hour),
			currentTime:  time.Now(),
			wantStatus:   "overdue",
			wantDeadline: time.Now().Add(-2 * time.Hour),
			wantPercent:  104.17,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sla := calculateSLA(tt.priority, tt.createdAt, tt.currentTime)
			
			assert.Equal(t, tt.wantStatus, sla.Status)
			assert.WithinDuration(t, tt.wantDeadline, sla.Deadline, 5*time.Minute)
			assert.InDelta(t, tt.wantPercent, sla.PercentUsed, 5.0)
		})
	}
}

func TestGetTicketSLAStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		ticketID   string
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name:       "Get SLA status for valid ticket",
			ticketID:   "100",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp, "sla_status")
				assert.Contains(t, resp, "sla_deadline")
				assert.Contains(t, resp, "sla_percent_used")
				assert.Contains(t, resp, "time_remaining")
				assert.Contains(t, resp, "business_hours_only")
			},
		},
		{
			name:       "Overdue ticket shows correct status",
			ticketID:   "101", // Mock as overdue
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "overdue", resp["sla_status"])
				assert.True(t, resp["sla_percent_used"].(float64) > 100)
				assert.Contains(t, resp["time_remaining"], "-") // Negative time
			},
		},
		{
			name:       "Closed ticket has no SLA",
			ticketID:   "102", // Mock as closed
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "not_applicable", resp["sla_status"])
				assert.Nil(t, resp["sla_deadline"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/api/tickets/:id/sla", handleGetTicketSLA)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/tickets/"+tt.ticketID+"/sla", nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.checkResp != nil {
				tt.checkResp(t, response)
			}
		})
	}
}

func TestEscalateTicket(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		ticketID   string
		formData   url.Values
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name:     "Manual escalation to manager",
			ticketID: "200",
			formData: url.Values{
				"escalation_level": {"manager"},
				"reason":           {"Customer complaint requires manager attention"},
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "Ticket escalated successfully", resp["message"])
				assert.Equal(t, "manager", resp["escalation_level"])
				assert.Contains(t, resp, "escalated_to")
				assert.Contains(t, resp, "escalated_at")
			},
		},
		{
			name:     "Escalation to senior agent",
			ticketID: "201",
			formData: url.Values{
				"escalation_level": {"senior_agent"},
				"reason":           {"Technical complexity requires senior expertise"},
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "senior_agent", resp["escalation_level"])
			},
		},
		{
			name:     "Escalation to executive",
			ticketID: "202",
			formData: url.Values{
				"escalation_level": {"executive"},
				"reason":           {"VIP customer issue"},
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "executive", resp["escalation_level"])
				assert.Equal(t, "5 very high", resp["new_priority"]) // Auto-upgrade to very high
			},
		},
		{
			name:     "Missing escalation reason",
			ticketID: "203",
			formData: url.Values{
				"escalation_level": {"manager"},
			},
			wantStatus: http.StatusBadRequest,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Escalation reason is required")
			},
		},
		{
			name:     "Invalid escalation level",
			ticketID: "204",
			formData: url.Values{
				"escalation_level": {"invalid"},
				"reason":           {"Test"},
			},
			wantStatus: http.StatusBadRequest,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Invalid escalation level")
			},
		},
		{
			name:     "Cannot escalate closed ticket",
			ticketID: "205", // Mock as closed
			formData: url.Values{
				"escalation_level": {"manager"},
				"reason":           {"Test"},
			},
			wantStatus: http.StatusBadRequest,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Cannot escalate closed ticket")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/api/tickets/:id/escalate", handleEscalateTicket)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/tickets/"+tt.ticketID+"/escalate", strings.NewReader(tt.formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.checkResp != nil {
				tt.checkResp(t, response)
			}
		})
	}
}

func TestAutoEscalation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name      string
		ticketID  string
		mockData  map[string]interface{}
		shouldEscalate bool
		escalationLevel string
	}{
		{
			name:     "Auto-escalate overdue high priority",
			ticketID: "300",
			mockData: map[string]interface{}{
				"priority":    "4 high",
				"created_at":  time.Now().Add(-5 * time.Hour), // Overdue for 4-hour SLA
				"status":      "open",
				"escalated":   false,
			},
			shouldEscalate: true,
			escalationLevel: "senior_agent",
		},
		{
			name:     "Auto-escalate very high priority after 30 min",
			ticketID: "301",
			mockData: map[string]interface{}{
				"priority":    "5 very high",
				"created_at":  time.Now().Add(-45 * time.Minute),
				"status":      "open",
				"escalated":   false,
			},
			shouldEscalate: true,
			escalationLevel: "manager",
		},
		{
			name:     "Don't escalate within SLA",
			ticketID: "302",
			mockData: map[string]interface{}{
				"priority":    "3 normal",
				"created_at":  time.Now().Add(-2 * time.Hour), // Within 8-hour SLA
				"status":      "open",
				"escalated":   false,
			},
			shouldEscalate: false,
			escalationLevel: "",
		},
		{
			name:     "Don't escalate already escalated ticket",
			ticketID: "303",
			mockData: map[string]interface{}{
				"priority":    "4 high",
				"created_at":  time.Now().Add(-5 * time.Hour),
				"status":      "open",
				"escalated":   true,
			},
			shouldEscalate: false,
			escalationLevel: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkAutoEscalation(tt.mockData)
			
			assert.Equal(t, tt.shouldEscalate, result.ShouldEscalate)
			if tt.shouldEscalate {
				assert.Equal(t, tt.escalationLevel, result.EscalationLevel)
			}
		})
	}
}

func TestSLAReport(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		queryParams string
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name:        "Get SLA report for date range",
			queryParams: "?start_date=2024-01-01&end_date=2024-01-31",
			wantStatus:  http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp, "total_tickets")
				assert.Contains(t, resp, "within_sla_count")
				assert.Contains(t, resp, "overdue_count")
				assert.Contains(t, resp, "sla_compliance_percent")
				assert.Contains(t, resp, "average_resolution_time")
				assert.Contains(t, resp, "by_priority")
				
				// Check by_priority breakdown
				priorities := resp["by_priority"].(map[string]interface{})
				assert.Contains(t, priorities, "5 very high")
				assert.Contains(t, priorities, "4 high")
				assert.Contains(t, priorities, "3 normal")
			},
		},
		{
			name:        "Get SLA report for specific queue",
			queryParams: "?queue_id=1&start_date=2024-01-01&end_date=2024-01-31",
			wantStatus:  http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp, "queue_name")
				assert.Equal(t, "General Support", resp["queue_name"])
			},
		},
		{
			name:        "Missing date range returns error",
			queryParams: "",
			wantStatus:  http.StatusBadRequest,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Date range required")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/api/reports/sla", handleSLAReport)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/reports/sla"+tt.queryParams, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.checkResp != nil {
				tt.checkResp(t, response)
			}
		})
	}
}

func TestSLAConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		formData   url.Values
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name: "Update SLA configuration",
			formData: url.Values{
				"very_high_hours": {"1"},
				"high_hours":      {"4"},
				"normal_hours":    {"8"},
				"low_hours":       {"24"},
				"very_low_hours":  {"48"},
				"business_hours_only": {"true"},
				"warning_threshold": {"75"}, // Warn at 75% of SLA
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "SLA configuration updated", resp["message"])
				config := resp["config"].(map[string]interface{})
				assert.Equal(t, float64(1), config["very_high_hours"])
				assert.Equal(t, float64(75), config["warning_threshold"])
				assert.True(t, config["business_hours_only"].(bool))
			},
		},
		{
			name: "Invalid hours value",
			formData: url.Values{
				"very_high_hours": {"0"}, // Can't be 0
			},
			wantStatus: http.StatusBadRequest,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "SLA hours must be greater than 0")
			},
		},
		{
			name: "Invalid warning threshold",
			formData: url.Values{
				"warning_threshold": {"150"}, // Can't be over 100%
			},
			wantStatus: http.StatusBadRequest,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Warning threshold must be between 1 and 100")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.PUT("/api/admin/sla-config", handleUpdateSLAConfig)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("PUT", "/api/admin/sla-config", strings.NewReader(tt.formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.checkResp != nil {
				tt.checkResp(t, response)
			}
		})
	}
}

func TestEscalationMatrix(t *testing.T) {
	tests := []struct {
		name            string
		priority        string
		hoursOverdue    float64
		previousLevel   string
		wantLevel       string
		wantNotifyList  []string
	}{
		{
			name:          "Normal priority first escalation",
			priority:      "3 normal",
			hoursOverdue:  2,
			previousLevel: "",
			wantLevel:     "senior_agent",
			wantNotifyList: []string{"senior_agents", "team_lead"},
		},
		{
			name:          "High priority second escalation",
			priority:      "4 high",
			hoursOverdue:  8,
			previousLevel: "senior_agent",
			wantLevel:     "manager",
			wantNotifyList: []string{"manager", "team_lead", "assigned_agent"},
		},
		{
			name:          "Very high immediate escalation",
			priority:      "5 very high",
			hoursOverdue:  0.5,
			previousLevel: "",
			wantLevel:     "manager",
			wantNotifyList: []string{"manager", "senior_agents", "team_lead"},
		},
		{
			name:          "Executive escalation for severe overdue",
			priority:      "5 very high",
			hoursOverdue:  24,
			previousLevel: "manager",
			wantLevel:     "executive",
			wantNotifyList: []string{"executive", "manager", "director"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineEscalationLevel(tt.priority, tt.hoursOverdue, tt.previousLevel)
			
			assert.Equal(t, tt.wantLevel, result.Level)
			assert.ElementsMatch(t, tt.wantNotifyList, result.NotifyList)
		})
	}
}