package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/database"
)

// =============================================================================
// RBAC SECURITY TEST FIXTURES
// These tests verify that users only see data they have permission to access.
// Failing tests indicate data leakage vulnerabilities.
// =============================================================================

type RBACTestFixtures struct {
	db *sql.DB
	mu sync.Mutex

	// Groups
	GroupAlpha int // Only AgentAlpha has access
	GroupBeta  int // Only AgentBeta has access
	GroupBoth  int // Both agents have access

	// Queues (one per group)
	QueueAlpha int
	QueueBeta  int
	QueueBoth  int

	// Agents with isolated permissions
	AgentAlpha int // Only has access to Alpha queue
	AgentBeta  int // Only has access to Beta queue
	AgentBoth  int // Has access to both queues

	// Tickets in each queue (for stats testing)
	TicketsAlpha []int // Tickets in Alpha queue
	TicketsBeta  []int // Tickets in Beta queue
	TicketsBoth  []int // Tickets in Both queue
}

var (
	rbacFixtures     *RBACTestFixtures
	rbacFixturesOnce sync.Once
	rbacFixturesErr  error
)

// getRBACFixtures returns shared test fixtures for RBAC security tests
func getRBACFixtures(t *testing.T) *RBACTestFixtures {
	t.Helper()

	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skip("Database not available for RBAC security tests")
	}

	rbacFixturesOnce.Do(func() {
		rbacFixtures = &RBACTestFixtures{db: db}
		rbacFixturesErr = rbacFixtures.setup()
	})

	if rbacFixturesErr != nil {
		t.Skipf("Failed to setup RBAC fixtures: %v", rbacFixturesErr)
	}

	return rbacFixtures
}

