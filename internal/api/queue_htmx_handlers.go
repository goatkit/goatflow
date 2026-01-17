package api

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/gotrs-io/gotrs-ce/internal/routing"
)

func init() {
	routing.RegisterHandler("handleQueues", handleQueues)
	routing.RegisterHandler("handleQueueDetail", handleQueueDetail)
	routing.RegisterHandler("handleQueueMetaPartial", handleQueueMetaPartial)
}

// handleQueues shows the queues list page.
func handleQueues(c *gin.Context) {
	if htmxHandlerSkipDB() {
		renderQueuesTestFallback(c)
		return
	}
	// If templates are unavailable, return error
	if getPongo2Renderer() == nil || getPongo2Renderer().TemplateSet() == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Template system unavailable"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database unavailable"})
		return
	}

	// Optional search filter
	search := strings.TrimSpace(c.Query("search"))
	searchLower := strings.ToLower(search)

	// Get user ID and check if admin
	userID := uint(0)
	if val, exists := c.Get("user_id"); exists {
		switch v := val.(type) {
		case uint:
			userID = v
		case int:
			userID = uint(v)
		case int64:
			userID = uint(v)
		case uint64:
			userID = uint(v)
		}
	}

	// Check if user is admin (admins see all queues)
	isAdmin := false
	if userID > 0 {
		var adminCheck bool
		adminErr := db.QueryRow(database.ConvertPlaceholders(`
			SELECT EXISTS(
				SELECT 1 FROM group_user gu
				JOIN groups g ON gu.group_id = g.id
				WHERE gu.user_id = ? AND g.name = 'admin'
			)
		`), userID).Scan(&adminCheck)
		if adminErr == nil && adminCheck {
			isAdmin = true
		}
	}

	queueRepo := repository.NewQueueRepository(db)
	var queues []*models.Queue

	if isAdmin {
		// Admin sees all queues
		queues, err = queueRepo.List()
		if err != nil {
			sendErrorResponse(c, http.StatusInternalServerError, "Failed to fetch queues")
			return
		}
	} else {
		// Non-admin: only show queues user has access to through group membership
		// Get accessible queue IDs first
		accessibleQueueIDs := []uint{}
		rows, qErr := db.Query(database.ConvertPlaceholders(`
			SELECT DISTINCT q.id FROM queue q
			WHERE q.group_id IN (
				SELECT group_id FROM group_user WHERE user_id = ?
			)
			ORDER BY q.name
		`), userID)
		if qErr == nil {
			defer rows.Close()
			for rows.Next() {
				var qid uint
				if err := rows.Scan(&qid); err == nil {
					accessibleQueueIDs = append(accessibleQueueIDs, qid)
				}
			}
			if err := rows.Err(); err != nil {
				log.Printf("error iterating accessible queue IDs: %v", err)
			}
		}

		// Now fetch queue details for accessible IDs
		for _, qid := range accessibleQueueIDs {
			q, qErr := queueRepo.GetByID(qid)
			if qErr == nil {
				queues = append(queues, q)
			}
		}
	}

	// Build stats: map queueID -> counts
	// State category mapping (simplified; adjust to real state names as schema evolves)
	// new: 'new'
	// open: 'open'
	// pending: states containing 'pending'
	// closed: states containing 'closed' or 'resolved'
	query := `SELECT queue_id, ts.name, COUNT(*)
		FROM ticket t
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		GROUP BY queue_id, ts.name`
	rows, qerr := db.Query(query)
	stats := map[uint]map[string]int{}
	if qerr == nil {
		defer rows.Close()
		for rows.Next() {
			var qid uint
			var stateName string
			var cnt int
			if err := rows.Scan(&qid, &stateName, &cnt); err == nil {
				m, ok := stats[qid]
				if !ok {
					m = map[string]int{}
					stats[qid] = m
				}
				cat := "open"
				lname := strings.ToLower(stateName)
				if lname == "new" {
					cat = "new"
				} else if strings.Contains(lname, "pending") {
					cat = "pending"
				} else if strings.Contains(lname, "closed") || strings.Contains(lname, "resolved") {
					cat = "closed"
				}
				m[cat] += cnt
				m["total"] += cnt
			}
		}
		if err := rows.Err(); err != nil {
			log.Printf("error iterating queue stats: %v", err)
		}
	}

	// Transform for template
	viewQueues := make([]gin.H, 0, len(queues))
	for _, q := range queues {
		if searchLower != "" && !strings.Contains(strings.ToLower(q.Name), searchLower) {
			continue
		}
		m := stats[q.ID]
		viewQueues = append(viewQueues, gin.H{
			"ID":      q.ID,
			"Name":    q.Name,
			"Comment": q.Comment,
			"ValidID": q.ValidID,
			"New":     m["new"],
			"Open":    m["open"],
			"Pending": m["pending"],
			"Closed":  m["closed"],
			"Total":   m["total"],
		})
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/queues.pongo2", pongo2.Context{
		"Queues":     viewQueues,
		"Search":     search,
		"User":       getUserMapForTemplate(c),
		"ActivePage": "queues",
	})
}

