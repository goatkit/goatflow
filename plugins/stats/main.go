//go:build tinygo.wasm

// Package main implements the stats WASM plugin for GoatKit.
// Provides ticket statistics dashboard widgets and API endpoints.
package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unsafe"
)

// Manifest defines the plugin's capabilities
var manifestJSON = `{
  "name": "stats",
  "version": "1.1.0",
  "description": "Ticket statistics and analytics",
  "author": "GoatFlow Team",
  "license": "Apache-2.0",
  "routes": [
    {
      "method": "GET",
      "path": "/api/plugins/stats/overview",
      "handler": "overview",
      "description": "Get ticket statistics overview (supports ?range=7d|30d|90d|all)",
      "middleware": ["auth"]
    },
    {
      "method": "GET",
      "path": "/api/plugins/stats/by-status",
      "handler": "by_status",
      "description": "Get ticket counts by status",
      "middleware": ["auth"]
    },
    {
      "method": "GET",
      "path": "/api/plugins/stats/by-queue",
      "handler": "by_queue",
      "description": "Get ticket counts by queue",
      "middleware": ["auth"]
    },
    {
      "method": "GET",
      "path": "/api/plugins/stats/by-priority",
      "handler": "by_priority",
      "description": "Get ticket counts by priority",
      "middleware": ["auth"]
    },
    {
      "method": "GET",
      "path": "/api/plugins/stats/by-type",
      "handler": "by_type",
      "description": "Get ticket counts by ticket type",
      "middleware": ["auth"]
    },
    {
      "method": "GET",
      "path": "/api/plugins/stats/by-owner",
      "handler": "by_owner",
      "description": "Get ticket counts by owner/agent",
      "middleware": ["auth"]
    },
    {
      "method": "GET",
      "path": "/api/plugins/stats/recent-activity",
      "handler": "recent_activity",
      "description": "Get recent ticket activity",
      "middleware": ["auth"]
    },
    {
      "method": "GET",
      "path": "/api/plugins/stats/timeline",
      "handler": "timeline",
      "description": "Get ticket creation timeline (daily counts)",
      "middleware": ["auth"]
    }
  ],
  "widgets": [
    {
      "id": "stats_overview",
      "title": "Ticket Overview",
      "handler": "widget_overview",
      "location": "dashboard",
      "size": "medium",
      "refreshable": true
    },
    {
      "id": "stats_by_status",
      "title": "Tickets by Status",
      "handler": "widget_by_status",
      "location": "dashboard",
      "size": "small",
      "refreshable": true
    },
    {
      "id": "stats_chart",
      "title": "Ticket Chart",
      "handler": "widget_chart",
      "location": "dashboard",
      "size": "large",
      "refreshable": true
    }
  ],
  "i18n": {
    "en": {
      "stats.title": "Statistics",
      "stats.overview": "Overview",
      "stats.open_tickets": "Open Tickets",
      "stats.new_today": "New Today",
      "stats.pending_tickets": "Pending",
      "stats.overdue_tickets": "Overdue",
      "stats.by_status": "By Status",
      "stats.by_queue": "By Queue",
      "stats.by_priority": "By Priority",
      "stats.by_type": "By Type",
      "stats.by_owner": "By Owner",
      "stats.no_data": "No data available",
      "stats.last_7_days": "Last 7 Days",
      "stats.last_30_days": "Last 30 Days",
      "stats.last_90_days": "Last 90 Days",
      "stats.all_time": "All Time"
    },
    "de": {
      "stats.title": "Statistiken",
      "stats.overview": "Übersicht",
      "stats.open_tickets": "Offene Tickets",
      "stats.new_today": "Heute neu",
      "stats.pending_tickets": "Wartend",
      "stats.overdue_tickets": "Überfällig",
      "stats.by_status": "Nach Status",
      "stats.by_queue": "Nach Warteschlange",
      "stats.by_priority": "Nach Priorität",
      "stats.by_type": "Nach Typ",
      "stats.by_owner": "Nach Besitzer",
      "stats.no_data": "Keine Daten verfügbar",
      "stats.last_7_days": "Letzte 7 Tage",
      "stats.last_30_days": "Letzte 30 Tage",
      "stats.last_90_days": "Letzte 90 Tage",
      "stats.all_time": "Gesamt"
    }
  },
  "error_codes": [
    {"code": "query_failed", "message": "Database query failed", "http_status": 500},
    {"code": "invalid_range", "message": "Invalid date range specified", "http_status": 400},
    {"code": "no_data", "message": "No statistics data available", "http_status": 404}
  ]
}`

