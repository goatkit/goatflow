package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/services"
)

const (
	ProtocolVersion = "2024-11-05"
	ServerName      = "goatflow-mcp"
	ServerVersion   = "0.6.5"
)

// Server handles MCP protocol messages.
// The MCP server acts as a multi-user proxy - each request is authenticated
// by an API token, and the token owner's permissions apply to all operations.
// Most tools delegate to existing APIs which enforce RBAC. Tools that bypass
// the API layer (like execute_sql) require admin group membership.
type Server struct {
	db          *sql.DB
	userID      int
	userLogin   string
	initialized bool
	permissions *services.PermissionService
}

// NewServer creates a new MCP server instance.
// The server inherits the permissions of the authenticated user (identified by userID).
func NewServer(db *sql.DB, userID int, userLogin string) *Server {
	return &Server{
		db:          db,
		userID:      userID,
		userLogin:   userLogin,
		permissions: services.NewPermissionService(db),
	}
}

// HandleMessage processes a JSON-RPC message and returns a response.
func (s *Server) HandleMessage(ctx context.Context, msg []byte) ([]byte, error) {
	var req Request
	if err := json.Unmarshal(msg, &req); err != nil {
		resp := ErrorResponse(nil, ErrCodeParse, "Parse error: "+err.Error())
		return json.Marshal(resp)
	}

	if req.JSONRPC != "2.0" {
		resp := ErrorResponse(req.ID, ErrCodeInvalidRequest, "Invalid JSON-RPC version")
		return json.Marshal(resp)
	}

	var resp Response
	switch req.Method {
	case "initialize":
		resp = s.handleInitialize(req)
	case "initialized":
		// Client acknowledgment, no response needed
		return nil, nil
	case "tools/list":
		resp = s.handleToolsList(req)
	case "tools/call":
		resp = s.handleToolsCall(ctx, req)
	case "ping":
		resp = SuccessResponse(req.ID, map[string]string{})
	default:
		resp = ErrorResponse(req.ID, ErrCodeMethodNotFound, "Method not found: "+req.Method)
	}

	return json.Marshal(resp)
}

func (s *Server) handleInitialize(req Request) Response {
	var params InitializeParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return ErrorResponse(req.ID, ErrCodeInvalidParams, "Invalid params: "+err.Error())
		}
	}

	s.initialized = true

	return SuccessResponse(req.ID, InitializeResult{
		ProtocolVersion: ProtocolVersion,
		ServerInfo: ServerInfo{
			Name:    ServerName,
			Version: ServerVersion,
		},
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapability{},
		},
	})
}

func (s *Server) handleToolsList(req Request) Response {
	return SuccessResponse(req.ID, ToolsListResult{
		Tools: ToolRegistry,
	})
}

func (s *Server) handleToolsCall(ctx context.Context, req Request) Response {
	var params ToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return ErrorResponse(req.ID, ErrCodeInvalidParams, "Invalid params: "+err.Error())
	}

	result, err := s.executeTool(ctx, params.Name, params.Arguments)
	if err != nil {
		return SuccessResponse(req.ID, ToolCallResult{
			Content: []ContentBlock{TextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		})
	}

	return SuccessResponse(req.ID, result)
}

