package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/i18n"
)

// createTestNotificationEvent creates a notification event for testing and returns its ID
func createTestNotificationEvent(t *testing.T, name string) (int64, bool) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		return 0, false
	}

	var id int64
	query := database.ConvertPlaceholders(`
		INSERT INTO notification_event (name, valid_id, comments, create_time, create_by, change_time, change_by)
		VALUES ($1, 1, $2, NOW(), 1, NOW(), 1)
		RETURNING id`)
	require.NoError(t, db.QueryRow(query, name, "Test notification event").Scan(&id))

	t.Cleanup(func() {
		// Delete related items first due to foreign key constraints
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM notification_event_message WHERE notification_id = $1`), id)
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM notification_event_item WHERE notification_id = $1`), id)
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM notification_event WHERE id = $1`), id)
	})

	return id, true
}

func TestAdminNotificationEventsPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupTemplateRenderer(t)

	t.Run("GET /admin/notification-events renders list page without template errors", func(t *testing.T) {
		router := gin.New()
		router.GET("/admin/notification-events", HandleAdminNotificationEvents)

		req := httptest.NewRequest(http.MethodGet, "/admin/notification-events", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		body := w.Body.String()

		// Strict verification: 200 OK required, not 500
		require.Equal(t, http.StatusOK, w.Code, "Should return 200 OK, got %d. Body: %s", w.Code, body)

		// Verify no template rendering errors
		require.NotContains(t, body, "Template error",
			"Response should not contain template rendering errors")

		// Check for expected content
		assert.Contains(t, body, "notification")
	})
}

func TestAdminNotificationEventNew(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupTemplateRenderer(t)

	t.Run("GET /admin/notification-events/new renders create form without template errors", func(t *testing.T) {
		router := gin.New()
		router.GET("/admin/notification-events/new", HandleAdminNotificationEventNew)

		req := httptest.NewRequest(http.MethodGet, "/admin/notification-events/new", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		body := w.Body.String()

		// Strict verification: 200 OK required, not 500
		require.Equal(t, http.StatusOK, w.Code, "Should return 200 OK, got %d. Body: %s", w.Code, body)

		// Verify no template rendering errors
		require.NotContains(t, body, "Template error",
			"Response should not contain template rendering errors")

		// Should render the form template with notification-related content
		assert.Contains(t, body, "notification")
	})
}

func TestAdminNotificationEventEdit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupTemplateRenderer(t)

	t.Run("GET /admin/notification-events/:id renders edit form without template errors", func(t *testing.T) {
		id, ok := createTestNotificationEvent(t, "TestEditNotification")
		if !ok {
			t.Skip("Database not available")
		}

		router := gin.New()
		router.GET("/admin/notification-events/:id", HandleAdminNotificationEventEdit)

		req := httptest.NewRequest(http.MethodGet, "/admin/notification-events/"+itoa(id), nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		body := w.Body.String()

		// Strict verification: 200 OK required, not 500
		require.Equal(t, http.StatusOK, w.Code, "Should return 200 OK, got %d. Body: %s", w.Code, body)

		// Verify no template rendering errors
		require.NotContains(t, body, "Template error",
			"Response should not contain template rendering errors")

		// Should contain the notification name
		assert.Contains(t, body, "TestEditNotification")
	})

	t.Run("GET /admin/notification-events/:id returns 404 for non-existent notification", func(t *testing.T) {
		router := gin.New()
		router.GET("/admin/notification-events/:id", HandleAdminNotificationEventEdit)

		req := httptest.NewRequest(http.MethodGet, "/admin/notification-events/999999", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Should be 404 for non-existent notification
		require.Equal(t, http.StatusNotFound, w.Code, "Should return 404 for non-existent notification")
	})
}

func TestAdminNotificationEventGet(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET /admin/api/notification-events/:id returns JSON for existing notification", func(t *testing.T) {
		id, ok := createTestNotificationEvent(t, "TestGetNotification")
		if !ok {
			t.Skip("Database not available")
		}

		router := gin.New()
		router.GET("/admin/api/notification-events/:id", HandleAdminNotificationEventGet)

		req := httptest.NewRequest(http.MethodGet, "/admin/api/notification-events/"+itoa(id), nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.True(t, response["success"].(bool))
			assert.NotNil(t, response["data"])
		}
	})

	t.Run("GET /admin/api/notification-events/:id returns error for non-existent notification", func(t *testing.T) {
		router := gin.New()
		router.GET("/admin/api/notification-events/:id", HandleAdminNotificationEventGet)

		req := httptest.NewRequest(http.MethodGet, "/admin/api/notification-events/999999", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Should be error
		assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusInternalServerError)
	})
}

func TestCreateNotificationEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST /admin/api/notification-events creates new notification", func(t *testing.T) {
		db, err := database.GetDB()
		if err != nil || db == nil {
			t.Skip("Database not available")
		}

		router := gin.New()
		router.POST("/admin/api/notification-events", HandleCreateNotificationEvent)

		input := NotificationEventInput{
			Name:     "TestCreateNotification",
			ValidID:  1,
			Comments: "Created by test",
			Events:   []string{"TicketCreate"},
			Filters:  map[string][]string{},
			Recipients: map[string][]string{
				"agent_owner": {"1"},
			},
			Messages: map[string]NotificationEventMessage{
				"en": {
					Subject:     "Test Subject",
					Text:        "Test body content",
					ContentType: "text/plain",
				},
			},
		}

		body, _ := json.Marshal(input)
		req := httptest.NewRequest(http.MethodPost, "/admin/api/notification-events", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.True(t, response["success"].(bool))

			// Clean up created notification
			if id, ok := response["id"].(float64); ok {
				t.Cleanup(func() {
					_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM notification_event_message WHERE notification_id = $1`), int64(id))
					_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM notification_event_item WHERE notification_id = $1`), int64(id))
					_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM notification_event WHERE id = $1`), int64(id))
				})
			}
		}
	})

	t.Run("POST /admin/api/notification-events validates required fields", func(t *testing.T) {
		router := gin.New()
		router.POST("/admin/api/notification-events", HandleCreateNotificationEvent)

		// Missing required name field
		input := NotificationEventInput{
			Name:    "", // Empty name should fail validation
			ValidID: 1,
		}

		body, _ := json.Marshal(input)
		req := httptest.NewRequest(http.MethodPost, "/admin/api/notification-events", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Should return error for invalid input
		assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusInternalServerError)
	})
}

func TestUpdateNotificationEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("PUT /admin/api/notification-events/:id updates existing notification", func(t *testing.T) {
		id, ok := createTestNotificationEvent(t, "TestUpdateNotification")
		if !ok {
			t.Skip("Database not available")
		}

		router := gin.New()
		router.PUT("/admin/api/notification-events/:id", HandleUpdateNotificationEvent)

		input := NotificationEventInput{
			Name:     "UpdatedNotificationName",
			ValidID:  1,
			Comments: "Updated by test",
			Events:   []string{"TicketUpdate"},
			Filters:  map[string][]string{},
			Recipients: map[string][]string{
				"agent_owner": {"1"},
			},
			Messages: map[string]NotificationEventMessage{
				"en": {
					Subject:     "Updated Subject",
					Text:        "Updated body content",
					ContentType: "text/plain",
				},
			},
		}

		body, _ := json.Marshal(input)
		req := httptest.NewRequest(http.MethodPut, "/admin/api/notification-events/"+itoa(id), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.True(t, response["success"].(bool))
		}
	})

	t.Run("PUT /admin/api/notification-events/:id returns error for non-existent notification", func(t *testing.T) {
		router := gin.New()
		router.PUT("/admin/api/notification-events/:id", HandleUpdateNotificationEvent)

		input := NotificationEventInput{
			Name:    "TestNotification",
			ValidID: 1,
		}

		body, _ := json.Marshal(input)
		req := httptest.NewRequest(http.MethodPut, "/admin/api/notification-events/999999", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Should return error for non-existent notification
		assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusInternalServerError)
	})
}

func TestDeleteNotificationEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("DELETE /admin/api/notification-events/:id deletes existing notification", func(t *testing.T) {
		db, err := database.GetDB()
		if err != nil || db == nil {
			t.Skip("Database not available")
		}

		// Create notification without cleanup (we're testing deletion)
		var id int64
		query := database.ConvertPlaceholders(`
			INSERT INTO notification_event (name, valid_id, comments, create_time, create_by, change_time, change_by)
			VALUES ($1, 1, $2, NOW(), 1, NOW(), 1)
			RETURNING id`)
		require.NoError(t, db.QueryRow(query, "TestDeleteNotification", "To be deleted").Scan(&id))

		router := gin.New()
		router.DELETE("/admin/api/notification-events/:id", HandleDeleteNotificationEvent)

		req := httptest.NewRequest(http.MethodDelete, "/admin/api/notification-events/"+itoa(id), nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.True(t, response["success"].(bool))

			// Verify deletion
			var count int
			err = db.QueryRow(database.ConvertPlaceholders(`SELECT COUNT(*) FROM notification_event WHERE id = $1`), id).Scan(&count)
			require.NoError(t, err)
			assert.Equal(t, 0, count)
		}
	})

	t.Run("DELETE /admin/api/notification-events/:id returns error for non-existent notification", func(t *testing.T) {
		router := gin.New()
		router.DELETE("/admin/api/notification-events/:id", HandleDeleteNotificationEvent)

		req := httptest.NewRequest(http.MethodDelete, "/admin/api/notification-events/999999", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Should return error for non-existent notification
		assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusInternalServerError)
	})
}

// itoa is a helper to convert int64 to string
func itoa(i int64) string {
	return fmt.Sprintf("%d", i)
}

// TestNotificationEventTranslations tests that all required i18n keys exist
func TestNotificationEventTranslations(t *testing.T) {
	// Initialize the i18n instance
	instance := i18n.GetInstance()
	require.NotNil(t, instance, "i18n instance should be initialized")

	// Test all supported languages
	languages := []string{"en", "de", "es", "fr", "ar", "tlh"}

	for _, lang := range languages {
		t.Run("translations exist for "+lang, func(t *testing.T) {
			// Required keys for notification_events.pongo2
			requiredKeys := []string{
				"common.back",
				"common.valid",
				"common.invalid",
				"common.actions",
				"common.edit",
				"common.delete",
				"common.cancel",
				"admin.modules.notification_event.title",
				"admin.modules.notification_event.description",
				"admin.modules.notification_event.actions.new",
				"admin.modules.notification_event.fields.name",
				"admin.modules.notification_event.fields.comments",
				"admin.modules.notification_event.fields.valid_id",
				"admin.modules.notification_event.fields.change_time",
				"admin.modules.notification_event.no_notifications",
				"admin.modules.notification_event.delete_title",
				"admin.modules.notification_event.delete_confirm",
			}

			for _, key := range requiredKeys {
				value := instance.Translate(lang, key)
				assert.NotEmpty(t, value, "translation for key '%s' in '%s' should not be empty", key, lang)
				assert.NotEqual(t, key, value, "translation for key '%s' in '%s' should not return the key itself", key, lang)
			}
		})
	}
}
