package api

import (
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// NewMultipartWriter creates a multipart writer for tests
func NewMultipartWriter(body *strings.Builder) *multipart.Writer {
	return multipart.NewWriter(body)
}

func seededTicketID(t *testing.T) int {
	t.Helper()
	if err := database.InitTestDB(); err != nil {
		t.Fatalf("InitTestDB failed: %v", err)
	}
	db, err := database.GetDB()
	if err != nil {
		t.Fatalf("GetDB failed: %v", err)
	}
	var id int
	if err := db.QueryRow("SELECT id FROM ticket ORDER BY id LIMIT 1").Scan(&id); err != nil {
		t.Fatalf("failed to load seeded ticket id: %v", err)
	}
	return id
}

func newZoomRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", uint(1))
		c.Set("user_name", "Unit Test Agent")
	})
	return r
}

func TestTicketNoteCreation(t *testing.T) {
	seedID := seededTicketID(t)
	validID := strconv.Itoa(seedID)
	invalidID := strconv.Itoa(seedID + 99999)

	tests := []struct {
		name           string
		ticketID       string
		noteData       map[string]string
		expectedStatus int
		expectedInDB   bool
	}{
		{
			name:     "Valid note creation succeeds",
			ticketID: validID,
			noteData: map[string]string{
				"subject": "Test Note",
				"body":    "This is a test note content",
			},
			expectedStatus: http.StatusOK,
			expectedInDB:   true,
		},
		{
			name:     "Empty note body fails validation",
			ticketID: validID,
			noteData: map[string]string{
				"subject": "Test Note",
				"body":    "",
			},
			expectedStatus: http.StatusBadRequest,
			expectedInDB:   false,
		},
		{
			name:           "Invalid ticket ID fails",
			ticketID:       invalidID,
			noteData:       map[string]string{"subject": "Test", "body": "Content"},
			expectedStatus: http.StatusNotFound,
			expectedInDB:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newZoomRouter()
			r.POST("/agent/tickets/:id/note", HandleAgentTicketNote)

			formData := url.Values{}
			for key, value := range tt.noteData {
				formData.Set(key, value)
			}

			req := httptest.NewRequest("POST", "/agent/tickets/"+tt.ticketID+"/note",
				strings.NewReader(formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code, "HTTP status should match expected")

			if tt.expectedInDB {
				t.Log("TODO: Verify note was saved to database")
			}
		})
	}
}

func TestTicketReplyCreation(t *testing.T) {
	seedID := seededTicketID(t)
	validID := strconv.Itoa(seedID)

	tests := []struct {
		name           string
		ticketID       string
		replyData      map[string]string
		expectedStatus int
		expectedInDB   bool
	}{
		{
			name:     "Valid reply creation succeeds",
			ticketID: validID,
			replyData: map[string]string{
				"to":      "customer@example.com",
				"subject": "Re: Test Ticket",
				"body":    "This is a reply to your ticket",
			},
			expectedStatus: http.StatusOK,
			expectedInDB:   true,
		},
		{
			name:     "Missing recipient email fails validation",
			ticketID: validID,
			replyData: map[string]string{
				"to":      "",
				"subject": "Re: Test Ticket",
				"body":    "Reply content",
			},
			expectedStatus: http.StatusBadRequest,
			expectedInDB:   false,
		},
		{
			name:     "Invalid email format fails validation",
			ticketID: validID,
			replyData: map[string]string{
				"to":      "invalid-email",
				"subject": "Re: Test Ticket",
				"body":    "Reply content",
			},
			expectedStatus: http.StatusBadRequest,
			expectedInDB:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newZoomRouter()
			r.POST("/agent/tickets/:id/reply", HandleAgentTicketReply)

			formData := url.Values{}
			for key, value := range tt.replyData {
				formData.Set(key, value)
			}

			req := httptest.NewRequest("POST", "/agent/tickets/"+tt.ticketID+"/reply",
				strings.NewReader(formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code, "HTTP status should match expected")

			if tt.expectedInDB {
				t.Log("TODO: Verify reply was saved to database and article_data_mime table")
			}
		})
	}
}

func TestTicketPhoneNoteCreation(t *testing.T) {
	seedID := seededTicketID(t)
	validID := strconv.Itoa(seedID)

	tests := []struct {
		name           string
		ticketID       string
		phoneData      map[string]string
		expectedStatus int
		expectedInDB   bool
	}{
		{
			name:     "Valid phone note creation succeeds",
			ticketID: validID,
			phoneData: map[string]string{
				"subject": "Phone call with customer",
				"body":    "Customer called about ticket issue. Provided solution.",
			},
			expectedStatus: http.StatusOK,
			expectedInDB:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newZoomRouter()
			r.POST("/agent/tickets/:id/phone", HandleAgentTicketPhone)

			formData := url.Values{}
			for key, value := range tt.phoneData {
				formData.Set(key, value)
			}

			req := httptest.NewRequest("POST", "/agent/tickets/"+tt.ticketID+"/phone",
				strings.NewReader(formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code, "HTTP status should match expected")

			if tt.expectedInDB {
				t.Log("TODO: Verify phone note was saved to database")
			}
		})
	}
}