func (s *Server) executeTool(ctx context.Context, name string, args map[string]any) (*ToolCallResult, error) {
	switch name {
	case "list_tickets":
		return s.toolListTickets(ctx, args)
	case "get_ticket":
		return s.toolGetTicket(ctx, args)
	case "create_ticket":
		return s.toolCreateTicket(ctx, args)
	case "update_ticket":
		return s.toolUpdateTicket(ctx, args)
	case "add_article":
		return s.toolAddArticle(ctx, args)
	case "list_queues":
		return s.toolListQueues(ctx, args)
	case "list_users":
		return s.toolListUsers(ctx, args)
	case "search_tickets":
		return s.toolSearchTickets(ctx, args)
	case "get_statistics":
		return s.toolGetStatistics(ctx, args)
	case "execute_sql":
		return s.toolExecuteSQL(ctx, args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// Helper to get int from args
func getInt(args map[string]any, key string, defaultVal int) int {
	if v, ok := args[key]; ok {
		switch val := v.(type) {
		case float64:
			return int(val)
		case int:
			return val
		case json.Number:
			if i, err := val.Int64(); err == nil {
				return int(i)
			}
		}
	}
	return defaultVal
}

// Helper to get string from args
func getString(args map[string]any, key string, defaultVal string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultVal
}

// Helper to get bool from args
func getBool(args map[string]any, key string, defaultVal bool) bool {
	if v, ok := args[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return defaultVal
}

// Tool implementations

func (s *Server) toolListTickets(ctx context.Context, args map[string]any) (*ToolCallResult, error) {
	limit := getInt(args, "limit", 20)
	if limit > 100 {
		limit = 100
	}
	offset := getInt(args, "offset", 0)

	// Get user's accessible queues (RBAC filtering)
	accessibleQueues, err := s.permissions.GetUserQueuePermissions(s.userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get queue permissions: %w", err)
	}
	if len(accessibleQueues) == 0 {
		// User has no queue access - return empty result
		return &ToolCallResult{
			Content: []ContentBlock{TextContent("[]")},
		}, nil
	}

	// Build queue ID list for IN clause
	queueIDs := make([]any, 0, len(accessibleQueues))
	for qid := range accessibleQueues {
		queueIDs = append(queueIDs, qid)
	}
	queuePlaceholders := strings.Repeat("?,", len(queueIDs))
	queuePlaceholders = queuePlaceholders[:len(queuePlaceholders)-1] // trim trailing comma

	query := `SELECT t.id, t.tn, t.title, 
		ts.name as state, q.name as queue, p.name as priority,
		u.login as owner
		FROM ticket t
		LEFT JOIN ticket_state ts ON t.ticket_state_id = ts.id
		LEFT JOIN queue q ON t.queue_id = q.id
		LEFT JOIN ticket_priority p ON t.ticket_priority_id = p.id
		LEFT JOIN users u ON t.user_id = u.id
		WHERE t.queue_id IN (` + queuePlaceholders + `)`

	var conditions []string
	var queryArgs []any
	queryArgs = append(queryArgs, queueIDs...)

	if queueID := getInt(args, "queue_id", 0); queueID > 0 {
		conditions = append(conditions, "t.queue_id = ?")
		queryArgs = append(queryArgs, queueID)
	}
	if stateID := getInt(args, "state_id", 0); stateID > 0 {
		conditions = append(conditions, "t.ticket_state_id = ?")
		queryArgs = append(queryArgs, stateID)
	}
	if ownerID := getInt(args, "owner_id", 0); ownerID > 0 {
		conditions = append(conditions, "t.user_id = ?")
		queryArgs = append(queryArgs, ownerID)
	}
	if customerID := getString(args, "customer_id", ""); customerID != "" {
		conditions = append(conditions, "t.customer_id = ?")
		queryArgs = append(queryArgs, customerID)
	}

	if len(conditions) > 0 {
		query += " AND " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY t.id DESC LIMIT ? OFFSET ?"
	queryArgs = append(queryArgs, limit, offset)

	query = database.ConvertPlaceholders(query)
	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		var id int64
		var tn, title string
		var state, queue, priority, owner sql.NullString

		if err := rows.Scan(&id, &tn, &title, &state, &queue, &priority, &owner); err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}

		results = append(results, map[string]any{
			"id":       id,
			"number":   tn,
			"title":    title,
			"state":    state.String,
			"queue":    queue.String,
			"priority": priority.String,
			"owner":    owner.String,
		})
	}

	output, _ := json.MarshalIndent(results, "", "  ")
	return &ToolCallResult{
		Content: []ContentBlock{TextContent(string(output))},
	}, nil
}

func (s *Server) toolGetTicket(ctx context.Context, args map[string]any) (*ToolCallResult, error) {
	ticketID := getInt(args, "ticket_id", 0)
	ticketNumber := getString(args, "ticket_number", "")
	includeArticles := getBool(args, "include_articles", true)

	if ticketID == 0 && ticketNumber == "" {
		return nil, fmt.Errorf("either ticket_id or ticket_number is required")
	}

	// First, check if user has access to this ticket's queue
	var checkQuery string
	var checkArg any
	if ticketID > 0 {
		checkQuery = "SELECT id, queue_id FROM ticket WHERE id = ?"
		checkArg = ticketID
	} else {
		checkQuery = "SELECT id, queue_id FROM ticket WHERE tn = ?"
		checkArg = ticketNumber
	}
	checkQuery = database.ConvertPlaceholders(checkQuery)

	var actualTicketID int64
	var queueID int
	if err := s.db.QueryRowContext(ctx, checkQuery, checkArg).Scan(&actualTicketID, &queueID); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("ticket not found")
		}
		return nil, fmt.Errorf("query failed: %w", err)
	}

	// Check queue permission (RBAC)
	canRead, err := s.permissions.CanReadQueue(s.userID, queueID)
	if err != nil {
		return nil, fmt.Errorf("failed to check permissions: %w", err)
	}
	if !canRead {
		// Security: return "not found" to avoid revealing ticket existence
		return nil, fmt.Errorf("ticket not found")
	}

	var query string
	var queryArg any
	if ticketID > 0 {
		query = `SELECT t.id, t.tn, t.title, t.customer_id, t.customer_user_id,
			ts.name as state, q.name as queue, p.name as priority,
			u.login as owner, t.create_time, t.change_time
			FROM ticket t
			LEFT JOIN ticket_state ts ON t.ticket_state_id = ts.id
			LEFT JOIN queue q ON t.queue_id = q.id
			LEFT JOIN ticket_priority p ON t.ticket_priority_id = p.id
			LEFT JOIN users u ON t.user_id = u.id
			WHERE t.id = ?`
		queryArg = ticketID
	} else {
		query = `SELECT t.id, t.tn, t.title, t.customer_id, t.customer_user_id,
			ts.name as state, q.name as queue, p.name as priority,
			u.login as owner, t.create_time, t.change_time
			FROM ticket t
			LEFT JOIN ticket_state ts ON t.ticket_state_id = ts.id
			LEFT JOIN queue q ON t.queue_id = q.id
			LEFT JOIN ticket_priority p ON t.ticket_priority_id = p.id
			LEFT JOIN users u ON t.user_id = u.id
			WHERE t.tn = ?`
		queryArg = ticketNumber
	}

	query = database.ConvertPlaceholders(query)
	row := s.db.QueryRowContext(ctx, query, queryArg)

	var id int64
	var tn, title string
	var customerID, customerUserID, state, queue, priority, owner sql.NullString
	var createTime, changeTime sql.NullTime

	if err := row.Scan(&id, &tn, &title, &customerID, &customerUserID, &state, &queue, &priority, &owner, &createTime, &changeTime); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("ticket not found")
		}
		return nil, fmt.Errorf("query failed: %w", err)
	}

	ticket := map[string]any{
		"id":               id,
		"number":           tn,
		"title":            title,
		"customer_id":      customerID.String,
		"customer_user_id": customerUserID.String,
		"state":            state.String,
		"queue":            queue.String,
		"priority":         priority.String,
		"owner":            owner.String,
		"create_time":      createTime.Time.Format("2006-01-02 15:04:05"),
		"change_time":      changeTime.Time.Format("2006-01-02 15:04:05"),
	}

	if includeArticles {
		articlesQuery := database.ConvertPlaceholders(`
			SELECT a.id, a.article_sender_type_id, adm.a_subject, adm.a_body, a.create_time,
				ast.name as sender_type
			FROM article a
			LEFT JOIN article_data_mime adm ON a.id = adm.article_id
			LEFT JOIN article_sender_type ast ON a.article_sender_type_id = ast.id
			WHERE a.ticket_id = ?
			ORDER BY a.create_time ASC`)

		rows, err := s.db.QueryContext(ctx, articlesQuery, id)
		if err != nil {
			return nil, fmt.Errorf("articles query failed: %w", err)
		}
		defer rows.Close()

		var articles []map[string]any
		for rows.Next() {
			var artID int64
			var senderTypeID int
			var subject, body sql.NullString
			var artCreateTime sql.NullTime
			var senderType sql.NullString

			if err := rows.Scan(&artID, &senderTypeID, &subject, &body, &artCreateTime, &senderType); err != nil {
				continue
			}

			articles = append(articles, map[string]any{
				"id":          artID,
				"sender_type": senderType.String,
				"subject":     subject.String,
				"body":        body.String,
				"create_time": artCreateTime.Time.Format("2006-01-02 15:04:05"),
			})
		}
		ticket["articles"] = articles
	}

	output, _ := json.MarshalIndent(ticket, "", "  ")
	return &ToolCallResult{
		Content: []ContentBlock{TextContent(string(output))},
	}, nil
}

