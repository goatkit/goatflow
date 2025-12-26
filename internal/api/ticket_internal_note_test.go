//go:build db

package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test-Driven Development for Internal Notes Feature
// Internal notes are agent-only communications not visible to customers

func TestCreateInternalNote(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		ticketID   string
		payload    map[string]interface{}
		userRole   string
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name:     "Agent creates internal note",
			ticketID: "1",
			payload: map[string]interface{}{
				"content":    "Customer has VIP status - expedite resolution",
				"visibility": "internal",
			},
			userRole:   "agent",
			wantStatus: http.StatusCreated,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "Internal note added successfully", resp["message"])
				assert.Contains(t, resp, "note_id")
				note := resp["note"].(map[string]interface{})
				assert.Equal(t, "internal", note["visibility"])
				assert.Equal(t, false, note["customer_visible"])
			},
		},
		{
			name:     "Admin creates internal note with mention",
			ticketID: "1",
			payload: map[string]interface{}{
				"content":    "@john.doe please review this customer's contract",
				"visibility": "internal",
				"mentions":   []string{"john.doe"},
			},
			userRole:   "admin",
			wantStatus: http.StatusCreated,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				note := resp["note"].(map[string]interface{})
				mentions := note["mentions"].([]interface{})
				assert.Contains(t, mentions, "john.doe")
				assert.Equal(t, true, note["has_mentions"])
			},
		},
		{
			name:     "Create note with attachments",
			ticketID: "1",
			payload: map[string]interface{}{
				"content":        "See attached internal documentation",
				"visibility":     "internal",
				"attachment_ids": []int{1, 2},
			},
			userRole:   "agent",
			wantStatus: http.StatusCreated,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				note := resp["note"].(map[string]interface{})
				attachments := note["attachments"].([]interface{})
				assert.Len(t, attachments, 2)
			},
		},
		{
			name:     "Create note with priority flag",
			ticketID: "1",
			payload: map[string]interface{}{
				"content":     "URGENT: Security issue detected",
				"visibility":  "internal",
				"is_priority": true,
				"category":    "security",
			},
			userRole:   "agent",
			wantStatus: http.StatusCreated,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				note := resp["note"].(map[string]interface{})
				assert.Equal(t, true, note["is_priority"])
				assert.Equal(t, "security", note["category"])
			},
		},
		{
			name:     "Customer cannot create internal notes",
			ticketID: "1",
			payload: map[string]interface{}{
				"content":    "Trying to add internal note",
				"visibility": "internal",
			},
			userRole:   "customer",
			wantStatus: http.StatusForbidden,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Only agents can create internal notes")
			},
		},
		{
			name:     "Missing content",
			ticketID: "1",
			payload: map[string]interface{}{
				"visibility": "internal",
			},
			userRole:   "agent",
			wantStatus: http.StatusBadRequest,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Content is required")
			},
		},
		{
			name:     "Invalid ticket ID",
			ticketID: "99999",
			payload: map[string]interface{}{
				"content":    "Note for non-existent ticket",
				"visibility": "internal",
			},
			userRole:   "agent",
			wantStatus: http.StatusNotFound,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Ticket not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", 1)
				c.Set("user_role", tt.userRole)
				c.Set("user_name", "Test User")
				c.Next()
			})
			router.POST("/api/tickets/:id/internal-notes", HandleCreateInternalNote)

			body, _ := json.Marshal(tt.payload)
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", fmt.Sprintf("/api/tickets/%s/internal-notes", tt.ticketID), bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
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