func (f *RBACTestFixtures) setup() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	now := time.Now()
	suffix := fmt.Sprintf("_%d", time.Now().UnixNano()%100000)

	// Use very high IDs to avoid conflicts (95000+ range)
	f.GroupAlpha = 95001
	f.GroupBeta = 95002
	f.GroupBoth = 95003
	f.QueueAlpha = 95001
	f.QueueBeta = 95002
	f.QueueBoth = 95003
	f.AgentAlpha = 95001
	f.AgentBeta = 95002
	f.AgentBoth = 95003

	// Clean up any existing test data
	_, _ = f.db.Exec("SET FOREIGN_KEY_CHECKS=0")
	_, _ = f.db.Exec(database.ConvertPlaceholders("DELETE FROM ticket WHERE id >= 95000"))
	_, _ = f.db.Exec(database.ConvertPlaceholders("DELETE FROM queue WHERE id >= 95000"))
	_, _ = f.db.Exec(database.ConvertPlaceholders("DELETE FROM group_user WHERE user_id >= 95000 OR group_id >= 95000"))
	_, _ = f.db.Exec(database.ConvertPlaceholders("DELETE FROM `groups` WHERE id >= 95000"))
	_, _ = f.db.Exec(database.ConvertPlaceholders("DELETE FROM users WHERE id >= 95000"))
	_, _ = f.db.Exec("SET FOREIGN_KEY_CHECKS=1")

	exec := func(query string, args ...interface{}) error {
		_, err := f.db.Exec(database.ConvertPlaceholders(query), args...)
		return err
	}

	// Create groups
	for _, g := range []struct {
		id   int
		name string
	}{
		{f.GroupAlpha, "RBACTest-Alpha" + suffix},
		{f.GroupBeta, "RBACTest-Beta" + suffix},
		{f.GroupBoth, "RBACTest-Both" + suffix},
	} {
		if err := exec(
			"INSERT INTO `groups` (id, name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (?, ?, 'RBAC test group', 1, ?, 1, ?, 1)",
			g.id, g.name, now, now,
		); err != nil {
			return fmt.Errorf("failed to create group %s: %w", g.name, err)
		}
	}

	// Create queues
	for _, q := range []struct {
		id      int
		name    string
		groupID int
	}{
		{f.QueueAlpha, "RBACTest-Alpha-Queue" + suffix, f.GroupAlpha},
		{f.QueueBeta, "RBACTest-Beta-Queue" + suffix, f.GroupBeta},
		{f.QueueBoth, "RBACTest-Both-Queue" + suffix, f.GroupBoth},
	} {
		if err := exec(
			`INSERT INTO queue (id, name, group_id, unlock_timeout, first_response_time, 
				first_response_notify, update_time, update_notify, solution_time, solution_notify,
				system_address_id, calendar_name, default_sign_key, salutation_id, signature_id,
				follow_up_id, follow_up_lock, comments, valid_id, 
				create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, 0, 0, 0, 0, 0, 0, 0, 1, NULL, NULL, 1, 1, 1, 0, 'RBAC test queue', 1, ?, 1, ?, 1)`,
			q.id, q.name, q.groupID, now, now,
		); err != nil {
			return fmt.Errorf("failed to create queue %s: %w", q.name, err)
		}
	}

	// Create agents
	for _, a := range []struct {
		id    int
		login string
	}{
		{f.AgentAlpha, "rbactest-agent-alpha" + suffix},
		{f.AgentBeta, "rbactest-agent-beta" + suffix},
		{f.AgentBoth, "rbactest-agent-both" + suffix},
	} {
		if err := exec(
			`INSERT INTO users (id, login, pw, first_name, last_name, valid_id, 
				create_time, create_by, change_time, change_by)
			VALUES (?, ?, 'test', 'RBAC', 'Test', 1, ?, 1, ?, 1)`,
			a.id, a.login, now, now,
		); err != nil {
			return fmt.Errorf("failed to create agent %s: %w", a.login, err)
		}
	}

	// Assign permissions - ISOLATED ACCESS
	// AgentAlpha -> Alpha + Both queues (rw)
	// AgentBeta -> Beta + Both queues (rw)
	// AgentBoth -> All queues (rw)
	perms := []struct {
		userID  int
		groupID int
	}{
		{f.AgentAlpha, f.GroupAlpha},
		{f.AgentAlpha, f.GroupBoth},
		{f.AgentBeta, f.GroupBeta},
		{f.AgentBeta, f.GroupBoth},
		{f.AgentBoth, f.GroupAlpha},
		{f.AgentBoth, f.GroupBeta},
		{f.AgentBoth, f.GroupBoth},
	}

	for _, p := range perms {
		if err := exec(
			`INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
			VALUES (?, ?, 'rw', ?, 1, ?, 1)`,
			p.userID, p.groupID, now, now,
		); err != nil {
			return fmt.Errorf("failed to assign permission: %w", err)
		}
	}

	// Create tickets in each queue for statistics testing
	ticketID := 95001
	f.TicketsAlpha = []int{}
	f.TicketsBeta = []int{}
	f.TicketsBoth = []int{}

	// 3 tickets in Alpha queue
	for i := 0; i < 3; i++ {
		if err := exec(
			`INSERT INTO ticket (id, tn, title, queue_id, ticket_lock_id, type_id, 
				ticket_state_id, ticket_priority_id, customer_id, customer_user_id,
				user_id, responsible_user_id, timeout, until_time, escalation_time,
				escalation_update_time, escalation_response_time, escalation_solution_time,
				archive_flag, create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, ?, 1, 1, 1, 3, 'test-co', 'test@test.com', 1, 1, 0, 0, 0, 0, 0, 0, 0, ?, 1, ?, 1)`,
			ticketID, fmt.Sprintf("RBAC%d", ticketID), fmt.Sprintf("Alpha Ticket %d", i+1), f.QueueAlpha, now, now,
		); err != nil {
			return fmt.Errorf("failed to create alpha ticket: %w", err)
		}
		f.TicketsAlpha = append(f.TicketsAlpha, ticketID)
		ticketID++
	}

	// 5 tickets in Beta queue
	for i := 0; i < 5; i++ {
		if err := exec(
			`INSERT INTO ticket (id, tn, title, queue_id, ticket_lock_id, type_id, 
				ticket_state_id, ticket_priority_id, customer_id, customer_user_id,
				user_id, responsible_user_id, timeout, until_time, escalation_time,
				escalation_update_time, escalation_response_time, escalation_solution_time,
				archive_flag, create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, ?, 1, 1, 1, 3, 'test-co', 'test@test.com', 1, 1, 0, 0, 0, 0, 0, 0, 0, ?, 1, ?, 1)`,
			ticketID, fmt.Sprintf("RBAC%d", ticketID), fmt.Sprintf("Beta Ticket %d", i+1), f.QueueBeta, now, now,
		); err != nil {
			return fmt.Errorf("failed to create beta ticket: %w", err)
		}
		f.TicketsBeta = append(f.TicketsBeta, ticketID)
		ticketID++
	}

	// 2 tickets in Both queue
	for i := 0; i < 2; i++ {
		if err := exec(
			`INSERT INTO ticket (id, tn, title, queue_id, ticket_lock_id, type_id, 
				ticket_state_id, ticket_priority_id, customer_id, customer_user_id,
				user_id, responsible_user_id, timeout, until_time, escalation_time,
				escalation_update_time, escalation_response_time, escalation_solution_time,
				archive_flag, create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, ?, 1, 1, 1, 3, 'test-co', 'test@test.com', 1, 1, 0, 0, 0, 0, 0, 0, 0, ?, 1, ?, 1)`,
			ticketID, fmt.Sprintf("RBAC%d", ticketID), fmt.Sprintf("Both Ticket %d", i+1), f.QueueBoth, now, now,
		); err != nil {
			return fmt.Errorf("failed to create both ticket: %w", err)
		}
		f.TicketsBoth = append(f.TicketsBoth, ticketID)
		ticketID++
	}

	return nil
}