//export gk_malloc
func gk_malloc(size uint32) uint32 {
	buf := make([]byte, size)
	return uint32(uintptr(unsafe.Pointer(&buf[0])))
}

//export gk_free
func gk_free(ptr uint32) {}

//export gk_register
func gk_register() uint64 {
	ptr := gk_malloc(uint32(len(manifestJSON)))
	dst := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), len(manifestJSON))
	copy(dst, manifestJSON)
	return (uint64(ptr) << 32) | uint64(len(manifestJSON))
}

//export gk_call
func gk_call(fnPtr, fnLen, argsPtr, argsLen uint32) uint64 {
	fn := readString(fnPtr, fnLen)
	args := readString(argsPtr, argsLen)

	var result string
	switch fn {
	case "overview":
		result = handleOverview(args)
	case "by_status":
		result = handleByStatus(args)
	case "by_queue":
		result = handleByQueue(args)
	case "by_priority":
		result = handleByPriority(args)
	case "by_type":
		result = handleByType(args)
	case "by_owner":
		result = handleByOwner(args)
	case "recent_activity":
		result = handleRecentActivity(args)
	case "timeline":
		result = handleTimeline(args)
	case "widget_overview":
		result = handleWidgetOverview(args)
	case "widget_by_status":
		result = handleWidgetByStatus()
	case "widget_chart":
		result = handleWidgetChart()
	default:
		result = `{"error":"unknown function: ` + fn + `"}`
	}

	ptr := gk_malloc(uint32(len(result)))
	dst := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), len(result))
	copy(dst, result)
	return (uint64(ptr) << 32) | uint64(len(result))
}

func readString(ptr, length uint32) string {
	if ptr == 0 || length == 0 {
		return ""
	}
	return unsafe.String((*byte)(unsafe.Pointer(uintptr(ptr))), length)
}

// Host API call helper
//
//go:wasmimport gk host_call
func hostCall(fnPtr, fnLen, argsPtr, argsLen uint32) uint64

func callHost(fn string, args any) ([]byte, error) {
	argsJSON, _ := json.Marshal(args)

	fnPtr := gk_malloc(uint32(len(fn)))
	copy(unsafe.Slice((*byte)(unsafe.Pointer(uintptr(fnPtr))), len(fn)), fn)

	argsPtr := gk_malloc(uint32(len(argsJSON)))
	copy(unsafe.Slice((*byte)(unsafe.Pointer(uintptr(argsPtr))), len(argsJSON)), argsJSON)

	result := hostCall(fnPtr, uint32(len(fn)), argsPtr, uint32(len(argsJSON)))
	if result == 0 {
		return nil, fmt.Errorf("host call failed")
	}

	ptr := uint32(result >> 32)
	length := uint32(result & 0xFFFFFFFF)
	return []byte(readString(ptr, length)), nil
}

func dbQuery(query string, args ...any) ([]map[string]any, error) {
	req := map[string]any{"query": query, "args": args}
	resp, err := callHost("db_query", req)
	if err != nil {
		return nil, err
	}
	var rows []map[string]any
	json.Unmarshal(resp, &rows)
	return rows, nil
}

// Request args parsing
type RequestArgs struct {
	Query map[string]string `json:"query"`
}

func parseArgs(argsJSON string) RequestArgs {
	var args RequestArgs
	json.Unmarshal([]byte(argsJSON), &args)
	if args.Query == nil {
		args.Query = make(map[string]string)
	}
	return args
}

// Date range filter - returns SQL WHERE clause fragment and whether it's active
func getDateFilter(args RequestArgs, dateColumn string) (string, bool) {
	rangeParam := args.Query["range"]
	if rangeParam == "" || rangeParam == "all" {
		return "", false
	}

	var days int
	switch rangeParam {
	case "7d":
		days = 7
	case "30d":
		days = 30
	case "90d":
		days = 90
	case "365d":
		days = 365
	default:
		return "", false
	}

	// Use DATE_SUB for MySQL/MariaDB compatibility
	return fmt.Sprintf("%s >= DATE_SUB(NOW(), INTERVAL %d DAY)", dateColumn, days), true
}