func TestGetInternalNotes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		ticketID   string
		query      string
		userRole   string
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name:       "Agent gets all internal notes",
			ticketID:   "1",
			query:      "",
			userRole:   "agent",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				notes := resp["notes"].([]interface{})
				assert.Greater(t, len(notes), 0)
				for _, n := range notes {
					note := n.(map[string]interface{})
					assert.Equal(t, "internal", note["visibility"])
				}
			},
		},
		{
			name:       "Filter by category",
			ticketID:   "1",
			query:      "?category=technical",
			userRole:   "agent",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				notes := resp["notes"].([]interface{})
				for _, n := range notes {
					note := n.(map[string]interface{})
					assert.Equal(t, "technical", note["category"])
				}
			},
		},
		{
			name:       "Filter priority notes only",
			ticketID:   "1",
			query:      "?priority=true",
			userRole:   "agent",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				notes := resp["notes"].([]interface{})
				for _, n := range notes {
					note := n.(map[string]interface{})
					assert.Equal(t, true, note["is_priority"])
				}
			},
		},
		{
			name:       "Search in notes",
			ticketID:   "1",
			query:      "?search=security",
			userRole:   "agent",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				notes := resp["notes"].([]interface{})
				for _, n := range notes {
					note := n.(map[string]interface{})
					content := strings.ToLower(note["content"].(string))
					assert.Contains(t, content, "security")
				}
			},
		},
		{
			name:       "Customer cannot see internal notes",
			ticketID:   "1",
			query:      "",
			userRole:   "customer",
			wantStatus: http.StatusForbidden,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "You don't have permission to view internal notes")
			},
		},
		{
			name:       "Get notes with mentions",
			ticketID:   "1",
			query:      "?has_mentions=true",
			userRole:   "agent",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				notes := resp["notes"].([]interface{})
				for _, n := range notes {
					note := n.(map[string]interface{})
					assert.Equal(t, true, note["has_mentions"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", 1)
				c.Set("user_role", tt.userRole)
				c.Next()
			})
			router.GET("/api/tickets/:id/internal-notes", HandleGetInternalNotes)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", fmt.Sprintf("/api/tickets/%s/internal-notes%s", tt.ticketID, tt.query), nil)
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

func TestUpdateInternalNote(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		ticketID   string
		noteID     string
		payload    map[string]interface{}
		userRole   string
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name:     "Update note content",
			ticketID: "1",
			noteID:   "1",
			payload: map[string]interface{}{
				"content": "Updated: Customer issue resolved via workaround",
			},
			userRole:   "agent",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "Internal note updated successfully", resp["message"])
				note := resp["note"].(map[string]interface{})
				assert.Contains(t, note["content"], "Updated:")
				assert.Equal(t, true, note["is_edited"])
			},
		},
		{
			name:     "Add priority flag",
			ticketID: "1",
			noteID:   "2",
			payload: map[string]interface{}{
				"is_priority": true,
				"category":    "escalation",
			},
			userRole:   "agent",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				note := resp["note"].(map[string]interface{})
				assert.Equal(t, true, note["is_priority"])
				assert.Equal(t, "escalation", note["category"])
			},
		},
		{
			name:     "Cannot update other's note",
			ticketID: "1",
			noteID:   "3", // Note created by different user
			payload: map[string]interface{}{
				"content": "Trying to update",
			},
			userRole:   "agent",
			wantStatus: http.StatusForbidden,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "You can only edit your own notes")
			},
		},
		{
			name:     "Admin can update any note",
			ticketID: "1",
			noteID:   "3",
			payload: map[string]interface{}{
				"content": "Admin update",
			},
			userRole:   "admin",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "Internal note updated successfully", resp["message"])
			},
		},
		{
			name:     "Note not found",
			ticketID: "1",
			noteID:   "99999",
			payload: map[string]interface{}{
				"content": "Update",
			},
			userRole:   "agent",
			wantStatus: http.StatusNotFound,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Internal note not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", 1)
				c.Set("user_role", tt.userRole)
				c.Next()
			})
			router.PUT("/api/tickets/:id/internal-notes/:note_id", HandleUpdateInternalNote)

			body, _ := json.Marshal(tt.payload)
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("PUT", fmt.Sprintf("/api/tickets/%s/internal-notes/%s", tt.ticketID, tt.noteID), bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
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

func TestDeleteInternalNote(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		ticketID   string
		noteID     string
		userRole   string
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name:       "Delete own note",
			ticketID:   "1",
			noteID:     "1",
			userRole:   "agent",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "Internal note deleted successfully", resp["message"])
			},
		},
		{
			name:       "Cannot delete other's note",
			ticketID:   "1",
			noteID:     "3",
			userRole:   "agent",
			wantStatus: http.StatusForbidden,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "You can only delete your own notes")
			},
		},
		{
			name:       "Admin can delete any note",
			ticketID:   "1",
			noteID:     "3",
			userRole:   "admin",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "Internal note deleted successfully", resp["message"])
			},
		},
		{
			name:       "Note not found",
			ticketID:   "1",
			noteID:     "99999",
			userRole:   "admin",
			wantStatus: http.StatusNotFound,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Internal note not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", 1)
				c.Set("user_role", tt.userRole)
				c.Next()
			})
			router.DELETE("/api/tickets/:id/internal-notes/:note_id", HandleDeleteInternalNote)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/tickets/%s/internal-notes/%s", tt.ticketID, tt.noteID), nil)
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