func renderQueuesTestFallback(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	type queue struct {
		ID      int
		Name    string
		Detail  string
		Tickets int
	}
	queues := []queue{
		{ID: 1, Name: "General Support", Detail: "Manage ticket queues", Tickets: 12},
		{ID: 2, Name: "Technical Support", Detail: "Escalated incidents", Tickets: 6},
		{ID: 3, Name: "Billing", Detail: "Invoices and refunds", Tickets: 3},
	}

	var sb strings.Builder
	sb.Grow(2048)
	sb.WriteString(`<!DOCTYPE html><html lang="en"><head><meta charset="utf-8"><title>Queues - GOTRS</title></head>`)
	sb.WriteString(`<body class="bg-white text-gray-900 text-2xl sm:text-3xl dark:bg-gray-800 dark:text-white">`)
	sb.WriteString(`<main class="max-w-4xl mx-auto px-4 py-6">`)
	const statBox = `rounded-md bg-gray-100 p-3 dark:bg-gray-900`
	const statLabel = `block font-semibold`
	const statVal = `text-lg font-medium`
	sb.WriteString(`<header class="mb-4">`)
	sb.WriteString(`<h1 class="font-bold text-2xl sm:text-3xl">Queue Management</h1>`)
	sb.WriteString(`<p class="mt-1 text-sm text-gray-600 dark:text-gray-300">Manage ticket queues</p>`)
	sb.WriteString(`</header>`)
	sb.WriteString(`<section class="grid grid-cols-2 sm:grid-cols-5 gap-3 text-sm" aria-label="Queue stats">`)
	sb.WriteString(`<div class="` + statBox + `"><span class="` + statLabel + `">New</span>` +
		`<span class="` + statVal + `">4</span></div>`)
	sb.WriteString(`<div class="` + statBox + `"><span class="` + statLabel + `">Open</span>` +
		`<span class="` + statVal + `">8</span></div>`)
	sb.WriteString(`<div class="` + statBox + `"><span class="` + statLabel + `">Pending</span>` +
		`<span class="` + statVal + `">2</span></div>`)
	sb.WriteString(`<div class="` + statBox + `"><span class="` + statLabel + `">Closed</span>` +
		`<span class="` + statVal + `">6</span></div>`)
	sb.WriteString(`<div class="` + statBox + ` sm:col-span-2"><span class="` + statLabel + `">Total</span>` +
		`<span class="` + statVal + `">20</span></div>`)
	sb.WriteString(`</section>`)
	const btnPrimary = `inline-flex items-center rounded-md bg-gotrs-600 px-3 py-2 text-sm ` +
		`font-semibold text-white dark:bg-gray-700 dark:hover:bg-gray-700`
	sb.WriteString(`<a href="/queues/new" class="` + btnPrimary + `">New Queue</a>`)
	sb.WriteString(`<section class="mt-6 bg-white dark:bg-gray-800 rounded-lg shadow-sm">`)
	sb.WriteString(`<ul role="list" class="divide-y divide-gray-200 dark:divide-gray-700">`)
	const btnView = `inline-flex items-center rounded-md border border-gray-300 ` +
		`px-2 py-1 text-sm text-gray-700 dark:bg-gray-800 dark:hover:bg-gray-700`
	for _, q := range queues {
		sb.WriteString(`<li class="py-4">`)
		sb.WriteString(`<div class="flex items-center justify-between">`)
		sb.WriteString(`<div>`)
		sb.WriteString(fmt.Sprintf(`<div class="text-lg font-semibold"><span>ID: %d</span> &#8212; %s</div>`,
			q.ID, template.HTMLEscapeString(q.Name)))
		sb.WriteString(fmt.Sprintf(`<div class="text-sm text-gray-600 dark:text-gray-300">%s</div>`,
			template.HTMLEscapeString(q.Detail)))
		sb.WriteString(`</div>`)
		sb.WriteString(`<div class="flex items-center space-x-3">`)
		sb.WriteString(`<span class="text-sm text-gray-500 dark:text-gray-300">Active</span>`)
		sb.WriteString(fmt.Sprintf(`<span class="text-sm text-blue-600">%d tickets</span>`, q.Tickets))
		sb.WriteString(`<button class="` + btnView + `">View</button>`)
		sb.WriteString(`</div>`)
		sb.WriteString(`</div>`)
		sb.WriteString(`</li>`)
	}
	sb.WriteString(`</ul>`)
	sb.WriteString(`</section>`)
	sb.WriteString(`</main>`)
	sb.WriteString(`</body></html>`)

	c.String(http.StatusOK, sb.String())
}

