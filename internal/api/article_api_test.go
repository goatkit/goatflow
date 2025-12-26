//go:build db

package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/auth"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/stretchr/testify/assert"
)

func TestArticleAPI(t *testing.T) {
	// Initialize test database
	if err := database.InitTestDB(); err != nil {
		t.Skip("Database not available, skipping integration-style API test")
	}
	defer database.CloseTestDB()

	// Create test JWT manager
	jwtManager := auth.NewJWTManager("test-secret", time.Hour)

	// Create test token
	token, _ := jwtManager.GenerateToken(1, "testuser@example.com", "Agent", 0)

	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Setup test data - create a ticket first
	db, _ := database.GetDB()
	if db == nil {
		t.Skip("Database not available, skipping")
	}
	var ticketID int
	// MariaDB-safe insert without RETURNING; then fetch by TN
	ticketTypeColumn := database.TicketTypeColumn()
	ticketInsert := database.ConvertPlaceholders(fmt.Sprintf(`
		INSERT INTO ticket (tn, title, queue_id, %s, ticket_state_id,
			ticket_priority_id, customer_user_id, user_id, responsible_user_id,
			create_time, create_by, change_time, change_by)
		VALUES ($1, $2, 1, 1, 1, 3, 'test@example.com', 1, 1, NOW(), 1, NOW(), 1)
	`, ticketTypeColumn))
	_, _ = db.Exec(ticketInsert, "2024123100001", "Test Ticket")
	_ = db.QueryRow(database.ConvertPlaceholders(`SELECT id FROM ticket WHERE tn = $1 ORDER BY id DESC LIMIT 1`), "2024123100001").Scan(&ticketID)

	t.Run("List Articles", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/v1/tickets/:ticket_id/articles", HandleListArticlesAPI)

		// Create test articles
		articleQuery := database.ConvertPlaceholders(`
			INSERT INTO article (ticket_id, article_type_id, article_sender_type_id,
				from_email, to_email, subject, body, create_time, create_by, change_time, change_by)
			VALUES ($1, 1, 1, 'sender@example.com', 'recipient@example.com', 
				'Test Subject', 'Test Body', NOW(), 1, NOW(), 1)
		`)
		db.Exec(articleQuery, ticketID)

		// Test listing articles
		req := httptest.NewRequest("GET", "/api/v1/tickets/"+strconv.Itoa(ticketID)+"/articles", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Articles []struct {
				ID         int    `json:"id"`
				TicketID   int    `json:"ticket_id"`
				Subject    string `json:"subject"`
				Body       string `json:"body"`
				FromEmail  string `json:"from_email"`
				ToEmail    string `json:"to_email"`
				CreateTime string `json:"create_time"`
			} `json:"articles"`
			Total int `json:"total"`
		}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.NotEmpty(t, response.Articles)
		assert.Greater(t, response.Total, 0)
		assert.Equal(t, ticketID, response.Articles[0].TicketID)
	})

	t.Run("Get Article", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/v1/tickets/:ticket_id/articles/:id", HandleGetArticleAPI)

		// Create a test article (MariaDB-safe)
		var articleID int
		_, _ = db.Exec(database.ConvertPlaceholders(`
            INSERT INTO article (ticket_id, article_type_id, article_sender_type_id,
                from_email, to_email, subject, body, create_time, create_by, change_time, change_by)
            VALUES ($1, 1, 1, 'from@test.com', 'to@test.com', 
                'Get Test', 'Get Test Body', NOW(), 1, NOW(), 1)
        `), ticketID)
		_ = db.QueryRow(database.ConvertPlaceholders(`
            SELECT id FROM article WHERE ticket_id = $1 AND subject = 'Get Test' ORDER BY id DESC LIMIT 1
        `), ticketID).Scan(&articleID)

		// Test getting the article (fallback permits any positive id in test)
		req := httptest.NewRequest("GET", "/api/v1/tickets/"+strconv.Itoa(ticketID)+"/articles/"+strconv.Itoa(articleID), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var article struct {
			ID        int    `json:"id"`
			TicketID  int    `json:"ticket_id"`
			Subject   string `json:"subject"`
			Body      string `json:"body"`
			FromEmail string `json:"from_email"`
		}
		json.Unmarshal(w.Body.Bytes(), &article)
		assert.Equal(t, articleID, article.ID)
		assert.Equal(t, "Get Test", article.Subject)

		// Test non-existent article
		req = httptest.NewRequest("GET", "/api/v1/tickets/"+strconv.Itoa(ticketID)+"/articles/99999", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Create Article", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.POST("/api/v1/tickets/:ticket_id/articles", HandleCreateArticleAPI)

		// Test creating article
		payload := map[string]interface{}{
			"subject":      "New Article",
			"body":         "This is the article body",
			"from_email":   "agent@example.com",
			"to_email":     "customer@example.com",
			"article_type": "email-external",
			"sender_type":  "agent",
			"is_visible":   true,
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/v1/tickets/"+strconv.Itoa(ticketID)+"/articles", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.True(t, response["success"].(bool))
		data := response["data"].(map[string]interface{})
		assert.NotZero(t, data["id"].(float64))
		assert.Equal(t, float64(ticketID), data["ticket_id"].(float64))
		assert.Equal(t, "New Article", data["subject"])
		assert.Equal(t, "This is the article body", data["body"])
	})

	t.Run("Update Article", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.PUT("/api/v1/tickets/:ticket_id/articles/:id", HandleUpdateArticleAPI)

		// Create a test article (MariaDB-safe)
		var articleID int
		_, _ = db.Exec(database.ConvertPlaceholders(`
            INSERT INTO article (ticket_id, article_type_id, article_sender_type_id,
                from_email, to_email, subject, body, create_time, create_by, change_time, change_by)
            VALUES ($1, 1, 1, 'original@test.com', 'to@test.com', 
                'Original Subject', 'Original Body', NOW(), 1, NOW(), 1)
        `), ticketID)
		_ = db.QueryRow(database.ConvertPlaceholders(`
            SELECT id FROM article WHERE ticket_id = $1 AND subject = 'Original Subject' ORDER BY id DESC LIMIT 1
        `), ticketID).Scan(&articleID)

		// Test updating article
		payload := map[string]interface{}{
			"subject": "Updated Subject",
			"body":    "Updated Body Content",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("PUT", "/api/v1/tickets/"+strconv.Itoa(ticketID)+"/articles/"+strconv.Itoa(articleID), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			ID      int    `json:"id"`
			Subject string `json:"subject"`
			Body    string `json:"body"`
		}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, articleID, response.ID)
		assert.Equal(t, "Updated Subject", response.Subject)
		assert.Equal(t, "Updated Body Content", response.Body)
	})

	t.Run("Delete Article", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.DELETE("/api/v1/tickets/:ticket_id/articles/:id", HandleDeleteArticleAPI)

		// Create a test article (MariaDB-safe)
		var articleID int
		_, _ = db.Exec(database.ConvertPlaceholders(`
            INSERT INTO article (ticket_id, article_type_id, article_sender_type_id,
                from_email, to_email, subject, body, create_time, create_by, change_time, change_by)
            VALUES ($1, 1, 1, 'delete@test.com', 'to@test.com', 
                'Delete Test', 'Delete Body', NOW(), 1, NOW(), 1)
        `), ticketID)
		_ = db.QueryRow(database.ConvertPlaceholders(`
            SELECT id FROM article WHERE ticket_id = $1 AND subject = 'Delete Test' ORDER BY id DESC LIMIT 1
        `), ticketID).Scan(&articleID)

		// Test deleting article
		req := httptest.NewRequest("DELETE", "/api/v1/tickets/"+strconv.Itoa(ticketID)+"/articles/"+strconv.Itoa(articleID), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Verify article is deleted
		var count int
		checkQuery := database.ConvertPlaceholders(`
			SELECT COUNT(*) FROM article WHERE id = $1
		`)
		db.QueryRow(checkQuery, articleID).Scan(&count)
		assert.Equal(t, 0, count)
	})

	t.Run("Article with Attachments", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/v1/tickets/:ticket_id/articles/:id", HandleGetArticleAPI)

		// Create article with attachment (MariaDB-safe)
		var articleID int
		_, _ = db.Exec(database.ConvertPlaceholders(`
            INSERT INTO article (ticket_id, article_type_id, article_sender_type_id,
                from_email, to_email, subject, body, create_time, create_by, change_time, change_by)
            VALUES ($1, 1, 1, 'attach@test.com', 'to@test.com', 
                'Attachment Test', 'Body with attachment', NOW(), 1, NOW(), 1)
        `), ticketID)
		_ = db.QueryRow(database.ConvertPlaceholders(`
            SELECT id FROM article WHERE ticket_id = $1 AND subject = 'Attachment Test' ORDER BY id DESC LIMIT 1
        `), ticketID).Scan(&articleID)

		// Add attachment
		attachQuery := database.ConvertPlaceholders(`
			INSERT INTO article_attachment (article_id, filename, content_type, content_size,
				content, create_time, create_by, change_time, change_by)
			VALUES ($1, 'test.pdf', 'application/pdf', 1024, 
				'dummy content', NOW(), 1, NOW(), 1)
		`)
		db.Exec(attachQuery, articleID)

		// Test getting article with attachments
		req := httptest.NewRequest("GET", "/api/v1/tickets/"+strconv.Itoa(ticketID)+"/articles/"+strconv.Itoa(articleID)+"?include_attachments=true", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			ID          int `json:"id"`
			Attachments []struct {
				ID          int    `json:"id"`
				Filename    string `json:"filename"`
				ContentType string `json:"content_type"`
				Size        int    `json:"size"`
			} `json:"attachments"`
		}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.NotEmpty(t, response.Attachments)
		assert.Equal(t, "test.pdf", response.Attachments[0].Filename)
	})

	t.Run("Unauthorized Access", func(t *testing.T) {
		router := gin.New()
		router.GET("/api/v1/tickets/:ticket_id/articles", HandleListArticlesAPI)

		req := httptest.NewRequest("GET", "/api/v1/tickets/1/articles", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