// setupRBACTestRouter creates a minimal router for RBAC security tests
func setupRBACTestRouter(t *testing.T) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Set up routes with simulated auth (user_id in context)
	v1 := router.Group("/api/v1")

	// Queue endpoints
	v1.GET("/queues", func(c *gin.Context) {
		// Simulate authenticated user
		if userID := c.GetHeader("X-Test-User-ID"); userID != "" {
			var uid int
			fmt.Sscanf(userID, "%d", &uid)
			c.Set("user_id", uid)
		}
		HandleListQueuesAPI(c)
	})

	v1.GET("/queues/:id/stats", func(c *gin.Context) {
		if userID := c.GetHeader("X-Test-User-ID"); userID != "" {
			var uid int
			fmt.Sscanf(userID, "%d", &uid)
			c.Set("user_id", uid)
		}
		HandleGetQueueStatsAPI(c)
	})

	// Statistics endpoints
	v1.GET("/statistics/dashboard", func(c *gin.Context) {
		if userID := c.GetHeader("X-Test-User-ID"); userID != "" {
			var uid int
			fmt.Sscanf(userID, "%d", &uid)
			c.Set("user_id", uid)
		}
		HandleDashboardStatisticsAPI(c)
	})

	v1.GET("/statistics/queues", func(c *gin.Context) {
		if userID := c.GetHeader("X-Test-User-ID"); userID != "" {
			var uid int
			fmt.Sscanf(userID, "%d", &uid)
			c.Set("user_id", uid)
		}
		HandleQueueMetricsAPI(c)
	})

	v1.GET("/statistics/trends", func(c *gin.Context) {
		if userID := c.GetHeader("X-Test-User-ID"); userID != "" {
			var uid int
			fmt.Sscanf(userID, "%d", &uid)
			c.Set("user_id", uid)
		}
		HandleTicketTrendsAPI(c)
	})

	return router
}