// API Handlers

func handleOverview(argsJSON string) string {
	args := parseArgs(argsJSON)
	dateFilter, hasDate := getDateFilter(args, "t.create_time")

	whereClause := ""
	if hasDate {
		whereClause = "WHERE " + dateFilter
	}

	query := fmt.Sprintf(`
		SELECT 
			COUNT(*) as total,
			SUM(CASE WHEN tst.name IN ('open', 'new') THEN 1 ELSE 0 END) as open_count,
			SUM(CASE WHEN tst.name IN ('pending auto', 'pending reminder') THEN 1 ELSE 0 END) as pending_count,
			SUM(CASE WHEN tst.name IN ('closed', 'merged', 'removed') THEN 1 ELSE 0 END) as closed_count
		FROM ticket t
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		JOIN ticket_state_type tst ON ts.type_id = tst.id
		%s
	`, whereClause)

	rows, err := dbQuery(query)
	if err != nil || len(rows) == 0 {
		return `{"total":0,"open":0,"pending":0,"closed":0}`
	}

	row := rows[0]
	result := map[string]any{
		"total":   toInt(row["total"]),
		"open":    toInt(row["open_count"]),
		"pending": toInt(row["pending_count"]),
		"closed":  toInt(row["closed_count"]),
		"range":   args.Query["range"],
	}
	data, _ := json.Marshal(result)
	return string(data)
}

func handleByStatus(argsJSON string) string {
	args := parseArgs(argsJSON)
	dateFilter, hasDate := getDateFilter(args, "t.create_time")

	whereClause := ""
	if hasDate {
		whereClause = "WHERE " + dateFilter
	}

	query := fmt.Sprintf(`
		SELECT ts.name as status, COUNT(*) as count
		FROM ticket t
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		%s
		GROUP BY ts.name
		ORDER BY count DESC
	`, whereClause)

	rows, err := dbQuery(query)
	if err != nil {
		return `{"statuses":[]}`
	}

	statuses := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		statuses = append(statuses, map[string]any{
			"name":  row["status"],
			"count": toInt(row["count"]),
		})
	}
	data, _ := json.Marshal(map[string]any{"statuses": statuses})
	return string(data)
}

func handleByQueue(argsJSON string) string {
	args := parseArgs(argsJSON)
	dateFilter, hasDate := getDateFilter(args, "t.create_time")

	whereClause := ""
	if hasDate {
		whereClause = "WHERE " + dateFilter
	}

	query := fmt.Sprintf(`
		SELECT q.name as queue, COUNT(*) as count
		FROM ticket t
		JOIN queue q ON t.queue_id = q.id
		%s
		GROUP BY q.name
		ORDER BY count DESC
		LIMIT 10
	`, whereClause)

	rows, err := dbQuery(query)
	if err != nil {
		return `{"queues":[]}`
	}

	queues := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		queues = append(queues, map[string]any{
			"name":  row["queue"],
			"count": toInt(row["count"]),
		})
	}
	data, _ := json.Marshal(map[string]any{"queues": queues})
	return string(data)
}

func handleByPriority(argsJSON string) string {
	args := parseArgs(argsJSON)
	dateFilter, hasDate := getDateFilter(args, "t.create_time")

	whereClause := ""
	if hasDate {
		whereClause = "WHERE " + dateFilter
	}

	query := fmt.Sprintf(`
		SELECT tp.name as priority, COUNT(*) as count
		FROM ticket t
		JOIN ticket_priority tp ON t.ticket_priority_id = tp.id
		%s
		GROUP BY tp.name
		ORDER BY tp.id
	`, whereClause)

	rows, err := dbQuery(query)
	if err != nil {
		return `{"priorities":[]}`
	}

	priorities := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		priorities = append(priorities, map[string]any{
			"name":  row["priority"],
			"count": toInt(row["count"]),
		})
	}
	data, _ := json.Marshal(map[string]any{"priorities": priorities})
	return string(data)
}