func (s *Server) toolCreateTicket(ctx context.Context, args map[string]any) (*ToolCallResult, error) {
	title := getString(args, "title", "")
	queueID := getInt(args, "queue_id", 0)
	priorityID := getInt(args, "priority_id", 3) // Default: normal
	stateID := getInt(args, "state_id", 1)       // Default: new
	customerUser := getString(args, "customer_user", "")
	body := getString(args, "body", "")

	if title == "" || queueID == 0 || body == "" {
		return nil, fmt.Errorf("title, queue_id, and body are required")
	}

	// Check 'create' permission on target queue (RBAC)
	canCreate, err := s.permissions.CanCreate(s.userID, queueID)
	if err != nil {
		return nil, fmt.Errorf("failed to check permissions: %w", err)
	}
	if !canCreate {
		return nil, fmt.Errorf("no permission to create tickets in this queue")
	}

	// Generate ticket number
	tn := fmt.Sprintf("%d%06d", time.Now().Unix()%1000000, time.Now().UnixNano()%1000000)

	now := time.Now()

	// Insert ticket
	ticketQuery := database.ConvertPlaceholders(`
		INSERT INTO ticket (tn, title, queue_id, ticket_lock_id, type_id, 
			ticket_state_id, ticket_priority_id, customer_id, customer_user_id,
			user_id, responsible_user_id, timeout, until_time, escalation_time,
			escalation_update_time, escalation_response_time, escalation_solution_time,
			archive_flag, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, 1, 1, ?, ?, ?, ?, ?, ?, 0, 0, 0, 0, 0, 0, 0, ?, ?, ?, ?)`)

	result, err := s.db.ExecContext(ctx, ticketQuery,
		tn, title, queueID, stateID, priorityID, "", customerUser,
		s.userID, s.userID, now, s.userID, now, s.userID)
	if err != nil {
		return nil, fmt.Errorf("failed to create ticket: %w", err)
	}

	ticketID, _ := result.LastInsertId()

	// Insert initial article
	articleQuery := database.ConvertPlaceholders(`
		INSERT INTO article (ticket_id, article_sender_type_id, communication_channel_id,
			is_visible_for_customer, create_time, create_by, change_time, change_by)
		VALUES (?, 1, 1, 1, ?, ?, ?, ?)`)

	articleResult, err := s.db.ExecContext(ctx, articleQuery,
		ticketID, now, s.userID, now, s.userID)
	if err != nil {
		return nil, fmt.Errorf("failed to create article: %w", err)
	}

	articleID, _ := articleResult.LastInsertId()

	// Insert article content
	mimeQuery := database.ConvertPlaceholders(`
		INSERT INTO article_data_mime (article_id, a_from, a_to, a_subject, a_body,
			a_content_type, incoming_time, create_time, create_by, change_time, change_by)
		VALUES (?, ?, '', ?, ?, 'text/plain', ?, ?, ?, ?, ?)`)

	fromUser := s.userLogin
	incomingTime := now.Unix()
	_, err = s.db.ExecContext(ctx, mimeQuery,
		articleID, fromUser, title, body, incomingTime, now, s.userID, now, s.userID)
	if err != nil {
		return nil, fmt.Errorf("failed to create article content: %w", err)
	}

	output, _ := json.MarshalIndent(map[string]any{
		"ticket_id":     ticketID,
		"ticket_number": tn,
		"article_id":    articleID,
		"message":       "Ticket created successfully",
	}, "", "  ")
	return &ToolCallResult{
		Content: []ContentBlock{TextContent(string(output))},
	}, nil
}

