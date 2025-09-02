package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
    "time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/gotrs-io/gotrs-ce/internal/auth"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

func TestSLAAPI(t *testing.T) {
	// Initialize test database
	database.InitTestDB()
	defer database.CloseTestDB()

	// Create test JWT manager
    jwtManager := auth.NewJWTManager("test-secret", time.Hour)

	// Create test token
    token, _ := jwtManager.GenerateToken(1, "testuser@example.com", "Agent", 0)

	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	t.Run("List SLAs", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/v1/slas", HandleListSLAsAPI)

		// Create test SLAs
		db, _ := database.GetDB()
		slaQuery := database.ConvertPlaceholders(`
			INSERT INTO sla (name, calendar_id, first_response_time, first_response_notify,
				update_time, update_notify, solution_time, solution_notify,
				valid_id, create_time, create_by, change_time, change_by)
			VALUES 
				($1, 1, 60, 50, 120, 100, 480, 400, 1, NOW(), 1, NOW(), 1),
				($2, 1, 30, 25, 60, 50, 240, 200, 1, NOW(), 1, NOW(), 1)
		`)
		db.Exec(slaQuery, "Premium SLA", "Standard SLA")

		// Test without filter
		req := httptest.NewRequest("GET", "/api/v1/slas", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			SLAs []struct {
				ID                 int    `json:"id"`
				Name               string `json:"name"`
				FirstResponseTime  int    `json:"first_response_time"`
				UpdateTime         int    `json:"update_time"`
				SolutionTime       int    `json:"solution_time"`
				ValidID            int    `json:"valid_id"`
			} `json:"slas"`
			Total int `json:"total"`
		}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.NotEmpty(t, response.SLAs)
		assert.Greater(t, response.Total, 0)

		// Test with valid filter
		req = httptest.NewRequest("GET", "/api/v1/slas?valid=true", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		json.Unmarshal(w.Body.Bytes(), &response)
		for _, sla := range response.SLAs {
			assert.Equal(t, 1, sla.ValidID)
		}
	})

	t.Run("Get SLA", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/v1/slas/:id", HandleGetSLAAPI)

		// Create a test SLA first
		db, _ := database.GetDB()
		var slaID int
		query := database.ConvertPlaceholders(`
			INSERT INTO sla (name, calendar_id, first_response_time, first_response_notify,
				update_time, update_notify, solution_time, solution_notify,
				valid_id, create_time, create_by, change_time, change_by)
			VALUES ($1, 1, 120, 100, 240, 200, 960, 800, 1, NOW(), 1, NOW(), 1)
			RETURNING id
		`)
		db.QueryRow(query, "Test SLA").Scan(&slaID)

		// Test getting the SLA
		req := httptest.NewRequest("GET", "/api/v1/slas/"+strconv.Itoa(slaID), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var sla struct {
			ID                int    `json:"id"`
			Name              string `json:"name"`
			FirstResponseTime int    `json:"first_response_time"`
			UpdateTime        int    `json:"update_time"`
			SolutionTime      int    `json:"solution_time"`
			ValidID           int    `json:"valid_id"`
		}
		json.Unmarshal(w.Body.Bytes(), &sla)
		assert.Equal(t, slaID, sla.ID)
		assert.Equal(t, "Test SLA", sla.Name)
		assert.Equal(t, 120, sla.FirstResponseTime)
		assert.Equal(t, 960, sla.SolutionTime)

		// Test non-existent SLA
		req = httptest.NewRequest("GET", "/api/v1/slas/99999", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Create SLA", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.POST("/api/v1/slas", HandleCreateSLAAPI)

		// Test creating SLA
		payload := map[string]interface{}{
			"name":                  "New SLA",
			"calendar_id":           1,
			"first_response_time":   90,
			"first_response_notify": 75,
			"update_time":           180,
			"update_notify":         150,
			"solution_time":         720,
			"solution_notify":       600,
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/v1/slas", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response struct {
			ID                int    `json:"id"`
			Name              string `json:"name"`
			FirstResponseTime int    `json:"first_response_time"`
			SolutionTime      int    `json:"solution_time"`
			ValidID           int    `json:"valid_id"`
		}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.NotZero(t, response.ID)
		assert.Equal(t, "New SLA", response.Name)
		assert.Equal(t, 90, response.FirstResponseTime)
		assert.Equal(t, 720, response.SolutionTime)
		assert.Equal(t, 1, response.ValidID)

		// Test duplicate name
		req = httptest.NewRequest("POST", "/api/v1/slas", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("Update SLA", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.PUT("/api/v1/slas/:id", HandleUpdateSLAAPI)

		// Create a test SLA
		db, _ := database.GetDB()
		var slaID int
		query := database.ConvertPlaceholders(`
			INSERT INTO sla (name, calendar_id, first_response_time, first_response_notify,
				update_time, update_notify, solution_time, solution_notify,
				valid_id, create_time, create_by, change_time, change_by)
			VALUES ($1, 1, 60, 50, 120, 100, 480, 400, 1, NOW(), 1, NOW(), 1)
			RETURNING id
		`)
		db.QueryRow(query, "Update Test SLA").Scan(&slaID)

		// Test updating SLA
		payload := map[string]interface{}{
			"name":               "Updated SLA Name",
			"first_response_time": 45,
			"solution_time":      360,
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("PUT", "/api/v1/slas/"+strconv.Itoa(slaID), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			ID                int    `json:"id"`
			Name              string `json:"name"`
			FirstResponseTime int    `json:"first_response_time"`
			SolutionTime      int    `json:"solution_time"`
		}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, slaID, response.ID)
		assert.Equal(t, "Updated SLA Name", response.Name)
		assert.Equal(t, 45, response.FirstResponseTime)
		assert.Equal(t, 360, response.SolutionTime)

		// Test updating non-existent SLA
		req = httptest.NewRequest("PUT", "/api/v1/slas/99999", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Delete SLA", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.DELETE("/api/v1/slas/:id", HandleDeleteSLAAPI)

		// Create a test SLA
		db, _ := database.GetDB()
		var slaID int
		query := database.ConvertPlaceholders(`
			INSERT INTO sla (name, calendar_id, first_response_time, first_response_notify,
				update_time, update_notify, solution_time, solution_notify,
				valid_id, create_time, create_by, change_time, change_by)
			VALUES ($1, 1, 60, 50, 120, 100, 480, 400, 1, NOW(), 1, NOW(), 1)
			RETURNING id
		`)
		db.QueryRow(query, "Delete Test SLA").Scan(&slaID)

		// Test soft deleting SLA
		req := httptest.NewRequest("DELETE", "/api/v1/slas/"+strconv.Itoa(slaID), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Verify soft delete
		var validID int
		checkQuery := database.ConvertPlaceholders(`
			SELECT valid_id FROM sla WHERE id = $1
		`)
		db.QueryRow(checkQuery, slaID).Scan(&validID)
		assert.Equal(t, 2, validID)

		// Test deleting non-existent SLA
		req = httptest.NewRequest("DELETE", "/api/v1/slas/99999", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("SLA Performance Metrics", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/v1/slas/:id/metrics", HandleSLAMetricsAPI)

		// Create test SLA
		db, _ := database.GetDB()
		var slaID int
		query := database.ConvertPlaceholders(`
			INSERT INTO sla (name, calendar_id, first_response_time, first_response_notify,
				update_time, update_notify, solution_time, solution_notify,
				valid_id, create_time, create_by, change_time, change_by)
			VALUES ($1, 1, 60, 50, 120, 100, 480, 400, 1, NOW(), 1, NOW(), 1)
			RETURNING id
		`)
		db.QueryRow(query, "Metrics Test SLA").Scan(&slaID)

		req := httptest.NewRequest("GET", "/api/v1/slas/"+strconv.Itoa(slaID)+"/metrics", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			SLAID   int `json:"sla_id"`
			Metrics struct {
				TotalTickets      int     `json:"total_tickets"`
				MetFirstResponse  int     `json:"met_first_response"`
				MetSolution       int     `json:"met_solution"`
				BreachedTickets   int     `json:"breached_tickets"`
				CompliancePercent float64 `json:"compliance_percent"`
			} `json:"metrics"`
		}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, slaID, response.SLAID)
		assert.NotNil(t, response.Metrics)
	})

	t.Run("Unauthorized Access", func(t *testing.T) {
		router := gin.New()
		router.GET("/api/v1/slas", HandleListSLAsAPI)

		req := httptest.NewRequest("GET", "/api/v1/slas", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}