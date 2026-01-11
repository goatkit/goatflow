package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/auth"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

func TestSLAAPI(t *testing.T) {
	// Initialize test database; skip if unavailable
	if err := database.InitTestDB(); err != nil {
		t.Skip("Database not available, skipping SLA API tests")
	}
	defer database.CloseTestDB()

	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skip("Database not available, skipping SLA API tests")
	}

	ensureSLATestSchema(t, db)

	// Create test JWT manager
	jwtManager := auth.NewJWTManager("test-secret", time.Hour)

	// Create test token
	token, _ := jwtManager.GenerateToken(1, "testuser@example.com", "Agent", 0)

	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	t.Run("List SLAs", func(t *testing.T) {
		resetSLATestData(t, db)

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/v1/slas", HandleListSLAsAPI)

		// Create test SLAs
		slaQuery := database.ConvertPlaceholders(`
			INSERT INTO sla (name, calendar_name, first_response_time, first_response_notify,
				update_time, update_notify, solution_time, solution_notify,
				valid_id, create_time, create_by, change_time, change_by)
			VALUES 
				(?, NULL, 60, 50, 120, 100, 480, 400, 1, NOW(), 1, NOW(), 1),
				(?, NULL, 30, 25, 60, 50, 240, 200, 1, NOW(), 1, NOW(), 1)
		`)
		_, err := db.Exec(slaQuery, "Premium SLA", "Standard SLA")
		require.NoError(t, err)

		// Test without filter
		req := httptest.NewRequest("GET", "/api/v1/slas", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			SLAs []struct {
				ID                int    `json:"id"`
				Name              string `json:"name"`
				FirstResponseTime int    `json:"first_response_time"`
				UpdateTime        int    `json:"update_time"`
				SolutionTime      int    `json:"solution_time"`
				ValidID           int    `json:"valid_id"`
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
		resetSLATestData(t, db)

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/v1/slas/:id", HandleGetSLAAPI)

		// Create a test SLA first
		slaID := insertTestSLA(t, db, insertSLAParams{
			Name:                "Test SLA",
			FirstResponseTime:   120,
			FirstResponseNotify: 100,
			UpdateTime:          240,
			UpdateNotify:        200,
			SolutionTime:        960,
			SolutionNotify:      800,
		})
		t.Logf("inserted SLA id=%d", slaID)

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
		resetSLATestData(t, db)

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.POST("/api/v1/slas", HandleCreateSLAAPI)

		// Test creating SLA
		payload := map[string]interface{}{
			"name":                  "New SLA",
			"calendar_name":         "Default",
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
		resetSLATestData(t, db)

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.PUT("/api/v1/slas/:id", HandleUpdateSLAAPI)

		// Create a test SLA
		slaID := insertTestSLA(t, db, insertSLAParams{
			Name:                "Update Test SLA",
			FirstResponseTime:   60,
			FirstResponseNotify: 50,
			UpdateTime:          120,
			UpdateNotify:        100,
			SolutionTime:        480,
			SolutionNotify:      400,
		})

		// Test updating SLA
		payload := map[string]interface{}{
			"name":                "Updated SLA Name",
			"first_response_time": 45,
			"solution_time":       360,
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
		resetSLATestData(t, db)

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.DELETE("/api/v1/slas/:id", HandleDeleteSLAAPI)

		// Create a test SLA
		slaID := insertTestSLA(t, db, insertSLAParams{
			Name:                "Delete Test SLA",
			FirstResponseTime:   60,
			FirstResponseNotify: 50,
			UpdateTime:          120,
			UpdateNotify:        100,
			SolutionTime:        480,
			SolutionNotify:      400,
		})

		// Test soft deleting SLA
		req := httptest.NewRequest("DELETE", "/api/v1/slas/"+strconv.Itoa(slaID), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Verify soft delete
		var validID int
		checkQuery := database.ConvertPlaceholders(`
			SELECT valid_id FROM sla WHERE id = ?
		`)
		err := db.QueryRow(checkQuery, slaID).Scan(&validID)
		require.NoError(t, err)
		assert.Equal(t, 2, validID)

		// Test deleting non-existent SLA
		req = httptest.NewRequest("DELETE", "/api/v1/slas/99999", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("SLA Performance Metrics", func(t *testing.T) {
		resetSLATestData(t, db)

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/v1/slas/:id/metrics", HandleSLAMetricsAPI)

		// Create test SLA
		slaID := insertTestSLA(t, db, insertSLAParams{
			Name:                "Metrics Test SLA",
			FirstResponseTime:   60,
			FirstResponseNotify: 50,
			UpdateTime:          120,
			UpdateNotify:        100,
			SolutionTime:        480,
			SolutionNotify:      400,
		})

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

type insertSLAParams struct {
	Name                string
	CalendarName        string
	FirstResponseTime   int
	FirstResponseNotify int
	UpdateTime          int
	UpdateNotify        int
	SolutionTime        int
	SolutionNotify      int
	ValidID             int
	CreateBy            int
	ChangeBy            int
}

func ensureSLATestSchema(t *testing.T, db *sql.DB) {
	t.Helper()

	var statements []string
	if database.IsMySQL() {
		statements = []string{
			`CREATE TABLE IF NOT EXISTS sla (
				id INT UNSIGNED NOT NULL AUTO_INCREMENT,
				name VARCHAR(255) NOT NULL,
				calendar_name VARCHAR(255) NULL,
				first_response_time INT NOT NULL,
				first_response_notify INT DEFAULT 0,
				update_time INT DEFAULT 0,
				update_notify INT DEFAULT 0,
				solution_time INT NOT NULL,
				solution_notify INT DEFAULT 0,
				valid_id INT DEFAULT 1,
				comments TEXT NULL,
				create_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				create_by INT DEFAULT 1,
				change_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
				change_by INT DEFAULT 1,
				PRIMARY KEY (id),
				UNIQUE KEY idx_sla_name (name)
			) ENGINE=InnoDB`,
			`ALTER TABLE sla ADD COLUMN IF NOT EXISTS comments TEXT NULL`,
			`CREATE TABLE IF NOT EXISTS tickets (
				id INT UNSIGNED NOT NULL AUTO_INCREMENT,
				ticket_number VARCHAR(255),
				sla_id INT,
				first_response_time INT,
				solution_time INT,
				create_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY (id),
				KEY idx_tickets_sla_id (sla_id)
			) ENGINE=InnoDB`,
		}
	} else {
		statements = []string{
			`CREATE TABLE IF NOT EXISTS sla (
				id SERIAL PRIMARY KEY,
				name VARCHAR(255) NOT NULL,
				calendar_name VARCHAR(255),
				first_response_time INTEGER NOT NULL,
				first_response_notify INTEGER DEFAULT 0,
				update_time INTEGER DEFAULT 0,
				update_notify INTEGER DEFAULT 0,
				solution_time INTEGER NOT NULL,
				solution_notify INTEGER DEFAULT 0,
				valid_id INTEGER DEFAULT 1,
				comments TEXT,
				create_time TIMESTAMPTZ DEFAULT NOW(),
				create_by INTEGER DEFAULT 1,
				change_time TIMESTAMPTZ DEFAULT NOW(),
				change_by INTEGER DEFAULT 1,
				UNIQUE (name)
			)`,
			`ALTER TABLE IF EXISTS sla ADD COLUMN IF NOT EXISTS comments TEXT`,
			`CREATE TABLE IF NOT EXISTS tickets (
				id SERIAL PRIMARY KEY,
				ticket_number VARCHAR(255),
				sla_id INTEGER,
				first_response_time INTEGER,
				solution_time INTEGER,
				create_time TIMESTAMPTZ DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS idx_tickets_sla_id ON tickets (sla_id)`,
		}
	}

	for _, stmt := range statements {
		_, err := db.Exec(stmt)
		require.NoError(t, err)
	}
}

func resetSLATestData(t *testing.T, db *sql.DB) {
	t.Helper()

	_, err := db.Exec("DELETE FROM tickets")
	require.NoError(t, err)
	_, err = db.Exec("DELETE FROM sla")
	require.NoError(t, err)
}

func insertTestSLA(t *testing.T, db *sql.DB, params insertSLAParams) int {
	t.Helper()

	calendarArg := interface{}(nil)
	if trimmed := strings.TrimSpace(params.CalendarName); trimmed != "" {
		calendarArg = trimmed
	}

	validID := params.ValidID
	if validID == 0 {
		validID = 1
	}
	createBy := params.CreateBy
	if createBy == 0 {
		createBy = 1
	}
	changeBy := params.ChangeBy
	if changeBy == 0 {
		changeBy = createBy
	}

	query := database.ConvertPlaceholders(`
		INSERT INTO sla (
			name, calendar_name,
			first_response_time, first_response_notify,
			update_time, update_notify,
			solution_time, solution_notify,
			valid_id, create_time, create_by, change_time, change_by
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?,
			?, NOW(), ?0, NOW(), ?1
		) RETURNING id
	`)

	query, useLastInsert := database.ConvertReturning(query)
	args := []interface{}{
		params.Name,
		calendarArg,
		params.FirstResponseTime,
		params.FirstResponseNotify,
		params.UpdateTime,
		params.UpdateNotify,
		params.SolutionTime,
		params.SolutionNotify,
		validID,
		createBy,
		changeBy,
	}

	if useLastInsert && database.IsMySQL() {
		res, err := db.Exec(query, args...)
		require.NoError(t, err)
		id, err := res.LastInsertId()
		require.NoError(t, err)
		if id == 0 {
			var fallbackID int64
			require.NoError(t, db.QueryRow("SELECT LAST_INSERT_ID()").Scan(&fallbackID))
			id = fallbackID
		}
		return int(id)
	}

	var slaID int
	err := db.QueryRow(query, args...).Scan(&slaID)
	require.NoError(t, err)
	return slaID
}
