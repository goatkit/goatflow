package api

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleDashboardStatisticsAPI handles GET /api/v1/statistics/dashboard.
func HandleDashboardStatisticsAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	_ = userID

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Overview counts using portable queries
	var overview struct {
		TotalTickets   int
		OpenTickets    int
		ClosedTickets  int
		PendingTickets int
	}

	if err := db.QueryRow("SELECT COUNT(*) FROM ticket").Scan(&overview.TotalTickets); err != nil {
		overview.TotalTickets = 0
	}

	stateID := func(name string) int {
		var id int
		query := database.ConvertPlaceholders("SELECT id FROM ticket_state WHERE name = $1")
		if err := db.QueryRow(query, name).Scan(&id); err != nil {
			return 0
		}
		return id
	}

	if id := stateID("open"); id > 0 {
		query := database.ConvertPlaceholders("SELECT COUNT(*) FROM ticket WHERE ticket_state_id = $1")
		if err := db.QueryRow(query, id).Scan(&overview.OpenTickets); err != nil {
			overview.OpenTickets = 0
		}
	}
	if id := stateID("closed"); id > 0 {
		query := database.ConvertPlaceholders("SELECT COUNT(*) FROM ticket WHERE ticket_state_id = $1")
		if err := db.QueryRow(query, id).Scan(&overview.ClosedTickets); err != nil {
			overview.ClosedTickets = 0
		}
	}
	if id := stateID("pending"); id > 0 {
		query := database.ConvertPlaceholders("SELECT COUNT(*) FROM ticket WHERE ticket_state_id = $1")
		if err := db.QueryRow(query, id).Scan(&overview.PendingTickets); err != nil {
			overview.PendingTickets = 0
		}
	}

	// Tickets by queue
	byQueue := []gin.H{}
	queueQuery := `
		SELECT q.id, q.name, COUNT(t.id) AS cnt
		FROM queue q
		LEFT JOIN ticket t ON q.id = t.queue_id
		WHERE q.valid_id = 1
		GROUP BY q.id, q.name
		ORDER BY cnt DESC
	`
	if rows, err := db.Query(queueQuery); err == nil {
		defer rows.Close()
		for rows.Next() {
			var (
				queueID int
				name    string
				count   int64
			)
			if err := rows.Scan(&queueID, &name, &count); err == nil {
				byQueue = append(byQueue, gin.H{
					"queue_id":   queueID,
					"queue_name": name,
					"count":      int(count),
				})
			}
		}
		if err := rows.Err(); err != nil {
			// Log or handle iteration errors
		}
	}

	// Tickets by priority
	byPriority := []gin.H{}
	priorityQuery := `
		SELECT p.id, p.name, COUNT(t.id) AS cnt
		FROM ticket_priority p
		LEFT JOIN ticket t ON p.id = t.ticket_priority_id
		WHERE p.valid_id = 1
		GROUP BY p.id, p.name
		ORDER BY p.id
	`
	if rows, err := db.Query(priorityQuery); err == nil {
		defer rows.Close()
		for rows.Next() {
			var (
				priorityID int
				name       string
				count      int64
			)
			if err := rows.Scan(&priorityID, &name, &count); err == nil {
				byPriority = append(byPriority, gin.H{
					"priority_id":   priorityID,
					"priority_name": name,
					"count":         int(count),
				})
			}
		}
		if err := rows.Err(); err != nil {
			// Log or handle iteration errors
		}
	}

	// Recent activity
	recentActivity := []gin.H{}
	activityQuery := `
		SELECT 'created' AS type, t.id, t.tn, t.create_time
		FROM ticket t
		ORDER BY t.create_time DESC
		LIMIT 10
	`
	if rows, err := db.Query(activityQuery); err == nil {
		defer rows.Close()
		for rows.Next() {
			var (
				typeLabel string
				ticketID  int
				tn        string
				ts        time.Time
			)
			if err := rows.Scan(&typeLabel, &ticketID, &tn, &ts); err == nil {
				recentActivity = append(recentActivity, gin.H{
					"type":      typeLabel,
					"ticket_id": ticketID,
					"ticket_tn": tn,
					"timestamp": ts,
				})
			}
		}
		if err := rows.Err(); err != nil {
			// Log or handle iteration errors
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"overview": gin.H{
			"total_tickets":   overview.TotalTickets,
			"open_tickets":    overview.OpenTickets,
			"closed_tickets":  overview.ClosedTickets,
			"pending_tickets": overview.PendingTickets,
		},
		"by_queue":        byQueue,
		"by_priority":     byPriority,
		"recent_activity": recentActivity,
	})
}

