package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestQueueDetailSimple(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if !dbAvailable() { t.Skip("DB not available (GOTRS_TEST_DB_READY!=1) - skipping queue detail HTML tests") }
	
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
		if !dbAvailable() { t.Skip("DB not available - skipping fragment variant") }
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
		assert.Contains(t, body, "Raw", "Should contain queue name")
	})
}