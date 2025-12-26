//go:build db

package api

import (
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestArticleParamFallbacks(t *testing.T) {
	gin.SetMode(gin.TestMode)

	if err := database.InitTestDB(); err != nil {
		t.Skip("Database not available; skipping param fallback test")
	}

	// List articles accepts :ticket_id
	r1 := gin.New()
	r1.Use(func(c *gin.Context) { c.Set("user_id", 1); c.Next() })
	r1.GET("/api/v1/tickets/:ticket_id/articles", HandleListArticlesAPI)

	w := httptest.NewRecorder()
	r1.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/tickets/123/articles", nil))
	assert.NotEqual(t, http.StatusBadRequest, w.Code)

	// List articles accepts :id (fallback)
	r2 := gin.New()
	r2.Use(func(c *gin.Context) { c.Set("user_id", 1); c.Next() })
	r2.GET("/api/v1/tickets/:id/articles", HandleListArticlesAPI)

	w = httptest.NewRecorder()
	r2.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/tickets/456/articles", nil))
	assert.NotEqual(t, http.StatusBadRequest, w.Code)

	// Get article accepts :article_id
	r3 := gin.New()
	r3.Use(func(c *gin.Context) { c.Set("user_id", 1); c.Next() })
	r3.GET("/api/v1/tickets/:ticket_id/articles/:article_id", HandleGetArticleAPI)

	w2 := httptest.NewRecorder()
	r3.ServeHTTP(w2, httptest.NewRequest("GET", "/api/v1/tickets/10/articles/20", nil))
	assert.NotEqual(t, http.StatusBadRequest, w2.Code)

	// Get article accepts :id for article
	r4 := gin.New()
	r4.Use(func(c *gin.Context) { c.Set("user_id", 1); c.Next() })
	r4.GET("/api/v1/tickets/:id/articles/:id", HandleGetArticleAPI)

	w2 = httptest.NewRecorder()
	r4.ServeHTTP(w2, httptest.NewRequest("GET", "/api/v1/tickets/10/articles/21", nil))
	assert.NotEqual(t, http.StatusBadRequest, w2.Code)
}