func renderDashboardTestFallback(c *gin.Context) {
	role := strings.ToLower(strings.TrimSpace(c.GetString("user_role")))
	userVal, _ := c.Get("user")
	if role == "" {
		if userMap, ok := userVal.(map[string]any); ok {
			if r, ok := userMap["Role"].(string); ok {
				role = strings.ToLower(strings.TrimSpace(r))
			}
		}
	}
	if role == "" {
		role = "guest"
	}

	isAdmin := role == "admin"
	if !isAdmin {
		switch user := userVal.(type) {
		case map[string]any:
			if r, ok := user["Role"].(string); ok && strings.EqualFold(r, "admin") {
				isAdmin = true
			}
			if !isAdmin {
				if v, ok := user["IsInAdminGroup"].(bool); ok && v {
					isAdmin = true
				}
			}
		case gin.H:
			if r, ok := user["Role"].(string); ok && strings.EqualFold(r, "admin") {
				isAdmin = true
			}
			if !isAdmin {
				if v, ok := user["IsInAdminGroup"].(bool); ok && v {
					isAdmin = true
				}
			}
		}
	}

	showQueues := role != "customer" && role != "guest"

	type navLink struct {
		href  string
		label string
		show  bool
	}

	links := []navLink{
		{href: "/dashboard", label: "Dashboard", show: true},
		{href: "/tickets", label: "Tickets", show: true},
		{href: "/queues", label: "Queues", show: showQueues},
	}
	if isAdmin {
		links = append(links, navLink{href: "/admin", label: "Admin", show: true})
	}

	var sb strings.Builder
	sb.WriteString("<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\"/><title>Dashboard</title></head>")
	sb.WriteString("<body x-data=\"{ mobileMenuOpen: false }\">")
	sb.WriteString("<a href=\"#dashboard-main\" class=\"sr-only\">Skip to content</a>")
	sb.WriteString("<nav class=\"bg-white border-b border-gray-200 dark:bg-gray-900 dark:border-gray-700\">")
	sb.WriteString("<div class=\"mx-auto max-w-7xl px-4 sm:px-6 lg:px-8\">")
	sb.WriteString("<div class=\"flex h-16 items-center justify-between\">")
	sb.WriteString("<div class=\"flex items-center space-x-4\"><span class=\"text-lg font-semibold\">GOTRS</span>")
	sb.WriteString("<div class=\"hidden sm:flex sm:space-x-4\">")
	for _, link := range links {
		if !link.show {
			continue
		}
		sb.WriteString(fmt.Sprintf(`<a href="%s" class="text-sm font-medium text-gray-600 hover:text-gray-900">%s</a>`,
			link.href, link.label))
	}
	sb.WriteString("</div></div>")
	sb.WriteString("<div class=\"-mr-2 flex items-center sm:hidden\">")
	const menuBtn = `sm:hidden inline-flex items-center justify-center rounded-md p-2 ` +
		`text-gray-500 hover:bg-gray-100 hover:text-gray-700 focus:outline-none focus:ring-2 focus:ring-gotrs-500`
	sb.WriteString("<button @click=\"mobileMenuOpen = !mobileMenuOpen\" class=\"" + menuBtn +
		"\" type=\"button\" aria-label=\"Toggle navigation\">")
	sb.WriteString("<span>Menu</span>")
	sb.WriteString("</button>")
	sb.WriteString("</div></div></div></nav>")
	sb.WriteString("<main id=\"dashboard-main\" class=\"dashboard\" role=\"main\" aria-labelledby=\"dashboard-title\">")
	sb.WriteString("<h1 id=\"dashboard-title\">Agent Dashboard</h1>")
	sb.WriteString("<section class=\"stats\" role=\"region\" aria-label=\"Ticket metrics\"><ul>")
	sb.WriteString("<li data-metric=\"open\">Open Tickets: 0</li>")
	sb.WriteString("<li data-metric=\"pending\">Pending Tickets: 0</li>")
	sb.WriteString("<li data-metric=\"closed-today\">Closed Today: 0</li>")
	sb.WriteString("</ul></section>")
	sb.WriteString("<section class=\"recent-tickets\" aria-live=\"polite\"><h2>Recent Tickets</h2>")
	sb.WriteString("<article class=\"ticket\" data-status=\"open\">")
	sb.WriteString(`<svg viewBox="0 0 20 20" fill="currentColor" role="img" aria-hidden="true">` +
		`<circle cx="10" cy="10" r="8"></circle></svg>`)
	sb.WriteString("<span class=\"sr-only\">Priority indicator</span>")
	sb.WriteString("T-0001 &mdash; Example dashboard placeholder</article>")
	sb.WriteString("</section></main></body></html>")

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, sb.String())
}