func TestInternalNoteMentions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		payload    map[string]interface{}
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name: "Auto-detect mentions in content",
			payload: map[string]interface{}{
				"content":    "Hey @john.doe and @jane.smith, please check this",
				"visibility": "internal",
			},
			wantStatus: http.StatusCreated,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				note := resp["note"].(map[string]interface{})
				mentions := note["mentions"].([]interface{})
				assert.Len(t, mentions, 2)
				assert.Contains(t, mentions, "john.doe")
				assert.Contains(t, mentions, "jane.smith")
			},
		},
		{
			name: "Handle team mentions",
			payload: map[string]interface{}{
				"content":    "@team-support please review",
				"visibility": "internal",
			},
			wantStatus: http.StatusCreated,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				note := resp["note"].(map[string]interface{})
				mentions := note["mentions"].([]interface{})
				assert.Contains(t, mentions, "team-support")
				assert.Equal(t, true, note["has_team_mention"])
			},
		},
		{
			name: "Notify mentioned users",
			payload: map[string]interface{}{
				"content":      "@john.doe urgent",
				"visibility":   "internal",
				"notify_users": true,
			},
			wantStatus: http.StatusCreated,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp, "notifications_sent")
				notifications := resp["notifications_sent"].([]interface{})
				assert.Len(t, notifications, 1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", 1)
				c.Set("user_role", "agent")
				c.Set("user_name", "Test User")
				c.Next()
			})
			router.POST("/api/tickets/:id/internal-notes", HandleCreateInternalNote)

			body, _ := json.Marshal(tt.payload)
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/tickets/1/internal-notes", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
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

func TestInternalNoteHistory(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Track edit history", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Set("user_role", "agent")
			c.Next()
		})
		router.GET("/api/tickets/:id/internal-notes/:note_id/history", handleGetInternalNoteHistory)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/tickets/1/internal-notes/1/history", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		history := response["history"].([]interface{})
		assert.Greater(t, len(history), 0)

		for _, h := range history {
			entry := h.(map[string]interface{})
			assert.Contains(t, entry, "content")
			assert.Contains(t, entry, "edited_by")
			assert.Contains(t, entry, "edited_at")
			assert.Contains(t, entry, "version")
		}
	})
}

func TestInternalNoteStatistics(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Get note statistics", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Set("user_role", "admin")
			c.Next()
		})
		router.GET("/api/tickets/:id/internal-notes/stats", handleGetInternalNoteStats)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/tickets/1/internal-notes/stats", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		stats := response["statistics"].(map[string]interface{})
		assert.Contains(t, stats, "total_notes")
		assert.Contains(t, stats, "priority_notes")
		assert.Contains(t, stats, "notes_with_mentions")
		assert.Contains(t, stats, "notes_by_category")
		assert.Contains(t, stats, "notes_by_author")
		assert.Contains(t, stats, "average_note_length")
	})
}

func TestInternalNoteExport(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		format     string
		wantStatus int
		checkResp  func(t *testing.T, w *httptest.ResponseRecorder)
	}{
		{
			name:       "Export as JSON",
			format:     "json",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
				assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
			},
		},
		{
			name:       "Export as CSV",
			format:     "csv",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Contains(t, w.Header().Get("Content-Type"), "text/csv")
				assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
			},
		},
		{
			name:       "Export as PDF",
			format:     "pdf",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Contains(t, w.Header().Get("Content-Type"), "application/pdf")
				assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", 1)
				c.Set("user_role", "agent")
				c.Set("user_name", "Test User")
				c.Next()
			})
			router.GET("/api/tickets/:id/internal-notes/export", handleExportInternalNotes)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", fmt.Sprintf("/api/tickets/1/internal-notes/export?format=%s", tt.format), nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			if tt.checkResp != nil {
				tt.checkResp(t, w)
			}
		})
	}
}

func TestInternalNoteTemplates(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Use note template", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Set("user_role", "agent")
			c.Next()
		})
		router.POST("/api/tickets/:id/internal-notes/from-template", handleCreateNoteFromTemplate)

		payload := map[string]interface{}{
			"template_id": 1,
			"variables": map[string]string{
				"issue_type": "Performance",
				"root_cause": "Database deadlock",
			},
		}

		body, _ := json.Marshal(payload)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/tickets/1/internal-notes/from-template", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		note := response["note"].(map[string]interface{})
		assert.Contains(t, note["content"], "Performance")
		assert.Contains(t, note["content"], "Database deadlock")
		assert.Equal(t, "internal", note["visibility"])
	})
}
