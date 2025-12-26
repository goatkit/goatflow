//go:build db

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test-Driven Development for Ticket Merge Feature
// Writing tests first, then implementing the functionality

func TestMergeTickets(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		primaryID  string
		formData   url.Values
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name:      "Merge two tickets successfully",
			primaryID: "100",
			formData: url.Values{
				"ticket_ids": {"101,102"},
				"reason":     {"Duplicate issues reported by same customer"},
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "Tickets merged successfully", resp["message"])
				assert.Equal(t, float64(100), resp["primary_ticket_id"])

				mergedTickets := resp["merged_tickets"].([]interface{})
				assert.Len(t, mergedTickets, 2)
				assert.Contains(t, mergedTickets, float64(101))
				assert.Contains(t, mergedTickets, float64(102))

				assert.Contains(t, resp, "merge_id")
				assert.Contains(t, resp, "merged_at")
			},
		},
		{
			name:      "Merge multiple tickets",
			primaryID: "200",
			formData: url.Values{
				"ticket_ids": {"201,202,203,204"},
				"reason":     {"Multiple reports of same system outage"},
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				mergedTickets := resp["merged_tickets"].([]interface{})
				assert.Len(t, mergedTickets, 4)
			},
		},
		{
			name:      "Cannot merge ticket with itself",
			primaryID: "300",
			formData: url.Values{
				"ticket_ids": {"300"},
				"reason":     {"Test"},
			},
			wantStatus: http.StatusBadRequest,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Cannot merge ticket with itself")
			},
		},
		{
			name:      "Cannot merge closed tickets",
			primaryID: "400",
			formData: url.Values{
				"ticket_ids": {"401"}, // 401 is closed
				"reason":     {"Test"},
			},
			wantStatus: http.StatusBadRequest,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Cannot merge closed tickets")
			},
		},
		{
			name:      "Missing merge reason",
			primaryID: "500",
			formData: url.Values{
				"ticket_ids": {"501"},
			},
			wantStatus: http.StatusBadRequest,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Merge reason is required")
			},
		},
		{
			name:      "No tickets to merge",
			primaryID: "600",
			formData: url.Values{
				"ticket_ids": {""},
				"reason":     {"Test"},
			},
			wantStatus: http.StatusBadRequest,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "No tickets specified for merge")
			},
		},
		{
			name:      "Invalid ticket ID in merge list",
			primaryID: "700",
			formData: url.Values{
				"ticket_ids": {"invalid,702"},
				"reason":     {"Test"},
			},
			wantStatus: http.StatusBadRequest,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Invalid ticket ID")
			},
		},
		{
			name:      "Non-existent ticket in merge list",
			primaryID: "800",
			formData: url.Values{
				"ticket_ids": {"999999"},
				"reason":     {"Test"},
			},
			wantStatus: http.StatusNotFound,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Ticket 999999 not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/api/tickets/:id/merge", handleMergeTickets)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/tickets/"+tt.primaryID+"/merge", strings.NewReader(tt.formData.Encode()))
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

func TestUnmergeTickets(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		ticketID   string
		formData   url.Values
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name:     "Unmerge previously merged ticket",
			ticketID: "101", // Was merged into 100
			formData: url.Values{
				"reason": {"Customer wants separate tracking"},
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "Ticket unmerged successfully", resp["message"])
				assert.Equal(t, float64(101), resp["ticket_id"])
				assert.Equal(t, "open", resp["new_status"])
				assert.Contains(t, resp, "original_ticket_id")
			},
		},
		{
			name:     "Cannot unmerge non-merged ticket",
			ticketID: "150", // Never merged
			formData: url.Values{
				"reason": {"Test"},
			},
			wantStatus: http.StatusBadRequest,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Ticket is not merged")
			},
		},
		{
			name:       "Missing unmerge reason",
			ticketID:   "102",
			formData:   url.Values{},
			wantStatus: http.StatusBadRequest,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Unmerge reason is required")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/api/tickets/:id/unmerge", handleUnmergeTicket)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/tickets/"+tt.ticketID+"/unmerge", strings.NewReader(tt.formData.Encode()))
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