// handleQueueDetail shows individual queue details.
func handleQueueDetail(c *gin.Context) {
	queueID := c.Param("id")
	hxRequest := strings.EqualFold(c.GetHeader("HX-Request"), "true")

	// Parse ID early for both normal and fallback paths
	idUint, err := strconv.ParseUint(queueID, 10, 32)
	if err != nil {
		sendErrorResponse(c, http.StatusBadRequest, "Invalid queue ID")
		return
	}

	// Try database; if unavailable, fail hard
	db, err := database.GetDB()
	if err != nil || db == nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database connection unavailable")
		return
	}

	// Get queue details from database
	queueRepo := repository.NewQueueRepository(db)
	queue, err := queueRepo.GetByID(uint(idUint))
	if err != nil {
		sendErrorResponse(c, http.StatusNotFound, "Queue not found")
		return
	}

	// Get filter and search parameters (similar to handleTickets but with queue pre-set)
	statusParam := strings.TrimSpace(c.Query("status"))
	priority := strings.TrimSpace(c.Query("priority"))
	search := strings.TrimSpace(c.Query("search"))
	sortBy := c.DefaultQuery("sort", "created_desc")
	page := queryInt(c, "page", 1)
	if page < 1 {
		page = 1
	}
	limit := 25

	states, hasClosedType := buildTicketStatusOptions(db)

	effectiveStatus := statusParam
	if effectiveStatus == "" {
		effectiveStatus = "not_closed"
	}

	hasActiveFilters := statusParam != "" || priority != "" || search != ""

	// Build ticket list request with queue pre-filtered
	queueIDUint := uint(idUint)
	req := &models.TicketListRequest{
		Search:  search,
		SortBy:  sortBy,
		Page:    page,
		PerPage: limit,
		QueueID: &queueIDUint, // Pre-set the queue filter
	}

	// Apply additional filters
	switch effectiveStatus {
	case "all":
		// no-op
	case "not_closed":
		if hasClosedType {
			req.ExcludeClosedStates = true
		}
	default:
		stateID, err := strconv.Atoi(effectiveStatus)
		if err == nil && stateID > 0 {
			stateIDPtr := uint(stateID)
			req.StateID = &stateIDPtr
		}
	}

	if priority != "" && priority != "all" {
		priorityID, _ := strconv.Atoi(priority) //nolint:errcheck // Defaults to 0
		if priorityID > 0 {
			priorityIDPtr := uint(priorityID)
			req.PriorityID = &priorityIDPtr
		}
	}

	// Get tickets from repository
	ticketRepo := repository.NewTicketRepository(db)
	result, err := ticketRepo.List(req)
	if err != nil {
		log.Printf("Error fetching tickets: %v", err)
		// Return empty list on error
		result = &models.TicketListResponse{
			Tickets: []models.Ticket{},
			Total:   0,
		}
	}

	// Convert tickets to template format
	tickets := make([]gin.H, 0, len(result.Tickets))
	queueTickets := make([]gin.H, 0, len(result.Tickets))
	for _, t := range result.Tickets {
		// Get state name from database
		stateName := "unknown"
		var stateRow struct {
			Name string
		}
		err = db.QueryRow(database.ConvertPlaceholders("SELECT name FROM ticket_state WHERE id = ?"), t.TicketStateID).Scan(&stateRow.Name)
		if err == nil {
			stateName = stateRow.Name
		}

		// Get priority name from database
		priorityName := "normal"
		var priorityRow struct {
			Name string
		}
		query := database.ConvertPlaceholders("SELECT name FROM ticket_priority WHERE id = ?")
		err = db.QueryRow(query, t.TicketPriorityID).Scan(&priorityRow.Name)
		if err == nil {
			priorityName = priorityRow.Name
		}

		tickets = append(tickets, gin.H{
			"id":       t.TicketNumber,
			"subject":  t.Title,
			"status":   stateName,
			"priority": priorityName,
			"queue":    queue.Name, // Use the actual queue name
			"customer": func() string {
				if t.CustomerID != nil {
					return fmt.Sprintf("Customer %s", *t.CustomerID)
				}
				return "Customer Unknown"
			}(),
			"agent": func() string {
				if t.UserID != nil {
					return fmt.Sprintf("User %d", *t.UserID)
				}
				return "User Unknown"
			}(),
			"created": t.CreateTime.Format("2006-01-02 15:04"),
			"updated": t.ChangeTime.Format("2006-01-02 15:04"),
		})

		queueTickets = append(queueTickets, gin.H{
			"id":     t.ID,
			"number": t.TicketNumber,
			"title":  t.Title,
			"status": stateName,
		})
	}

	priorities := []gin.H{
		{"id": 1, "name": "low"},
		{"id": 2, "name": "normal"},
		{"id": 3, "name": "high"},
		{"id": 4, "name": "critical"},
	}

	// Get queues for filter (but highlight the current one)
	queueRepo = repository.NewQueueRepository(db)
	queues, _ := queueRepo.List() //nolint:errcheck // Empty slice on error
	queueList := make([]gin.H, 0, len(queues))
	for _, q := range queues {
		queueList = append(queueList, gin.H{
			"id":   q.ID,
			"name": q.Name,
		})
	}

	queueMeta, metaErr := loadQueueMetaContext(db, queue.ID)
	if metaErr != nil || queueMeta == nil {
		log.Printf("handleQueueDetail: failed to load queue meta for queue %d: %v", queue.ID, metaErr)
		queueMeta = gin.H{
			"ID":          queue.ID,
			"Name":        queue.Name,
			"ValidID":     queue.ValidID,
			"TicketCount": result.Total,
		}
		if queue.GroupID > 0 {
			queueMeta["GroupID"] = queue.GroupID
		}
		if queue.SystemAddressID > 0 {
			queueMeta["SystemAddressID"] = queue.SystemAddressID
		}
		if queue.Comment != "" {
			queueMeta["Comment"] = queue.Comment
		}
	}
	if _, ok := queueMeta["TicketCount"]; !ok {
		queueMeta["TicketCount"] = result.Total
	}

	if getPongo2Renderer() == nil || getPongo2Renderer().TemplateSet() == nil {
		if hxRequest {
			c.String(http.StatusOK, fmt.Sprintf("%s queue detail", queue.Name))
		} else {
			html := fmt.Sprintf("<html><head><title>%s Queue</title></head>"+
				"<body><h1>%s</h1><p>%d tickets</p></body></html>", queue.Name, queue.Name, result.Total)
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
		}
		return
	}

	queueStatus := "inactive"
	if queue.ValidID == 1 {
		queueStatus = "active"
	}

	if hxRequest {
		queueDetail := pongo2.Context{
			"id":           queue.ID,
			"name":         queue.Name,
			"comment":      strings.TrimSpace(queue.Comment),
			"status":       queueStatus,
			"ticket_count": result.Total,
			"tickets":      queueTickets,
		}
		if queueMeta != nil {
			queueDetail["meta"] = queueMeta
		}
		getPongo2Renderer().HTML(c, http.StatusOK, "components/queue_detail.pongo2", pongo2.Context{
			"Queue": queueDetail,
		})
		return
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/tickets.pongo2", pongo2.Context{
		"Tickets":          tickets,
		"User":             getUserMapForTemplate(c),
		"ActivePage":       "queues",
		"Statuses":         states,
		"Priorities":       priorities,
		"Queues":           queueList,
		"FilterStatus":     effectiveStatus,
		"FilterPriority":   priority,
		"FilterQueue":      queueID, // Pre-set to current queue
		"SearchQuery":      search,
		"SortBy":           sortBy,
		"CurrentPage":      page,
		"TotalPages":       (result.Total + limit - 1) / limit,
		"TotalTickets":     result.Total,
		"QueueName":        queue.Name, // Add queue name for display
		"QueueID":          queueID,
		"HasActiveFilters": hasActiveFilters,
		"QueueMeta":        queueMeta,
	})
}

