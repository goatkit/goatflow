package api

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/service"
)

// extractUserIDForRBAC extracts user ID from gin context for RBAC checks
// Returns 0 if not authenticated
func extractUserIDForRBAC(c *gin.Context) int {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		return 0
	}

	switch v := userIDVal.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case uint:
		return int(v)
	case uint64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

// buildQueueFilterClause creates SQL WHERE clause fragment for RBAC queue filtering
func buildQueueFilterClause(queueIDs []int, queueIDColumn string) (string, []interface{}) {
	if len(queueIDs) == 0 {
		return queueIDColumn + " IN (NULL)", nil // Will match nothing
	}

	placeholders := make([]string, len(queueIDs))
	args := make([]interface{}, len(queueIDs))
	for i, id := range queueIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	return queueIDColumn + " IN (" + strings.Join(placeholders, ",") + ")", args
}

// statsScope is the set of queues whose tickets a statistics caller may count.
type statsScope struct {
	all      bool // admin: every queue
	queueIDs []int
}

// filter returns the SQL fragment restricting queueIDColumn to the scope.
func (s statsScope) filter(queueIDColumn string) (string, []interface{}) {
	if s.all {
		return "1 = 1", nil
	}
	return buildQueueFilterClause(s.queueIDs, queueIDColumn)
}

// statisticsScope authenticates a statistics request and resolves the queues the
// caller may read. Statistics are agent-only: customer JWTs and customer API tokens
// get 403, because their user_id is a customer_user id, not a users id. Admins
// count every queue; other agents count the queues they hold 'ro' (or 'rw') on,
// directly or through a role. On failure the error response is already written.
func statisticsScope(c *gin.Context) (*sql.DB, statsScope, bool) {
	userID := extractUserIDForRBAC(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return nil, statsScope{}, false
	}
	if isCustomer, _ := c.Get("is_customer"); isCustomer == true || c.GetString("user_role") == "Customer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Statistics are only available to agents"})
		return nil, statsScope{}, false
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return nil, statsScope{}, false
	}

	scope, err := resolveStatsScope(c, db, userID)
	if err != nil {
		log.Printf("statistics: resolving queue access for user %d: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
		return nil, statsScope{}, false
	}
	return db, scope, true
}

// resolveStatsScope reuses the queue_ro middleware's result when it ran and
// otherwise asks the queue access service with the same rules.
func resolveStatsScope(c *gin.Context, db *sql.DB, userID int) (statsScope, error) {
	if isAdmin, exists := c.Get("is_queue_admin"); exists {
		if isAdmin == true {
			return statsScope{all: true}, nil
		}
		if ids, ok := c.Get("accessible_queue_ids"); ok {
			if uids, ok := ids.([]uint); ok {
				scope := statsScope{queueIDs: make([]int, len(uids))}
				for i, id := range uids {
					scope.queueIDs[i] = int(id)
				}
				return scope, nil
			}
		}
	}

	svc := service.NewQueueAccessService(db)
	ctx := c.Request.Context()
	isAdmin, err := svc.IsAdmin(ctx, uint(userID))
	if err != nil {
		return statsScope{}, err
	}
	if isAdmin {
		return statsScope{all: true}, nil
	}
	uids, err := svc.GetAccessibleQueueIDs(ctx, uint(userID), "ro")
	if err != nil {
		return statsScope{}, err
	}
	scope := statsScope{queueIDs: make([]int, len(uids))}
	for i, id := range uids {
		scope.queueIDs[i] = int(id)
	}
	return scope, nil
}

// statsDBError logs a failed statistics query and answers 500 instead of
// reporting zeros that would hide the failure.
func statsDBError(c *gin.Context, what string, err error) {
	log.Printf("statistics: %s: %v", what, err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load statistics"})
}

// statsIntParam reads a positive integer query parameter (default def, at most max).
// On a bad value it answers 400 and returns ok=false.
func statsIntParam(c *gin.Context, name string, def, max int) (int, bool) {
	raw := c.Query(name)
	if raw == "" {
		return def, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 || n > max {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("%s must be an integer between 1 and %d", name, max)})
		return 0, false
	}
	return n, true
}