func (s *Server) toolUpdateTicket(ctx context.Context, args map[string]any) (*ToolCallResult, error) {
	ticketID := getInt(args, "ticket_id", 0)
	if ticketID == 0 {
		return nil, fmt.Errorf("ticket_id is required")
	}

	// Get ticket's queue for permission check
	var queueID int
	checkQuery := database.ConvertPlaceholders("SELECT queue_id FROM ticket WHERE id = ?")
	if err := s.db.QueryRowContext(ctx, checkQuery, ticketID).Scan(&queueID); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("ticket not found")
		}
		return nil, fmt.Errorf("query failed: %w", err)
	}

	// Check base read permission first
	canRead, err := s.permissions.CanReadQueue(s.userID, queueID)
	if err != nil {
		return nil, fmt.Errorf("failed to check permissions: %w", err)
	}
	if !canRead {
		return nil, fmt.Errorf("ticket not found") // Security: don't reveal existence
	}

	// Build update query based on provided fields
	var updates []string
	var updateArgs []any

	if title := getString(args, "title", ""); title != "" {
		updates = append(updates, "title = ?")
		updateArgs = append(updateArgs, title)
	}

	if stateID := getInt(args, "state_id", 0); stateID > 0 {
		updates = append(updates, "ticket_state_id = ?")
		updateArgs = append(updateArgs, stateID)
	}

	if priorityID := getInt(args, "priority_id", 0); priorityID > 0 {
		// Check priority permission
		canPriority, err := s.permissions.CanChangePriority(s.userID, int64(ticketID))
		if err != nil {
			return nil, fmt.Errorf("failed to check permissions: %w", err)
		}
		if !canPriority {
			return nil, fmt.Errorf("no permission to change priority")
		}
		updates = append(updates, "ticket_priority_id = ?")
		updateArgs = append(updateArgs, priorityID)
	}

	if newQueueID := getInt(args, "queue_id", 0); newQueueID > 0 && newQueueID != queueID {
		// Check move_into permission on target queue
		canMove, err := s.permissions.CanMoveInto(s.userID, newQueueID)
		if err != nil {
			return nil, fmt.Errorf("failed to check permissions: %w", err)
		}
		if !canMove {
			return nil, fmt.Errorf("no permission to move ticket to this queue")
		}
		updates = append(updates, "queue_id = ?")
		updateArgs = append(updateArgs, newQueueID)
	}

	if ownerID := getInt(args, "owner_id", 0); ownerID > 0 {
		// Check owner permission
		canOwn, err := s.permissions.CanBeOwner(s.userID, queueID)
		if err != nil {
			return nil, fmt.Errorf("failed to check permissions: %w", err)
		}
		if !canOwn {
			return nil, fmt.Errorf("no permission to change owner")
		}
		updates = append(updates, "user_id = ?")
		updateArgs = append(updateArgs, ownerID)
	}

	if len(updates) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}

	// Add change tracking
	updates = append(updates, "change_time = ?", "change_by = ?")
	updateArgs = append(updateArgs, time.Now(), s.userID, ticketID)

	query := database.ConvertPlaceholders(
		"UPDATE ticket SET " + strings.Join(updates, ", ") + " WHERE id = ?")

	_, err = s.db.ExecContext(ctx, query, updateArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to update ticket: %w", err)
	}

	output, _ := json.MarshalIndent(map[string]any{
		"ticket_id": ticketID,
		"message":   "Ticket updated successfully",
		"updated":   strings.Join(updates[:len(updates)-2], ", "), // Exclude change_time, change_by
	}, "", "  ")
	return &ToolCallResult{
		Content: []ContentBlock{TextContent(string(output))},
	}, nil
}

