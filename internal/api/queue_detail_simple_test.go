package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/goatkit/goatflow/internal/database"
)

func TestQueueDetailSimple(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if !dbAvailable() {
		t.Skip("DB not available (GOATFLOW_TEST_DB_READY!=1) - skipping queue detail HTML tests")
	}

	queueName := getQueueNameForTest(t, 1)

	router := gin.New()
	router.GET("/api/queues/:id", handleQueueDetail)

	// Test regular request (should return full HTML page)
	t.Run("Regular request returns full HTML", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/queues/1", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()

		// Check for HTML elements - be more flexible with the check
		assert.True(t, strings.Contains(body, "<html"), "Response should contain <html tag")
		assert.True(t, strings.Contains(body, "</html>"), "Response should contain closing </html> tag")
		assert.True(t, strings.Contains(body, "<head"), "Response should contain <head tag")
		assert.True(t, strings.Contains(body, "</head>"), "Response should contain closing </head> tag")
	})

	// Test HTMX request (should return fragment)
	t.Run("HTMX request returns fragment", func(t *testing.T) {
		if !dbAvailable() {
			t.Skip("DB not available - skipping fragment variant")
		}
		req, _ := http.NewRequest("GET", "/api/queues/1", nil)
		req.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()

		// Fragment should NOT have full HTML structure
		assert.False(t, strings.Contains(body, "<html"), "Fragment should not contain <html tag")
		assert.False(t, strings.Contains(body, "</html>"), "Fragment should not contain </html> tag")

		// But should have the queue detail content
		assert.Contains(t, body, queueName, "Should contain queue name")
	})
}

func getQueueNameForTest(t *testing.T, id int) string {
	db, err := database.GetDB()
	if err != nil {
		t.Fatalf("failed to get database connection: %v", err)
	}
	if db == nil {
		t.Fatalf("database connection is nil")
	}
	var name string
	query := database.ConvertPlaceholders("SELECT name FROM queue WHERE id = ?")
	if err := db.QueryRow(query, id).Scan(&name); err != nil {
		t.Fatalf("failed to fetch queue %d name: %v", id, err)
	}
	return name
}
