package api

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// statsFixtureTicket is one ticket row inserted by the statistics fixture.
type statsFixtureTicket struct {
	id          int64
	queueID     int
	state       string // ticket_state.name from the seed data
	priorityID  int
	customer    string
	responsible int
	created     time.Time
	changed     time.Time
}

// statsFixture owns two queues and the agents that can (or cannot) read them.
//
//	queueA: readable by agentA and agentAll
//	queueB: readable by agentAll only
//	agentNone: valid agent without any queue permission
//	agentAdmin: member of the admin group only (admins count every queue)
type statsFixture struct {
	db  *sql.DB
	now time.Time

	queueA, queueB                             int
	groupA                                     int
	agentA, agentAll, agentNone, agentAdmin    int
	loginA, loginAll, loginNone, loginAdmin    string
	customer1, customer2, customer3, customer4 string
	tickets                                    []statsFixtureTicket
	stateIDs                                   map[string]int

	// addQueue creates a queue in groupID with the given valid_id; addTicket
	// inserts a ticket and returns its id. Both are cleaned up with the fixture.
	addQueue  func(groupID, validID int) int
	addTicket func(tk statsFixtureTicket) int64
}

func newStatsFixture(t *testing.T) *statsFixture {
	t.Helper()

	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)

	f := &statsFixture{db: db, now: time.Now().UTC().Truncate(time.Second), stateIDs: map[string]int{}}
	sfx := fmt.Sprintf("%d", time.Now().UnixNano())

	var groupIDs, queueIDs, userIDs []int
	t.Cleanup(func() {
		for _, q := range []struct {
			sql string
			ids []int
		}{
			{"DELETE FROM article WHERE ticket_id IN (SELECT id FROM ticket WHERE queue_id IN (%s))", queueIDs},
			{"DELETE FROM ticket WHERE queue_id IN (%s)", queueIDs},
			{"DELETE FROM group_user WHERE user_id IN (%s)", userIDs},
			{"DELETE FROM queue WHERE id IN (%s)", queueIDs},
			{"DELETE FROM `groups` WHERE id IN (%s)", groupIDs},
			{"DELETE FROM users WHERE id IN (%s)", userIDs},
		} {
			if len(q.ids) == 0 {
				continue
			}
			args := make([]interface{}, len(q.ids))
			for i, id := range q.ids {
				args[i] = id
			}
			placeholders := strings.TrimSuffix(strings.Repeat("?,", len(q.ids)), ",")
			if _, err := db.Exec(database.ConvertPlaceholders(fmt.Sprintf(q.sql, placeholders)), args...); err != nil {
				t.Errorf("cleanup %q: %v", q.sql, err)
			}
		}
	})

	mustID := func(id int64, err error) int {
		t.Helper()
		require.NoError(t, err)
		return int(id)
	}
	newGroup := func(name string) int {
		id := mustID(database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(
			"INSERT INTO `groups` (name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (?, 'statistics test', 1, ?, 1, ?, 1) RETURNING id"),
			name, f.now, f.now))
		groupIDs = append(groupIDs, id)
		return id
	}
	newQueue := func(name string, groupID, validID int) int {
		id := mustID(database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
			INSERT INTO queue (name, group_id, system_address_id, salutation_id, signature_id,
				follow_up_id, follow_up_lock, comments, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, ?, 1, 1, 1, 1, 0, 'statistics test', ?, ?, 1, ?, 1) RETURNING id`),
			name, groupID, validID, f.now, f.now))
		queueIDs = append(queueIDs, id)
		return id
	}
	newAgent := func(login string) int {
		id := mustID(database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
			INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, 'x', 'Stats', 'Agent', 1, ?, 1, ?, 1) RETURNING id`),
			login, f.now, f.now))
		userIDs = append(userIDs, id)
		return id
	}
	grant := func(userID, groupID int, key string) {
		t.Helper()
		_, err := db.Exec(database.ConvertPlaceholders(`INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, ?, 1, ?, 1)`), userID, groupID, key, f.now, f.now)
		require.NoError(t, err)
	}

	groupA := newGroup("stats_a_" + sfx)
	groupB := newGroup("stats_b_" + sfx)
	f.groupA = groupA
	f.queueA = newQueue("stats_qa_"+sfx, groupA, 1)
	f.queueB = newQueue("stats_qb_"+sfx, groupB, 1)
	f.addQueue = func(groupID, validID int) int {
		return newQueue(fmt.Sprintf("stats_q%d_%s", len(queueIDs), sfx), groupID, validID)
	}

	f.loginA, f.loginAll, f.loginNone, f.loginAdmin = "stats_a_"+sfx, "stats_all_"+sfx, "stats_none_"+sfx, "stats_admin_"+sfx
	f.agentA = newAgent(f.loginA)
	f.agentAll = newAgent(f.loginAll)
	f.agentNone = newAgent(f.loginNone)
	f.agentAdmin = newAgent(f.loginAdmin)
	grant(f.agentA, groupA, "ro")
	grant(f.agentAll, groupA, "rw")
	grant(f.agentAll, groupB, "rw")
	// A non-read permission on queueB must not expose its tickets to agentA.
	grant(f.agentA, groupB, "create")

	var adminGroupID int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders("SELECT id FROM `groups` WHERE name = 'admin'")).Scan(&adminGroupID))
	grant(f.agentAdmin, adminGroupID, "rw")

	rows, err := db.Query(database.ConvertPlaceholders("SELECT id, name FROM ticket_state"))
	require.NoError(t, err)
	for rows.Next() {
		var id int
		var name string
		require.NoError(t, rows.Scan(&id, &name))
		f.stateIDs[name] = id
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())

	f.customer1, f.customer2, f.customer3, f.customer4 = "c1_"+sfx, "c2_"+sfx, "c3_"+sfx, "c4_"+sfx
	h, d := time.Hour, 24*time.Hour
	f.tickets = []statsFixtureTicket{
		{queueID: f.queueA, state: "new", priorityID: 3, customer: f.customer1, responsible: f.agentA, created: f.now.Add(-1 * h), changed: f.now.Add(-1 * h)},
		{queueID: f.queueA, state: "open", priorityID: 3, customer: f.customer1, responsible: f.agentA, created: f.now.Add(-30 * h), changed: f.now.Add(-30 * h)},
		{queueID: f.queueA, state: "closed successful", priorityID: 4, customer: f.customer2, responsible: f.agentA, created: f.now.Add(-3 * d), changed: f.now.Add(-1 * d)},
		{queueID: f.queueA, state: "pending reminder", priorityID: 2, customer: f.customer2, responsible: f.agentAll, created: f.now.Add(-5 * d), changed: f.now.Add(-5 * d)},
		{queueID: f.queueA, state: "closed successful", priorityID: 1, customer: f.customer4, responsible: f.agentA, created: f.now.Add(-40 * d), changed: f.now.Add(-40 * d)},
		{queueID: f.queueB, state: "open", priorityID: 5, customer: f.customer3, responsible: f.agentAll, created: f.now.Add(-1 * h), changed: f.now.Add(-1 * h)},
		{queueID: f.queueB, state: "closed unsuccessful", priorityID: 5, customer: f.customer3, responsible: f.agentAll, created: f.now.Add(-2 * d), changed: f.now.Add(-2 * d)},
	}
	f.addTicket = func(tk statsFixtureTicket) int64 {
		t.Helper()
		stateID, ok := f.stateIDs[tk.state]
		require.True(t, ok, "seed state %q missing", tk.state)
		n := len(f.tickets)
		tk.id = int64(mustID(database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
			INSERT INTO ticket (tn, title, queue_id, ticket_lock_id, type_id, user_id, responsible_user_id,
				ticket_priority_id, ticket_state_id, customer_id, customer_user_id, timeout, until_time,
				escalation_time, escalation_update_time, escalation_response_time, escalation_solution_time,
				archive_flag, create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, 1, 1, 1, ?, ?, ?, 'stats-co', ?, 0, 0, 0, 0, 0, 0, 0, ?, 1, ?, 1) RETURNING id`),
			fmt.Sprintf("ST%s%d", sfx[len(sfx)-9:], n), fmt.Sprintf("stats ticket %d", n), tk.queueID,
			tk.responsible, tk.priorityID, stateID, tk.customer, tk.created, tk.changed)))
		f.tickets = append(f.tickets, tk)
		return tk.id
	}
	initial := f.tickets
	f.tickets = nil
	for _, tk := range initial {
		f.addTicket(tk)
	}

	// Articles written by agentA: one on a queueA ticket, one on a queueB ticket.
	for _, ticketID := range []int64{f.tickets[0].id, f.tickets[5].id} {
		_, err := db.Exec(database.ConvertPlaceholders(`
			INSERT INTO article (ticket_id, article_sender_type_id, communication_channel_id, is_visible_for_customer,
				create_time, create_by, change_time, change_by)
			VALUES (?, 1, 3, 0, ?, ?, ?, ?)`),
			ticketID, f.now.Add(-time.Hour), f.agentA, f.now.Add(-time.Hour), f.agentA)
		require.NoError(t, err)
	}

	return f
}

func statsToken(t *testing.T, userID int, login, role string) string {
	t.Helper()
	return testSessionToken(t, uint(userID), login, login, role, false, 0)
}

func statsGet(t *testing.T, router http.Handler, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func statsGetJSON(t *testing.T, router http.Handler, path, token string, out interface{}) {
	t.Helper()
	w := statsGet(t, router, path, token)
	require.Equal(t, http.StatusOK, w.Code, "GET %s: %s", path, w.Body.String())
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), out), w.Body.String())
}

var statisticsPaths = []string{
	"/api/v1/statistics/dashboard",
	"/api/v1/statistics/trends",
	"/api/v1/statistics/agents",
	"/api/v1/statistics/queues",
	"/api/v1/statistics/analytics",
	"/api/v1/statistics/customers",
	"/api/v1/statistics/export",
	"/api/v1/ticket-states/statistics",
}

func TestStatisticsRoutesAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newStatsFixture(t)
	router := NewSimpleRouter()

	agentToken := statsToken(t, f.agentAll, f.loginAll, "Agent")
	noQueueToken := statsToken(t, f.agentNone, f.loginNone, "Agent")
	// A customer whose customer_user id collides with agentAll's users.id must not get agentAll's view.
	customerToken := statsToken(t, f.agentAll, "customer@example.com", "Customer")

	for _, path := range statisticsPaths {
		t.Run(path, func(t *testing.T) {
			assert.Equal(t, http.StatusUnauthorized, statsGet(t, router, path, "").Code, "no token")

			w := statsGet(t, router, path, customerToken)
			assert.Equal(t, http.StatusForbidden, w.Code, "customer token: %s", w.Body.String())

			w = statsGet(t, router, path, noQueueToken)
			assert.Equal(t, http.StatusForbidden, w.Code, "agent without queue access: %s", w.Body.String())

			w = statsGet(t, router, path, agentToken)
			assert.Equal(t, http.StatusOK, w.Code, "agent: %s", w.Body.String())
		})
	}

	for _, path := range []string{
		"/api/v1/statistics/trends?period=weekly",
		"/api/v1/statistics/agents?period=1y",
		"/api/v1/statistics/analytics?type=yearly",
		"/api/v1/statistics/export?format=xml",
		"/api/v1/statistics/export?type=everything",
		"/api/v1/statistics/export?period=1y",
		"/api/v1/statistics/customers?top=0",
		"/api/v1/statistics/customers?top=-5",
		"/api/v1/statistics/customers?top=101",
		"/api/v1/statistics/customers?top=lots",
	} {
		w := statsGet(t, router, path, agentToken)
		assert.Equal(t, http.StatusBadRequest, w.Code, "%s: %s", path, w.Body.String())
	}
}

func TestStatisticsDashboard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newStatsFixture(t)
	router := NewSimpleRouter()
	// A ticket in an invalid queue: listed nowhere, so counted nowhere.
	invalidQueue := f.addQueue(f.groupA, 2)
	f.addTicket(statsFixtureTicket{queueID: invalidQueue, state: "open", priorityID: 3, customer: f.customer1,
		responsible: f.agentA, created: f.now.Add(-time.Hour), changed: f.now.Add(-time.Hour)})

	type dashboard struct {
		Overview struct {
			Total   int `json:"total_tickets"`
			Open    int `json:"open_tickets"`
			Closed  int `json:"closed_tickets"`
			Pending int `json:"pending_tickets"`
		} `json:"overview"`
		ByQueue []struct {
			QueueID int `json:"queue_id"`
			Count   int `json:"count"`
		} `json:"by_queue"`
		ByPriority []struct {
			PriorityID int `json:"priority_id"`
			Count      int `json:"count"`
		} `json:"by_priority"`
		RecentActivity []struct {
			TicketID int64 `json:"ticket_id"`
		} `json:"recent_activity"`
	}

	t.Run("agent with queueA only", func(t *testing.T) {
		var resp dashboard
		statsGetJSON(t, router, "/api/v1/statistics/dashboard", statsToken(t, f.agentA, f.loginA, "Agent"), &resp)

		assert.Equal(t, 5, resp.Overview.Total)
		assert.Equal(t, 2, resp.Overview.Open)
		assert.Equal(t, 2, resp.Overview.Closed)
		assert.Equal(t, 1, resp.Overview.Pending)

		require.Len(t, resp.ByQueue, 1, "queueB must not be listed")
		assert.Equal(t, f.queueA, resp.ByQueue[0].QueueID)
		assert.Equal(t, 5, resp.ByQueue[0].Count)

		byPriority := map[int]int{}
		for _, p := range resp.ByPriority {
			byPriority[p.PriorityID] = p.Count
		}
		assert.Equal(t, map[int]int{1: 1, 2: 1, 3: 2, 4: 1, 5: 0}, byPriority)

		var recent []int64
		for _, a := range resp.RecentActivity {
			recent = append(recent, a.TicketID)
		}
		tk := f.tickets
		assert.Equal(t, []int64{tk[0].id, tk[1].id, tk[2].id, tk[3].id, tk[4].id}, recent)
	})

	t.Run("agent with both queues", func(t *testing.T) {
		var resp dashboard
		statsGetJSON(t, router, "/api/v1/statistics/dashboard", statsToken(t, f.agentAll, f.loginAll, "Agent"), &resp)

		assert.Equal(t, 7, resp.Overview.Total)
		assert.Equal(t, 3, resp.Overview.Open)
		assert.Equal(t, 3, resp.Overview.Closed)
		assert.Equal(t, 1, resp.Overview.Pending)
		byQueue := map[int]int{}
		for _, q := range resp.ByQueue {
			byQueue[q.QueueID] = q.Count
		}
		assert.Equal(t, map[int]int{f.queueA: 5, f.queueB: 2}, byQueue)
	})

	t.Run("admin counts every valid queue, consistently with by_queue", func(t *testing.T) {
		var resp dashboard
		statsGetJSON(t, router, "/api/v1/statistics/dashboard", statsToken(t, f.agentAdmin, f.loginAdmin, "Agent"), &resp)

		byQueue := map[int]int{}
		sum := 0
		for _, q := range resp.ByQueue {
			byQueue[q.QueueID] = q.Count
			sum += q.Count
		}
		assert.Equal(t, 5, byQueue[f.queueA])
		assert.Equal(t, 2, byQueue[f.queueB])
		_, listed := byQueue[invalidQueue]
		assert.False(t, listed, "invalid queues are not listed")

		var total int
		require.NoError(t, f.db.QueryRow(database.ConvertPlaceholders(
			"SELECT COUNT(*) FROM ticket t JOIN queue q ON q.id = t.queue_id WHERE q.valid_id = 1")).Scan(&total))
		assert.Equal(t, total, resp.Overview.Total, "tickets in invalid queues are not counted")
		assert.Equal(t, sum, resp.Overview.Total, "overview total equals the sum of by_queue")
	})
}

// TestStatisticsTrendsOpen: trends[].open is the number of tickets open at the
// end of each bucket, including tickets created before the window that are
// still open, not a running sum of created minus closed inside the window.
func TestStatisticsTrendsOpen(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newStatsFixture(t)
	router := NewSimpleRouter()
	d := 24 * time.Hour
	// Open since before the window, and closed inside the window after being
	// created before it.
	f.addTicket(statsFixtureTicket{queueID: f.queueA, state: "open", priorityID: 3, customer: f.customer1,
		responsible: f.agentA, created: f.now.Add(-20 * d), changed: f.now.Add(-20 * d)})
	f.addTicket(statsFixtureTicket{queueID: f.queueA, state: "closed successful", priorityID: 3, customer: f.customer1,
		responsible: f.agentA, created: f.now.Add(-15 * d), changed: f.now.Add(-2 * d)})

	var resp struct {
		Trends []struct {
			Date string `json:"date"`
			Open int    `json:"open"`
		} `json:"trends"`
	}
	statsGetJSON(t, router, "/api/v1/statistics/trends?period=daily&days=7", statsToken(t, f.agentA, f.loginA, "Agent"), &resp)
	require.Len(t, resp.Trends, 7)

	// Expected: queueA tickets created before the end of the day that were
	// not closed before the end of the day (close time = last change).
	for _, bucket := range resp.Trends {
		day, err := time.Parse("2006-01-02", bucket.Date)
		require.NoError(t, err)
		end := day.Add(d)
		want := 0
		for _, tk := range f.tickets {
			if tk.queueID != f.queueA || !tk.created.Before(end) {
				continue
			}
			if strings.HasPrefix(tk.state, "closed") && tk.changed.Before(end) {
				continue
			}
			want++
		}
		assert.Equal(t, want, bucket.Open, "open at the end of %s", bucket.Date)
	}
	assert.Equal(t, 4, resp.Trends[6].Open, "today: new, open, pending and the 20-day-old open ticket")
}

func TestStatisticsTrends(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newStatsFixture(t)
	router := NewSimpleRouter()

	type trends struct {
		Period string `json:"period"`
		Trends []struct {
			Date    string `json:"date"`
			Created int    `json:"created"`
			Closed  int    `json:"closed"`
		} `json:"trends"`
		Summary struct {
			TotalCreated int `json:"total_created"`
			TotalClosed  int `json:"total_closed"`
		} `json:"summary"`
	}
	tk := f.tickets

	// queueA tickets inside the 7-day window: tickets 0-3 created, ticket 2 closed.
	var daily trends
	statsGetJSON(t, router, "/api/v1/statistics/trends?period=daily&days=7", statsToken(t, f.agentA, f.loginA, "Agent"), &daily)
	require.Len(t, daily.Trends, 7)
	assert.Equal(t, 4, daily.Summary.TotalCreated)
	assert.Equal(t, 1, daily.Summary.TotalClosed)
	created, closed := map[string]int{}, map[string]int{}
	for _, d := range daily.Trends {
		created[d.Date] += d.Created
		closed[d.Date] += d.Closed
	}
	wantCreated := map[string]int{}
	for _, i := range []int{0, 1, 2, 3} {
		wantCreated[tk[i].created.Format("2006-01-02")]++
	}
	for day, n := range wantCreated {
		assert.Equal(t, n, created[day], "created on %s", day)
	}
	assert.Equal(t, 1, closed[tk[2].changed.Format("2006-01-02")])

	// agentAll also sees queueB: tickets 5 and 6 created, ticket 6 closed.
	statsGetJSON(t, router, "/api/v1/statistics/trends?period=daily&days=7", statsToken(t, f.agentAll, f.loginAll, "Agent"), &daily)
	assert.Equal(t, 6, daily.Summary.TotalCreated)
	assert.Equal(t, 2, daily.Summary.TotalClosed)

	// A 3-month window always contains the 40-day-old ticket 4 (created and closed then).
	var monthly trends
	statsGetJSON(t, router, "/api/v1/statistics/trends?period=monthly&months=3", statsToken(t, f.agentA, f.loginA, "Agent"), &monthly)
	require.Len(t, monthly.Trends, 3)
	assert.Equal(t, 5, monthly.Summary.TotalCreated)
	assert.Equal(t, 2, monthly.Summary.TotalClosed)
}

func TestStatisticsAgents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newStatsFixture(t)
	router := NewSimpleRouter()

	type agentRow struct {
		AgentID  int `json:"agent_id"`
		Assigned int `json:"tickets_assigned"`
		Closed   int `json:"tickets_closed"`
		Articles int `json:"articles_created"`
	}
	get := func(token string) map[int]agentRow {
		var resp struct {
			Period string     `json:"period"`
			Agents []agentRow `json:"agents"`
		}
		statsGetJSON(t, router, "/api/v1/statistics/agents?period=7d", token, &resp)
		assert.Equal(t, "7d", resp.Period)
		out := map[int]agentRow{}
		for _, a := range resp.Agents {
			out[a.AgentID] = a
		}
		return out
	}

	viewA := get(statsToken(t, f.agentA, f.loginA, "Agent"))
	assert.Equal(t, map[int]agentRow{
		f.agentA:   {AgentID: f.agentA, Assigned: 3, Closed: 1, Articles: 1},
		f.agentAll: {AgentID: f.agentAll, Assigned: 1, Closed: 0, Articles: 0},
	}, viewA, "an agent sees only agents active on tickets in queues they can read, not the roster")

	viewAll := get(statsToken(t, f.agentAll, f.loginAll, "Agent"))
	assert.Equal(t, map[int]agentRow{
		f.agentA:   {AgentID: f.agentA, Assigned: 3, Closed: 1, Articles: 2},
		f.agentAll: {AgentID: f.agentAll, Assigned: 3, Closed: 1, Articles: 0},
	}, viewAll)

	viewAdmin := get(statsToken(t, f.agentAdmin, f.loginAdmin, "Agent"))
	assert.Equal(t, agentRow{AgentID: f.agentNone}, viewAdmin[f.agentNone], "admins see every valid agent, also without activity")
	assert.Equal(t, agentRow{AgentID: f.agentA, Assigned: 3, Closed: 1, Articles: 2}, viewAdmin[f.agentA])
}

func TestStatisticsQueues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newStatsFixture(t)
	router := NewSimpleRouter()

	type queueRow struct {
		QueueID int `json:"queue_id"`
		Total   int `json:"total_tickets"`
		Open    int `json:"open_tickets"`
		Backlog int `json:"backlog"`
	}
	var resp struct {
		Queues []queueRow `json:"queues"`
		Totals struct {
			AllQueues int `json:"all_queues"`
			Total     int `json:"total_tickets"`
			Open      int `json:"total_open"`
		} `json:"totals"`
	}
	statsGetJSON(t, router, "/api/v1/statistics/queues", statsToken(t, f.agentA, f.loginA, "Agent"), &resp)
	assert.Equal(t, []queueRow{{QueueID: f.queueA, Total: 5, Open: 2, Backlog: 1}}, resp.Queues)
	assert.Equal(t, 1, resp.Totals.AllQueues)
	assert.Equal(t, 5, resp.Totals.Total)
	assert.Equal(t, 2, resp.Totals.Open)

	statsGetJSON(t, router, "/api/v1/statistics/queues", statsToken(t, f.agentAll, f.loginAll, "Agent"), &resp)
	assert.ElementsMatch(t, []queueRow{
		{QueueID: f.queueA, Total: 5, Open: 2, Backlog: 1},
		{QueueID: f.queueB, Total: 2, Open: 1, Backlog: 0},
	}, resp.Queues)
}

func TestStatisticsAnalytics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newStatsFixture(t)
	router := NewSimpleRouter()
	tk := f.tickets
	token := statsToken(t, f.agentA, f.loginA, "Agent")

	// queueA inside the default 30-day window: tickets 0-3 created, ticket 2 closed.
	var hourly struct {
		Type string `json:"type"`
		Days int    `json:"days"`
		Data []struct {
			Hour    int `json:"hour"`
			Created int `json:"created"`
			Closed  int `json:"closed"`
		} `json:"data"`
		PeakHours []int `json:"peak_hours"`
	}
	statsGetJSON(t, router, "/api/v1/statistics/analytics?type=hourly", token, &hourly)
	assert.Equal(t, 30, hourly.Days)
	require.Len(t, hourly.Data, 24)
	wantCreated, wantClosed := make([]int, 24), make([]int, 24)
	for _, i := range []int{0, 1, 2, 3} {
		wantCreated[tk[i].created.Hour()]++
	}
	wantClosed[tk[2].changed.Hour()]++
	gotCreated, gotClosed := make([]int, 24), make([]int, 24)
	for _, d := range hourly.Data {
		gotCreated[d.Hour], gotClosed[d.Hour] = d.Created, d.Closed
	}
	assert.Equal(t, wantCreated, gotCreated)
	assert.Equal(t, wantClosed, gotClosed)
	var wantPeak []int
	maxCreated := 0
	for h, n := range wantCreated {
		if n > maxCreated {
			maxCreated, wantPeak = n, []int{h}
		} else if n == maxCreated && n > 0 {
			wantPeak = append(wantPeak, h)
		}
	}
	assert.Equal(t, wantPeak, hourly.PeakHours)

	var weekly struct {
		Data []struct {
			Day     string `json:"day"`
			Created int    `json:"created"`
			Closed  int    `json:"closed"`
		} `json:"data"`
	}
	statsGetJSON(t, router, "/api/v1/statistics/analytics?type=day_of_week&days=60", token, &weekly)
	require.Len(t, weekly.Data, 7)
	wantDayCreated, wantDayClosed := map[string]int{}, map[string]int{}
	for _, i := range []int{0, 1, 2, 3, 4} {
		wantDayCreated[tk[i].created.Weekday().String()]++
	}
	wantDayClosed[tk[2].changed.Weekday().String()]++
	wantDayClosed[tk[4].changed.Weekday().String()]++
	for _, d := range weekly.Data {
		assert.Equal(t, wantDayCreated[d.Day], d.Created, "created on %s", d.Day)
		assert.Equal(t, wantDayClosed[d.Day], d.Closed, "closed on %s", d.Day)
	}
}

func TestStatisticsCustomers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newStatsFixture(t)
	router := NewSimpleRouter()
	tk := f.tickets

	type customerRow struct {
		CustomerID   string `json:"customer_id"`
		TicketCount  int    `json:"ticket_count"`
		OpenTickets  int    `json:"open_tickets"`
		LastActivity string `json:"last_activity"`
	}
	var resp struct {
		TopCustomers []customerRow `json:"top_customers"`
		Metrics      struct {
			Total   int     `json:"total_customers"`
			Active  int     `json:"active_customers"`
			New     int     `json:"new_customers_this_month"`
			AvgTkts float64 `json:"avg_tickets_per_customer"`
		} `json:"customer_metrics"`
	}
	statsGetJSON(t, router, "/api/v1/statistics/customers?top=10", statsToken(t, f.agentA, f.loginA, "Agent"), &resp)

	// customer1: tickets 0,1 (new + open); customer2: tickets 2,3 (closed + pending); customer4: ticket 4 (40 days old).
	assert.Equal(t, []customerRow{
		{CustomerID: f.customer1, TicketCount: 2, OpenTickets: 2, LastActivity: tk[0].created.Format(time.RFC3339)},
		{CustomerID: f.customer2, TicketCount: 2, OpenTickets: 0, LastActivity: tk[2].created.Format(time.RFC3339)},
		{CustomerID: f.customer4, TicketCount: 1, OpenTickets: 0, LastActivity: tk[4].created.Format(time.RFC3339)},
	}, resp.TopCustomers)
	assert.Equal(t, 3, resp.Metrics.Total)
	assert.Equal(t, 2, resp.Metrics.Active)
	assert.InDelta(t, 5.0/3.0, resp.Metrics.AvgTkts, 0.0001)

	monthStart := time.Date(f.now.Year(), f.now.Month(), 1, 0, 0, 0, 0, time.UTC)
	wantNew := 0
	for _, first := range []time.Time{tk[1].created, tk[3].created, tk[4].created} {
		if !first.Before(monthStart) {
			wantNew++
		}
	}
	assert.Equal(t, wantNew, resp.Metrics.New)

	statsGetJSON(t, router, "/api/v1/statistics/customers?top=1", statsToken(t, f.agentAll, f.loginAll, "Agent"), &resp)
	require.Len(t, resp.TopCustomers, 1)
	assert.Equal(t, 4, resp.Metrics.Total, "agentAll also sees customer3 on queueB")
}

func TestStatisticsExport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newStatsFixture(t)
	router := NewSimpleRouter()
	tk := f.tickets
	token := statsToken(t, f.agentA, f.loginA, "Agent")

	var summary struct {
		Period  string `json:"period"`
		Summary struct {
			Total   int `json:"total_tickets"`
			Open    int `json:"open_tickets"`
			Closed  int `json:"closed_tickets"`
			Pending int `json:"pending_tickets"`
		} `json:"summary"`
	}
	statsGetJSON(t, router, "/api/v1/statistics/export?format=json&type=summary&period=7d", token, &summary)
	assert.Equal(t, "7d", summary.Period)
	assert.Equal(t, 4, summary.Summary.Total)
	assert.Equal(t, 2, summary.Summary.Open)
	assert.Equal(t, 1, summary.Summary.Closed)
	assert.Equal(t, 1, summary.Summary.Pending)

	var tickets []struct {
		Title    string `json:"title"`
		Customer string `json:"customer"`
	}
	statsGetJSON(t, router, "/api/v1/statistics/export?format=json&type=tickets&period=30d", token, &tickets)
	var titles []string
	for _, x := range tickets {
		titles = append(titles, x.Title)
	}
	assert.Equal(t, []string{"stats ticket 0", "stats ticket 1", "stats ticket 2", "stats ticket 3"}, titles)

	w := statsGet(t, router, "/api/v1/statistics/export?format=csv&type=tickets&period=24h", token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Header().Get("Content-Type"), "text/csv")
	assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
	records, err := csv.NewReader(strings.NewReader(w.Body.String())).ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 2, "header + ticket 0 (the only queueA ticket from the last 24h)")
	assert.Equal(t, "Ticket Number", records[0][0])
	assert.Equal(t, "stats ticket 0", records[1][1])
	assert.Equal(t, tk[0].customer, records[1][5])

	w = statsGet(t, router, "/api/v1/statistics/export?format=csv&type=summary&period=7d", token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	records, err = csv.NewReader(strings.NewReader(w.Body.String())).ReadAll()
	require.NoError(t, err)
	assert.Equal(t, [][]string{
		{"Metric", "Value"},
		{"total_tickets", "4"},
		{"open_tickets", "2"},
		{"closed_tickets", "1"},
		{"pending_tickets", "1"},
	}, records)
}

func TestTicketStateStatistics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newStatsFixture(t)
	router := NewSimpleRouter()

	type stateStats struct {
		Statistics []struct {
			StateID     int    `json:"state_id"`
			StateName   string `json:"state_name"`
			TypeID      int    `json:"type_id"`
			TicketCount int    `json:"ticket_count"`
		} `json:"statistics"`
		TotalTickets int `json:"total_tickets"`
	}
	countsByState := func(resp stateStats) map[string]int {
		out := map[string]int{}
		for _, s := range resp.Statistics {
			out[s.StateName] = s.TicketCount
			assert.Equal(t, f.stateIDs[s.StateName], s.StateID)
		}
		return out
	}

	var resp stateStats
	statsGetJSON(t, router, "/api/v1/ticket-states/statistics", statsToken(t, f.agentA, f.loginA, "Agent"), &resp)
	got := countsByState(resp)
	assert.Equal(t, 1, got["new"])
	assert.Equal(t, 1, got["open"])
	assert.Equal(t, 1, got["pending reminder"])
	assert.Equal(t, 2, got["closed successful"])
	assert.Equal(t, 0, got["closed unsuccessful"], "queueB ticket must not be counted for agentA")
	assert.Equal(t, 5, resp.TotalTickets)

	statsGetJSON(t, router, "/api/v1/ticket-states/statistics", statsToken(t, f.agentAll, f.loginAll, "Agent"), &resp)
	got = countsByState(resp)
	assert.Equal(t, 2, got["open"])
	assert.Equal(t, 1, got["closed unsuccessful"])
	assert.Equal(t, 7, resp.TotalTickets)
}
