//go:build tinygo.wasm

// Package main implements the stats WASM plugin for GoatKit.
// Provides ticket statistics dashboard widgets and API endpoints.
//
// Time handling: ticket timestamps are stored as the server's local wall
// clock, so every window boundary (start of today, now-30d, ...) is computed
// here from the host's time_now answer and bound as a "YYYY-MM-DD HH:MM:SS"
// parameter. No SQL date function is used, so MySQL and PostgreSQL give the
// same answer whatever their session time zone.
//
// Queue access: non-admin agents only see tickets in queues whose group they
// hold rw on, directly (group_user) or through a role (role_user ->
// group_role), the same rule as service.QueueAccessService. A call without an
// identified user sees nothing.
//
// Errors: a failed query never renders as a number. Widgets show the
// "unavailable" state; API routes answer {"error":"query_failed"} (HTTP 500).
package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

const pluginVersion = "2.1.0"

// Manifest defines the plugin's capabilities
var manifestJSON = `{
  "name": "stats",
  "version": "` + pluginVersion + `",
  "description": "Ticket statistics, reporting, and analytics",
  "author": "GoatFlow Team",
  "license": "Apache-2.0",
  "routes": [
    {
      "method": "GET",
      "path": "/api/plugins/stats/overview",
      "handler": "overview",
      "description": "Get ticket statistics overview (supports ?range=7d|30d|90d|365d|all)",
      "middleware": ["auth"]
    },
    {
      "method": "GET",
      "path": "/api/plugins/stats/by-status",
      "handler": "by_status",
      "description": "Get ticket counts by status (supports ?range=7d|30d|90d|365d|all)",
      "middleware": ["auth"]
    },
    {
      "method": "GET",
      "path": "/api/plugins/stats/by-queue",
      "handler": "by_queue",
      "description": "Get ticket counts by queue (supports ?range=7d|30d|90d|365d|all)",
      "middleware": ["auth"]
    },
    {
      "method": "GET",
      "path": "/api/plugins/stats/by-priority",
      "handler": "by_priority",
      "description": "Get ticket counts by priority (supports ?range=7d|30d|90d|365d|all)",
      "middleware": ["auth"]
    },
    {
      "method": "GET",
      "path": "/api/plugins/stats/by-type",
      "handler": "by_type",
      "description": "Get ticket counts by ticket type (supports ?range=7d|30d|90d|365d|all)",
      "middleware": ["auth"]
    },
    {
      "method": "GET",
      "path": "/api/plugins/stats/by-owner",
      "handler": "by_owner",
      "description": "Get ticket counts by owner/agent (supports ?range=7d|30d|90d|365d|all)",
      "middleware": ["auth"]
    },
    {
      "method": "GET",
      "path": "/api/plugins/stats/recent-activity",
      "handler": "recent_activity",
      "description": "Get recently changed tickets (supports ?limit=1..50)",
      "middleware": ["auth"]
    },
    {
      "method": "GET",
      "path": "/api/plugins/stats/timeline",
      "handler": "timeline",
      "description": "Get daily ticket creation counts (supports ?range=7d|30d|90d, default 30d)",
      "middleware": ["auth"]
    },
    {
      "method": "GET",
      "path": "/api/plugins/stats/sla-compliance",
      "handler": "sla_compliance",
      "description": "Open tickets with a running SLA per queue, and how many are past their escalation time (supports ?range=7d|30d|90d|365d|all)",
      "middleware": ["auth"]
    },
    {
      "method": "GET",
      "path": "/api/plugins/stats/time-tracking",
      "handler": "time_tracking",
      "description": "Time tracking analytics by agent and queue (supports ?range=7d|30d|90d|365d|all)",
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
  "resources": {
    "memory_mb": 32,
    "call_timeout": "15s",
    "permissions": [
      {
        "type": "db",
        "access": "read",
        "scope": ["ticket", "ticket_state", "ticket_state_type", "queue", "ticket_priority", "ticket_type", "users", "user_preferences", "groups", "group_user", "role_user", "roles", "group_role", "time_accounting"]
      },
      {"type": "email"}
    ]
  },
  "error_codes": [
    {"code": "query_failed", "message": "Database query failed", "http_status": 500},
    {"code": "invalid_range", "message": "Invalid date range specified", "http_status": 400}
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
		result = handleRoute(args, routeOverview)
	case "by_status":
		result = handleRoute(args, routeByStatus)
	case "by_queue":
		result = handleRoute(args, routeByQueue)
	case "by_priority":
		result = handleRoute(args, routeByPriority)
	case "by_type":
		result = handleRoute(args, routeByType)
	case "by_owner":
		result = handleRoute(args, routeByOwner)
	case "recent_activity":
		result = handleRoute(args, routeRecentActivity)
	case "timeline":
		result = handleRoute(args, routeTimeline)
	case "sla_compliance":
		result = handleRoute(args, routeSLACompliance)
	case "time_tracking":
		result = handleRoute(args, routeTimeTracking)
	case "widget_overview":
		result = handleWidget(args, widgetOverview)
	case "widget_by_status":
		result = handleWidget(args, widgetByStatus)
	case "widget_chart":
		result = handleWidget(args, widgetChart)
	case "widget_sla":
		result = handleWidget(args, widgetSLA)
	case "widget_time_tracking":
		result = handleWidget(args, widgetTimeTracking)
	case "report_email":
		result = handleReportEmail()
	case "__health_ping__":
		result = handleHealthPing()
	default:
		result = jsonString(map[string]any{"error": "unknown function: " + fn, "status": 404})
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
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}

	fnPtr := gk_malloc(uint32(len(fn)))
	copy(unsafe.Slice((*byte)(unsafe.Pointer(uintptr(fnPtr))), len(fn)), fn)

	argsPtr := gk_malloc(uint32(len(argsJSON)))
	copy(unsafe.Slice((*byte)(unsafe.Pointer(uintptr(argsPtr))), len(argsJSON)), argsJSON)

	result := hostCall(fnPtr, uint32(len(fn)), argsPtr, uint32(len(argsJSON)))
	if result == 0 {
		return nil, fmt.Errorf("host call %s failed", fn)
	}

	ptr := uint32(result >> 32)
	length := uint32(result & 0xFFFFFFFF)
	return []byte(readString(ptr, length)), nil
}

func dbQuery(query string, args ...any) ([]map[string]any, error) {
	if args == nil {
		args = []any{}
	}
	resp, err := callHost("db_query", map[string]any{"query": query, "args": args})
	if err != nil {
		return nil, err
	}
	var rows []map[string]any
	if err := json.Unmarshal(resp, &rows); err != nil {
		return nil, fmt.Errorf("decode db_query result: %v", err)
	}
	return rows, nil
}

// queryOne runs an aggregate query that must return exactly one row.
func queryOne(query string, args ...any) (map[string]any, error) {
	rows, err := dbQuery(query, args...)
	if err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, fmt.Errorf("aggregate query returned %d rows", len(rows))
	}
	return rows[0], nil
}

//go:wasmimport gk log
func hostLog(level, msgPtr, msgLen uint32)

// Host log levels (see wasm runtime hostLog).
const (
	levelInfo  = 1
	levelWarn  = 2
	levelError = 3
)

func writeLog(level uint32, message string) {
	if message == "" {
		return
	}
	ptr := gk_malloc(uint32(len(message)))
	copy(unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), len(message)), message)
	hostLog(level, ptr, uint32(len(message)))
}

func logError(message string) {
	writeLog(levelError, message)
}

func jsonString(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func handleHealthPing() string {
	var m struct {
		Routes  []json.RawMessage `json:"routes"`
		Widgets []json.RawMessage `json:"widgets"`
		Jobs    []json.RawMessage `json:"jobs"`
	}
	json.Unmarshal([]byte(manifestJSON), &m)
	return jsonString(map[string]any{
		"status":  "ok",
		"runtime": "tinygo-wasm",
		"version": pluginVersion,
		"routes":  len(m.Routes),
		"widgets": len(m.Widgets),
		"jobs":    len(m.Jobs),
	})
}

// --- Request context ---

// callArgs are the host-supplied args: query parameters at the top level plus
// the caller envelope (_user_id, _is_admin, _lang).
type callArgs struct {
	Range   any  `json:"range"`
	Limit   any  `json:"limit"`
	UserID  any  `json:"_user_id"`
	IsAdmin bool `json:"_is_admin"`
}

func parseCallArgs(argsJSON string) callArgs {
	var a callArgs
	json.Unmarshal([]byte(argsJSON), &a)
	return a
}

// scope is whose tickets a call may count.
type scope struct {
	admin  bool
	userID int
}

func (a callArgs) scope() scope {
	return scope{admin: a.IsAdmin, userID: toInt(a.UserID)}
}

// agentQueueFilter restricts t.queue_id to valid queues of valid groups the
// user holds rw on directly or through a valid role. Both ? are the user id.
const agentQueueFilter = `
		  AND t.queue_id IN (
			SELECT aq.id FROM queue aq
			JOIN ` + "`groups`" + ` ag ON ag.id = aq.group_id
			WHERE aq.valid_id = 1 AND ag.valid_id = 1
			  AND aq.group_id IN (
				SELECT gu.group_id FROM group_user gu
				WHERE gu.user_id = ? AND gu.permission_key = 'rw'
				UNION
				SELECT gr.group_id FROM role_user ru
				JOIN roles r ON r.id = ru.role_id
				JOIN group_role gr ON gr.role_id = ru.role_id
				WHERE ru.user_id = ? AND r.valid_id = 1
				  AND gr.permission_key = 'rw' AND gr.permission_value = 1
			  )
		  )`

// queueFilter returns the WHERE fragment (starting with AND, on alias t) and
// its args. Admins see every queue; an unidentified caller sees nothing.
func (s scope) queueFilter() (string, []any) {
	if s.admin {
		return "", nil
	}
	if s.userID <= 0 {
		return " AND 1=0", nil
	}
	return agentQueueFilter, []any{s.userID, s.userID}
}

// --- Time ---

const sqlTimestamp = "2006-01-02 15:04:05"

// serverNow returns the host's current time in the server's local zone.
func serverNow() (time.Time, error) {
	resp, err := callHost("time_now", map[string]any{})
	if err != nil {
		return time.Time{}, err
	}
	var r struct {
		Now string `json:"now"`
	}
	if err := json.Unmarshal(resp, &r); err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339Nano, r.Now)
}

// wall formats t as the wall-clock literal the ticket tables store.
func wall(t time.Time) string {
	return t.Format(sqlTimestamp)
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// dateRange is a "last N days" window; days == 0 means all time.
type dateRange struct {
	name string
	days int
}

type requestError struct {
	code    string
	status  int
	message string
}

func (e *requestError) Error() string { return e.message }

func parseRange(v any, allowed map[string]int, def string) (dateRange, error) {
	s, _ := v.(string)
	if s == "" {
		s = def
	}
	days, ok := allowed[s]
	if !ok {
		return dateRange{}, &requestError{code: "invalid_range", status: 400, message: "invalid range " + strconv.Quote(s)}
	}
	return dateRange{name: s, days: days}, nil
}

var statRanges = map[string]int{"all": 0, "7d": 7, "30d": 30, "90d": 90, "365d": 365}

// filter returns " AND col >= ?" and its arg, or nothing for all time.
func (r dateRange) filter(col string, now time.Time) (string, []any) {
	if r.days == 0 {
		return "", nil
	}
	return " AND " + col + " >= ?", []any{wall(now.AddDate(0, 0, -r.days))}
}

// --- Conversions ---

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
		if f, err := strconv.ParseFloat(n, 64); err == nil {
			return int(f)
		}
		return 0
	default:
		return 0
	}
}

// toFloat converts numeric values, including DECIMAL sums returned as strings.
func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case string:
		f, _ := strconv.ParseFloat(n, 64)
		return f
	default:
		return 0
	}
}

func toStr(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

var htmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&#34;", "'", "&#39;")

func esc(v any) string {
	return htmlEscaper.Replace(toStr(v))
}

func personName(v any) string {
	return strings.TrimSpace(toStr(v))
}

// --- Route plumbing ---

type routeFn func(a callArgs, now time.Time) (map[string]any, error)

func handleRoute(argsJSON string, fn routeFn) string {
	now, err := serverNow()
	if err != nil {
		logError("stats: server time unavailable: " + err.Error())
		return jsonString(map[string]any{"error": "query_failed", "status": 500})
	}
	result, err := fn(parseCallArgs(argsJSON), now)
	if err != nil {
		if re, ok := err.(*requestError); ok {
			return jsonString(map[string]any{"error": re.code, "message": re.message, "status": re.status})
		}
		logError("stats: " + err.Error())
		return jsonString(map[string]any{"error": "query_failed", "status": 500})
	}
	return jsonString(result)
}

// --- Statistics (shared by routes and the weekly report) ---

const stateJoins = `
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		JOIN ticket_state_type tst ON ts.type_id = tst.id`

func statOverview(sc scope, r dateRange, now time.Time) (map[string]any, error) {
	dateSQL, dateArgs := r.filter("t.create_time", now)
	qf, qArgs := sc.queueFilter()
	row, err := queryOne(`
		SELECT
			COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN tst.name IN ('new', 'open') THEN 1 ELSE 0 END), 0) AS open_count,
			COALESCE(SUM(CASE WHEN tst.name IN ('pending reminder', 'pending auto') THEN 1 ELSE 0 END), 0) AS pending_count,
			COALESCE(SUM(CASE WHEN tst.name IN ('closed', 'merged', 'removed') THEN 1 ELSE 0 END), 0) AS closed_count
		FROM ticket t`+stateJoins+`
		WHERE 1=1`+dateSQL+qf, //nolint:gk-sql-sprintf // fixed fragments; values bound via ?
		append(dateArgs, qArgs...)...)
	if err != nil {
		return nil, fmt.Errorf("overview: %v", err)
	}
	return map[string]any{
		"total":   toInt(row["total"]),
		"open":    toInt(row["open_count"]),
		"pending": toInt(row["pending_count"]),
		"closed":  toInt(row["closed_count"]),
		"range":   r.name,
	}, nil
}

// statGrouped runs "SELECT <label>, COUNT(*) ... GROUP BY ..." and returns
// [{name, count}] under key.
func statGrouped(sc scope, r dateRange, now time.Time, key, query string, suffix string) (map[string]any, error) {
	dateSQL, dateArgs := r.filter("t.create_time", now)
	qf, qArgs := sc.queueFilter()
	rows, err := dbQuery(query+dateSQL+qf+suffix, append(dateArgs, qArgs...)...) //nolint:gk-sql-sprintf // fixed fragments; values bound via ?
	if err != nil {
		return nil, fmt.Errorf("%s: %v", key, err)
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{"name": row["name"], "count": toInt(row["count"])})
	}
	return map[string]any{key: items, "range": r.name}, nil
}

func statByStatus(sc scope, r dateRange, now time.Time) (map[string]any, error) {
	return statGrouped(sc, r, now, "statuses", `
		SELECT ts.name AS name, COUNT(*) AS count
		FROM ticket t
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		WHERE 1=1`, `
		GROUP BY ts.name
		ORDER BY count DESC, ts.name`)
}

func statByQueue(sc scope, r dateRange, now time.Time) (map[string]any, error) {
	return statGrouped(sc, r, now, "queues", `
		SELECT q.name AS name, COUNT(*) AS count
		FROM ticket t
		JOIN queue q ON t.queue_id = q.id
		WHERE 1=1`, `
		GROUP BY q.name
		ORDER BY count DESC, q.name
		LIMIT 10`)
}

func routeOverview(a callArgs, now time.Time) (map[string]any, error) {
	r, err := parseRange(a.Range, statRanges, "all")
	if err != nil {
		return nil, err
	}
	return statOverview(a.scope(), r, now)
}

func routeByStatus(a callArgs, now time.Time) (map[string]any, error) {
	r, err := parseRange(a.Range, statRanges, "all")
	if err != nil {
		return nil, err
	}
	return statByStatus(a.scope(), r, now)
}

func routeByQueue(a callArgs, now time.Time) (map[string]any, error) {
	r, err := parseRange(a.Range, statRanges, "all")
	if err != nil {
		return nil, err
	}
	return statByQueue(a.scope(), r, now)
}

func routeByPriority(a callArgs, now time.Time) (map[string]any, error) {
	r, err := parseRange(a.Range, statRanges, "all")
	if err != nil {
		return nil, err
	}
	return statGrouped(a.scope(), r, now, "priorities", `
		SELECT tp.name AS name, COUNT(*) AS count
		FROM ticket t
		JOIN ticket_priority tp ON t.ticket_priority_id = tp.id
		WHERE 1=1`, `
		GROUP BY tp.id, tp.name
		ORDER BY tp.id`)
}

func routeByType(a callArgs, now time.Time) (map[string]any, error) {
	r, err := parseRange(a.Range, statRanges, "all")
	if err != nil {
		return nil, err
	}
	return statGrouped(a.scope(), r, now, "types", `
		SELECT COALESCE(tt.name, 'Unclassified') AS name, COUNT(*) AS count
		FROM ticket t
		LEFT JOIN ticket_type tt ON t.type_id = tt.id
		WHERE 1=1`, `
		GROUP BY tt.name
		ORDER BY count DESC`)
}

func routeByOwner(a callArgs, now time.Time) (map[string]any, error) {
	r, err := parseRange(a.Range, statRanges, "all")
	if err != nil {
		return nil, err
	}
	res, err := statGrouped(a.scope(), r, now, "owners", `
		SELECT CONCAT(u.first_name, ' ', u.last_name) AS name, COUNT(*) AS count
		FROM ticket t
		JOIN users u ON t.user_id = u.id
		WHERE t.user_id > 1`, `
		GROUP BY u.id, u.first_name, u.last_name
		ORDER BY count DESC
		LIMIT 10`)
	if err != nil {
		return nil, err
	}
	for _, o := range res["owners"].([]map[string]any) {
		o["name"] = personName(o["name"])
	}
	return res, nil
}

func routeRecentActivity(a callArgs, now time.Time) (map[string]any, error) {
	limit := 10
	if a.Limit != nil {
		limit = toInt(a.Limit)
		if limit < 1 || limit > 50 {
			return nil, &requestError{code: "invalid_limit", status: 400, message: "limit must be between 1 and 50"}
		}
	}
	qf, qArgs := a.scope().queueFilter()
	rows, err := dbQuery(`
		SELECT t.tn AS ticket_number, t.title, ts.name AS status, t.change_time AS changed_at
		FROM ticket t
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		WHERE 1=1`+qf+`
		ORDER BY t.change_time DESC
		LIMIT ?`, append(qArgs, limit)...) //nolint:gk-sql-sprintf // fixed fragments; values bound via ?
	if err != nil {
		return nil, fmt.Errorf("recent activity: %v", err)
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
	return map[string]any{"activity": activity}, nil
}

// dailyCounts returns ticket creation counts for the last `days` local days
// (oldest first, today last). Each day is a bound [start, next start) pair,
// so no SQL date function is involved.
func dailyCounts(sc scope, days int, now time.Time) ([]time.Time, []int, error) {
	today := startOfDay(now)
	starts := make([]time.Time, days)
	cols := make([]string, days)
	args := make([]any, 0, days*2+2)
	for i := range days {
		y, m, d := today.Date()
		start := time.Date(y, m, d-(days-1-i), 0, 0, 0, 0, today.Location())
		next := time.Date(y, m, d-(days-1-i)+1, 0, 0, 0, 0, today.Location())
		starts[i] = start
		cols[i] = fmt.Sprintf("COALESCE(SUM(CASE WHEN t.create_time >= ? AND t.create_time < ? THEN 1 ELSE 0 END), 0) AS d%d", i)
		args = append(args, wall(start), wall(next))
	}
	qf, qArgs := sc.queueFilter()
	args = append(args, wall(starts[0]))
	args = append(args, qArgs...)
	row, err := queryOne(`
		SELECT `+strings.Join(cols, ",\n\t\t\t")+`
		FROM ticket t
		WHERE t.create_time >= ?`+qf, args...) //nolint:gk-sql-sprintf // generated column list; values bound via ?
	if err != nil {
		return nil, nil, fmt.Errorf("daily counts: %v", err)
	}
	counts := make([]int, days)
	for i := range counts {
		counts[i] = toInt(row[fmt.Sprintf("d%d", i)])
	}
	return starts, counts, nil
}

func routeTimeline(a callArgs, now time.Time) (map[string]any, error) {
	r, err := parseRange(a.Range, map[string]int{"7d": 7, "30d": 30, "90d": 90}, "30d")
	if err != nil {
		return nil, err
	}
	starts, counts, err := dailyCounts(a.scope(), r.days, now)
	if err != nil {
		return nil, err
	}
	timeline := make([]map[string]any, len(starts))
	for i := range starts {
		timeline[i] = map[string]any{"date": starts[i].Format("2006-01-02"), "count": counts[i]}
	}
	return map[string]any{"timeline": timeline, "days": r.days}, nil
}

// statSLA counts, per queue, open tickets with a running SLA
// (escalation_time > 0) and how many of them are past it. Closing a ticket
// clears its escalation times, so closed tickets carry no SLA outcome and are
// not counted.
func statSLA(sc scope, r dateRange, now time.Time, limit int) ([]map[string]any, error) {
	dateSQL, dateArgs := r.filter("t.create_time", now)
	qf, qArgs := sc.queueFilter()
	args := append([]any{now.Unix()}, dateArgs...)
	args = append(args, qArgs...)
	suffix := `
		GROUP BY q.id, q.name
		ORDER BY breached DESC, q.name`
	if limit > 0 {
		suffix += fmt.Sprintf("\n\t\tLIMIT %d", limit)
	}
	rows, err := dbQuery(`
		SELECT q.id AS queue_id, q.name AS queue,
			COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN t.escalation_time < ? THEN 1 ELSE 0 END), 0) AS breached
		FROM ticket t
		JOIN queue q ON t.queue_id = q.id`+stateJoins+`
		WHERE tst.name IN ('new', 'open', 'pending reminder', 'pending auto')
		  AND t.escalation_time > 0`+dateSQL+qf+suffix, args...) //nolint:gk-sql-sprintf // fixed fragments; values bound via ?
	if err != nil {
		return nil, fmt.Errorf("sla: %v", err)
	}
	queues := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		total := toInt(row["total"])
		breached := toInt(row["breached"])
		met := total - breached
		queues = append(queues, map[string]any{
			"queue_id": toInt(row["queue_id"]),
			"queue":    row["queue"],
			"total":    total,
			"met":      met,
			"breached": breached,
			"rate":     met * 100 / total,
		})
	}
	return queues, nil
}

func routeSLACompliance(a callArgs, now time.Time) (map[string]any, error) {
	r, err := parseRange(a.Range, statRanges, "all")
	if err != nil {
		return nil, err
	}
	queues, err := statSLA(a.scope(), r, now, 0)
	if err != nil {
		return nil, err
	}
	return map[string]any{"queues": queues, "range": r.name}, nil
}

func hours(minutes float64) string {
	return fmt.Sprintf("%.1f", minutes/60)
}

func statTimeTracking(sc scope, r dateRange, now time.Time) (map[string]any, error) {
	dateSQL, dateArgs := r.filter("ta.create_time", now)
	qf, qArgs := sc.queueFilter()
	args := append(dateArgs, qArgs...)

	agentRows, err := dbQuery(`
		SELECT CONCAT(u.first_name, ' ', u.last_name) AS agent,
			SUM(ta.time_unit) AS total_minutes,
			COUNT(DISTINCT ta.ticket_id) AS ticket_count
		FROM time_accounting ta
		JOIN ticket t ON ta.ticket_id = t.id
		JOIN users u ON ta.create_by = u.id
		WHERE 1=1`+dateSQL+qf+`
		GROUP BY u.id, u.first_name, u.last_name
		ORDER BY total_minutes DESC
		LIMIT 10`, args...) //nolint:gk-sql-sprintf // fixed fragments; values bound via ?
	if err != nil {
		return nil, fmt.Errorf("time by agent: %v", err)
	}
	agents := make([]map[string]any, 0, len(agentRows))
	for _, row := range agentRows {
		minutes := toFloat(row["total_minutes"])
		agents = append(agents, map[string]any{
			"agent":        personName(row["agent"]),
			"minutes":      minutes,
			"hours":        hours(minutes),
			"ticket_count": toInt(row["ticket_count"]),
		})
	}

	queueRows, err := dbQuery(`
		SELECT q.name AS queue,
			SUM(ta.time_unit) AS total_minutes,
			COUNT(DISTINCT ta.ticket_id) AS ticket_count
		FROM time_accounting ta
		JOIN ticket t ON ta.ticket_id = t.id
		JOIN queue q ON t.queue_id = q.id
		WHERE 1=1`+dateSQL+qf+`
		GROUP BY q.id, q.name
		ORDER BY total_minutes DESC
		LIMIT 10`, args...) //nolint:gk-sql-sprintf // fixed fragments; values bound via ?
	if err != nil {
		return nil, fmt.Errorf("time by queue: %v", err)
	}
	queues := make([]map[string]any, 0, len(queueRows))
	for _, row := range queueRows {
		minutes := toFloat(row["total_minutes"])
		queues = append(queues, map[string]any{
			"queue":        row["queue"],
			"minutes":      minutes,
			"hours":        hours(minutes),
			"ticket_count": toInt(row["ticket_count"]),
		})
	}

	total, err := queryOne(`
		SELECT COALESCE(SUM(ta.time_unit), 0) AS total
		FROM time_accounting ta
		JOIN ticket t ON ta.ticket_id = t.id
		WHERE 1=1`+dateSQL+qf, args...) //nolint:gk-sql-sprintf // fixed fragments; values bound via ?
	if err != nil {
		return nil, fmt.Errorf("time total: %v", err)
	}
	totalMinutes := toFloat(total["total"])

	return map[string]any{
		"by_agent":      agents,
		"by_queue":      queues,
		"total_minutes": totalMinutes,
		"total_hours":   hours(totalMinutes),
		"range":         r.name,
	}, nil
}

func routeTimeTracking(a callArgs, now time.Time) (map[string]any, error) {
	r, err := parseRange(a.Range, statRanges, "all")
	if err != nil {
		return nil, err
	}
	return statTimeTracking(a.scope(), r, now)
}

// --- Widgets ---

type widgetFn func(sc scope, now time.Time) (string, error)

// handleWidget renders a widget; a failed query renders the unavailable
// state instead of numbers.
func handleWidget(argsJSON string, fn widgetFn) string {
	sc := parseCallArgs(argsJSON).scope()
	now, err := serverNow()
	var html string
	if err == nil {
		html, err = fn(sc, now)
	}
	if err != nil {
		logError("stats widget: " + err.Error())
		html = unavailableHTML()
	}
	return jsonString(map[string]string{"html": html})
}

func unavailableHTML() string {
	msg := "This widget is currently unavailable. Please try again later."
	if resp, err := callHost("translate", map[string]any{"key": "dashboard.widget_unavailable"}); err == nil {
		var r struct {
			Value string `json:"value"`
		}
		if json.Unmarshal(resp, &r) == nil && r.Value != "" && r.Value != "dashboard.widget_unavailable" {
			msg = r.Value
		}
	}
	return `<div class="stats-unavailable text-center py-4" role="alert" style="color: var(--gk-text-muted);">` + esc(msg) + `</div>`
}

func widgetOverview(sc scope, now time.Time) (string, error) {
	today := startOfDay(now)
	y, m, d := today.Date()
	tomorrow := time.Date(y, m, d+1, 0, 0, 0, 0, today.Location())
	qf, qArgs := sc.queueFilter()
	args := append([]any{wall(today), wall(tomorrow), now.Unix()}, qArgs...)

	row, err := queryOne(`
		SELECT
			COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN tst.name IN ('new', 'open') THEN 1 ELSE 0 END), 0) AS open_count,
			COALESCE(SUM(CASE WHEN tst.name IN ('closed', 'merged', 'removed') THEN 1 ELSE 0 END), 0) AS closed_count,
			COALESCE(SUM(CASE WHEN t.create_time >= ? AND t.create_time < ? THEN 1 ELSE 0 END), 0) AS new_today,
			COALESCE(SUM(CASE WHEN tst.name IN ('pending reminder', 'pending auto') THEN 1 ELSE 0 END), 0) AS pending_count,
			COALESCE(SUM(CASE WHEN tst.name IN ('new', 'open', 'pending reminder', 'pending auto')
				AND t.escalation_time > 0 AND t.escalation_time < ? THEN 1 ELSE 0 END), 0) AS overdue
		FROM ticket t`+stateJoins+`
		WHERE 1=1`+qf, args...) //nolint:gk-sql-sprintf // fixed fragments; values bound via ?
	if err != nil {
		return "", fmt.Errorf("overview: %v", err)
	}

	return fmt.Sprintf(`
