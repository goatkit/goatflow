//go:build db

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test-Driven Development for Ticket Edit Feature
// Writing tests first, then implementing the functionality

func TestGetTicketEditForm(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		ticketID   string
		wantStatus int
		checkBody  func(t *testing.T, body string)
	}{
		{
			name:       "Valid ticket ID returns edit form",
			ticketID:   "123",
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				// Should return an edit form with current ticket data
				assert.Contains(t, body, "Edit Ticket")
				assert.Contains(t, body, "form")
				assert.Contains(t, body, "subject")
				assert.Contains(t, body, "priority")
				assert.Contains(t, body, "queue")
				assert.Contains(t, body, "Save Changes")
			},
		},
		{
			name:       "Invalid ticket ID returns error",
			ticketID:   "invalid",
			wantStatus: http.StatusBadRequest,
			checkBody: func(t *testing.T, body string) {
				assert.Contains(t, body, "Invalid ticket ID")
			},
		},
		{
			name:       "Non-existent ticket returns 404",
			ticketID:   "999999",
			wantStatus: http.StatusNotFound,
			checkBody: func(t *testing.T, body string) {
				assert.Contains(t, body, "Ticket not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/tickets/:id/edit", handleTicketEditForm)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/tickets/"+tt.ticketID+"/edit", nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.checkBody != nil {
				tt.checkBody(t, w.Body.String())
			}
		})
	}
}

func TestUpdateTicketHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		ticketID   string
		formData   url.Values
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{}, headers http.Header)
	}{
		{
			name:     "Update all editable fields",
			ticketID: "123",
			formData: url.Values{
				"subject":  {"Updated Subject"},
				"priority": {"4 high"},
				"queue_id": {"2"},
				"type_id":  {"3"},
				"status":   {"open"},
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}, headers http.Header) {
				assert.Equal(t, "Ticket updated successfully", resp["message"])
				assert.Equal(t, "Updated Subject", resp["subject"])
				assert.Equal(t, "4 high", resp["priority"])
				assert.Equal(t, float64(2), resp["queue_id"])
				assert.Equal(t, float64(3), resp["type_id"])
				assert.Equal(t, "open", resp["status"])

				// Check for HTMX trigger to show success message
				assert.Contains(t, headers.Get("HX-Trigger"), "ticketUpdated")
			},
		},
		{
			name:     "Update only subject",
			ticketID: "124",
			formData: url.Values{
				"subject": {"Only Subject Updated"},
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}, headers http.Header) {
				assert.Equal(t, "Ticket updated successfully", resp["message"])
				assert.Equal(t, "Only Subject Updated", resp["subject"])
				// Other fields should remain unchanged (returned from DB)
			},
		},
		{
			name:     "Update with empty subject fails",
			ticketID: "125",
			formData: url.Values{
				"subject":  {""},
				"priority": {"3 normal"},
			},
			wantStatus: http.StatusBadRequest,
			checkResp: func(t *testing.T, resp map[string]interface{}, headers http.Header) {
				assert.Contains(t, resp["error"], "Subject cannot be empty")
			},
		},
		{
			name:     "Update non-existent ticket",
			ticketID: "999999",
			formData: url.Values{
				"subject": {"Test"},
			},
			wantStatus: http.StatusNotFound,
			checkResp: func(t *testing.T, resp map[string]interface{}, headers http.Header) {
				assert.Contains(t, resp["error"], "Ticket not found")
			},
		},
		{
			name:     "Invalid ticket ID format",
			ticketID: "invalid",
			formData: url.Values{
				"subject": {"Test"},
			},
			wantStatus: http.StatusBadRequest,
			checkResp: func(t *testing.T, resp map[string]interface{}, headers http.Header) {
				assert.Contains(t, resp["error"], "Invalid ticket ID")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.PUT("/api/tickets/:id", handleUpdateTicketEnhanced)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("PUT", "/api/tickets/"+tt.ticketID, strings.NewReader(tt.formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.checkResp != nil {
				tt.checkResp(t, response, w.Header())
			}
		})
	}
}

func TestTicketEditPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		ticketID   string
		userRole   string
		userID     int
		assignedTo int
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name:       "Admin can edit any ticket",
			ticketID:   "200",
			userRole:   "admin",
			userID:     1,
			assignedTo: 2, // Assigned to different user
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "Ticket updated successfully", resp["message"])
			},
		},
		{
			name:       "Agent can edit assigned ticket",
			ticketID:   "201",
			userRole:   "agent",
			userID:     3,
			assignedTo: 3, // Assigned to same user
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "Ticket updated successfully", resp["message"])
			},
		},
		{
			name:       "Agent cannot edit unassigned ticket",
			ticketID:   "202",
			userRole:   "agent",
			userID:     3,
			assignedTo: 0, // Unassigned
			wantStatus: http.StatusForbidden,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "not authorized")
			},
		},
		{
			name:       "Agent cannot edit ticket assigned to others",
			ticketID:   "203",
			userRole:   "agent",
			userID:     3,
			assignedTo: 4, // Assigned to different agent
			wantStatus: http.StatusForbidden,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "not authorized")
			},
		},
		{
			name:       "Customer cannot edit tickets",
			ticketID:   "204",
			userRole:   "customer",
			userID:     5,
			assignedTo: 0,
			wantStatus: http.StatusForbidden,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "not authorized")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()

			// Add middleware to set user context
			router.Use(func(c *gin.Context) {
				c.Set("user_id", tt.userID)
				c.Set("user_role", tt.userRole)
				c.Next()
			})

			router.PUT("/api/tickets/:id", handleUpdateTicketEnhanced)

			formData := url.Values{
				"subject": {"Permission Test Update"},
			}

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("PUT", "/api/tickets/"+tt.ticketID, strings.NewReader(formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			// Simulate ticket assignment in context
			type ctxKey string
			req = req.WithContext(context.WithValue(req.Context(), ctxKey("ticket_assigned_to"), tt.assignedTo))

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

func TestTicketEditHistory(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		ticketID   string
		changes    url.Values
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name:     "Edit creates history entry",
			ticketID: "300",
			changes: url.Values{
				"subject":  {"Changed Subject"},
				"priority": {"5 very high"},
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				// Should return history entry
				assert.Contains(t, resp, "history_id")
				assert.Equal(t, "subject,priority", resp["fields_changed"])
				assert.Contains(t, resp, "changed_by")
				assert.Contains(t, resp, "changed_at")
			},
		},
		{
			name:     "No changes returns no history",
			ticketID: "301",
			changes: url.Values{
				"subject": {"Same Subject"}, // Assume this is the current value
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "No changes made", resp["message"])
				assert.Nil(t, resp["history_id"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.PUT("/api/tickets/:id", handleUpdateTicketEnhanced)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("PUT", "/api/tickets/"+tt.ticketID, strings.NewReader(tt.changes.Encode()))
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

func TestTicketEditValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		ticketID   string
		formData   url.Values
		wantStatus int
		wantError  string
	}{
		{
			name:     "Subject too long",
			ticketID: "400",
			formData: url.Values{
				"subject": {strings.Repeat("a", 256)}, // Max 255 chars
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "Subject must be less than 255 characters",
		},
		{
			name:     "Invalid priority value",
			ticketID: "401",
			formData: url.Values{
				"priority": {"invalid"},
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "Invalid priority value",
		},
		{
			name:     "Invalid queue ID",
			ticketID: "402",
			formData: url.Values{
				"queue_id": {"not-a-number"},
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "Invalid queue ID",
		},
		{
			name:     "Invalid status value",
			ticketID: "403",
			formData: url.Values{
				"status": {"invalid_status"},
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "Invalid status. Must be one of: new, open, pending, closed",
		},
		{
			name:     "Cannot edit closed ticket",
			ticketID: "404", // Assume this ticket is closed
			formData: url.Values{
				"subject": {"Try to edit closed ticket"},
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "Cannot edit closed ticket",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.PUT("/api/tickets/:id", handleUpdateTicketEnhanced)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("PUT", "/api/tickets/"+tt.ticketID, strings.NewReader(tt.formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Contains(t, response["error"], tt.wantError)
		})
	}
}

func TestTicketBulkEdit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		ticketIDs  []string
		formData   url.Values
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name:      "Bulk update priority",
			ticketIDs: []string{"501", "502", "503"},
			formData: url.Values{
				"ticket_ids": {"501,502,503"},
				"priority":   {"4 high"},
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, float64(3), resp["updated_count"])
				assert.Equal(t, "Tickets updated successfully", resp["message"])

				// Check individual results
				results := resp["results"].([]interface{})
				assert.Len(t, results, 3)
			},
		},
		{
			name:      "Bulk assign to agent",
			ticketIDs: []string{"504", "505"},
			formData: url.Values{
				"ticket_ids":  {"504,505"},
				"assigned_to": {"10"},
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, float64(2), resp["updated_count"])
			},
		},
		{
			name:      "Bulk update with some failures",
			ticketIDs: []string{"506", "999999", "507"}, // 999999 doesn't exist
			formData: url.Values{
				"ticket_ids": {"506,999999,507"},
				"status":     {"pending"},
			},
			wantStatus: http.StatusPartialContent, // 206
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, float64(2), resp["updated_count"])
				assert.Equal(t, float64(1), resp["failed_count"])

				failures := resp["failures"].([]interface{})
				assert.Len(t, failures, 1)
			},
		},
		{
			name:      "No ticket IDs provided",
			ticketIDs: []string{},
			formData: url.Values{
				"priority": {"3 normal"},
			},
			wantStatus: http.StatusBadRequest,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "No ticket IDs provided")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.PUT("/api/tickets/bulk", handleBulkUpdateTickets)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("PUT", "/api/tickets/bulk", strings.NewReader(tt.formData.Encode()))
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