func (s *Server) toolAddArticle(ctx context.Context, args map[string]any) (*ToolCallResult, error) {
	ticketID := getInt(args, "ticket_id", 0)
	body := getString(args, "body", "")
	subject := getString(args, "subject", "")
	articleType := getString(args, "article_type", "note-internal")

	if ticketID == 0 || body == "" {
		return nil, fmt.Errorf("ticket_id and body are required")
	}

	// Get ticket info for permission check and default subject
	var queueID int
	var ticketTitle string
	checkQuery := database.ConvertPlaceholders("SELECT queue_id, title FROM ticket WHERE id = ?")
	if err := s.db.QueryRowContext(ctx, checkQuery, ticketID).Scan(&queueID, &ticketTitle); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("ticket not found")
		}
		return nil, fmt.Errorf("query failed: %w", err)
	}

	// Check 'note' permission (RBAC)
	canNote, err := s.permissions.CanAddNote(s.userID, int64(ticketID))
	if err != nil {
		return nil, fmt.Errorf("failed to check permissions: %w", err)
	}
	if !canNote {
		return nil, fmt.Errorf("ticket not found") // Security: don't reveal existence
	}

	// Default subject to ticket title
	if subject == "" {
		subject = ticketTitle
	}

	// Determine visibility based on article type
	isVisibleForCustomer := 0
	senderTypeID := 1 // agent
	if articleType == "note-external" || articleType == "email-external" {
		isVisibleForCustomer = 1
	}

	now := time.Now()

	// Insert article
	articleQuery := database.ConvertPlaceholders(`
		INSERT INTO article (ticket_id, article_sender_type_id, communication_channel_id,
			is_visible_for_customer, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 1, ?, ?, ?, ?, ?)`)

	articleResult, err := s.db.ExecContext(ctx, articleQuery,
		ticketID, senderTypeID, isVisibleForCustomer, now, s.userID, now, s.userID)
	if err != nil {
		return nil, fmt.Errorf("failed to create article: %w", err)
	}

	articleID, _ := articleResult.LastInsertId()

	// Insert article content
	mimeQuery := database.ConvertPlaceholders(`
		INSERT INTO article_data_mime (article_id, a_from, a_to, a_subject, a_body,
			a_content_type, incoming_time, create_time, create_by, change_time, change_by)
		VALUES (?, ?, '', ?, ?, 'text/plain', ?, ?, ?, ?, ?)`)

	fromUser := s.userLogin
	incomingTime := now.Unix()
	_, err = s.db.ExecContext(ctx, mimeQuery,
		articleID, fromUser, subject, body, incomingTime, now, s.userID, now, s.userID)
	if err != nil {
		return nil, fmt.Errorf("failed to create article content: %w", err)
	}

	// Update ticket change time
	updateQuery := database.ConvertPlaceholders("UPDATE ticket SET change_time = ?, change_by = ? WHERE id = ?")
	_, _ = s.db.ExecContext(ctx, updateQuery, now, s.userID, ticketID)

	output, _ := json.MarshalIndent(map[string]any{
		"article_id":   articleID,
		"ticket_id":    ticketID,
		"article_type": articleType,
		"message":      "Article added successfully",
	}, "", "  ")
	return &ToolCallResult{
		Content: []ContentBlock{TextContent(string(output))},
	}, nil
}