<style>.gk-stat-link{text-decoration:none;color:inherit;display:block;border-radius:var(--gk-radius,8px);transition:transform .15s,box-shadow .15s}.gk-stat-link:hover{transform:translateY(-2px);box-shadow:0 4px 12px rgba(0,0,0,.15)}</style>
<div class="stats-overview grid grid-cols-3 gap-4 mb-4">
  <a href="/tickets?status=all" class="gk-stat-link">
    <div class="gk-stat-card text-center">
      <div class="gk-stat-value" data-stat="total">%d</div>
      <div class="gk-stat-label">Total</div>
    </div>
  </a>
  <a href="/tickets?status=open" class="gk-stat-link">
    <div class="gk-stat-card success text-center">
      <div class="gk-stat-value" data-stat="open">%d</div>
      <div class="gk-stat-label">Open</div>
    </div>
  </a>
  <a href="/tickets?status=closed" class="gk-stat-link">
    <div class="gk-stat-card text-center">
      <div class="gk-stat-value" data-stat="closed">%d</div>
      <div class="gk-stat-label">Closed</div>
    </div>
  </a>
</div>
<div class="stats-overview grid grid-cols-3 gap-4">
  <a href="/tickets?status=open&sort=create_time&order=desc" class="gk-stat-link">
    <div class="gk-stat-card success text-center">
      <div class="gk-stat-value" data-stat="new_today">%d</div>
      <div class="gk-stat-label">New Today</div>
    </div>
  </a>
  <a href="/tickets?status=pending" class="gk-stat-link">
    <div class="gk-stat-card warning text-center">
      <div class="gk-stat-value" data-stat="pending">%d</div>
      <div class="gk-stat-label">Pending</div>
    </div>
  </a>
  <a href="/tickets?status=overdue" class="gk-stat-link">
    <div class="gk-stat-card error text-center">
      <div class="gk-stat-value" data-stat="overdue">%d</div>
      <div class="gk-stat-label">Overdue</div>
    </div>
  </a>