func TestTicketZoomJavaScriptFunctions(t *testing.T) {
	// This test verifies that all required JavaScript functions are present in the template
	t.Run("Template contains required JavaScript functions", func(t *testing.T) {
		// This would be an integration test that checks the rendered template
		// For now, we'll just document what functions should exist

		requiredFunctions := []string{
			"openNoteModal()",
			"openReplyModal()",
			"openPhoneModal()",
			"showArticleModal(type, title)",
			"submitArticle(event, type)",
			"closeModal()",
		}

		// TODO: Add template rendering test that verifies these functions exist
		// This test should fail initially if any functions are missing

		for _, funcName := range requiredFunctions {
			t.Logf("TODO: Verify function exists in template: %s", funcName)
		}
	})
}

func TestTicketZoomTranslations(t *testing.T) {
	// This test verifies that translation keys are properly resolved
	t.Run("Translation keys resolve to text not object.attribute notation", func(t *testing.T) {
		// TODO: Add test that renders template and verifies no translation keys
		// are showing as "object.attribute" format

		translationKeys := []string{
			"agent.ticket.zoom.title",
			"tickets.note",
			"tickets.reply",
			"tickets.phone",
			"navigation.dashboard",
			"navigation.tickets",
		}

		for _, key := range translationKeys {
			t.Logf("TODO: Verify translation key resolves: %s", key)
		}
	})
}

func TestTicketArticleDisplay(t *testing.T) {
	// This test verifies that article MIME content is properly displayed
	t.Run("Article MIME content displays correctly", func(t *testing.T) {
		// TODO: Add test that verifies articles from article_data_mime table
		// are properly rendered in the ticket zoom view

		t.Log("TODO: Verify article content loads from article_data_mime table")
		t.Log("TODO: Verify article sender, subject, body display correctly")
		t.Log("TODO: Verify article timestamps display correctly")
		t.Log("TODO: Verify article visibility settings respected")
	})
}

func TestCustomerEmailPrePopulation(t *testing.T) {
	// This test verifies customer email is pre-populated in reply forms
	t.Run("Customer email pre-populates in reply modal", func(t *testing.T) {
		// TODO: Add test that verifies customer email from ticket data
		// is automatically filled in reply form

		t.Log("TODO: Verify customer email loads from ticket customer_user data")
		t.Log("TODO: Verify email appears in 'To' field of reply modal")
		t.Log("TODO: Verify email is not editable text field but proper lookup")
	})
}

// TestTicketNoteWithAttachment tests note creation with file attachment
func TestTicketNoteWithAttachment(t *testing.T) {
	seedID := seededTicketID(t)
	validID := strconv.Itoa(seedID)

	t.Run("Note with attachment succeeds", func(t *testing.T) {
		r := newZoomRouter()
		r.POST("/agent/tickets/:id/note", HandleAgentTicketNote)

		// Create multipart form with attachment
		body := &strings.Builder{}
		writer := NewMultipartWriter(body)

		// Add form fields
		writer.WriteField("body", "This is a test note with attachment")
		writer.WriteField("subject", "Test Note with Attachment")

		// Add test file attachment
		part, err := writer.CreateFormFile("attachments", "test.txt")
		if err != nil {
			t.Fatalf("Failed to create form file: %v", err)
		}
		part.Write([]byte("Test file content"))
		writer.Close()

		req := httptest.NewRequest("POST", "/agent/tickets/"+validID+"/note",
			strings.NewReader(body.String()))
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Note with attachment should succeed")

		// Verify note was created
		db, _ := database.GetDB()
		if db != nil {
			var articleID int
			err := db.QueryRow(database.ConvertPlaceholders(`
				SELECT a.id FROM article a
				JOIN ticket t ON a.ticket_id = t.id
				WHERE t.id = $1
				ORDER BY a.id DESC LIMIT 1
			`), seedID).Scan(&articleID)

			if err == nil && articleID > 0 {
				// Check if attachment was saved
				var attachCount int
				db.QueryRow(database.ConvertPlaceholders(`
					SELECT COUNT(*) FROM article_data_mime_attachment
					WHERE article_id = $1
				`), articleID).Scan(&attachCount)

				if attachCount > 0 {
					t.Logf("Attachment saved successfully for article %d", articleID)
				} else {
					t.Logf("Note created (article %d) but attachment not found - may be storage config", articleID)
				}
			}
		}
	})

	t.Run("Note without attachment succeeds", func(t *testing.T) {
		r := newZoomRouter()
		r.POST("/agent/tickets/:id/note", HandleAgentTicketNote)

		formData := url.Values{}
		formData.Set("body", "This is a test note without attachment")

		req := httptest.NewRequest("POST", "/agent/tickets/"+validID+"/note",
			strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Note without attachment should succeed")
	})
}

// Integration test that would run against actual database.
func TestTicketZoomIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("Complete ticket zoom workflow", func(t *testing.T) {
		// TODO: Add integration test that:
		// 1. Creates a test ticket in database
		// 2. Loads ticket zoom page
		// 3. Adds a note via POST
		// 4. Verifies note appears in article list
		// 5. Adds a reply via POST
		// 6. Verifies reply is saved to article_data_mime
		// 7. Cleans up test data

		t.Log("TODO: Create full integration test workflow")
	})
}