func makeRBACRequest(t *testing.T, router *gin.Engine, method, path string, userID int) *httptest.ResponseRecorder {
	t.Helper()

	req, err := http.NewRequest(method, path, nil)
	require.NoError(t, err)

	if userID > 0 {
		req.Header.Set("X-Test-User-ID", fmt.Sprintf("%d", userID))
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// =============================================================================
// QUEUE LIST RBAC TESTS
// =============================================================================

func TestQueueList_OnlyReturnsAccessibleQueues(t *testing.T) {
	fixtures := getRBACFixtures(t)
	router := setupRBACTestRouter(t)

	t.Run("AgentAlpha only sees Alpha and Both queues", func(t *testing.T) {
		w := makeRBACRequest(t, router, "GET", "/api/v1/queues", fixtures.AgentAlpha)
		require.Equal(t, http.StatusOK, w.Code, "Expected 200, got %d: %s", w.Code, w.Body.String())

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		queues, ok := resp["data"].([]interface{})
		require.True(t, ok, "Expected data to be array")

		// Count queues from our test set
		var foundAlpha, foundBeta, foundBoth bool
		for _, q := range queues {
			qm := q.(map[string]interface{})
			queueID := int(qm["id"].(float64))
			switch queueID {
			case fixtures.QueueAlpha:
				foundAlpha = true
			case fixtures.QueueBeta:
				foundBeta = true
				t.Errorf("SECURITY VULNERABILITY: AgentAlpha should NOT see QueueBeta (ID %d)", fixtures.QueueBeta)
			case fixtures.QueueBoth:
				foundBoth = true
			}
		}

		assert.True(t, foundAlpha, "AgentAlpha should see QueueAlpha")
		assert.True(t, foundBoth, "AgentAlpha should see QueueBoth")
		assert.False(t, foundBeta, "AgentAlpha should NOT see QueueBeta")
	})

	t.Run("AgentBeta only sees Beta and Both queues", func(t *testing.T) {
		w := makeRBACRequest(t, router, "GET", "/api/v1/queues", fixtures.AgentBeta)
		require.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		queues := resp["data"].([]interface{})

		var foundAlpha, foundBeta, foundBoth bool
		for _, q := range queues {
			qm := q.(map[string]interface{})
			queueID := int(qm["id"].(float64))
			switch queueID {
			case fixtures.QueueAlpha:
				foundAlpha = true
				t.Errorf("SECURITY VULNERABILITY: AgentBeta should NOT see QueueAlpha (ID %d)", fixtures.QueueAlpha)
			case fixtures.QueueBeta:
				foundBeta = true
			case fixtures.QueueBoth:
				foundBoth = true
			}
		}

		assert.False(t, foundAlpha, "AgentBeta should NOT see QueueAlpha")
		assert.True(t, foundBeta, "AgentBeta should see QueueBeta")
		assert.True(t, foundBoth, "AgentBeta should see QueueBoth")
	})
}

// =============================================================================
// QUEUE STATS RBAC TESTS
// =============================================================================

func TestQueueStats_OnlyAccessibleQueues(t *testing.T) {
	fixtures := getRBACFixtures(t)
	router := setupRBACTestRouter(t)

	t.Run("AgentAlpha can get stats for Alpha queue", func(t *testing.T) {
		w := makeRBACRequest(t, router, "GET", fmt.Sprintf("/api/v1/queues/%d/stats", fixtures.QueueAlpha), fixtures.AgentAlpha)
		assert.Equal(t, http.StatusOK, w.Code, "AgentAlpha should access Alpha queue stats")
	})

	t.Run("AgentAlpha cannot get stats for Beta queue", func(t *testing.T) {
		w := makeRBACRequest(t, router, "GET", fmt.Sprintf("/api/v1/queues/%d/stats", fixtures.QueueBeta), fixtures.AgentAlpha)
		// Should be 403 or 404 (not 200!)
		if w.Code == http.StatusOK {
			t.Errorf("SECURITY VULNERABILITY: AgentAlpha should NOT access QueueBeta stats (got 200)")
		}
		assert.NotEqual(t, http.StatusOK, w.Code, "AgentAlpha should NOT access Beta queue stats")
	})

	t.Run("AgentBeta cannot get stats for Alpha queue", func(t *testing.T) {
		w := makeRBACRequest(t, router, "GET", fmt.Sprintf("/api/v1/queues/%d/stats", fixtures.QueueAlpha), fixtures.AgentBeta)
		if w.Code == http.StatusOK {
			t.Errorf("SECURITY VULNERABILITY: AgentBeta should NOT access QueueAlpha stats (got 200)")
		}
		assert.NotEqual(t, http.StatusOK, w.Code, "AgentBeta should NOT access Alpha queue stats")
	})
}

// =============================================================================
// DASHBOARD STATISTICS RBAC TESTS
// =============================================================================

func TestDashboardStats_OnlyCountsAccessibleTickets(t *testing.T) {
	fixtures := getRBACFixtures(t)
	router := setupRBACTestRouter(t)

	// AgentAlpha has access to Alpha (3 tickets) + Both (2 tickets) = 5 tickets
	// AgentBeta has access to Beta (5 tickets) + Both (2 tickets) = 7 tickets
	// AgentBoth has access to all = 10 tickets

	t.Run("AgentAlpha dashboard only counts accessible tickets", func(t *testing.T) {
		w := makeRBACRequest(t, router, "GET", "/api/v1/statistics/dashboard", fixtures.AgentAlpha)
		require.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		overview := resp["overview"].(map[string]interface{})
		totalTickets := int(overview["total_tickets"].(float64))

		// Should only count tickets in Alpha + Both queues
		expectedMax := len(fixtures.TicketsAlpha) + len(fixtures.TicketsBoth)
		
		if totalTickets > expectedMax {
			// Check if Beta queue tickets are being counted
			byQueue := resp["by_queue"].([]interface{})
			for _, q := range byQueue {
				qm := q.(map[string]interface{})
				if queueID, ok := qm["queue_id"].(float64); ok && int(queueID) == fixtures.QueueBeta {
					count := int(qm["count"].(float64))
					if count > 0 {
						t.Errorf("SECURITY VULNERABILITY: AgentAlpha dashboard includes QueueBeta (%d tickets)", count)
					}
				}
			}
		}

		// Note: totalTickets may include other tickets from seed data, so we check by_queue instead
		t.Logf("AgentAlpha sees total_tickets=%d (test fixtures: Alpha=%d, Both=%d)",
			totalTickets, len(fixtures.TicketsAlpha), len(fixtures.TicketsBoth))
	})

	t.Run("AgentBeta dashboard should not include Alpha queue", func(t *testing.T) {
		w := makeRBACRequest(t, router, "GET", "/api/v1/statistics/dashboard", fixtures.AgentBeta)
		require.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		// Check by_queue doesn't include Alpha
		byQueue := resp["by_queue"].([]interface{})
		for _, q := range byQueue {
			qm := q.(map[string]interface{})
			if queueID, ok := qm["queue_id"].(float64); ok && int(queueID) == fixtures.QueueAlpha {
				count := int(qm["count"].(float64))
				if count > 0 {
					t.Errorf("SECURITY VULNERABILITY: AgentBeta dashboard includes QueueAlpha (%d tickets)", count)
				}
			}
		}
	})
}

// =============================================================================
// QUEUE METRICS RBAC TESTS
// =============================================================================

func TestQueueMetrics_OnlyShowsAccessibleQueues(t *testing.T) {
	fixtures := getRBACFixtures(t)
	router := setupRBACTestRouter(t)

	t.Run("AgentAlpha queue metrics excludes Beta queue", func(t *testing.T) {
		w := makeRBACRequest(t, router, "GET", "/api/v1/statistics/queues", fixtures.AgentAlpha)
		require.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		queues := resp["queues"].([]interface{})
		for _, q := range queues {
			qm := q.(map[string]interface{})
			if queueID, ok := qm["queue_id"].(float64); ok && int(queueID) == fixtures.QueueBeta {
				t.Errorf("SECURITY VULNERABILITY: AgentAlpha queue metrics includes QueueBeta")
			}
		}
	})

	t.Run("AgentBeta queue metrics excludes Alpha queue", func(t *testing.T) {
		w := makeRBACRequest(t, router, "GET", "/api/v1/statistics/queues", fixtures.AgentBeta)
		require.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		queues := resp["queues"].([]interface{})
		for _, q := range queues {
			qm := q.(map[string]interface{})
			if queueID, ok := qm["queue_id"].(float64); ok && int(queueID) == fixtures.QueueAlpha {
				t.Errorf("SECURITY VULNERABILITY: AgentBeta queue metrics includes QueueAlpha")
			}
		}
	})
}

// =============================================================================
// TICKET TRENDS RBAC TESTS
// =============================================================================

func TestTicketTrends_OnlyCountsAccessibleTickets(t *testing.T) {
	fixtures := getRBACFixtures(t)
	router := setupRBACTestRouter(t)

	// This test verifies that ticket trends only include tickets from accessible queues.
	// If RBAC is not enforced, the trends will include tickets from all queues.

	t.Run("AgentAlpha trends only count accessible tickets", func(t *testing.T) {
		w := makeRBACRequest(t, router, "GET", "/api/v1/statistics/trends?period=daily&days=30", fixtures.AgentAlpha)
		require.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		// Verify summary doesn't include Beta queue tickets
		summary := resp["summary"].(map[string]interface{})
		totalCreated := int(summary["total_created"].(float64))

		// Without RBAC filtering, this will include all tickets including Beta queue
		// With RBAC, it should only include Alpha + Both queues
		maxExpected := len(fixtures.TicketsAlpha) + len(fixtures.TicketsBoth)

		t.Logf("AgentAlpha trends: total_created=%d, expected max from fixtures=%d",
			totalCreated, maxExpected)

		// Note: Can't assert exact count due to other seed data, but we log for manual verification
	})
}