</div>`, toInt(row["total"]), toInt(row["open_count"]), toInt(row["closed_count"]),
		toInt(row["new_today"]), toInt(row["pending_count"]), toInt(row["overdue"])), nil
}

func widgetByStatus(sc scope, now time.Time) (string, error) {
	qf, qArgs := sc.queueFilter()
	rows, err := dbQuery(`
		SELECT ts.id AS state_id, ts.name AS status, COUNT(*) AS count
		FROM ticket t
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		WHERE 1=1`+qf+`
		GROUP BY ts.id, ts.name
		ORDER BY count DESC, ts.name
		LIMIT 5`, qArgs...) //nolint:gk-sql-sprintf // fixed fragments; values bound via ?
	if err != nil {
		return "", fmt.Errorf("by status: %v", err)
	}
	if len(rows) == 0 {
		return `<div class="stats-by-status"><div class="text-center py-4" style="color: var(--gk-text-muted);">No data</div></div>`, nil
	}
	var b strings.Builder
	b.WriteString(`<div class="stats-by-status">`)
	for i, row := range rows {
		border := `border-bottom: 1px solid var(--gk-border-default);`
		if i == len(rows)-1 {
			border = ""
		}
		fmt.Fprintf(&b, `
  <a href="/tickets?status=%d" style="text-decoration:none;color:inherit;display:block;">
    <div class="flex justify-between items-center py-2" style="%s">
      <span class="capitalize" style="color: var(--gk-text-primary);">%s</span>
      <span class="gk-badge gk-badge-muted">%d</span>
    </div>
  </a>`, toInt(row["state_id"]), border, esc(row["status"]), toInt(row["count"]))
	}
	b.WriteString(`</div>`)
	return b.String(), nil
}

func widgetChart(sc scope, now time.Time) (string, error) {
	starts, counts, err := dailyCounts(sc, 30, now)
	if err != nil {
		return "", err
	}
	labels := make([]string, len(starts))
	points := make([]string, len(counts))
	for i := range starts {
		labels[i] = `"` + starts[i].Format("01-02") + `"`
		points[i] = strconv.Itoa(counts[i])
	}

	return fmt.Sprintf(`
