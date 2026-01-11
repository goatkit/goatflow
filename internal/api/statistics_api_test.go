package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/gotrs-io/gotrs-ce/internal/auth"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

func TestStatisticsAPI(t *testing.T) {
	// Initialize test database; skip if unavailable
	if err := database.InitTestDB(); err != nil {
		t.Skip("Database not available, skipping Statistics API tests")
	}
	defer database.CloseTestDB()

	// Create test JWT manager
	jwtManager := auth.NewJWTManager("test-secret", time.Hour)

	// Create test token
	token, _ := jwtManager.GenerateToken(1, "testuser@example.com", "Agent", 0)

	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Setup comprehensive test data
	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skip("Database not available, skipping integration test setup")
	}

	// Create test tickets with various states and dates
	ticketTypeColumn := database.TicketTypeColumn()
	ticketQuery := database.ConvertPlaceholders(fmt.Sprintf(`
		INSERT INTO tickets (tn, title, queue_id, %s, ticket_state_id, 
			ticket_priority_id, customer_user_id, user_id, responsible_user_id,
			create_time, create_by, change_time, change_by)
		VALUES 
			(?, 'Open ticket 1', 1, 1, 1, 3, 'customer1@example.com', 1, 1, NOW() - INTERVAL '7 days', 1, NOW(), 1),
			(?, 'Open ticket 2', 1, 1, 1, 2, 'customer2@example.com', 2, 2, NOW() - INTERVAL '3 days', 1, NOW(), 1),
			(?, 'Closed ticket 1', 2, 1, 2, 3, 'customer3@example.com', 1, 1, NOW() - INTERVAL '14 days', 1, NOW() - INTERVAL '10 days', 1),
			(?, 'Closed ticket 2', 2, 1, 2, 1, 'customer4@example.com', 2, 2, NOW() - INTERVAL '1 day', 1, NOW(), 1),
			(?, 'Pending ticket', 1, 1, 3, 2, 'customer5@example.com', 1, 2, NOW() - INTERVAL '2 days', 1, NOW(), 1)
	`, ticketTypeColumn))
	db.Exec(ticketQuery, "2024120100001", "2024120100002", "2024120100003", "2024120100004", "2024120100005")

	// Create test articles for response time metrics
	articleQuery := database.ConvertPlaceholders(`
		INSERT INTO article (ticket_id, article_type_id, article_sender_type_id,
			from_email, to_email, subject, body, create_time, create_by, change_time, change_by)
		VALUES 
			(1, 1, 3, 'customer1@example.com', 'support@example.com', 'Initial request', 'Help needed', NOW() - INTERVAL '7 days', 1, NOW(), 1),
			(1, 1, 1, 'support@example.com', 'customer1@example.com', 'Response', 'We are looking into it', NOW() - INTERVAL '6 days', 1, NOW(), 1),
			(2, 1, 3, 'customer2@example.com', 'support@example.com', 'Problem', 'System down', NOW() - INTERVAL '3 days', 1, NOW(), 1)
	`)
	db.Exec(articleQuery)

	t.Run("Dashboard Statistics", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/v1/statistics/dashboard", HandleDashboardStatisticsAPI)

		req := httptest.NewRequest("GET", "/api/v1/statistics/dashboard", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Overview struct {
				TotalTickets   int `json:"total_tickets"`
				OpenTickets    int `json:"open_tickets"`
				ClosedTickets  int `json:"closed_tickets"`
				PendingTickets int `json:"pending_tickets"`
			} `json:"overview"`
			ByQueue []struct {
				QueueID   int    `json:"queue_id"`
				QueueName string `json:"queue_name"`
				Count     int    `json:"count"`
			} `json:"by_queue"`
			ByPriority []struct {
				PriorityID   int    `json:"priority_id"`
				PriorityName string `json:"priority_name"`
				Count        int    `json:"count"`
			} `json:"by_priority"`
			RecentActivity []struct {
				Type      string    `json:"type"`
				TicketID  int       `json:"ticket_id"`
				TicketTN  string    `json:"ticket_tn"`
				Timestamp time.Time `json:"timestamp"`
			} `json:"recent_activity"`
		}

		json.Unmarshal(w.Body.Bytes(), &response)
		assert.NotZero(t, response.Overview.TotalTickets)
		assert.NotEmpty(t, response.ByQueue)
		assert.NotEmpty(t, response.ByPriority)
	})

	t.Run("Ticket Trends", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/v1/statistics/trends", HandleTicketTrendsAPI)

		// Test daily trends for last 7 days
		req := httptest.NewRequest("GET", "/api/v1/statistics/trends?period=daily&days=7", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Period string `json:"period"`
			Days   int    `json:"days"`
			Trends []struct {
				Date    string `json:"date"`
				Created int    `json:"created"`
				Closed  int    `json:"closed"`
				Open    int    `json:"open"`
			} `json:"trends"`
			Summary struct {
				TotalCreated  int     `json:"total_created"`
				TotalClosed   int     `json:"total_closed"`
				AveragePerDay float64 `json:"average_per_day"`
				ClosureRate   float64 `json:"closure_rate"`
			} `json:"summary"`
		}

		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "daily", response.Period)
		assert.Equal(t, 7, response.Days)
		assert.NotEmpty(t, response.Trends)

		// Test monthly trends
		req = httptest.NewRequest("GET", "/api/v1/statistics/trends?period=monthly&months=3", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Agent Performance", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/v1/statistics/agents", HandleAgentPerformanceAPI)

		req := httptest.NewRequest("GET", "/api/v1/statistics/agents?period=7d", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Period string `json:"period"`
			Agents []struct {
				AgentID              int     `json:"agent_id"`
				AgentName            string  `json:"agent_name"`
				TicketsAssigned      int     `json:"tickets_assigned"`
				TicketsClosed        int     `json:"tickets_closed"`
				ArticlesCreated      int     `json:"articles_created"`
				AvgResponseTime      float64 `json:"avg_response_time_hours"`
				AvgResolutionTime    float64 `json:"avg_resolution_time_hours"`
				CustomerSatisfaction float64 `json:"customer_satisfaction"`
			} `json:"agents"`
			TopPerformers []struct {
				AgentID   int     `json:"agent_id"`
				AgentName string  `json:"agent_name"`
				Metric    string  `json:"metric"`
				Value     float64 `json:"value"`
			} `json:"top_performers"`
		}

		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "7d", response.Period)
		assert.NotNil(t, response.Agents)
	})

	t.Run("Queue Metrics", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/v1/statistics/queues", HandleQueueMetricsAPI)

		req := httptest.NewRequest("GET", "/api/v1/statistics/queues", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Queues []struct {
				QueueID           int     `json:"queue_id"`
				QueueName         string  `json:"queue_name"`
				TotalTickets      int     `json:"total_tickets"`
				OpenTickets       int     `json:"open_tickets"`
				AvgWaitTime       float64 `json:"avg_wait_time_hours"`
				AvgResolutionTime float64 `json:"avg_resolution_time_hours"`
				Backlog           int     `json:"backlog"`
				SLACompliance     float64 `json:"sla_compliance_percent"`
			} `json:"queues"`
			Totals struct {
				AllQueues         int     `json:"all_queues"`
				TotalTickets      int     `json:"total_tickets"`
				TotalOpen         int     `json:"total_open"`
				OverallCompliance float64 `json:"overall_compliance_percent"`
			} `json:"totals"`
		}

		json.Unmarshal(w.Body.Bytes(), &response)
		assert.NotEmpty(t, response.Queues)
		assert.NotZero(t, response.Totals.AllQueues)
	})

	t.Run("Time-based Analytics", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/v1/statistics/analytics", HandleTimeBasedAnalyticsAPI)

		// Hourly distribution
		req := httptest.NewRequest("GET", "/api/v1/statistics/analytics?type=hourly", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var hourlyResponse struct {
			Type string `json:"type"`
			Data []struct {
				Hour    int `json:"hour"`
				Created int `json:"created"`
				Closed  int `json:"closed"`
			} `json:"data"`
			PeakHours []int `json:"peak_hours"`
		}

		json.Unmarshal(w.Body.Bytes(), &hourlyResponse)
		assert.Equal(t, "hourly", hourlyResponse.Type)
		assert.Len(t, hourlyResponse.Data, 24) // 24 hours

		// Day of week distribution
		req = httptest.NewRequest("GET", "/api/v1/statistics/analytics?type=day_of_week", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var weekResponse struct {
			Type string `json:"type"`
			Data []struct {
				Day     string `json:"day"`
				Created int    `json:"created"`
				Closed  int    `json:"closed"`
			} `json:"data"`
			BusiestDays []string `json:"busiest_days"`
		}

		json.Unmarshal(w.Body.Bytes(), &weekResponse)
		assert.Equal(t, "day_of_week", weekResponse.Type)
		assert.Len(t, weekResponse.Data, 7) // 7 days of week
	})

	t.Run("Customer Statistics", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/v1/statistics/customers", HandleCustomerStatisticsAPI)

		req := httptest.NewRequest("GET", "/api/v1/statistics/customers?top=10", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			TopCustomers []struct {
				CustomerID    string `json:"customer_id"`
				CustomerEmail string `json:"customer_email"`
				TicketCount   int    `json:"ticket_count"`
				OpenTickets   int    `json:"open_tickets"`
				LastActivity  string `json:"last_activity"`
			} `json:"top_customers"`
			CustomerMetrics struct {
				TotalCustomers        int     `json:"total_customers"`
				ActiveCustomers       int     `json:"active_customers"`
				NewCustomersThisMonth int     `json:"new_customers_this_month"`
				AvgTicketsPerCustomer float64 `json:"avg_tickets_per_customer"`
			} `json:"customer_metrics"`
		}

		json.Unmarshal(w.Body.Bytes(), &response)
		assert.NotNil(t, response.TopCustomers)
		assert.NotZero(t, response.CustomerMetrics.TotalCustomers)
	})

	t.Run("Export Statistics", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/v1/statistics/export", HandleExportStatisticsAPI)

		// Test CSV export
		req := httptest.NewRequest("GET", "/api/v1/statistics/export?format=csv&type=tickets&period=7d", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "text/csv")
		assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")

		// Test JSON export
		req = httptest.NewRequest("GET", "/api/v1/statistics/export?format=json&type=summary", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	})

	t.Run("Unauthorized Access", func(t *testing.T) {
		router := gin.New()
		router.GET("/api/v1/statistics/dashboard", HandleDashboardStatisticsAPI)

		req := httptest.NewRequest("GET", "/api/v1/statistics/dashboard", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