func handleByType(argsJSON string) string {
	args := parseArgs(argsJSON)
	dateFilter, hasDate := getDateFilter(args, "t.create_time")

	whereClause := ""
	if hasDate {
		whereClause = "WHERE " + dateFilter
	}

	query := fmt.Sprintf(`
		SELECT COALESCE(tt.name, 'Unclassified') as type, COUNT(*) as count
		FROM ticket t
		LEFT JOIN ticket_type tt ON t.type_id = tt.id
		%s
		GROUP BY tt.name
		ORDER BY count DESC
	`, whereClause)

	rows, err := dbQuery(query)
	if err != nil {
		return `{"types":[]}`
	}

	types := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		types = append(types, map[string]any{
			"name":  row["type"],
			"count": toInt(row["count"]),
		})
	}
	data, _ := json.Marshal(map[string]any{"types": types})
	return string(data)
}

func handleByOwner(argsJSON string) string {
	args := parseArgs(argsJSON)
	dateFilter, hasDate := getDateFilter(args, "t.create_time")

	whereClause := "WHERE t.user_id > 1" // Exclude system user
	if hasDate {
		whereClause += " AND " + dateFilter
	}

	query := fmt.Sprintf(`
		SELECT CONCAT(u.first_name, ' ', u.last_name) as owner, COUNT(*) as count
		FROM ticket t
		JOIN users u ON t.user_id = u.id
		%s
		GROUP BY u.id, u.first_name, u.last_name
		ORDER BY count DESC
		LIMIT 10
	`, whereClause)

	rows, err := dbQuery(query)
	if err != nil {
		return `{"owners":[]}`
	}

	owners := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		name := row["owner"]
		if name == nil || name == " " {
			name = "Unassigned"
		}
		owners = append(owners, map[string]any{
			"name":  name,
			"count": toInt(row["count"]),
		})
	}
	data, _ := json.Marshal(map[string]any{"owners": owners})
	return string(data)
}

func handleRecentActivity(argsJSON string) string {
	args := parseArgs(argsJSON)
	limit := 10
	if l := args.Query["limit"]; l != "" {
		fmt.Sscanf(l, "%d", &limit)
		if limit > 50 {
			limit = 50
		}
	}

	query := fmt.Sprintf(`
		SELECT t.tn as ticket_number, t.title, ts.name as status, t.change_time as changed_at
		FROM ticket t
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		ORDER BY t.change_time DESC
		LIMIT %d
	`, limit)

	rows, err := dbQuery(query)
	if err != nil {
		return `{"activity":[]}`
	}

	activity := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		activity = append(activity, map[string]any{
			"ticket_number": row["ticket_number"],
			"title":         row["title"],
			"status":        row["status"],
			"changed_at":    row["changed_at"],
		})
	}
	data, _ := json.Marshal(map[string]any{"activity": activity})
	return string(data)
}

func handleTimeline(argsJSON string) string {
	args := parseArgs(argsJSON)
	days := 30 // Default to 30 days
	if rangeParam := args.Query["range"]; rangeParam != "" {
		switch rangeParam {
		case "7d":
			days = 7
		case "30d":
			days = 30
		case "90d":
			days = 90
		}
	}

	query := fmt.Sprintf(`
		SELECT DATE(t.create_time) as date, COUNT(*) as count
		FROM ticket t
		WHERE t.create_time >= DATE_SUB(NOW(), INTERVAL %d DAY)
		GROUP BY DATE(t.create_time)
		ORDER BY date
	`, days)

	rows, err := dbQuery(query)
	if err != nil {
		return `{"timeline":[]}`
	}

	timeline := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		timeline = append(timeline, map[string]any{
			"date":  row["date"],
			"count": toInt(row["count"]),
		})
	}
	data, _ := json.Marshal(map[string]any{"timeline": timeline, "days": days})
	return string(data)
}

// Widget Handlers