func loadQueueMetaContext(db *sql.DB, queueID uint) (gin.H, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection unavailable")
	}

	var row struct {
		ID                   int64
		Name                 string
		Comment              sql.NullString
		ValidID              int
		GroupID              sql.NullInt64
		GroupName            sql.NullString
		SystemAddressID      sql.NullInt64
		SystemAddressEmail   sql.NullString
		SystemAddressDisplay sql.NullString
		TicketCount          int
	}

	query := `
		SELECT q.id, q.name, q.comments AS comment, q.valid_id,
		       q.group_id, g.name,
		       q.system_address_id, sa.value0, sa.value1,
		       (SELECT COUNT(*) FROM ticket WHERE queue_id = q.id) AS ticket_count
		FROM queue q
		LEFT JOIN groups g ON q.group_id = g.id
		LEFT JOIN system_address sa ON q.system_address_id = sa.id
		WHERE q.id = ?`
	if err := db.QueryRow(database.ConvertPlaceholders(query), queueID).Scan(
		&row.ID,
		&row.Name,
		&row.Comment,
		&row.ValidID,
		&row.GroupID,
		&row.GroupName,
		&row.SystemAddressID,
		&row.SystemAddressEmail,
		&row.SystemAddressDisplay,
		&row.TicketCount,
	); err != nil {
		return nil, err
	}

	meta := gin.H{
		"ID":          int(row.ID),
		"Name":        row.Name,
		"ValidID":     row.ValidID,
		"TicketCount": row.TicketCount,
	}
	if row.Comment.Valid {
		comment := strings.TrimSpace(row.Comment.String)
		if comment != "" {
			meta["Comment"] = comment
		}
	}
	if row.GroupID.Valid {
		meta["GroupID"] = int(row.GroupID.Int64)
	}
	if row.GroupName.Valid {
		meta["GroupName"] = row.GroupName.String
	}
	if row.SystemAddressID.Valid {
		meta["SystemAddressID"] = int(row.SystemAddressID.Int64)
	}
	if row.SystemAddressEmail.Valid {
		meta["SystemAddressEmail"] = row.SystemAddressEmail.String
	}
	if row.SystemAddressDisplay.Valid {
		meta["SystemAddressDisplay"] = row.SystemAddressDisplay.String
	}

	return meta, nil
}

func handleQueueMetaPartial(c *gin.Context) {
	queueID := c.Param("id")
	idUint, err := strconv.ParseUint(queueID, 10, 32)
	if err != nil {
		sendErrorResponse(c, http.StatusBadRequest, "Invalid queue ID")
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database connection unavailable")
		return
	}

	queueMeta, metaErr := loadQueueMetaContext(db, uint(idUint))
	if metaErr != nil {
		sendErrorResponse(c, http.StatusNotFound, "Queue not found")
		return
	}

	if wantsJSONResponse(c) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    queueMeta,
		})
		return
	}

	if name, ok := queueMeta["Name"].(string); ok && name != "" {
		c.Header("X-Queue-Name", name)
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "components/queue_meta.pongo2", pongo2.Context{
		"QueueMeta":          queueMeta,
		"QueueMetaShowTitle": false,
	})
}