// statsLookback maps the period query parameter shared by the agents and export
// endpoints to a duration. On a bad value it answers 400 and returns ok=false.
func statsLookback(c *gin.Context) (string, time.Duration, bool) {
	period := c.DefaultQuery("period", "7d")
	switch period {
	case "24h":
		return period, 24 * time.Hour, true
	case "7d":
		return period, 7 * 24 * time.Hour, true
	case "30d":
		return period, 30 * 24 * time.Hour, true
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": "period must be one of 24h, 7d, 30d"})
	return "", 0, false
}

// Statistics state buckets, derived from ticket_state_type.name. State type IDs
// differ between GoatFlow's seed data and OTRS imports, so only names are used.
const (
	statsOpen    = "open"    // state types new and open
	statsClosed  = "closed"  // state type closed
	statsPending = "pending" // state types pending reminder and pending auto
)

func ticketStateCategory(stateTypeName string) string {
	switch {
	case stateTypeName == "new" || stateTypeName == "open":
		return statsOpen
	case stateTypeName == "closed":
		return statsClosed
	case strings.HasPrefix(stateTypeName, "pending"):
		return statsPending
	}
	return ""
}

// statsCounts holds ticket counts per state bucket.
type statsCounts struct {
	total, open, closed, pending int
}

// countTicketsByCategory counts the tickets in scope, optionally only those
// created at or after since.
func countTicketsByCategory(db *sql.DB, scope statsScope, since *time.Time) (statsCounts, error) {
	filter, args := scope.filter("t.queue_id")
	query := `
		SELECT tst.name, COUNT(t.id)
		FROM ticket t
		JOIN ticket_state ts ON ts.id = t.ticket_state_id
		JOIN ticket_state_type tst ON tst.id = ts.type_id
		WHERE ` + filter
	if since != nil {
		query += " AND t.create_time >= ?"
		args = append(args, *since)
	}
	query += " GROUP BY tst.name"

	rows, err := db.Query(database.ConvertPlaceholders(query), args...)
	if err != nil {
		return statsCounts{}, err
	}
	defer rows.Close()

	var counts statsCounts
	for rows.Next() {
		var (
			typeName string
			n        int
		)
		if err := rows.Scan(&typeName, &n); err != nil {
			return statsCounts{}, err
		}
		counts.total += n
		switch ticketStateCategory(typeName) {
		case statsOpen:
			counts.open += n
		case statsClosed:
			counts.closed += n
		case statsPending:
			counts.pending += n
		}
	}
	return counts, rows.Err()
}

// statsTicket is one ticket as seen by the statistics aggregations.
type statsTicket struct {
	queueID       int
	responsibleID int
	customer      string
	created       time.Time
	changed       time.Time
	category      string
}

// closedAt reports when a closed ticket was closed. The ticket's last change
// stands in for the close time.
func (t statsTicket) closedAt() (time.Time, bool) {
	return t.changed, t.category == statsClosed
}

// loadStatsTickets loads the tickets in scope. With since set, only tickets
// created or changed at or after since are returned.
func loadStatsTickets(db *sql.DB, scope statsScope, since *time.Time) ([]statsTicket, error) {
	filter, args := scope.filter("t.queue_id")
	query := `
		SELECT t.queue_id, t.responsible_user_id, t.customer_user_id, t.create_time, t.change_time, tst.name
		FROM ticket t
		JOIN ticket_state ts ON ts.id = t.ticket_state_id
		JOIN ticket_state_type tst ON tst.id = ts.type_id
		WHERE ` + filter
	if since != nil {
		query += " AND (t.create_time >= ? OR t.change_time >= ?)"
		args = append(args, *since, *since)
	}

	rows, err := db.Query(database.ConvertPlaceholders(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tickets []statsTicket
	for rows.Next() {
		var (
			tk       statsTicket
			customer sql.NullString
			typeName string
		)
		if err := rows.Scan(&tk.queueID, &tk.responsibleID, &customer, &tk.created, &tk.changed, &typeName); err != nil {
			return nil, err
		}
		tk.customer = strings.TrimSpace(customer.String)
		tk.created = tk.created.UTC()
		tk.changed = tk.changed.UTC()
		tk.category = ticketStateCategory(typeName)
		tickets = append(tickets, tk)
	}
	return tickets, rows.Err()
}

// HandleDashboardStatisticsAPI handles GET /api/v1/statistics/dashboard.
//
//	@Summary		Get dashboard statistics
//	@Description	Retrieve dashboard statistics (ticket counts, trends) - RBAC filtered
//	@Tags			Statistics
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}	"Dashboard statistics"
//	@Failure		401	{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/statistics/dashboard [get]
func HandleDashboardStatisticsAPI(c *gin.Context) {
	db, scope, ok := statisticsScope(c)
	if !ok {
		return
	}

	overview, err := countTicketsByCategory(db, scope, nil)
	if err != nil {
		statsDBError(c, "dashboard overview", err)
		return
	}

	queueFilter, queueArgs := scope.filter("q.id")
	byQueue := []gin.H{}
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT q.id, q.name, COUNT(t.id) AS cnt
		FROM queue q
		LEFT JOIN ticket t ON t.queue_id = q.id
		WHERE q.valid_id = 1 AND `+queueFilter+`
		GROUP BY q.id, q.name
		ORDER BY cnt DESC, q.name`), queueArgs...)
	if err != nil {
		statsDBError(c, "dashboard by queue", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var (
			queueID, count int
			name           string
		)
		if err := rows.Scan(&queueID, &name, &count); err != nil {
			statsDBError(c, "dashboard by queue", err)
			return
		}
		byQueue = append(byQueue, gin.H{"queue_id": queueID, "queue_name": name, "count": count})
	}
	if err := rows.Err(); err != nil {
		statsDBError(c, "dashboard by queue", err)
		return
	}

	ticketFilter, ticketArgs := scope.filter("t.queue_id")
	byPriority := []gin.H{}
	prows, err := db.Query(database.ConvertPlaceholders(`
		SELECT p.id, p.name, COUNT(t.id) AS cnt
		FROM ticket_priority p
		LEFT JOIN ticket t ON t.ticket_priority_id = p.id AND `+ticketFilter+`
		WHERE p.valid_id = 1
		GROUP BY p.id, p.name
		ORDER BY p.id`), ticketArgs...)
	if err != nil {
		statsDBError(c, "dashboard by priority", err)
		return
	}
	defer prows.Close()
	for prows.Next() {
		var (
			priorityID, count int
			name              string
		)
		if err := prows.Scan(&priorityID, &name, &count); err != nil {
			statsDBError(c, "dashboard by priority", err)
			return
		}
		byPriority = append(byPriority, gin.H{"priority_id": priorityID, "priority_name": name, "count": count})
	}
	if err := prows.Err(); err != nil {
		statsDBError(c, "dashboard by priority", err)
		return
	}

	recentActivity := []gin.H{}
	arows, err := db.Query(database.ConvertPlaceholders(`
		SELECT t.id, t.tn, t.create_time
		FROM ticket t
		WHERE `+ticketFilter+`
		ORDER BY t.create_time DESC, t.id DESC
		LIMIT 10`), ticketArgs...)
	if err != nil {
		statsDBError(c, "dashboard recent activity", err)
		return
	}
	defer arows.Close()
	for arows.Next() {
		var (
			ticketID int64
			tn       string
			ts       time.Time
		)
		if err := arows.Scan(&ticketID, &tn, &ts); err != nil {
			statsDBError(c, "dashboard recent activity", err)
			return
		}
		recentActivity = append(recentActivity, gin.H{
			"type":      "created",
			"ticket_id": ticketID,
			"ticket_tn": tn,
			"timestamp": ts.UTC(),
		})
	}
	if err := arows.Err(); err != nil {
		statsDBError(c, "dashboard recent activity", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"overview": gin.H{
			"total_tickets":   overview.total,
			"open_tickets":    overview.open,
			"closed_tickets":  overview.closed,
			"pending_tickets": overview.pending,
		},
		"by_queue":        byQueue,
		"by_priority":     byPriority,
		"recent_activity": recentActivity,
	})
}

// HandleTicketTrendsAPI handles GET /api/v1/statistics/trends.
//
//	@Summary		Get ticket trends
//	@Description	Retrieve ticket creation/resolution trends over time
//	@Tags			Statistics
//	@Accept			json
//	@Produce		json
//	@Param			period	query		string	false	"Time period (day, week, month)"
//	@Success		200		{object}	map[string]interface{}	"Trend data"
//	@Failure		401		{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/statistics/trends [get]
func HandleTicketTrendsAPI(c *gin.Context) {
	db, scope, ok := statisticsScope(c)
	if !ok {
		return
	}

	period := c.DefaultQuery("period", "daily")
	if period != "daily" && period != "monthly" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "period must be daily or monthly"})
		return
	}

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	var (
		start   time.Time
		buckets []string
		keyFmt  string
		size    int
	)
	if period == "daily" {
		days, ok := statsIntParam(c, "days", 7, 366)
		if !ok {
			return
		}
		size, keyFmt = days, "2006-01-02"
		start = today.AddDate(0, 0, -(days - 1))
		for d := start; !d.After(today); d = d.AddDate(0, 0, 1) {
			buckets = append(buckets, d.Format(keyFmt))
		}
	} else {
		months, ok := statsIntParam(c, "months", 3, 24)
		if !ok {
			return
		}
		size, keyFmt = months, "2006-01"
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -(months - 1), 0)
		for m := start; !m.After(today); m = m.AddDate(0, 1, 0) {
			buckets = append(buckets, m.Format(keyFmt))
		}
	}

	tickets, err := loadStatsTickets(db, scope, &start)
	if err != nil {
		statsDBError(c, "trends", err)
		return
	}

	createdCounts := map[string]int{}
	closedCounts := map[string]int{}
	for _, tk := range tickets {
		createdCounts[tk.created.Format(keyFmt)]++
		if closedAt, closed := tk.closedAt(); closed {
			closedCounts[closedAt.Format(keyFmt)]++
		}
	}

	trends := make([]gin.H, 0, len(buckets))
	var totalCreated, totalClosed, open int
	for _, key := range buckets {
		created, closed := createdCounts[key], closedCounts[key]
		totalCreated += created
		totalClosed += closed
		open += created - closed
		if open < 0 {
			open = 0
		}
		trends = append(trends, gin.H{"date": key, "created": created, "closed": closed, "open": open})
	}

	windowDays := int(today.Sub(start).Hours()/24) + 1
	closureRate := 0.0
	if totalCreated > 0 {
		closureRate = float64(totalClosed) / float64(totalCreated) * 100
	}
	sizeKey := "days"
	if period == "monthly" {
		sizeKey = "months"
	}
	c.JSON(http.StatusOK, gin.H{
		"period": period,
		sizeKey:  size,
		"trends": trends,
		"summary": gin.H{
			"total_created":   totalCreated,
			"total_closed":    totalClosed,
			"average_per_day": float64(totalCreated) / float64(windowDays),
			"closure_rate":    closureRate,
		},
	})
}

// HandleAgentPerformanceAPI handles GET /api/v1/statistics/agents.
//
//	@Summary		Get agent performance
//	@Description	Get agent performance statistics
//	@Tags			Statistics
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}	"Agent performance data"
//	@Failure		401	{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/statistics/agents [get]
func HandleAgentPerformanceAPI(c *gin.Context) {
	db, scope, ok := statisticsScope(c)
	if !ok {
		return
	}
	period, lookback, ok := statsLookback(c)
	if !ok {
		return
	}
	start := time.Now().UTC().Add(-lookback)

	type agentStats struct {
		id       int
		name     string
		assigned int
		closed   int
		articles int
	}

	userRows, err := db.Query(database.ConvertPlaceholders("SELECT id, login FROM users WHERE valid_id = 1"))
	if err != nil {
		statsDBError(c, "agents", err)
		return
	}
	defer userRows.Close()
	agentMap := make(map[int]*agentStats)
	for userRows.Next() {
		var a agentStats
		if err := userRows.Scan(&a.id, &a.name); err != nil {
			statsDBError(c, "agents", err)
			return
		}
		agentMap[a.id] = &a
	}
	if err := userRows.Err(); err != nil {
		statsDBError(c, "agents", err)
		return
	}

	tickets, err := loadStatsTickets(db, scope, &start)
	if err != nil {
		statsDBError(c, "agent tickets", err)
		return
	}
	for _, tk := range tickets {
		stats, ok := agentMap[tk.responsibleID]
		if !ok {
			continue
		}
		if !tk.created.Before(start) {
			stats.assigned++
		}
		if closedAt, closed := tk.closedAt(); closed && !closedAt.Before(start) {
			stats.closed++
		}
	}

	articleFilter, articleArgs := scope.filter("t.queue_id")
	articleRows, err := db.Query(database.ConvertPlaceholders(`
		SELECT a.create_by, COUNT(*)
		FROM article a
		JOIN ticket t ON a.ticket_id = t.id
		WHERE a.create_time >= ? AND `+articleFilter+`
		GROUP BY a.create_by`), append([]interface{}{start}, articleArgs...)...)
	if err != nil {
		statsDBError(c, "agent articles", err)
		return
	}
	defer articleRows.Close()
	for articleRows.Next() {
		var creatorID, count int
		if err := articleRows.Scan(&creatorID, &count); err != nil {
			statsDBError(c, "agent articles", err)
			return
		}
		if stats, ok := agentMap[creatorID]; ok {
			stats.articles = count
		}
	}
	if err := articleRows.Err(); err != nil {
		statsDBError(c, "agent articles", err)
		return
	}

	agentList := make([]agentStats, 0, len(agentMap))
	for _, stats := range agentMap {
		agentList = append(agentList, *stats)
	}
	sort.Slice(agentList, func(i, j int) bool {
		if agentList[i].closed != agentList[j].closed {
			return agentList[i].closed > agentList[j].closed
		}
		if agentList[i].assigned != agentList[j].assigned {
			return agentList[i].assigned > agentList[j].assigned
		}
		return agentList[i].name < agentList[j].name
	})

	agents := make([]gin.H, 0, len(agentList))
	topPerformers := make([]gin.H, 0, 3)
	for _, stats := range agentList {
		agents = append(agents, gin.H{
			"agent_id":         stats.id,
			"agent_name":       stats.name,
			"tickets_assigned": stats.assigned,
			"tickets_closed":   stats.closed,
			"articles_created": stats.articles,
		})
		if len(topPerformers) < 3 && stats.closed > 0 {
			topPerformers = append(topPerformers, gin.H{
				"agent_id":   stats.id,
				"agent_name": stats.name,
				"metric":     "tickets_closed",
				"value":      float64(stats.closed),
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"period":         period,
		"agents":         agents,
		"top_performers": topPerformers,
	})
}

// HandleQueueMetricsAPI handles GET /api/v1/statistics/queues.
//
//	@Summary		Get queue metrics
//	@Description	Get queue performance metrics
//	@Tags			Statistics
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}	"Queue metrics"
//	@Failure		401	{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/statistics/queues [get]
func HandleQueueMetricsAPI(c *gin.Context) {
	db, scope, ok := statisticsScope(c)
	if !ok {
		return
	}

	type queueStats struct {
		id      int
		name    string
		total   int
		open    int
		backlog int
	}

	queueFilter, queueArgs := scope.filter("id")
	queueRows, err := db.Query(database.ConvertPlaceholders("SELECT id, name FROM queue WHERE valid_id = 1 AND "+queueFilter), queueArgs...)
	if err != nil {
		statsDBError(c, "queues", err)
		return
	}
	defer queueRows.Close()
	queueMap := make(map[int]*queueStats)
	for queueRows.Next() {
		var q queueStats
		if err := queueRows.Scan(&q.id, &q.name); err != nil {
			statsDBError(c, "queues", err)
			return
		}
		queueMap[q.id] = &q
	}
	if err := queueRows.Err(); err != nil {
		statsDBError(c, "queues", err)
		return
	}

	tickets, err := loadStatsTickets(db, scope, nil)
	if err != nil {
		statsDBError(c, "queue tickets", err)
		return
	}
	// Backlog: open tickets older than a day.
	threshold := time.Now().UTC().Add(-24 * time.Hour)
	for _, tk := range tickets {
		stats, ok := queueMap[tk.queueID]
		if !ok {
			continue
		}
		stats.total++
		if tk.category == statsOpen {
			stats.open++
			if tk.created.Before(threshold) {
				stats.backlog++
			}
		}
	}

	queueList := make([]queueStats, 0, len(queueMap))
	for _, stats := range queueMap {
		queueList = append(queueList, *stats)
	}
	sort.Slice(queueList, func(i, j int) bool {
		if queueList[i].total == queueList[j].total {
			return queueList[i].name < queueList[j].name
		}
		return queueList[i].total > queueList[j].total
	})

	queues := make([]gin.H, 0, len(queueList))
	var totalTickets, totalOpen int
	for _, stats := range queueList {
		queues = append(queues, gin.H{
			"queue_id":      stats.id,
			"queue_name":    stats.name,
			"total_tickets": stats.total,
			"open_tickets":  stats.open,
			"backlog":       stats.backlog,
		})
		totalTickets += stats.total
		totalOpen += stats.open
	}

	c.JSON(http.StatusOK, gin.H{
		"queues": queues,
		"totals": gin.H{
			"all_queues":    len(queueList),
			"total_tickets": totalTickets,
			"total_open":    totalOpen,
		},
	})
}

// HandleTimeBasedAnalyticsAPI handles GET /api/v1/statistics/analytics.
//
//	@Summary		Get time-based analytics
//	@Description	Get time-based ticket analytics
//	@Tags			Statistics
//	@Accept			json
//	@Produce		json
//	@Param			period	query		string	false	"Time period"
//	@Success		200		{object}	map[string]interface{}	"Analytics data"
//	@Failure		401		{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/statistics/analytics [get]
func HandleTimeBasedAnalyticsAPI(c *gin.Context) {
	db, scope, ok := statisticsScope(c)
	if !ok {
		return
	}

	analysisType := c.DefaultQuery("type", "hourly")
	if analysisType != "hourly" && analysisType != "day_of_week" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type must be hourly or day_of_week"})
		return
	}
	days, ok := statsIntParam(c, "days", 30, 366)
	if !ok {
		return
	}
	start := time.Now().UTC().AddDate(0, 0, -days)

	tickets, err := loadStatsTickets(db, scope, &start)
	if err != nil {
		statsDBError(c, "analytics", err)
		return
	}

	// Buckets are UTC hours of the day (0-23) or weekdays (Monday first).
	nBuckets := 24
	bucket := func(ts time.Time) int { return ts.Hour() }
	if analysisType == "day_of_week" {
		nBuckets = 7
		bucket = func(ts time.Time) int { return (int(ts.Weekday()) + 6) % 7 }
	}
	created := make([]int, nBuckets)
	closed := make([]int, nBuckets)
	for _, tk := range tickets {
		if !tk.created.Before(start) {
			created[bucket(tk.created)]++
		}
		if closedAt, isClosed := tk.closedAt(); isClosed && !closedAt.Before(start) {
			closed[bucket(closedAt)]++
		}
	}

	// Peak buckets: those with the most created tickets (none when nothing was created).
	peaks := []int{}
	maxCreated := 0
	for i, n := range created {
		if n > maxCreated {
			maxCreated, peaks = n, []int{i}
		} else if n == maxCreated && n > 0 {
			peaks = append(peaks, i)
		}
	}

	data := make([]gin.H, nBuckets)
	if analysisType == "hourly" {
		for hour := range data {
			data[hour] = gin.H{"hour": hour, "created": created[hour], "closed": closed[hour]}
		}
		c.JSON(http.StatusOK, gin.H{"type": analysisType, "days": days, "data": data, "peak_hours": peaks})
		return
	}

	dayName := func(i int) string { return time.Weekday((i + 1) % 7).String() }
	busiestDays := make([]string, len(peaks))
	for i, p := range peaks {
		busiestDays[i] = dayName(p)
	}
	for i := range data {
		data[i] = gin.H{"day": dayName(i), "created": created[i], "closed": closed[i]}
	}
	c.JSON(http.StatusOK, gin.H{"type": analysisType, "days": days, "data": data, "busiest_days": busiestDays})
}

// HandleCustomerStatisticsAPI handles GET /api/v1/statistics/customers.
//
//	@Summary		Get customer statistics
//	@Description	Get customer-related statistics
//	@Tags			Statistics
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}	"Customer statistics"
//	@Failure		401	{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/statistics/customers [get]
func HandleCustomerStatisticsAPI(c *gin.Context) {
	db, scope, ok := statisticsScope(c)
	if !ok {
		return
	}

	topInt := 10
	if raw := c.Query("top"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "top must be an integer"})
			return
		}
		topInt = n
	}

	tickets, err := loadStatsTickets(db, scope, nil)
	if err != nil {
		statsDBError(c, "customers", err)
		return
	}

	type customerStats struct {
		id           string
		ticketCount  int
		openTickets  int
		firstTicket  time.Time
		lastActivity time.Time
	}

	now := time.Now().UTC()
	activeThreshold := now.AddDate(0, 0, -30)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	totalTickets := 0
	customerMap := make(map[string]*customerStats)
	for _, tk := range tickets {
		if tk.customer == "" {
			continue
		}
		stats, ok := customerMap[tk.customer]
		if !ok {
			stats = &customerStats{id: tk.customer, firstTicket: tk.created}
			customerMap[tk.customer] = stats
		}
		stats.ticketCount++
		totalTickets++
		if tk.category == statsOpen {
			stats.openTickets++
		}
		if tk.created.After(stats.lastActivity) {
			stats.lastActivity = tk.created
		}
		if tk.created.Before(stats.firstTicket) {
			stats.firstTicket = tk.created
		}
	}

	customerList := make([]customerStats, 0, len(customerMap))
	var activeCustomers, newThisMonth int
	for _, stats := range customerMap {
		customerList = append(customerList, *stats)
		if stats.lastActivity.After(activeThreshold) {
			activeCustomers++
		}
		if !stats.firstTicket.Before(monthStart) {
			newThisMonth++
		}
	}
	sort.Slice(customerList, func(i, j int) bool {
		if customerList[i].ticketCount == customerList[j].ticketCount {
			return customerList[i].lastActivity.After(customerList[j].lastActivity)
		}
		return customerList[i].ticketCount > customerList[j].ticketCount
	})

	limit := topInt
	if limit <= 0 || limit > len(customerList) {
		limit = len(customerList)
	}
	top := customerList[:limit]

	// Registered customers' e-mail addresses (unregistered ones have none).
	emails := map[string]string{}
	if len(top) > 0 {
		placeholders := make([]string, len(top))
		args := make([]interface{}, len(top))
		for i, stats := range top {
			placeholders[i] = "?"
			args[i] = stats.id
		}
		rows, err := db.Query(database.ConvertPlaceholders(
			"SELECT login, email FROM customer_user WHERE login IN ("+strings.Join(placeholders, ",")+")"), args...)
		if err != nil {
			statsDBError(c, "customer e-mails", err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var login, email string
			if err := rows.Scan(&login, &email); err != nil {
				statsDBError(c, "customer e-mails", err)
				return
			}
			emails[login] = email
		}
		if err := rows.Err(); err != nil {
			statsDBError(c, "customer e-mails", err)
			return
		}
	}

	topCustomers := make([]gin.H, 0, len(top))
	for _, stats := range top {
		topCustomers = append(topCustomers, gin.H{
			"customer_id":    stats.id,
			"customer_email": emails[stats.id],
			"ticket_count":   stats.ticketCount,
			"open_tickets":   stats.openTickets,
			"last_activity":  stats.lastActivity.Format(time.RFC3339),
		})
	}

	avgTicketsPerCustomer := 0.0
	if len(customerList) > 0 {
		avgTicketsPerCustomer = float64(totalTickets) / float64(len(customerList))
	}
	c.JSON(http.StatusOK, gin.H{
		"top_customers": topCustomers,
		"customer_metrics": gin.H{
			"total_customers":          len(customerList),
			"active_customers":         activeCustomers,
			"new_customers_this_month": newThisMonth,
			"avg_tickets_per_customer": avgTicketsPerCustomer,
		},
	})
}

// HandleExportStatisticsAPI handles GET /api/v1/statistics/export.
//
//	@Summary		Export statistics
//	@Description	Export statistics data
//	@Tags			Statistics
//	@Accept			json
//	@Produce		json
//	@Param			format	query		string	false	"Export format (csv, json)"
//	@Success		200		{object}	map[string]interface{}	"Exported data"
//	@Failure		401		{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/statistics/export [get]
func HandleExportStatisticsAPI(c *gin.Context) {
	db, scope, ok := statisticsScope(c)
	if !ok {
		return
	}

	format := c.DefaultQuery("format", "json")
	if format != "json" && format != "csv" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "format must be json or csv"})
		return
	}
	exportType := c.DefaultQuery("type", "summary")
	if exportType != "summary" && exportType != "tickets" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type must be summary or tickets"})
		return
	}
	period, lookback, ok := statsLookback(c)
	if !ok {
		return
	}
	now := time.Now().UTC()
	start := now.Add(-lookback)

	var (
		data      interface{}
		csvHeader []string
		csvRows   [][]string
	)
	if exportType == "tickets" {
		filter, args := scope.filter("t.queue_id")
		rows, err := db.Query(database.ConvertPlaceholders(`
			SELECT t.tn, t.title, q.name, ts.name, tp.name, t.customer_user_id, t.create_time
			FROM ticket t
			JOIN queue q ON t.queue_id = q.id
			JOIN ticket_state ts ON t.ticket_state_id = ts.id
			JOIN ticket_priority tp ON t.ticket_priority_id = tp.id
			WHERE t.create_time >= ? AND `+filter+`
			ORDER BY t.create_time DESC, t.id DESC`), append([]interface{}{start}, args...)...)
		if err != nil {
			statsDBError(c, "export tickets", err)
			return
		}
		defer rows.Close()

		tickets := []gin.H{}
		csvHeader = []string{"Ticket Number", "Title", "Queue", "State", "Priority", "Customer", "Created"}
		for rows.Next() {
			var (
				tn, queue, state, priority string
				title, customer            sql.NullString
				created                    time.Time
			)
			if err := rows.Scan(&tn, &title, &queue, &state, &priority, &customer, &created); err != nil {
				statsDBError(c, "export tickets", err)
				return
			}
			createdStr := created.UTC().Format("2006-01-02 15:04:05")
			tickets = append(tickets, gin.H{
				"ticket_number": tn,
				"title":         title.String,
				"queue":         queue,
				"state":         state,
				"priority":      priority,
				"customer":      customer.String,
				"created":       createdStr,
			})
			csvRows = append(csvRows, []string{tn, title.String, queue, state, priority, customer.String, createdStr})
		}
		if err := rows.Err(); err != nil {
			statsDBError(c, "export tickets", err)
			return
		}
		data = tickets
	} else {
		counts, err := countTicketsByCategory(db, scope, &start)
		if err != nil {
			statsDBError(c, "export summary", err)
			return
		}
		data = gin.H{
			"export_date": now.Format(time.RFC3339),
			"period":      period,
			"type":        exportType,
			"summary": gin.H{
				"total_tickets":   counts.total,
				"open_tickets":    counts.open,
				"closed_tickets":  counts.closed,
				"pending_tickets": counts.pending,
			},
		}
		csvHeader = []string{"Metric", "Value"}
		csvRows = [][]string{
			{"total_tickets", strconv.Itoa(counts.total)},
			{"open_tickets", strconv.Itoa(counts.open)},
			{"closed_tickets", strconv.Itoa(counts.closed)},
			{"pending_tickets", strconv.Itoa(counts.pending)},
		}
	}

	stamp := now.Format("20060102_150405")
	if format == "csv" {
		var buf bytes.Buffer
		writer := csv.NewWriter(&buf)
		if err := writer.Write(csvHeader); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write CSV"})
			return
		}
		if err := writer.WriteAll(csvRows); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write CSV"})
			return
		}
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=statistics_%s.csv", stamp))
		c.Data(http.StatusOK, "text/csv", buf.Bytes())
		return
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal JSON"})
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=statistics_%s.json", stamp))
	c.Data(http.StatusOK, "application/json", jsonData)
}

// HandleTicketStateStatisticsAPI handles GET /api/v1/ticket-states/statistics.
//
//	@Summary		Get state statistics
//	@Description	Get ticket counts by state
//	@Tags			States
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}	"State statistics"
//	@Failure		401	{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/ticket-states/statistics [get]
func HandleTicketStateStatisticsAPI(c *gin.Context) {
	db, scope, ok := statisticsScope(c)
	if !ok {
		return
	}

	filter, args := scope.filter("t.queue_id")
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT ts.id, ts.name, ts.type_id, COUNT(t.id)
		FROM ticket_state ts
		LEFT JOIN ticket t ON t.ticket_state_id = ts.id AND `+filter+`
		WHERE ts.valid_id = 1
		GROUP BY ts.id, ts.name, ts.type_id
		ORDER BY ts.id`), args...)
	if err != nil {
		statsDBError(c, "ticket state statistics", err)
		return
	}
	defer rows.Close()

	statistics := []gin.H{}
	totalTickets := 0
	for rows.Next() {
		var stateID, typeID, count int
		var name string
		if err := rows.Scan(&stateID, &name, &typeID, &count); err != nil {
			statsDBError(c, "ticket state statistics", err)
			return
		}
		statistics = append(statistics, gin.H{
			"state_id":     stateID,
			"state_name":   name,
			"type_id":      typeID,
			"ticket_count": count,
		})
		totalTickets += count
	}
	if err := rows.Err(); err != nil {
		statsDBError(c, "ticket state statistics", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"statistics":    statistics,
		"total_tickets": totalTickets,
	})
}