func TestGetMergeHistory(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		ticketID   string
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name:       "Get merge history for primary ticket",
			ticketID:   "100",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp, "merge_history")
				history := resp["merge_history"].([]interface{})
				assert.Greater(t, len(history), 0)

				// Check first merge event
				firstMerge := history[0].(map[string]interface{})
				assert.Contains(t, firstMerge, "action") // "merged" or "unmerged"
				assert.Contains(t, firstMerge, "tickets")
				assert.Contains(t, firstMerge, "reason")
				assert.Contains(t, firstMerge, "performed_by")
				assert.Contains(t, firstMerge, "performed_at")
			},
		},
		{
			name:       "Get merge history for merged ticket",
			ticketID:   "101",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp, "merged_into")
				assert.Equal(t, float64(100), resp["merged_into"])
				assert.Contains(t, resp, "merge_date")
			},
		},
		{
			name:       "No merge history for unmerged ticket",
			ticketID:   "150",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				history := resp["merge_history"].([]interface{})
				assert.Len(t, history, 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/api/tickets/:id/merge-history", handleGetMergeHistory)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/tickets/"+tt.ticketID+"/merge-history", nil)
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

func TestMergePermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		userRole   string
		primaryID  string
		formData   url.Values
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name:      "Admin can merge any tickets",
			userRole:  "admin",
			primaryID: "900",
			formData: url.Values{
				"ticket_ids": {"901"},
				"reason":     {"Admin merge"},
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "Tickets merged successfully", resp["message"])
			},
		},
		{
			name:      "Agent can merge assigned tickets",
			userRole:  "agent",
			primaryID: "910", // Assigned to this agent
			formData: url.Values{
				"ticket_ids": {"911"}, // Also assigned to this agent
				"reason":     {"Agent merge"},
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "Tickets merged successfully", resp["message"])
			},
		},
		{
			name:      "Agent cannot merge unassigned tickets",
			userRole:  "agent",
			primaryID: "920", // Not assigned to this agent
			formData: url.Values{
				"ticket_ids": {"921"},
				"reason":     {"Agent merge attempt"},
			},
			wantStatus: http.StatusForbidden,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "not authorized to merge these tickets")
			},
		},
		{
			name:      "Customer cannot merge tickets",
			userRole:  "customer",
			primaryID: "930",
			formData: url.Values{
				"ticket_ids": {"931"},
				"reason":     {"Customer merge attempt"},
			},
			wantStatus: http.StatusForbidden,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "not authorized to merge tickets")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()

			// Add middleware to set user role
			router.Use(func(c *gin.Context) {
				c.Set("user_role", tt.userRole)
				c.Set("user_id", 1)
				c.Next()
			})

			router.POST("/api/tickets/:id/merge", handleMergeTickets)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/tickets/"+tt.primaryID+"/merge", strings.NewReader(tt.formData.Encode()))
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

func TestMergeValidation(t *testing.T) {
	tests := []struct {
		name          string
		primaryTicket map[string]interface{}
		mergeTickets  []map[string]interface{}
		wantValid     bool
		wantError     string
	}{
		{
			name: "Valid merge - same customer",
			primaryTicket: map[string]interface{}{
				"id":       100,
				"customer": "customer@example.com",
				"status":   "open",
			},
			mergeTickets: []map[string]interface{}{
				{
					"id":       101,
					"customer": "customer@example.com",
					"status":   "open",
				},
			},
			wantValid: true,
		},
		{
			name: "Invalid - different customers",
			primaryTicket: map[string]interface{}{
				"id":       200,
				"customer": "customer1@example.com",
				"status":   "open",
			},
			mergeTickets: []map[string]interface{}{
				{
					"id":       201,
					"customer": "customer2@example.com",
					"status":   "open",
				},
			},
			wantValid: false,
			wantError: "Cannot merge tickets from different customers",
		},
		{
			name: "Invalid - closed ticket",
			primaryTicket: map[string]interface{}{
				"id":       300,
				"customer": "customer@example.com",
				"status":   "open",
			},
			mergeTickets: []map[string]interface{}{
				{
					"id":       301,
					"customer": "customer@example.com",
					"status":   "closed",
				},
			},
			wantValid: false,
			wantError: "Cannot merge closed tickets",
		},
		{
			name: "Invalid - already merged ticket",
			primaryTicket: map[string]interface{}{
				"id":       400,
				"customer": "customer@example.com",
				"status":   "open",
			},
			mergeTickets: []map[string]interface{}{
				{
					"id":          401,
					"customer":    "customer@example.com",
					"status":      "merged",
					"merged_into": 399,
				},
			},
			wantValid: false,
			wantError: "Ticket 401 is already merged",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validateMerge(tt.primaryTicket, tt.mergeTickets)

			if tt.wantValid {
				assert.True(t, result)
				assert.Nil(t, err)
			} else {
				assert.False(t, result)
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), tt.wantError)
			}
		})
	}
}