func (s *Server) toolListQueues(ctx context.Context, args map[string]any) (*ToolCallResult, error) {
	// Get user's accessible queues (RBAC filtering)
	accessibleQueues, err := s.permissions.GetUserQueuePermissions(s.userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get queue permissions: %w", err)
	}
	if len(accessibleQueues) == 0 {
		return &ToolCallResult{
			Content: []ContentBlock{TextContent("[]")},
		}, nil
	}

	// Build queue ID list for IN clause
	queueIDs := make([]any, 0, len(accessibleQueues))
	for qid := range accessibleQueues {
		queueIDs = append(queueIDs, qid)
	}
	queuePlaceholders := strings.Repeat("?,", len(queueIDs))
	queuePlaceholders = queuePlaceholders[:len(queuePlaceholders)-1]

	query := database.ConvertPlaceholders(`
		SELECT q.id, q.name, q.group_id, g.name as group_name
		FROM queue q
		LEFT JOIN ` + "`groups`" + ` g ON q.group_id = g.id
		WHERE q.valid_id = 1 AND q.id IN (` + queuePlaceholders + `)
		ORDER BY q.name`)

	rows, err := s.db.QueryContext(ctx, query, queueIDs...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		var id, groupID int64
		var name string
		var groupName sql.NullString

		if err := rows.Scan(&id, &name, &groupID, &groupName); err != nil {
			continue
		}

		results = append(results, map[string]any{
			"id":         id,
			"name":       name,
			"group_id":   groupID,
			"group_name": groupName.String,
		})
	}

	output, _ := json.MarshalIndent(results, "", "  ")
	return &ToolCallResult{
		Content: []ContentBlock{TextContent(string(output))},
	}, nil
}