func handleWidgetOverview(argsJSON string) string {
	// Parse RBAC queue context from args (passed by host via GetPluginWidgets)
	var widgetArgs struct {
		IsQueueAdmin     bool   `json:"is_queue_admin"`
		AccessibleQueues []any  `json:"accessible_queue_ids"`
	}
	json.Unmarshal([]byte(argsJSON), &widgetArgs)

	// Build queue filter SQL fragment
	queueFilter := ""
	if !widgetArgs.IsQueueAdmin && len(widgetArgs.AccessibleQueues) > 0 {
		ids := make([]string, 0, len(widgetArgs.AccessibleQueues))
		for _, id := range widgetArgs.AccessibleQueues {
			ids = append(ids, fmt.Sprintf("%d", toInt(id)))
		}
		queueFilter = " AND t.queue_id IN (" + strings.Join(ids, ",") + ")"
	}

	// Open tickets (state type 'open' or 'new')
	openRows, err := dbQuery(fmt.Sprintf(`
		SELECT COUNT(*) as cnt
		FROM ticket t
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		JOIN ticket_state_type tst ON ts.type_id = tst.id
		WHERE tst.name IN ('open', 'new')%s
	`, queueFilter))
	open := 0
	if err == nil && len(openRows) > 0 {
		open = toInt(openRows[0]["cnt"])
	}

	// New today (created today)
	newTodayRows, err := dbQuery(fmt.Sprintf(`
		SELECT COUNT(*) as cnt
		FROM ticket t
		WHERE DATE(t.create_time) = CURDATE()%s
	`, queueFilter))
	newToday := 0
	if err == nil && len(newTodayRows) > 0 {
		newToday = toInt(newTodayRows[0]["cnt"])
	}

	// Pending tickets
	pendingRows, err := dbQuery(fmt.Sprintf(`
		SELECT COUNT(*) as cnt
		FROM ticket t
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		JOIN ticket_state_type tst ON ts.type_id = tst.id
		WHERE tst.name IN ('pending auto', 'pending reminder')%s
	`, queueFilter))
	pending := 0
	if err == nil && len(pendingRows) > 0 {
		pending = toInt(pendingRows[0]["cnt"])
	}

	// Overdue: open/pending tickets past SLA escalation time
	// escalation_time is an epoch int (0 = no escalation set)
	overdueRows, err := dbQuery(fmt.Sprintf(`
		SELECT COUNT(*) as cnt
		FROM ticket t
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		JOIN ticket_state_type tst ON ts.type_id = tst.id
		WHERE tst.name IN ('open', 'new', 'pending auto', 'pending reminder')
		  AND t.escalation_time > 0
		  AND t.escalation_time < UNIX_TIMESTAMP()%s
	`, queueFilter))
	overdue := 0
	if err == nil && len(overdueRows) > 0 {
		overdue = toInt(overdueRows[0]["cnt"])
	}
	// If query failed (e.g. column missing), overdue stays 0

	// Total tickets
	totalQuery := "SELECT COUNT(*) as cnt FROM ticket t WHERE 1=1" + queueFilter
	totalRows, err := dbQuery(totalQuery)
	total := 0
	if err == nil && len(totalRows) > 0 {
		total = toInt(totalRows[0]["cnt"])
	}

	// Closed tickets
	closedRows, err := dbQuery(fmt.Sprintf(`
		SELECT COUNT(*) as cnt
		FROM ticket t
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		JOIN ticket_state_type tst ON ts.type_id = tst.id
		WHERE tst.name IN ('closed', 'merged', 'removed')%s
	`, queueFilter))
	closed := 0
	if err == nil && len(closedRows) > 0 {
		closed = toInt(closedRows[0]["cnt"])
	}

	html := fmt.Sprintf(`
<div class="stats-overview grid grid-cols-3 gap-4 mb-4">
  <div class="gk-stat-card text-center">
    <div class="gk-stat-value">%d</div>
    <div class="gk-stat-label">Total</div>
  </div>
  <div class="gk-stat-card success text-center">
    <div class="gk-stat-value">%d</div>
    <div class="gk-stat-label">Open</div>
  </div>
  <div class="gk-stat-card text-center">
    <div class="gk-stat-value">%d</div>
    <div class="gk-stat-label">Closed</div>
  </div>
</div>
<div class="stats-overview grid grid-cols-3 gap-4">
  <div class="gk-stat-card success text-center">
    <div class="gk-stat-value">%d</div>
    <div class="gk-stat-label">New Today</div>
  </div>
  <div class="gk-stat-card warning text-center">
    <div class="gk-stat-value">%d</div>
    <div class="gk-stat-label">Pending</div>
  </div>
  <div class="gk-stat-card error text-center">
    <div class="gk-stat-value">%d</div>
    <div class="gk-stat-label">Overdue</div>
  </div>
</div>`, total, open, closed, newToday, pending, overdue)

	result := map[string]string{"html": html}
	data, _ := json.Marshal(result)
	return string(data)
}