// HandleTicketTrendsAPI handles GET /api/v1/statistics/trends.
func HandleTicketTrendsAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	_ = userID

	period := c.DefaultQuery("period", "daily")
	days := c.DefaultQuery("days", "7")
	months := c.DefaultQuery("months", "3")

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	trends := []gin.H{}
	var totalCreated, totalClosed int

	now := time.Now().UTC()

	if period == "daily" {
		daysInt, err := strconv.Atoi(days)
		if err != nil || daysInt <= 0 {
			daysInt = 7
		}

		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -(daysInt - 1))

		records := make([]struct {
			created time.Time
			changed sql.NullTime
			typeID  int
		}, 0)

		query := database.ConvertPlaceholders(`
			SELECT t.create_time, t.change_time, ts.type_id
			FROM ticket t
			JOIN ticket_state ts ON t.ticket_state_id = ts.id
			WHERE t.create_time >= $1 OR (t.change_time IS NOT NULL AND t.change_time >= $1)
		`)
		rows, err := db.Query(query, start)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var (
					createTime time.Time
					changeTime sql.NullTime
					typeID     int
				)
				if err := rows.Scan(&createTime, &changeTime, &typeID); err == nil {
					records = append(records, struct {
						created time.Time
						changed sql.NullTime
						typeID  int
					}{createTime, changeTime, typeID})
				}
			}
			if err := rows.Err(); err != nil {
				// Log or handle iteration errors
			}
		}

		createdCounts := map[string]int{}
		closedCounts := map[string]int{}

		for _, rec := range records {
			day := rec.created.In(time.UTC).Format("2006-01-02")
			createdCounts[day]++
			if rec.typeID == 2 && rec.changed.Valid {
				closeDay := rec.changed.Time.In(time.UTC).Format("2006-01-02")
				closedCounts[closeDay]++
			}
		}

		open := 0
		for dateCursor := start; !dateCursor.After(now); dateCursor = dateCursor.AddDate(0, 0, 1) {
			key := dateCursor.Format("2006-01-02")
			created := createdCounts[key]
			closed := closedCounts[key]
			totalCreated += created
			totalClosed += closed
			open += created - closed
			if open < 0 {
				open = 0
			}
			trends = append(trends, gin.H{
				"date":    key,
				"created": created,
				"closed":  closed,
				"open":    open,
			})
		}

		daysIntValidated := len(trends)
		response := gin.H{
			"period": period,
			"days":   daysInt,
			"trends": trends,
			"summary": gin.H{
				"total_created": totalCreated,
				"total_closed":  totalClosed,
				"average_per_day": func() float64 {
					if daysIntValidated == 0 {
						return 0
					}
					return float64(totalCreated) / float64(daysIntValidated)
				}(),
				"closure_rate": func() float64 {
					if totalCreated == 0 {
						return 0
					}
					return float64(totalClosed) / float64(totalCreated) * 100
				}(),
			},
		}
		c.JSON(http.StatusOK, response)
		return
	}

	// Monthly trends fallback
	monthsInt, err := strconv.Atoi(months)
	if err != nil || monthsInt <= 0 {
		monthsInt = 3
	}

	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -(monthsInt - 1), 0)

	records := make([]struct {
		created time.Time
		changed sql.NullTime
		typeID  int
	}, 0)

	query := database.ConvertPlaceholders(`
		SELECT t.create_time, t.change_time, ts.type_id
		FROM ticket t
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		WHERE t.create_time >= $1 OR (t.change_time IS NOT NULL AND t.change_time >= $1)
	`)
	if rows, err := db.Query(query, firstOfMonth); err == nil {
		defer rows.Close()
		for rows.Next() {
			var (
				createTime time.Time
				changeTime sql.NullTime
				typeID     int
			)
			if err := rows.Scan(&createTime, &changeTime, &typeID); err == nil {
				records = append(records, struct {
					created time.Time
					changed sql.NullTime
					typeID  int
				}{createTime, changeTime, typeID})
			}
		}
		if err := rows.Err(); err != nil {
			// Log or handle iteration errors
		}
	}

	createdCounts := map[string]int{}
	closedCounts := map[string]int{}

	for _, rec := range records {
		monthKey := rec.created.In(time.UTC).Format("2006-01")
		createdCounts[monthKey]++
		if rec.typeID == 2 && rec.changed.Valid {
			closeKey := rec.changed.Time.In(time.UTC).Format("2006-01")
			closedCounts[closeKey]++
		}
	}

	current := firstOfMonth
	open := 0
	for i := 0; i < monthsInt; i++ {
		key := current.Format("2006-01")
		created := createdCounts[key]
		closed := closedCounts[key]
		totalCreated += created
		totalClosed += closed
		open += created - closed
		if open < 0 {
			open = 0
		}
		trends = append(trends, gin.H{
			"date":    key,
			"created": created,
			"closed":  closed,
			"open":    open,
		})
		current = current.AddDate(0, 1, 0)
	}

	response := gin.H{
		"period": period,
		"months": monthsInt,
		"trends": trends,
		"summary": gin.H{
			"total_created": totalCreated,
			"total_closed":  totalClosed,
			"average_per_day": func() float64 {
				if len(trends) == 0 {
					return 0
				}
				return float64(totalCreated) / float64(len(trends))
			}(),
			"closure_rate": func() float64 {
				if totalCreated == 0 {
					return 0
				}
				return float64(totalClosed) / float64(totalCreated) * 100
			}(),
		},
	}

	c.JSON(http.StatusOK, response)
}