func (s *Server) toolListUsers(ctx context.Context, args map[string]any) (*ToolCallResult, error) {
	validOnly := getBool(args, "valid", true)
	limit := getInt(args, "limit", 50)

	query := `SELECT id, login, first_name, last_name, title, valid_id
		FROM users WHERE 1=1`

	var queryArgs []any
	if validOnly {
		query += " AND valid_id = 1"
	}
	query += " ORDER BY login LIMIT ?"
	queryArgs = append(queryArgs, limit)

	query = database.ConvertPlaceholders(query)
	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		var id, validID int64
		var login string
		var firstName, lastName, title sql.NullString

		if err := rows.Scan(&id, &login, &firstName, &lastName, &title, &validID); err != nil {
			continue
		}

		results = append(results, map[string]any{
			"id":         id,
			"login":      login,
			"first_name": firstName.String,
			"last_name":  lastName.String,
			"title":      title.String,
			"valid":      validID == 1,
		})
	}

	output, _ := json.MarshalIndent(results, "", "  ")
	return &ToolCallResult{
		Content: []ContentBlock{TextContent(string(output))},
	}, nil
}

func (s *Server) toolSearchTickets(ctx context.Context, args map[string]any) (*ToolCallResult, error) {
	queryStr := getString(args, "query", "")
	if queryStr == "" {
		return nil, fmt.Errorf("query is required")
	}
	limit := getInt(args, "limit", 20)

	// Get user's accessible queues (RBAC filtering)
	accessibleQueues, err := s.permissions.GetUserQueuePermissions(s.userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get queue permissions: %w", err)
	}
	if len(accessibleQueues) == 0 {
		// User has no queue access - return empty result
		return &ToolCallResult{
			Content: []ContentBlock{TextContent("[]")},
		}, nil
	}

	// Build queue ID list for IN clause
	queueIDs := make([]any, 0, len(accessibleQueues))
	for qid := range accessibleQueues {
		queueIDs = append(queueIDs, qid)
	}
	queuePlaceholders := strings.Repeat("?,", len(queueIDs))
	queuePlaceholders = queuePlaceholders[:len(queuePlaceholders)-1]

	// Simple LIKE search across title and ticket number, filtered by accessible queues
	searchTerm := "%" + queryStr + "%"
	query := `
		SELECT t.id, t.tn, t.title, ts.name as state, q.name as queue
		FROM ticket t
		LEFT JOIN ticket_state ts ON t.ticket_state_id = ts.id
		LEFT JOIN queue q ON t.queue_id = q.id
		WHERE t.queue_id IN (` + queuePlaceholders + `)
		  AND (LOWER(t.title) LIKE LOWER(?) OR t.tn LIKE ?)
		ORDER BY t.id DESC
		LIMIT ?`
	query = database.ConvertPlaceholders(query)

	queryArgs := append(queueIDs, searchTerm, searchTerm, limit)
	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		var id int64
		var tn, title string
		var state, queue sql.NullString

		if err := rows.Scan(&id, &tn, &title, &state, &queue); err != nil {
			continue
		}

		results = append(results, map[string]any{
			"id":     id,
			"number": tn,
			"title":  title,
			"state":  state.String,
			"queue":  queue.String,
		})
	}

	output, _ := json.MarshalIndent(results, "", "  ")
	return &ToolCallResult{
		Content: []ContentBlock{TextContent(string(output))},
	}, nil
}