func handleWidgetByStatus() string {
	rows, err := dbQuery(`
		SELECT ts.name as status, COUNT(*) as count
		FROM ticket t
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		GROUP BY ts.name
		ORDER BY count DESC
		LIMIT 5
	`)

	var items string
	if err == nil {
		for _, row := range rows {
			items += fmt.Sprintf(`
  <div class="flex justify-between items-center py-2" style="border-bottom: 1px solid var(--gk-border-default);">
    <span class="capitalize" style="color: var(--gk-text-primary);">%s</span>
    <span class="gk-badge gk-badge-muted">%d</span>
  </div>`, row["status"], toInt(row["count"]))
		}
	}

	if items == "" {
		items = `<div class="text-center py-4" style="color: var(--gk-text-muted);">No data</div>`
	}

	// Remove trailing border from last item
	items = strings.Replace(items, "border-bottom: 1px solid var(--gk-border-default);\">\n    <span class=\"capitalize\"", "\">\n    <span class=\"capitalize\"", 1)

	html := fmt.Sprintf(`<div class="stats-by-status">%s</div>`, items)
	result := map[string]string{"html": html}
	data, _ := json.Marshal(result)
	return string(data)
}

func handleWidgetChart() string {
	// Get timeline data for chart
	rows, err := dbQuery(`
		SELECT DATE(t.create_time) as date, COUNT(*) as count
		FROM ticket t
		WHERE t.create_time >= DATE_SUB(NOW(), INTERVAL 30 DAY)
		GROUP BY DATE(t.create_time)
		ORDER BY date
	`)

	var labels, dataPoints []string
	if err == nil {
		for _, row := range rows {
			date := fmt.Sprintf("%v", row["date"])
			if len(date) >= 10 {
				labels = append(labels, `"`+date[5:10]+`"`) // MM-DD format
			}
			dataPoints = append(dataPoints, fmt.Sprintf("%d", toInt(row["count"])))
		}
	}

	labelsJS := "[" + strings.Join(labels, ",") + "]"
	dataJS := "[" + strings.Join(dataPoints, ",") + "]"

	// Chart.js via CDN - renders a line chart
	html := fmt.Sprintf(`
<div class="stats-chart">
  <canvas id="statsChart" height="200"></canvas>
  <script src="/static/vendor/chart.min.js"></script>
  <script>
    (function() {
      const ctx = document.getElementById('statsChart').getContext('2d');
      new Chart(ctx, {
        type: 'line',
        data: {
          labels: %s,
          datasets: [{
            label: 'Tickets Created',
            data: %s,
            borderColor: getComputedStyle(document.documentElement).getPropertyValue('--gk-primary').trim() || '#00E5FF',
            backgroundColor: 'rgba(0, 229, 255, 0.1)',
            fill: true,
            tension: 0.3
          }]
        },
        options: {
          responsive: true,
          maintainAspectRatio: false,
          plugins: {
            legend: { display: false }
          },
          scales: {
            y: { beginAtZero: true, grid: { color: 'rgba(255,255,255,0.1)' } },
            x: { grid: { display: false } }
          }
        }
      });
    })();
  </script>
</div>`, labelsJS, dataJS)

	result := map[string]string{"html": html}
	data, _ := json.Marshal(result)
	return string(data)
}

// toInt converts various types to int (mirrors internal/convert.ToInt)
// WASM plugins can't import internal packages, so we replicate the logic here.
func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case string:
		// MariaDB returns SUM/aggregates as strings
		if i, err := strconv.Atoi(n); err == nil {
			return i
		}
		return 0
	default:
		return 0
	}
}

func main() {}