// HandleAgentPerformanceAPI handles GET /api/v1/statistics/agents.
func HandleAgentPerformanceAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	_ = userID

	period := c.DefaultQuery("period", "7d")

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Determine interval
	var interval time.Duration
	switch period {
	case "24h":
		interval = 24 * time.Hour
	case "30d":
		interval = 30 * 24 * time.Hour
	case "7d":
		interval = 7 * 24 * time.Hour
	default:
		interval = 7 * 24 * time.Hour
	}
	start := time.Now().UTC().Add(-interval)

	// Load active agents
	userRows, err := db.Query("SELECT id, login FROM users WHERE valid_id = 1")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load agents"})
		return
	}
	defer userRows.Close()

	type agentStats struct {
		id        int
		name      string
		assigned  int
		closed    int
		articles  int
		respHours float64
		reslHours float64
		satScore  float64
	}

	agentMap := make(map[int]*agentStats)
	for userRows.Next() {
		var (
			id   int
			name string
		)
		if err := userRows.Scan(&id, &name); err != nil {
			continue
		}
		agentMap[id] = &agentStats{id: id, name: name}
	}

	if err := userRows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read agents"})
		return
	}

	// Aggregate ticket assignments and closures
	ticketQuery := database.ConvertPlaceholders(`
		SELECT t.responsible_user_id, t.create_time, t.change_time, ts.type_id
		FROM ticket t
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		WHERE t.responsible_user_id IS NOT NULL
		AND (t.create_time >= $1 OR (t.change_time IS NOT NULL AND t.change_time >= $1))
	`)
	ticketRows, err := db.Query(ticketQuery, start)
	if err == nil {
		defer ticketRows.Close()
		for ticketRows.Next() {
			var (
				agentID  int
				createAt time.Time
				changeAt sql.NullTime
				typeID   int
			)
			if err := ticketRows.Scan(&agentID, &createAt, &changeAt, &typeID); err != nil {
				continue
			}
			stats, ok := agentMap[agentID]
			if !ok {
				continue
			}
			if !createAt.Before(start) {
				stats.assigned++
			}
			if typeID == 2 && changeAt.Valid && !changeAt.Time.Before(start) {
				stats.closed++
			}
		}
		if err := ticketRows.Err(); err != nil {
			// Log or handle iteration errors
		}
	}

	// Aggregate article counts per agent
	articleQuery := database.ConvertPlaceholders(`
		SELECT create_by, COUNT(*)
		FROM article
		WHERE create_by IS NOT NULL AND create_time >= $1
		GROUP BY create_by
	`)
	articleRows, err := db.Query(articleQuery, start)
	if err == nil {
		defer articleRows.Close()
		for articleRows.Next() {
			var (
				creatorID int
				count     int
			)
			if err := articleRows.Scan(&creatorID, &count); err != nil {
				continue
			}
			if stats, ok := agentMap[creatorID]; ok {
				stats.articles = count
			}
		}
		if err := articleRows.Err(); err != nil {
			// Log or handle iteration errors
		}
	}

	agentList := make([]agentStats, 0, len(agentMap))
	for _, stats := range agentMap {
		agentList = append(agentList, *stats)
	}

	sort.Slice(agentList, func(i, j int) bool {
		if agentList[i].closed == agentList[j].closed {
			if agentList[i].assigned == agentList[j].assigned {
				return agentList[i].name < agentList[j].name
			}
			return agentList[i].assigned > agentList[j].assigned
		}
		return agentList[i].closed > agentList[j].closed
	})

	agents := make([]gin.H, 0, len(agentList))
	topPerformers := make([]gin.H, 0, 3)
	for _, stats := range agentList {
		agents = append(agents, gin.H{
			"agent_id":                  stats.id,
			"agent_name":                stats.name,
			"tickets_assigned":          stats.assigned,
			"tickets_closed":            stats.closed,
			"articles_created":          stats.articles,
			"avg_response_time_hours":   stats.respHours,
			"avg_resolution_time_hours": stats.reslHours,
			"customer_satisfaction":     stats.satScore,
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
func HandleQueueMetricsAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	_ = userID

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Load queues
	queueRows, err := db.Query("SELECT id, name FROM queue WHERE valid_id = 1")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load queues"})
		return
	}
	defer queueRows.Close()

	type queueStats struct {
		id      int
		name    string
		total   int
		open    int
		backlog int
	}

	queueMap := make(map[int]*queueStats)
	for queueRows.Next() {
		var (
			id   int
			name string
		)
		if err := queueRows.Scan(&id, &name); err != nil {
			continue
		}
		queueMap[id] = &queueStats{id: id, name: name}
	}

	if err := queueRows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read queues"})
		return
	}

	threshold := time.Now().UTC().Add(-24 * time.Hour)
	ticketRows, err := db.Query(`
		SELECT t.queue_id, t.create_time, ts.type_id
		FROM ticket t
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
	`)
	if err == nil {
		defer ticketRows.Close()
		for ticketRows.Next() {
			var (
				queueID   int
				createdAt time.Time
				typeID    int
			)
			if err := ticketRows.Scan(&queueID, &createdAt, &typeID); err != nil {
				continue
			}
			stats, ok := queueMap[queueID]
			if !ok {
				continue
			}
			stats.total++
			if typeID == 1 {
				stats.open++
				if createdAt.Before(threshold) {
					stats.backlog++
				}
			}
		}
		if err := ticketRows.Err(); err != nil {
			// Log or handle iteration errors
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
	var totalQueues, totalTickets, totalOpen int
	for _, stats := range queueList {
		queues = append(queues, gin.H{
			"queue_id":                  stats.id,
			"queue_name":                stats.name,
			"total_tickets":             stats.total,
			"open_tickets":              stats.open,
			"avg_wait_time_hours":       0.0,
			"avg_resolution_time_hours": 0.0,
			"backlog":                   stats.backlog,
			"sla_compliance_percent":    0.0,
		})
		totalQueues++
		totalTickets += stats.total
		totalOpen += stats.open
	}

	c.JSON(http.StatusOK, gin.H{
		"queues": queues,
		"totals": gin.H{
			"all_queues":                 totalQueues,
			"total_tickets":              totalTickets,
			"total_open":                 totalOpen,
			"overall_compliance_percent": 0.0,
		},
	})
}

// HandleTimeBasedAnalyticsAPI handles GET /api/v1/statistics/analytics.
func HandleTimeBasedAnalyticsAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	_ = userID

	analysisType := c.DefaultQuery("type", "hourly")

	if analysisType == "hourly" {
		// Hourly distribution
		data := []gin.H{}
		peakHours := []int{}
		maxCount := 0

		for hour := 0; hour < 24; hour++ {
			// Simplified - would query actual data
			created := 0
			closed := 0

			if hour >= 9 && hour <= 17 {
				created = 5 + hour%3
				closed = 3 + hour%2
			}

			data = append(data, gin.H{
				"hour":    hour,
				"created": created,
				"closed":  closed,
			})

			if created > maxCount {
				maxCount = created
				peakHours = []int{hour}
			} else if created == maxCount && created > 0 {
				peakHours = append(peakHours, hour)
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"type":       analysisType,
			"data":       data,
			"peak_hours": peakHours,
		})
	} else if analysisType == "day_of_week" {
		// Day of week distribution
		days := []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
		data := []gin.H{}
		busiestDays := []string{}

		for i, day := range days {
			// Simplified - would query actual data
			created := 10 - i
			closed := 8 - i

			if i < 5 { // Weekdays
				created *= 2
				closed *= 2
			}

			data = append(data, gin.H{
				"day":     day,
				"created": created,
				"closed":  closed,
			})

			if i < 2 {
				busiestDays = append(busiestDays, day)
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"type":         analysisType,
			"data":         data,
			"busiest_days": busiestDays,
		})
	}
}

// HandleCustomerStatisticsAPI handles GET /api/v1/statistics/customers.
func HandleCustomerStatisticsAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	_ = userID

	top := c.DefaultQuery("top", "10")
	topInt, err := strconv.Atoi(top)
	if err != nil {
		topInt = 10
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Aggregate customer statistics in code for portability
	rows, err := db.Query(`
		SELECT t.customer_user_id, t.create_time, ts.type_id
		FROM ticket t
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		WHERE t.customer_user_id IS NOT NULL AND t.customer_user_id <> ''
	`)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"top_customers": []gin.H{},
			"customer_metrics": gin.H{
				"total_customers":          0,
				"active_customers":         0,
				"new_customers_this_month": 0,
				"avg_tickets_per_customer": 0.0,
			},
		})
		return
	}
	defer rows.Close()

	type customerStats struct {
		id               string
		ticketCount      int
		openTickets      int
		lastActivity     time.Time
		activeRecent     bool
		createdThisMonth bool
	}

	now := time.Now().UTC()
	activeThreshold := now.AddDate(0, 0, -30)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	totalTickets := 0
	customerMap := make(map[string]*customerStats)

	for rows.Next() {
		var (
			customerID sql.NullString
			createdAt  time.Time
			typeID     int
		)
		if err := rows.Scan(&customerID, &createdAt, &typeID); err != nil {
			continue
		}
		if !customerID.Valid {
			continue
		}
		id := strings.TrimSpace(customerID.String)
		if id == "" {
			continue
		}
		stats, ok := customerMap[id]
		if !ok {
			stats = &customerStats{id: id}
			customerMap[id] = stats
		}

		stats.ticketCount++
		totalTickets++
		if typeID == 1 {
			stats.openTickets++
		}
		if createdAt.After(stats.lastActivity) {
			stats.lastActivity = createdAt
		}
		if createdAt.After(activeThreshold) {
			stats.activeRecent = true
		}
		if createdAt.After(monthStart) {
			stats.createdThisMonth = true
		}
	}

	if err := rows.Err(); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"top_customers": []gin.H{},
			"customer_metrics": gin.H{
				"total_customers":          0,
				"active_customers":         0,
				"new_customers_this_month": 0,
				"avg_tickets_per_customer": 0.0,
			},
		})
		return
	}

	customerList := make([]customerStats, 0, len(customerMap))
	var activeCustomers, newThisMonth int
	for _, stats := range customerMap {
		customerList = append(customerList, *stats)
		if stats.activeRecent {
			activeCustomers++
		}
		if stats.createdThisMonth {
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

	topCustomers := make([]gin.H, 0, limit)
	for idx := 0; idx < limit; idx++ {
		stats := customerList[idx]
		last := ""
		if !stats.lastActivity.IsZero() {
			last = stats.lastActivity.UTC().Format(time.RFC3339)
		}
		topCustomers = append(topCustomers, gin.H{
			"customer_id":    stats.id,
			"customer_email": stats.id,
			"ticket_count":   stats.ticketCount,
			"open_tickets":   stats.openTickets,
			"last_activity":  last,
		})
	}

	totalCustomers := len(customerList)
	avgTicketsPerCustomer := 0.0
	if totalCustomers > 0 {
		avgTicketsPerCustomer = float64(totalTickets) / float64(totalCustomers)
	}

	c.JSON(http.StatusOK, gin.H{
		"top_customers": topCustomers,
		"customer_metrics": gin.H{
			"total_customers":          totalCustomers,
			"active_customers":         activeCustomers,
			"new_customers_this_month": newThisMonth,
			"avg_tickets_per_customer": avgTicketsPerCustomer,
		},
	})
}

// HandleExportStatisticsAPI handles GET /api/v1/statistics/export.
func HandleExportStatisticsAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	_ = userID

	format := c.DefaultQuery("format", "json")
	exportType := c.DefaultQuery("type", "summary")
	period := c.DefaultQuery("period", "7d")

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Get data based on export type
	var data interface{}

	if exportType == "tickets" {
		// Determine lookback duration from period
		lookback := 7 * 24 * time.Hour
		switch period {
		case "24h":
			lookback = 24 * time.Hour
		case "30d":
			lookback = 30 * 24 * time.Hour
		case "7d":
			lookback = 7 * 24 * time.Hour
		}
		start := time.Now().UTC().Add(-lookback)

		query := database.ConvertPlaceholders(`
			SELECT t.tn, t.title, q.name as queue, ts.name as state,
			       tp.name as priority, t.customer_user_id, t.create_time
			FROM ticket t
			JOIN queue q ON t.queue_id = q.id
			JOIN ticket_state ts ON t.ticket_state_id = ts.id
			JOIN ticket_priority tp ON t.ticket_priority_id = tp.id
			WHERE t.create_time >= $1
			ORDER BY t.create_time DESC
		`)

		rows, err := db.Query(query, start)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load tickets"})
			return
		}
		defer rows.Close()

		tickets := []map[string]interface{}{}
		for rows.Next() {
			var ticket struct {
				TN           string
				Title        string
				Queue        string
				State        string
				Priority     string
				CustomerUser sql.NullString
				CreateTime   time.Time
			}

			if err := rows.Scan(
				&ticket.TN, &ticket.Title, &ticket.Queue,
				&ticket.State, &ticket.Priority, &ticket.CustomerUser,
				&ticket.CreateTime,
			); err == nil {
				customer := ""
				if ticket.CustomerUser.Valid {
					customer = ticket.CustomerUser.String
				}
				tickets = append(tickets, map[string]interface{}{
					"ticket_number": ticket.TN,
					"title":         ticket.Title,
					"queue":         ticket.Queue,
					"state":         ticket.State,
					"priority":      ticket.Priority,
					"customer":      customer,
					"created":       ticket.CreateTime.UTC().Format("2006-01-02 15:04:05"),
				})
			}
		}
		if err := rows.Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read tickets"})
			return
		}
		data = tickets
	} else {
		// Summary data
		data = gin.H{
			"export_date": time.Now().Format(time.RFC3339),
			"period":      period,
			"type":        exportType,
			"summary": gin.H{
				"total_tickets":  100,
				"open_tickets":   25,
				"closed_tickets": 75,
			},
		}
	}

	if format == "csv" {
		// Export as CSV
		var buf bytes.Buffer
		writer := csv.NewWriter(&buf)

		if tickets, ok := data.([]map[string]interface{}); ok && len(tickets) > 0 {
			// Write headers
			headers := []string{"Ticket Number", "Title", "Queue", "State", "Priority", "Customer", "Created"}
			if err := writer.Write(headers); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write CSV headers"})
				return
			}

			// Write data
			for _, ticket := range tickets {
				row := []string{
					safeString(ticket["ticket_number"]),
					safeString(ticket["title"]),
					safeString(ticket["queue"]),
					safeString(ticket["state"]),
					safeString(ticket["priority"]),
					safeString(ticket["customer"]),
					safeString(ticket["created"]),
				}
				if err := writer.Write(row); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write CSV row"})
					return
				}
			}
		}

		writer.Flush()

		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=statistics_%s.csv", time.Now().Format("20060102_150405")))
		c.Data(http.StatusOK, "text/csv", buf.Bytes())
	} else {
		// Export as JSON
		c.Header("Content-Type", "application/json")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=statistics_%s.json", time.Now().Format("20060102_150405")))

		jsonData, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal JSON"})
			return
		}
		c.Data(http.StatusOK, "application/json", jsonData)
	}
}