func (s *Server) toolGetStatistics(ctx context.Context, args map[string]any) (*ToolCallResult, error) {
	stats := make(map[string]any)

	// Get user's accessible queues (RBAC filtering)
	accessibleQueues, err := s.permissions.GetUserQueuePermissions(s.userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get queue permissions: %w", err)
	}
	if len(accessibleQueues) == 0 {
		stats["tickets_by_state"] = map[string]int{}
		stats["tickets_by_queue"] = map[string]int{}
		stats["total_tickets"] = 0
		stats["total_users"] = 0
		output, _ := json.MarshalIndent(stats, "", "  ")
		return &ToolCallResult{
			Content: []ContentBlock{TextContent(string(output))},
		}, nil
	}

	// Build queue ID list for IN clause
	queueIDs := make([]any, 0, len(accessibleQueues))
	for qid := range accessibleQueues {
		queueIDs = append(queueIDs, qid)
	}
	queuePlaceholders := strings.Repeat("?,", len(queueIDs))
	queuePlaceholders = queuePlaceholders[:len(queuePlaceholders)-1]

	// Tickets by state (filtered by accessible queues)
	stateQuery := database.ConvertPlaceholders(`
		SELECT ts.name, COUNT(*) as count
		FROM ticket t
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		WHERE t.queue_id IN (` + queuePlaceholders + `)
		GROUP BY ts.name`)

	rows, err := s.db.QueryContext(ctx, stateQuery, queueIDs...)
	if err == nil {
		byState := make(map[string]int)
		for rows.Next() {
			var name string
			var count int
			if rows.Scan(&name, &count) == nil {
				byState[name] = count
			}
		}
		rows.Close()
		stats["tickets_by_state"] = byState
	}

	// Tickets by queue (filtered by accessible queues)
	queueQuery := database.ConvertPlaceholders(`
		SELECT q.name, COUNT(*) as count
		FROM ticket t
		JOIN queue q ON t.queue_id = q.id
		WHERE t.queue_id IN (` + queuePlaceholders + `)
		GROUP BY q.name`)

	rows, err = s.db.QueryContext(ctx, queueQuery, queueIDs...)
	if err == nil {
		byQueue := make(map[string]int)
		for rows.Next() {
			var name string
			var count int
			if rows.Scan(&name, &count) == nil {
				byQueue[name] = count
			}
		}
		rows.Close()
		stats["tickets_by_queue"] = byQueue
	}

	// Total counts (filtered by accessible queues)
	var totalTickets int
	countQuery := database.ConvertPlaceholders(
		"SELECT COUNT(*) FROM ticket WHERE queue_id IN (" + queuePlaceholders + ")")
	s.db.QueryRowContext(ctx, countQuery, queueIDs...).Scan(&totalTickets)
	stats["total_tickets"] = totalTickets

	// User count is generally not sensitive
	var totalUsers int
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE valid_id = 1").Scan(&totalUsers)
	stats["total_users"] = totalUsers

	output, _ := json.MarshalIndent(stats, "", "  ")
	return &ToolCallResult{
		Content: []ContentBlock{TextContent(string(output))},
	}, nil
}

func (s *Server) toolExecuteSQL(ctx context.Context, args map[string]any) (*ToolCallResult, error) {
	// Security: execute_sql requires admin group membership.
	// This tool bypasses the normal API layer and its RBAC checks,
	// so it's restricted to administrators only.
	isAdmin, err := s.permissions.IsInGroup(s.userID, "admin")
	if err != nil {
		return nil, fmt.Errorf("failed to check permissions: %w", err)
	}
	if !isAdmin {
		return nil, fmt.Errorf("execute_sql requires admin group membership")
	}

	query := getString(args, "query", "")
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	// Security: Only allow SELECT queries
	trimmed := strings.TrimSpace(strings.ToUpper(query))
	if !strings.HasPrefix(trimmed, "SELECT") {
		return nil, fmt.Errorf("only SELECT queries are allowed")
	}

	// Get args if provided
	var queryArgs []any
	if argsVal, ok := args["args"]; ok {
		if argsSlice, ok := argsVal.([]any); ok {
			queryArgs = argsSlice
		}
	}

	query = database.ConvertPlaceholders(query)
	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("get columns failed: %w", err)
	}

	var results []map[string]any
	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}

		row := make(map[string]any)
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}

	output, _ := json.MarshalIndent(map[string]any{
		"columns":    columns,
		"rows":       results,
		"rows_count": len(results),
	}, "", "  ")
	return &ToolCallResult{
		Content: []ContentBlock{TextContent(string(output))},
	}, nil
}