<div class="stats-chart">
  <canvas id="statsChart" height="200"></canvas>
  <script src="/static/vendor/chart.min.js"></script>
  <script>
    (function() {
      const ctx = document.getElementById('statsChart').getContext('2d');
      new Chart(ctx, {
        type: 'line',
        data: {
          labels: [%s],
          datasets: [{
            label: 'Tickets Created',
            data: [%s],
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
</div>`, strings.Join(labels, ","), strings.Join(points, ",")), nil
}

func widgetSLA(sc scope, now time.Time) (string, error) {
	queues, err := statSLA(sc, dateRange{name: "all"}, now, 5)
	if err != nil {
		return "", err
	}
	if len(queues) == 0 {
		return `<div class="stats-sla"><div class="text-center py-4" style="color: var(--gk-text-muted);">No SLA data</div></div>`, nil
	}
	var b strings.Builder
	b.WriteString(`<div class="stats-sla">`)
	for i, q := range queues {
		rate := q["rate"].(int)
		color := "var(--gk-success)"
		if rate < 80 {
			color = "var(--gk-danger)"
		} else if rate < 95 {
			color = "var(--gk-warning)"
		}
		border := `border-bottom: 1px solid var(--gk-border-default);`
		if i == len(queues)-1 {
			border = ""
		}
		fmt.Fprintf(&b, `
  <a href="/tickets?queue=%d" style="text-decoration:none;color:inherit;display:block;">
    <div class="flex justify-between items-center py-2" style="%s">
      <span style="color: var(--gk-text-primary);">%s</span>
      <span class="gk-badge" style="background:%s;color:#fff;" title="%d/%d">%d%%</span>
    </div>
  </a>`, q["queue_id"].(int), border, esc(q["queue"]), color, q["met"].(int), q["total"].(int), rate)
	}
	b.WriteString(`</div>`)
	return b.String(), nil
}

func widgetTimeTracking(sc scope, now time.Time) (string, error) {
	qf, qArgs := sc.queueFilter()
	args := append([]any{wall(now.AddDate(0, 0, -30))}, qArgs...)

	agentRows, err := dbQuery(`
		SELECT u.id AS user_id, CONCAT(u.first_name, ' ', u.last_name) AS agent,
			SUM(ta.time_unit) AS total_minutes
		FROM time_accounting ta
		JOIN users u ON ta.create_by = u.id
		JOIN ticket t ON ta.ticket_id = t.id
		WHERE ta.create_time >= ?`+qf+`
		GROUP BY u.id, u.first_name, u.last_name
		ORDER BY total_minutes DESC
		LIMIT 5`, args...) //nolint:gk-sql-sprintf // fixed fragments; values bound via ?
	if err != nil {
		return "", fmt.Errorf("time by agent: %v", err)
	}
	total, err := queryOne(`
		SELECT COALESCE(SUM(ta.time_unit), 0) AS total
		FROM time_accounting ta
		JOIN ticket t ON ta.ticket_id = t.id
		WHERE ta.create_time >= ?`+qf, args...) //nolint:gk-sql-sprintf // fixed fragments; values bound via ?
	if err != nil {
		return "", fmt.Errorf("time total: %v", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, `
<div class="stats-time-tracking">
  <div class="gk-stat-card text-center mb-4">
    <div class="gk-stat-value">%s</div>
    <div class="gk-stat-label">Hours (30d)</div>
  </div>`, hours(toFloat(total["total"])))

	if len(agentRows) == 0 {
		b.WriteString(`<div class="text-center py-4" style="color: var(--gk-text-muted);">No time data</div>`)
	}
	for i, row := range agentRows {
		border := `border-bottom: 1px solid var(--gk-border-default);`
		if i == len(agentRows)-1 {
			border = ""
		}
		fmt.Fprintf(&b, `
  <a href="/tickets?owner=%d" style="text-decoration:none;color:inherit;display:block;">
    <div class="flex justify-between items-center py-2" style="%s">
      <span style="color: var(--gk-text-primary);">%s</span>
      <span class="gk-badge gk-badge-muted">%sh</span>
    </div>
  </a>`, toInt(row["user_id"]), border, esc(personName(row["agent"])), hours(toFloat(row["total_minutes"])))
	}
	b.WriteString(`</div>`)
	return b.String(), nil
}

// --- Scheduled Report Email ---

// handleReportEmail mails last week's statistics to valid admin-group
// members. Any failed query aborts the run: a report with invented zeros is
// worse than no report.
func handleReportEmail() string {
	fail := func(err error) string {
		logError("stats weekly report: " + err.Error())
		return jsonString(map[string]any{"error": "query_failed", "status": 500})
	}
	now, err := serverNow()
	if err != nil {
		return fail(err)
	}
	all := scope{admin: true}
	week := dateRange{name: "7d", days: 7}

	overview, err := statOverview(all, week, now)
	if err != nil {
		return fail(err)
	}
	byQueue, err := statByQueue(all, week, now)
	if err != nil {
		return fail(err)
	}
	sla, err := statSLA(all, week, now, 0)
	if err != nil {
		return fail(err)
	}
	timeData, err := statTimeTracking(all, week, now)
	if err != nil {
		return fail(err)
	}

	html := buildReportHTML(overview, byQueue["queues"].([]map[string]any), sla, timeData)

	// Agent email addresses live in user_preferences (UserEmail); admins are
	// rw members of the admin group, directly or through a role.
	adminRows, err := dbQuery(`
		SELECT DISTINCT u.id AS id, up.preferences_value AS email
		FROM users u
		JOIN user_preferences up ON up.user_id = u.id AND up.preferences_key = 'UserEmail'
		WHERE u.valid_id = 1 AND u.id IN (
			SELECT gu.user_id FROM group_user gu
			JOIN ` + "`groups`" + ` g ON g.id = gu.group_id
			WHERE g.name = 'admin' AND g.valid_id = 1 AND gu.permission_key = 'rw'
			UNION
			SELECT ru.user_id FROM role_user ru
			JOIN roles r ON r.id = ru.role_id
			JOIN group_role gr ON gr.role_id = ru.role_id
			JOIN ` + "`groups`" + ` g ON g.id = gr.group_id
			WHERE g.name = 'admin' AND g.valid_id = 1 AND r.valid_id = 1
			  AND gr.permission_key = 'rw' AND gr.permission_value = 1
		)
		ORDER BY u.id`)
	if err != nil {
		return fail(err)
	}

	sent, failed := 0, 0
	for _, row := range adminRows {
		email := toStr(row["email"])
		if email == "" {
			continue
		}
		if _, err := callHost("send_email", map[string]any{
			"to":      email,
			"subject": "GoatFlow Weekly Statistics Report",
			"body":    html,
			"html":    true,
		}); err != nil {
			failed++
			continue
		}
		sent++
	}

	if sent == 0 && failed == 0 {
		writeLog(levelWarn, "stats weekly report: no admin recipients with an email address")
		return jsonString(map[string]any{"status": "skipped", "reason": "no recipients"})
	}
	if failed > 0 {
		logError(fmt.Sprintf("stats weekly report: sending failed for %d of %d recipients", failed, sent+failed))
	}
	if sent == 0 {
		return jsonString(map[string]any{"error": "send_failed", "status": 500})
	}
	writeLog(levelInfo, fmt.Sprintf("stats weekly report sent to %d recipients", sent))
	return jsonString(map[string]any{"status": "sent", "recipients": sent, "failed": failed})
}

func buildReportHTML(overview map[string]any, queues, sla []map[string]any, timeData map[string]any) string {
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

	b.WriteString(`<div class="header"><h1>Weekly Statistics Report</h1><p style="margin:8px 0 0;opacity:0.8;font-size:13px;">Tickets created in the last 7 days</p></div>`)

	fmt.Fprintf(&b, `<div class="section"><h2>Overview</h2><div class="stat-grid">
<div class="stat"><div class="stat-value">%d</div><div class="stat-label">Total</div></div>
<div class="stat"><div class="stat-value">%d</div><div class="stat-label">Open</div></div>
<div class="stat"><div class="stat-value">%d</div><div class="stat-label">Pending</div></div>
<div class="stat"><div class="stat-value">%d</div><div class="stat-label">Closed</div></div>
</div></div>`, overview["total"].(int), overview["open"].(int), overview["pending"].(int), overview["closed"].(int))

	if len(queues) > 0 {
		b.WriteString(`<div class="section"><h2>Top Queues</h2><table><tr><th>Queue</th><th>Tickets</th></tr>`)
		for i, q := range queues {
			if i >= 5 {
				break
			}
			fmt.Fprintf(&b, `<tr><td>%s</td><td>%d</td></tr>`, esc(q["name"]), q["count"].(int))
		}
		b.WriteString(`</table></div>`)
	}

	if len(sla) > 0 {
		b.WriteString(`<div class="section"><h2>Open Tickets Within SLA</h2><table><tr><th>Queue</th><th>Within SLA</th><th>Breached</th><th>Rate</th></tr>`)
		for _, q := range sla {
			fmt.Fprintf(&b, `<tr><td>%s</td><td>%d</td><td>%d</td><td>%d%%</td></tr>`,
				esc(q["queue"]), q["met"].(int), q["breached"].(int), q["rate"].(int))
		}
		b.WriteString(`</table></div>`)
	}

	fmt.Fprintf(&b, `<div class="section"><h2>Time Tracking</h2><p>Total hours logged: <strong>%s</strong></p>`, timeData["total_hours"].(string))
	if agents := timeData["by_agent"].([]map[string]any); len(agents) > 0 {
		b.WriteString(`<table><tr><th>Agent</th><th>Hours</th><th>Tickets</th></tr>`)
		for i, a := range agents {
			if i >= 5 {
				break
			}
			fmt.Fprintf(&b, `<tr><td>%s</td><td>%s</td><td>%d</td></tr>`,
				esc(a["agent"]), a["hours"].(string), a["ticket_count"].(int))
		}
		b.WriteString(`</table>`)
	}
	b.WriteString(`</div>`)

	b.WriteString(`<div class="footer">Generated by GoatFlow Statistics Plugin</div></div></body></html>`)

	return b.String()
}

func main() {}
