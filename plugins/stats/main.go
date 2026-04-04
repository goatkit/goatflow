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
  "version": "2.0.0",
  "description": "Ticket statistics, reporting, and analytics",
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
    },
    {
      "method": "GET",
      "path": "/api/plugins/stats/sla-compliance",
      "handler": "sla_compliance",
      "description": "SLA compliance rates by queue (supports ?range=7d|30d|90d|all)",
      "middleware": ["auth"]
    },
    {
      "method": "GET",
      "path": "/api/plugins/stats/time-tracking",
      "handler": "time_tracking",
      "description": "Time tracking analytics by agent and queue (supports ?range=7d|30d|90d|all)",
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
    },
    {
      "id": "stats_sla",
      "title": "SLA Compliance",
      "handler": "widget_sla",
      "location": "dashboard",
      "size": "medium",
      "refreshable": true
    },
    {
      "id": "stats_time_tracking",
      "title": "Time Tracking",
      "handler": "widget_time_tracking",
      "location": "dashboard",
      "size": "medium",
      "refreshable": true
    }
  ],
  "jobs": [
    {
      "id": "weekly-report",
      "handler": "report_email",
      "schedule": "0 8 * * 1",
      "description": "Send weekly statistics report via email every Monday at 08:00",
      "enabled": true,
      "timeout": "2m"
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
      "stats.all_time": "All Time",
      "stats.sla_compliance": "SLA Compliance",
      "stats.sla_met": "Met",
      "stats.sla_breached": "Breached",
      "stats.time_tracking": "Time Tracking",
      "stats.total_hours": "Total Hours",
      "stats.weekly_report": "Weekly Report",
      "stats.weekly_report_subject": "Weekly Statistics Report"
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
      "stats.all_time": "Gesamt",
      "stats.sla_compliance": "SLA-Einhaltung",
      "stats.sla_met": "Eingehalten",
      "stats.sla_breached": "Verletzt",
      "stats.time_tracking": "Zeiterfassung",
      "stats.total_hours": "Gesamtstunden",
      "stats.weekly_report": "Wochenbericht",
      "stats.weekly_report_subject": "Wöchentlicher Statistikbericht"
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
	case "sla_compliance":
		result = handleSLACompliance(args)
	case "time_tracking":
		result = handleTimeTracking(args)
	case "widget_overview":
		result = handleWidgetOverview(args)
	case "widget_by_status":
		result = handleWidgetByStatus()
	case "widget_chart":
		result = handleWidgetChart()
	case "widget_sla":
		result = handleWidgetSLA()
	case "widget_time_tracking":
		result = handleWidgetTimeTracking()
	case "report_email":
		result = handleReportEmail(args)
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

	query := `
		SELECT t.tn as ticket_number, t.title, ts.name as status, t.change_time as changed_at
		FROM ticket t
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		ORDER BY t.change_time DESC
		LIMIT ?
	`

	rows, err := dbQuery(query, limit)
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

// --- SLA Compliance ---

func handleSLACompliance(argsJSON string) string {
	args := parseArgs(argsJSON)
	dateFilter, hasDate := getDateFilter(args, "t.create_time")

	whereClause := ""
	if hasDate {
		whereClause = "AND " + dateFilter
	}

	// SLA compliance: tickets with escalation_time set vs those that breached.
	// escalation_time > 0 means SLA is configured; breached if escalation_time < close time or still open past escalation.
	query := fmt.Sprintf(`
		SELECT q.name as queue,
			COUNT(*) as total,
			SUM(CASE
				WHEN t.escalation_time > 0 AND (
					tst.name IN ('closed', 'merged', 'removed')
					AND UNIX_TIMESTAMP(t.change_time) <= t.escalation_time
				) THEN 1
				WHEN t.escalation_time = 0 THEN 1
				ELSE 0
			END) as met,
			SUM(CASE
				WHEN t.escalation_time > 0 AND (
					(tst.name IN ('closed', 'merged', 'removed') AND UNIX_TIMESTAMP(t.change_time) > t.escalation_time)
					OR (tst.name NOT IN ('closed', 'merged', 'removed') AND t.escalation_time < UNIX_TIMESTAMP())
				) THEN 1
				ELSE 0
			END) as breached
		FROM ticket t
		JOIN queue q ON t.queue_id = q.id
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		JOIN ticket_state_type tst ON ts.type_id = tst.id
		WHERE 1=1 %s
		GROUP BY q.id, q.name
		ORDER BY queue
	`, whereClause)

	rows, err := dbQuery(query)
	if err != nil {
		return `{"queues":[]}`
	}

	queues := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		total := toInt(row["total"])
		met := toInt(row["met"])
		rate := 0.0
		if total > 0 {
			rate = float64(met) / float64(total) * 100
		}
		queues = append(queues, map[string]any{
			"queue":    row["queue"],
			"total":    total,
			"met":      met,
			"breached": toInt(row["breached"]),
			"rate":     int(rate),
		})
	}
	data, _ := json.Marshal(map[string]any{"queues": queues})
	return string(data)
}

// --- Time Tracking ---

func handleTimeTracking(argsJSON string) string {
	args := parseArgs(argsJSON)
	dateFilter, hasDate := getDateFilter(args, "ta.create_time")

	whereClause := ""
	if hasDate {
		whereClause = "WHERE " + dateFilter
	}

	// By agent
	agentQuery := fmt.Sprintf(`
		SELECT CONCAT(u.first_name, ' ', u.last_name) as agent,
			SUM(ta.time_unit) as total_minutes,
			COUNT(DISTINCT ta.ticket_id) as ticket_count
		FROM time_accounting ta
		JOIN users u ON ta.create_by = u.id
		%s
		GROUP BY u.id, u.first_name, u.last_name
		ORDER BY total_minutes DESC
		LIMIT 10
	`, whereClause)

	agentRows, err := dbQuery(agentQuery)
	agents := make([]map[string]any, 0)
	if err == nil {
		for _, row := range agentRows {
			name := row["agent"]
			if name == nil || name == " " {
				name = "Unknown"
			}
			minutes := toInt(row["total_minutes"])
			agents = append(agents, map[string]any{
				"agent":        name,
				"minutes":      minutes,
				"hours":        fmt.Sprintf("%.1f", float64(minutes)/60),
				"ticket_count": toInt(row["ticket_count"]),
			})
		}
	}

	// By queue
	queueQuery := fmt.Sprintf(`
		SELECT q.name as queue,
			SUM(ta.time_unit) as total_minutes,
			COUNT(DISTINCT ta.ticket_id) as ticket_count
		FROM time_accounting ta
		JOIN ticket t ON ta.ticket_id = t.id
		JOIN queue q ON t.queue_id = q.id
		%s
		GROUP BY q.id, q.name
		ORDER BY total_minutes DESC
		LIMIT 10
	`, whereClause)

	queueRows, err := dbQuery(queueQuery)
	queues := make([]map[string]any, 0)
	if err == nil {
		for _, row := range queueRows {
			minutes := toInt(row["total_minutes"])
			queues = append(queues, map[string]any{
				"queue":        row["queue"],
				"minutes":      minutes,
				"hours":        fmt.Sprintf("%.1f", float64(minutes)/60),
				"ticket_count": toInt(row["ticket_count"]),
			})
		}
	}

	// Total
	totalQuery := "SELECT COALESCE(SUM(time_unit), 0) as total FROM time_accounting"
	if whereClause != "" {
		totalQuery = "SELECT COALESCE(SUM(ta.time_unit), 0) as total FROM time_accounting ta " + whereClause
	}
	totalRows, _ := dbQuery(totalQuery)
	totalMinutes := 0
	if len(totalRows) > 0 {
		totalMinutes = toInt(totalRows[0]["total"])
	}

	data, _ := json.Marshal(map[string]any{
		"by_agent":      agents,
		"by_queue":      queues,
		"total_minutes": totalMinutes,
		"total_hours":   fmt.Sprintf("%.1f", float64(totalMinutes)/60),
	})
	return string(data)
}

// --- Scheduled Report Email ---

func handleReportEmail(argsJSON string) string {
	// Gather statistics for the report
	overview := handleOverview(`{"query":{"range":"7d"}}`)
	var overviewData map[string]any
	json.Unmarshal([]byte(overview), &overviewData)

	byStatus := handleByStatus(`{"query":{"range":"7d"}}`)
	var statusData map[string]any
	json.Unmarshal([]byte(byStatus), &statusData)

	byQueue := handleByQueue(`{"query":{"range":"7d"}}`)
	var queueData map[string]any
	json.Unmarshal([]byte(byQueue), &queueData)

	sla := handleSLACompliance(`{"query":{"range":"7d"}}`)
	var slaData map[string]any
	json.Unmarshal([]byte(sla), &slaData)

	timeTrack := handleTimeTracking(`{"query":{"range":"7d"}}`)
	var timeData map[string]any
	json.Unmarshal([]byte(timeTrack), &timeData)

	// Build HTML email body
	html := buildReportHTML(overviewData, statusData, queueData, slaData, timeData)

	// Get admin email addresses for report delivery
	adminRows, err := dbQuery(`
		SELECT u.email FROM users u
		JOIN group_user gu ON u.id = gu.user_id
		JOIN groups g ON gu.group_id = g.id
		WHERE g.name = 'admin' AND u.valid_id = 1 AND gu.permission_key = 'rw'
		ORDER BY u.id LIMIT 10
	`)
	if err != nil || len(adminRows) == 0 {
		callHost("log", map[string]any{
			"level":   "warn",
			"message": "No admin recipients found for weekly report",
		})
		return `{"status":"skipped","reason":"no recipients"}`
	}

	sent := 0
	for _, row := range adminRows {
		email, _ := row["email"].(string)
		if email == "" {
			continue
		}
		callHost("send_email", map[string]any{
			"to":      email,
			"subject": "GoatFlow Weekly Statistics Report",
			"body":    html,
			"html":    true,
		})
		sent++
	}

	callHost("log", map[string]any{
		"level":   "info",
		"message": fmt.Sprintf("Weekly report sent to %d recipients", sent),
	})

	data, _ := json.Marshal(map[string]any{"status": "sent", "recipients": sent})
	return string(data)
}

func buildReportHTML(overview, status, queue, sla, timeData map[string]any) string {
	var b strings.Builder

	b.WriteString(`<!DOCTYPE html><html><head><style>
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; margin: 0; padding: 20px; background: #f5f5f5; }
.container { max-width: 600px; margin: 0 auto; background: #fff; border-radius: 8px; overflow: hidden; }
.header { background: #1a1a2e; color: #fff; padding: 24px; text-align: center; }
.header h1 { margin: 0; font-size: 20px; }
.section { padding: 20px 24px; border-bottom: 1px solid #eee; }
.section h2 { margin: 0 0 12px; font-size: 16px; color: #333; }
.stat-grid { display: flex; gap: 12px; flex-wrap: wrap; }
.stat { flex: 1; min-width: 80px; text-align: center; padding: 12px; background: #f8f9fa; border-radius: 6px; }
.stat-value { font-size: 24px; font-weight: 700; color: #1a1a2e; }
.stat-label { font-size: 11px; color: #666; text-transform: uppercase; margin-top: 4px; }
table { width: 100%; border-collapse: collapse; }
td, th { padding: 8px 12px; text-align: left; border-bottom: 1px solid #eee; font-size: 13px; }
th { color: #666; font-weight: 600; }
.footer { padding: 16px 24px; text-align: center; color: #999; font-size: 11px; }
</style></head><body><div class="container">`)

	b.WriteString(`<div class="header"><h1>Weekly Statistics Report</h1><p style="margin:8px 0 0;opacity:0.8;font-size:13px;">Last 7 days</p></div>`)

	// Overview section
	total := toInt(overview["total"])
	open := toInt(overview["open"])
	pending := toInt(overview["pending"])
	closed := toInt(overview["closed"])
	b.WriteString(fmt.Sprintf(`<div class="section"><h2>Overview</h2><div class="stat-grid">
<div class="stat"><div class="stat-value">%d</div><div class="stat-label">Total</div></div>
<div class="stat"><div class="stat-value">%d</div><div class="stat-label">Open</div></div>
<div class="stat"><div class="stat-value">%d</div><div class="stat-label">Pending</div></div>
<div class="stat"><div class="stat-value">%d</div><div class="stat-label">Closed</div></div>
</div></div>`, total, open, pending, closed))

	// Top queues
	if queues, ok := queue["queues"].([]any); ok && len(queues) > 0 {
		b.WriteString(`<div class="section"><h2>Top Queues</h2><table><tr><th>Queue</th><th>Tickets</th></tr>`)
		for i, q := range queues {
			if i >= 5 {
				break
			}
			if qm, ok := q.(map[string]any); ok {
				b.WriteString(fmt.Sprintf(`<tr><td>%v</td><td>%d</td></tr>`, qm["name"], toInt(qm["count"])))
			}
		}
		b.WriteString(`</table></div>`)
	}

	// SLA compliance
	if slaQueues, ok := sla["queues"].([]any); ok && len(slaQueues) > 0 {
		b.WriteString(`<div class="section"><h2>SLA Compliance</h2><table><tr><th>Queue</th><th>Met</th><th>Breached</th><th>Rate</th></tr>`)
		for _, q := range slaQueues {
			if qm, ok := q.(map[string]any); ok {
				b.WriteString(fmt.Sprintf(`<tr><td>%v</td><td>%d</td><td>%d</td><td>%d%%</td></tr>`,
					qm["queue"], toInt(qm["met"]), toInt(qm["breached"]), toInt(qm["rate"])))
			}
		}
		b.WriteString(`</table></div>`)
	}

	// Time tracking
	totalHours := "0.0"
	if h, ok := timeData["total_hours"].(string); ok {
		totalHours = h
	}
	b.WriteString(fmt.Sprintf(`<div class="section"><h2>Time Tracking</h2><p>Total hours logged: <strong>%s</strong></p>`, totalHours))
	if agents, ok := timeData["by_agent"].([]any); ok && len(agents) > 0 {
		b.WriteString(`<table><tr><th>Agent</th><th>Hours</th><th>Tickets</th></tr>`)
		for i, a := range agents {
			if i >= 5 {
				break
			}
			if am, ok := a.(map[string]any); ok {
				b.WriteString(fmt.Sprintf(`<tr><td>%v</td><td>%v</td><td>%d</td></tr>`,
					am["agent"], am["hours"], toInt(am["ticket_count"])))
			}
		}
		b.WriteString(`</table>`)
	}
	b.WriteString(`</div>`)

	b.WriteString(`<div class="footer">Generated by GoatFlow Statistics Plugin</div></div></body></html>`)

	return b.String()
}

// --- Dashboard Widgets ---

func handleWidgetSLA() string {
	rows, err := dbQuery(`
		SELECT q.name as queue,
			COUNT(*) as total,
			SUM(CASE
				WHEN t.escalation_time > 0 AND (
					tst.name IN ('closed', 'merged', 'removed')
					AND UNIX_TIMESTAMP(t.change_time) <= t.escalation_time
				) THEN 1
				WHEN t.escalation_time = 0 THEN 1
				ELSE 0
			END) as met,
			SUM(CASE
				WHEN t.escalation_time > 0 AND (
					(tst.name IN ('closed', 'merged', 'removed') AND UNIX_TIMESTAMP(t.change_time) > t.escalation_time)
					OR (tst.name NOT IN ('closed', 'merged', 'removed') AND t.escalation_time < UNIX_TIMESTAMP())
				) THEN 1
				ELSE 0
			END) as breached
		FROM ticket t
		JOIN queue q ON t.queue_id = q.id
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		JOIN ticket_state_type tst ON ts.type_id = tst.id
		WHERE t.create_time >= DATE_SUB(NOW(), INTERVAL 30 DAY)
		GROUP BY q.id, q.name
		ORDER BY queue
		LIMIT 5
	`)

	var items string
	if err == nil {
		for _, row := range rows {
			total := toInt(row["total"])
			met := toInt(row["met"])
			rate := 0
			if total > 0 {
				rate = met * 100 / total
			}
			color := "var(--gk-success)"
			if rate < 80 {
				color = "var(--gk-danger)"
			} else if rate < 95 {
				color = "var(--gk-warning)"
			}
			items += fmt.Sprintf(`
  <div class="flex justify-between items-center py-2" style="border-bottom: 1px solid var(--gk-border-default);">
    <span style="color: var(--gk-text-primary);">%s</span>
    <span class="gk-badge" style="background:%s;color:#fff;">%d%%</span>
  </div>`, row["queue"], color, rate)
		}
	}

	if items == "" {
		items = `<div class="text-center py-4" style="color: var(--gk-text-muted);">No SLA data</div>`
	}

	html := fmt.Sprintf(`<div class="stats-sla">%s</div>`, items)
	result := map[string]string{"html": html}
	data, _ := json.Marshal(result)
	return string(data)
}

func handleWidgetTimeTracking() string {
	agentRows, err := dbQuery(`
		SELECT CONCAT(u.first_name, ' ', u.last_name) as agent,
			SUM(ta.time_unit) as total_minutes
		FROM time_accounting ta
		JOIN users u ON ta.create_by = u.id
		WHERE ta.create_time >= DATE_SUB(NOW(), INTERVAL 30 DAY)
		GROUP BY u.id, u.first_name, u.last_name
		ORDER BY total_minutes DESC
		LIMIT 5
	`)

	// Total
	totalRows, _ := dbQuery(`
		SELECT COALESCE(SUM(time_unit), 0) as total
		FROM time_accounting
		WHERE create_time >= DATE_SUB(NOW(), INTERVAL 30 DAY)
	`)
	totalMin := 0
	if len(totalRows) > 0 {
		totalMin = toInt(totalRows[0]["total"])
	}

	html := fmt.Sprintf(`
<div class="stats-time-tracking">
  <div class="gk-stat-card text-center mb-4">
    <div class="gk-stat-value">%.1f</div>
    <div class="gk-stat-label">Hours (30d)</div>
  </div>`, float64(totalMin)/60)

	if err == nil && len(agentRows) > 0 {
		for _, row := range agentRows {
			name := row["agent"]
			if name == nil || name == " " {
				name = "Unknown"
			}
			minutes := toInt(row["total_minutes"])
			html += fmt.Sprintf(`
  <div class="flex justify-between items-center py-2" style="border-bottom: 1px solid var(--gk-border-default);">
    <span style="color: var(--gk-text-primary);">%s</span>
    <span class="gk-badge gk-badge-muted">%.1fh</span>
  </div>`, name, float64(minutes)/60)
		}
	} else {
		html += `<div class="text-center py-4" style="color: var(--gk-text-muted);">No time data</div>`
	}

	html += `</div>`
	result := map[string]string{"html": html}
	data, _ := json.Marshal(result)
	return string(data)
}

func main() {}
