package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestTicketWorkflowStates(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		ticketID       string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should show workflow state diagram",
			ticketID:       "1",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should display current state
				assert.Contains(t, body, `data-current-state="open"`)
				
				// Should show available transitions
				assert.Contains(t, body, "Mark as Pending")
				assert.Contains(t, body, "Resolve Ticket")
				assert.Contains(t, body, "Close Ticket")
				
				// Should show state history
				assert.Contains(t, body, "State History")
				assert.Contains(t, body, "Changed from new to open")
			},
		},
		{
			name:           "should display state badges correctly",
			ticketID:       "1",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// State badges with appropriate colors
				assert.Contains(t, body, `class="state-badge state-new"`)     // Blue
				assert.Contains(t, body, `class="state-badge state-open"`)    // Green
				assert.Contains(t, body, `class="state-badge state-pending"`) // Yellow
				assert.Contains(t, body, `class="state-badge state-resolved"`) // Purple
				assert.Contains(t, body, `class="state-badge state-closed"`)  // Gray
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/tickets/:id/workflow", handleTicketWorkflow)

			req, _ := http.NewRequest("GET", "/tickets/"+tt.ticketID+"/workflow", nil)
			req.Header.Set("HX-Request", "true")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestTicketStateTransitions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		ticketID       string
		currentState   string
		newState       string
		expectedStatus int
		shouldSucceed  bool
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should transition from new to open",
			ticketID:       "1",
			currentState:   "new",
			newState:       "open",
			expectedStatus: http.StatusOK,
			shouldSucceed:  true,
			checkResponse: func(t *testing.T, body string) {
				var response map[string]interface{}
				json.Unmarshal([]byte(body), &response)
				assert.Equal(t, true, response["success"])
				assert.Equal(t, "open", response["new_state"])
				assert.Contains(t, response["message"], "Ticket opened")
			},
		},
		{
			name:           "should transition from open to pending",
			ticketID:       "1",
			currentState:   "open",
			newState:       "pending",
			expectedStatus: http.StatusOK,
			shouldSucceed:  true,
			checkResponse: func(t *testing.T, body string) {
				var response map[string]interface{}
				json.Unmarshal([]byte(body), &response)
				assert.Equal(t, true, response["success"])
				assert.Equal(t, "pending", response["new_state"])
				assert.Contains(t, response["message"], "Ticket marked as pending")
			},
		},
		{
			name:           "should transition from pending back to open",
			ticketID:       "1",
			currentState:   "pending",
			newState:       "open",
			expectedStatus: http.StatusOK,
			shouldSucceed:  true,
			checkResponse: func(t *testing.T, body string) {
				var response map[string]interface{}
				json.Unmarshal([]byte(body), &response)
				assert.Equal(t, true, response["success"])
				assert.Equal(t, "open", response["new_state"])
			},
		},
		{
			name:           "should transition from open to resolved",
			ticketID:       "1",
			currentState:   "open",
			newState:       "resolved",
			expectedStatus: http.StatusOK,
			shouldSucceed:  true,
			checkResponse: func(t *testing.T, body string) {
				var response map[string]interface{}
				json.Unmarshal([]byte(body), &response)
				assert.Equal(t, true, response["success"])
				assert.Equal(t, "resolved", response["new_state"])
				assert.Contains(t, response["message"], "Ticket resolved")
			},
		},
		{
			name:           "should transition from resolved to closed",
			ticketID:       "1",
			currentState:   "resolved",
			newState:       "closed",
			expectedStatus: http.StatusOK,
			shouldSucceed:  true,
			checkResponse: func(t *testing.T, body string) {
				var response map[string]interface{}
				json.Unmarshal([]byte(body), &response)
				assert.Equal(t, true, response["success"])
				assert.Equal(t, "closed", response["new_state"])
				assert.Contains(t, response["message"], "Ticket closed")
			},
		},
		{
			name:           "should allow reopening closed ticket",
			ticketID:       "1",
			currentState:   "closed",
			newState:       "open",
			expectedStatus: http.StatusOK,
			shouldSucceed:  true,
			checkResponse: func(t *testing.T, body string) {
				var response map[string]interface{}
				json.Unmarshal([]byte(body), &response)
				assert.Equal(t, true, response["success"])
				assert.Equal(t, "open", response["new_state"])
				assert.Contains(t, response["message"], "Ticket reopened")
			},
		},
		{
			name:           "should reject invalid transition from new to resolved",
			ticketID:       "1",
			currentState:   "new",
			newState:       "resolved",
			expectedStatus: http.StatusBadRequest,
			shouldSucceed:  false,
			checkResponse: func(t *testing.T, body string) {
				var response map[string]interface{}
				json.Unmarshal([]byte(body), &response)
				assert.Equal(t, false, response["success"])
				assert.Contains(t, response["error"], "Invalid state transition")
			},
		},
		{
			name:           "should reject invalid transition from new to closed",
			ticketID:       "1",
			currentState:   "new",
			newState:       "closed",
			expectedStatus: http.StatusBadRequest,
			shouldSucceed:  false,
			checkResponse: func(t *testing.T, body string) {
				var response map[string]interface{}
				json.Unmarshal([]byte(body), &response)
				assert.Equal(t, false, response["success"])
				assert.Contains(t, response["error"], "Cannot close ticket that hasn't been resolved")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/tickets/:id/transition", handleTicketTransition)

			formData := "current_state=" + tt.currentState + "&new_state=" + tt.newState
			// Add reason for transitions that require it
			if tt.newState == "pending" && tt.shouldSucceed {
				formData += "&reason=Test reason for pending"
			}
			if tt.newState == "resolved" && tt.shouldSucceed {
				formData += "&reason=Test resolution notes"
			}
			req, _ := http.NewRequest("POST", "/tickets/"+tt.ticketID+"/transition", 
				strings.NewReader(formData))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("HX-Request", "true")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestTicketWorkflowReasons(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		ticketID       string
		newState       string
		reason         string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should require reason for pending state",
			ticketID:       "1",
			newState:       "pending",
			reason:         "",
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "Reason required for pending state")
			},
		},
		{
			name:           "should accept reason for pending state",
			ticketID:       "1",
			newState:       "pending",
			reason:         "Waiting for customer response",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				var response map[string]interface{}
				json.Unmarshal([]byte(body), &response)
				assert.Equal(t, true, response["success"])
				assert.Contains(t, response["reason"], "Waiting for customer response")
			},
		},
		{
			name:           "should require resolution notes when resolving",
			ticketID:       "1",
			newState:       "resolved",
			reason:         "",
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "Resolution notes required")
			},
		},
		{
			name:           "should accept resolution notes",
			ticketID:       "1",
			newState:       "resolved",
			reason:         "Issue fixed by restarting service",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				var response map[string]interface{}
				json.Unmarshal([]byte(body), &response)
				assert.Equal(t, true, response["success"])
				assert.Contains(t, response["resolution"], "Issue fixed by restarting service")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/tickets/:id/transition", handleTicketTransition)

			formData := "new_state=" + tt.newState + "&reason=" + tt.reason
			req, _ := http.NewRequest("POST", "/tickets/"+tt.ticketID+"/transition", 
				strings.NewReader(formData))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("HX-Request", "true")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestTicketWorkflowHistory(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		ticketID       string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should show state transition history",
			ticketID:       "1",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should show timeline of state changes
				assert.Contains(t, body, "State History")
				assert.Contains(t, body, "Created as new")
				assert.Contains(t, body, "Changed to open")
				assert.Contains(t, body, "by Demo User")
				assert.Contains(t, body, "ago")
				
				// Should show transition reasons
				assert.Contains(t, body, "Reason:")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/tickets/:id/history", handleTicketHistory)

			req, _ := http.NewRequest("GET", "/tickets/"+tt.ticketID+"/history", nil)
			req.Header.Set("HX-Request", "true")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestTicketWorkflowAutomation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		ticketID       string
		trigger        string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should auto-open ticket on first agent response",
			ticketID:       "1",
			trigger:        "agent_response",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				var response map[string]interface{}
				json.Unmarshal([]byte(body), &response)
				assert.Equal(t, "open", response["new_state"])
				assert.Contains(t, response["message"], "Ticket automatically opened")
			},
		},
		{
			name:           "should auto-reopen on customer response to resolved ticket",
			ticketID:       "1",
			trigger:        "customer_response",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				var response map[string]interface{}
				json.Unmarshal([]byte(body), &response)
				assert.Equal(t, "open", response["new_state"])
				assert.Contains(t, response["message"], "Ticket reopened due to customer response")
			},
		},
		{
			name:           "should auto-close resolved tickets after timeout",
			ticketID:       "1",
			trigger:        "auto_close_timeout",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				var response map[string]interface{}
				json.Unmarshal([]byte(body), &response)
				assert.Equal(t, "closed", response["new_state"])
				assert.Contains(t, response["message"], "Ticket auto-closed after resolution timeout")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/tickets/:id/auto-transition", handleTicketAutoTransition)

			formData := "trigger=" + tt.trigger
			req, _ := http.NewRequest("POST", "/tickets/"+tt.ticketID+"/auto-transition", 
				strings.NewReader(formData))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestTicketWorkflowPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		userRole       string
		ticketID       string
		newState       string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "agent should be able to transition states",
			userRole:       "agent",
			ticketID:       "1",
			newState:       "pending",
			expectedStatus: http.StatusBadRequest, // Will fail without reason
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "Reason required")
			},
		},
		{
			name:           "customer should not be able to resolve ticket",
			userRole:       "customer",
			ticketID:       "1",
			newState:       "resolved",
			expectedStatus: http.StatusForbidden,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "Permission denied")
			},
		},
		{
			name:           "customer can request reopening",
			userRole:       "customer",
			ticketID:       "1",
			newState:       "reopen_requested",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				var response map[string]interface{}
				json.Unmarshal([]byte(body), &response)
				assert.Equal(t, true, response["success"])
				assert.Contains(t, response["message"], "Reopen request submitted")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			// Add middleware to set user role
			router.Use(func(c *gin.Context) {
				c.Set("user_role", tt.userRole)
				c.Next()
			})
			router.POST("/tickets/:id/transition", handleTicketTransition)

			formData := "new_state=" + tt.newState
			req, _ := http.NewRequest("POST", "/tickets/"+tt.ticketID+"/transition", 
				strings.NewReader(formData))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}