package api

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/routing"
	"github.com/goatkit/goatflow/internal/repository"
)

func init() {
	routing.RegisterHandler("handleQueues", handleQueues)
	routing.RegisterHandler("handleQueueDetail", handleQueueDetail)
	routing.RegisterHandler("handleQueueMetaPartial", handleQueueMetaPartial)
}

// handleQueues shows the queues list page.
func handleQueues(c *gin.Context) {
	// If templates are unavailable, return error
	if getPongo2Renderer() == nil || getPongo2Renderer().TemplateSet() == nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Template system unavailable")
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		log.Printf("handleQueues: database unavailable: %v", err)
		sendErrorResponse(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	// Optional search filter
	search := strings.TrimSpace(c.Query("search"))
	searchLower := strings.ToLower(search)

	// Get user ID and check if admin
	userID := GetUserIDFromCtxUint(c, 0)

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
	rows, qerr := db.Query(database.ConvertPlaceholders(query))
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

	states, hasClosedType, err := buildTicketStatusOptions(db)
	if err != nil {
		log.Printf("queue %d detail: %v", idUint, err)
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to load ticket states")
		return
	}

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
	if metaErr != nil {
		log.Printf("handleQueueDetail: failed to load queue meta for queue %d: %v", queue.ID, metaErr)
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to load queue details")
		return
	}
	if _, ok := queueMeta["TicketCount"]; !ok {
		queueMeta["TicketCount"] = result.Total
	}

	if getPongo2Renderer() == nil || getPongo2Renderer().TemplateSet() == nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Template renderer unavailable")
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