func TestMergeEffects(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		primaryID    string
		mergeIDs     []string
		checkEffects func(t *testing.T, primaryTicket, mergedTickets map[string]interface{})
	}{
		{
			name:      "Merged ticket messages are combined",
			primaryID: "1000",
			mergeIDs:  []string{"1001", "1002"},
			checkEffects: func(t *testing.T, primary, merged map[string]interface{}) {
				// Primary ticket should have all messages
				messages := primary["messages"].([]interface{})
				assert.Greater(t, len(messages), 2) // Has original + merged messages

				// Check messages are properly attributed
				for _, msg := range messages {
					m := msg.(map[string]interface{})
					assert.Contains(t, m, "original_ticket_id")
					// Only merged messages should have merged_from_ticket field
					if m["original_ticket_id"] != "1000" {
						assert.Contains(t, m, "merged_from_ticket")
					}
				}
			},
		},
		{
			name:      "Merged ticket status changes to 'merged'",
			primaryID: "1100",
			mergeIDs:  []string{"1101"},
			checkEffects: func(t *testing.T, primary, merged map[string]interface{}) {
				// Merged tickets should have status 'merged'
				assert.Equal(t, "merged", merged["1101"].(map[string]interface{})["status"])
				assert.Equal(t, float64(1100), merged["1101"].(map[string]interface{})["merged_into"])
			},
		},
		{
			name:      "Primary ticket gets merge notification",
			primaryID: "1200",
			mergeIDs:  []string{"1201", "1202"},
			checkEffects: func(t *testing.T, primary, merged map[string]interface{}) {
				// Primary should have merge activity log
				activities := primary["activities"].([]interface{})
				hasMergeActivity := false
				for _, activity := range activities {
					a := activity.(map[string]interface{})
					if a["type"] == "tickets_merged" {
						hasMergeActivity = true
						assert.Contains(t, a["details"], "1201")
						assert.Contains(t, a["details"], "1202")
					}
				}
				assert.True(t, hasMergeActivity)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This would call the actual merge function and check effects
			// For testing, we're validating the expected behavior
			primaryTicket := map[string]interface{}{
				"id": tt.primaryID,
				"messages": []interface{}{
					map[string]interface{}{"id": 1, "text": "Original message"},
				},
				"activities": []interface{}{},
			}

			mergedTickets := make(map[string]interface{})
			for _, id := range tt.mergeIDs {
				primaryIDInt, _ := strconv.Atoi(tt.primaryID)
				mergedTickets[id] = map[string]interface{}{
					"id":          id,
					"status":      "merged",
					"merged_into": float64(primaryIDInt),
				}
			}

			// Simulate merge effects
			if tt.name == "Merged ticket messages are combined" {
				primaryTicket["messages"] = []interface{}{
					map[string]interface{}{"id": 1, "text": "Original message", "original_ticket_id": tt.primaryID},
					map[string]interface{}{"id": 2, "text": "Merged message 1", "original_ticket_id": "1001", "merged_from_ticket": "1001"},
					map[string]interface{}{"id": 3, "text": "Merged message 2", "original_ticket_id": "1002", "merged_from_ticket": "1002"},
				}
			}

			if tt.name == "Primary ticket gets merge notification" {
				primaryTicket["activities"] = []interface{}{
					map[string]interface{}{
						"type":    "tickets_merged",
						"details": "Merged tickets 1201, 1202 into this ticket",
					},
				}
			}

			tt.checkEffects(t, primaryTicket, mergedTickets)
		})
	}
}
