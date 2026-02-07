package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/ticketnumber"
)

// TicketRepository handles database operations for tickets.
type TicketRepository struct {
	db        *sql.DB
	generator ticketnumber.Generator
	store     ticketnumber.CounterStore

	historyTypeCache map[string]int
	historyMu        sync.RWMutex
}

// ExecContext represents the subset of database operations needed to insert history entries.
type ExecContext interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
}

var defaultTicketNumberGen ticketnumber.Generator
var defaultTicketNumberStore ticketnumber.CounterStore

// SetTicketNumberGenerator injects the global ticket number generator and store used by new repositories.
func SetTicketNumberGenerator(gen ticketnumber.Generator, store ticketnumber.CounterStore) {
	defaultTicketNumberGen = gen
	defaultTicketNumberStore = store
}

// TicketNumberGeneratorInfo returns current generator name and date-based flag.
func TicketNumberGeneratorInfo() (string, bool) {
	if defaultTicketNumberGen == nil {
		return "", false
	}
	return defaultTicketNumberGen.Name(), defaultTicketNumberGen.IsDateBased()
}

// NewTicketRepository creates a new ticket repository.
func NewTicketRepository(db *sql.DB) *TicketRepository {
	return &TicketRepository{
		db:               db,
		generator:        defaultTicketNumberGen,
		store:            defaultTicketNumberStore,
		historyTypeCache: make(map[string]int),
	}
}

// QueueExists checks whether a queue with the given ID exists.
func (r *TicketRepository) QueueExists(queueID int) (bool, error) {
	var exists bool
	// Keep SQL shape matching tests: SELECT EXISTS(SELECT 1 FROM queue ...)
	q := database.ConvertPlaceholders("SELECT EXISTS(SELECT 1 FROM queue WHERE id = ?)")
	if err := r.db.QueryRow(q, queueID).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

// GetTicketStateByID returns the ticket state row for a given ID if it exists.
func (r *TicketRepository) GetTicketStateByID(stateID int) (*models.TicketState, error) {
	row := r.db.QueryRow(database.ConvertPlaceholders(`
		SELECT id, name, type_id, valid_id,
		       create_time, create_by, change_time, change_by
		FROM ticket_state
		WHERE id = ?
	`), stateID)
	var state models.TicketState
	if err := row.Scan(
		&state.ID,
		&state.Name,
		&state.TypeID,
		&state.ValidID,
		&state.CreateTime,
		&state.CreateBy,
		&state.ChangeTime,
		&state.ChangeBy,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil //nolint:nilnil
		}
		return nil, err
	}
	return &state, nil
}

// GetDB returns the database connection.
func (r *TicketRepository) GetDB() *sql.DB {
	return r.db
}

// Create creates a new ticket in the database.
func (r *TicketRepository) Create(ticket *models.Ticket) error {
	if (r.generator == nil || r.store == nil) && (defaultTicketNumberGen != nil && defaultTicketNumberStore != nil) {
		r.generator = defaultTicketNumberGen
		r.store = defaultTicketNumberStore
	}
	if r.generator == nil || r.store == nil {
		return fmt.Errorf("ticket number generator not initialized")
	}

	const randomRetries = 5
	try := 0
	for {
		try++
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		n, err := r.generator.Next(ctx, r.store)
		cancel()
		if err != nil {
			return fmt.Errorf("ticket number generation failed: %w", err)
		}
		ticket.TicketNumber = n
		ticket.CreateTime = time.Now()
		ticket.ChangeTime = time.Now()

		err = r.insertTicket(ticket)
		if err == nil {
			return nil
		}

		if r.generator.Name() == "Random" && isUniqueTNError(err) && try < randomRetries {
			log.Printf("⚠️  Random TN collision on %s (attempt %d) retrying", n, try)
			continue
		}
		return err
	}
}

// insertTicket performs the actual INSERT returning ticket id.
func (r *TicketRepository) insertTicket(ticket *models.Ticket) error {
	query := fmt.Sprintf(`
		INSERT INTO ticket (
			tn, title, queue_id, ticket_lock_id, %s,
			service_id, sla_id, user_id, responsible_user_id,
			customer_id, customer_user_id, ticket_state_id,
			ticket_priority_id, timeout, until_time, escalation_time,
			escalation_update_time, escalation_response_time,
			escalation_solution_time, archive_flag,
			create_time, create_by, change_time, change_by
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		) RETURNING id`, database.TicketTypeColumn())

	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)

	// Use adapter for database-specific handling
	adapter := database.GetAdapter()
	args := []interface{}{
		ticket.TicketNumber,
		ticket.Title,
		ticket.QueueID,
		ticket.TicketLockID,
		valueOrNil(ticket.TypeID),
		valueOrNil(ticket.ServiceID),
		valueOrNil(ticket.SLAID),
		valueOrNil(ticket.UserID),
		valueOrNil(ticket.ResponsibleUserID),
		valueOrNil(ticket.CustomerID),
		valueOrNil(ticket.CustomerUserID),
		ticket.TicketStateID,
		ticket.TicketPriorityID,
		ticket.Timeout,
		ticket.UntilTime,
		ticket.EscalationTime,
		ticket.EscalationUpdateTime,
		ticket.EscalationResponseTime,
		ticket.EscalationSolutionTime,
		ticket.ArchiveFlag,
		ticket.CreateTime,
		ticket.CreateBy,
		ticket.ChangeTime,
		ticket.ChangeBy,
	}
	ticketID, err := adapter.InsertWithReturning(
		r.db,
		query,
		args...,
	)
	if err != nil {
		return err
	}
	ticket.ID = int(ticketID)
	return nil
}

func valueOrNil[T any](ptr *T) interface{} {
	if ptr == nil {
		return nil
	}
	return *ptr
}

// isUniqueTNError detects a unique constraint violation on the ticket number.
func isUniqueTNError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if strings.Contains(msg, "unique") && strings.Contains(msg, "tn") {
		return true
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false
	}
	return false
}

// GetByID retrieves a ticket by its ID.
func (r *TicketRepository) GetByID(id uint) (*models.Ticket, error) {
	query := `
		SELECT
			t.id, t.tn, t.title, t.queue_id, t.ticket_lock_id, t.type_id,
			t.service_id, t.sla_id, t.user_id, t.responsible_user_id,
			t.customer_id, t.customer_user_id, t.ticket_state_id,
			t.ticket_priority_id, t.until_time, t.escalation_time,
			t.escalation_update_time, t.escalation_response_time,
			t.escalation_solution_time, t.archive_flag,
			t.create_time, t.create_by, t.change_time, t.change_by
		FROM ticket t
		WHERE t.id = ?`

	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)

	var ticket models.Ticket
	err := r.db.QueryRow(query, id).Scan(
		&ticket.ID,
		&ticket.TicketNumber,
		&ticket.Title,
		&ticket.QueueID,
		&ticket.TicketLockID,
		&ticket.TypeID,
		&ticket.ServiceID,
		&ticket.SLAID,
		&ticket.UserID,
		&ticket.ResponsibleUserID,
		&ticket.CustomerID,
		&ticket.CustomerUserID,
		&ticket.TicketStateID,
		&ticket.TicketPriorityID,
		&ticket.UntilTime,
		&ticket.EscalationTime,
		&ticket.EscalationUpdateTime,
		&ticket.EscalationResponseTime,
		&ticket.EscalationSolutionTime,
		&ticket.ArchiveFlag,
		&ticket.CreateTime,
		&ticket.CreateBy,
		&ticket.ChangeTime,
		&ticket.ChangeBy,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("ticket not found")
	}

	return &ticket, err
}

// GetByTN retrieves a ticket by its ticket number.
func (r *TicketRepository) GetByTN(tn string) (*models.Ticket, error) {
	query := `
		SELECT
			t.id, t.tn, t.title, t.queue_id, t.ticket_lock_id, t.type_id,
			t.service_id, t.sla_id, t.user_id, t.responsible_user_id,
			t.customer_id, t.customer_user_id, t.ticket_state_id,
			t.ticket_priority_id, t.until_time, t.escalation_time,
			t.escalation_update_time, t.escalation_response_time,
			t.escalation_solution_time, t.archive_flag,
			t.create_time, t.create_by, t.change_time, t.change_by
		FROM ticket t
		WHERE t.tn = ?`

	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)

	var ticket models.Ticket
	err := r.db.QueryRow(query, tn).Scan(
		&ticket.ID,
		&ticket.TicketNumber,
		&ticket.Title,
		&ticket.QueueID,
		&ticket.TicketLockID,
		&ticket.TypeID,
		&ticket.ServiceID,
		&ticket.SLAID,
		&ticket.UserID,
		&ticket.ResponsibleUserID,
		&ticket.CustomerID,
		&ticket.CustomerUserID,
		&ticket.TicketStateID,
		&ticket.TicketPriorityID,
		&ticket.UntilTime,
		&ticket.EscalationTime,
		&ticket.EscalationUpdateTime,
		&ticket.EscalationResponseTime,
		&ticket.EscalationSolutionTime,
		&ticket.ArchiveFlag,
		&ticket.CreateTime,
		&ticket.CreateBy,
		&ticket.ChangeTime,
		&ticket.ChangeBy,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("ticket not found")
	}

	return &ticket, err
}

// Update updates a ticket in the database.
func (r *TicketRepository) Update(ticket *models.Ticket) error {
	ticket.ChangeTime = time.Now()

	query := `
		UPDATE ticket SET
			title = ?,
			queue_id = ?,
			ticket_lock_id = ?,
			type_id = ?,
			service_id = ?,
			sla_id = ?,
			user_id = ?,
			responsible_user_id = ?,
			customer_id = ?,
			customer_user_id = ?,
			ticket_state_id = ?,
			ticket_priority_id = ?,
			until_time = ?,
			escalation_time = ?,
			escalation_update_time = ?,
			escalation_response_time = ?,
			escalation_solution_time = ?,
			archive_flag = ?,
			change_time = ?,
			change_by = ?
		WHERE id = ?`

	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)

	result, err := r.db.Exec(
		query,
		ticket.ID,
		ticket.Title,
		ticket.QueueID,
		ticket.TicketLockID,
		ticket.TypeID,
		ticket.ServiceID,
		ticket.SLAID,
		ticket.UserID,
		ticket.ResponsibleUserID,
		ticket.CustomerID,
		ticket.CustomerUserID,
		ticket.TicketStateID,
		ticket.TicketPriorityID,
		ticket.UntilTime,
		ticket.EscalationTime,
		ticket.EscalationUpdateTime,
		ticket.EscalationResponseTime,
		ticket.EscalationSolutionTime,
		ticket.ArchiveFlag,
		ticket.ChangeTime,
		ticket.ChangeBy,
	)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("ticket not found")
	}

	return nil
}

// Delete deletes a ticket from the database.
func (r *TicketRepository) Delete(id uint) error {
	query := `DELETE FROM ticket WHERE id = ?`
	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)
	result, err := r.db.Exec(query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("ticket not found")
	}

	return nil
}

// List retrieves a paginated list of tickets with filters.
func (r *TicketRepository) List(req *models.TicketListRequest) (*models.TicketListResponse, error) {
	// Set defaults
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PerPage <= 0 {
		req.PerPage = 25
	}
	if req.PerPage > 100 {
		req.PerPage = 100
	}

	// Build base query
	baseQuery := `FROM ticket t
	LEFT JOIN ticket_state ts ON t.ticket_state_id = ts.id
	LEFT JOIN ticket_state_type tst ON ts.type_id = tst.id
	WHERE 1=1`
	countQuery := `SELECT COUNT(*) ` + baseQuery
	selectQuery := `
		SELECT
			t.id, t.tn, t.title, t.queue_id, t.ticket_lock_id, t.type_id,
			t.service_id, t.sla_id, t.user_id, t.responsible_user_id,
			t.customer_id, t.customer_user_id, t.ticket_state_id,
			t.ticket_priority_id, t.until_time, t.escalation_time,
			t.escalation_update_time, t.escalation_response_time,
			t.escalation_solution_time, t.archive_flag,
			t.create_time, t.create_by, t.change_time, t.change_by
		` + baseQuery

	// Build filters
	var filters []string
	var args []interface{}

	if req.QueueID != nil {
		filters = append(filters, " AND t.queue_id = ?")
		args = append(args, *req.QueueID)
	}

	if req.StateID != nil {
		filters = append(filters, " AND t.ticket_state_id = ?")
		args = append(args, *req.StateID)
	}

	if req.ExcludeClosedStates {
		filters = append(filters, " AND (tst.name IS NULL OR LOWER(tst.name) != 'closed')")
	}

	if req.PriorityID != nil {
		filters = append(filters, " AND t.ticket_priority_id = ?")
		args = append(args, *req.PriorityID)
	}

	if req.CustomerID != nil {
		filters = append(filters, " AND t.customer_id = ?")
		args = append(args, *req.CustomerID)
	}

	if req.OwnerID != nil {
		filters = append(filters, " AND t.user_id = ?")
		args = append(args, *req.OwnerID)
	}

	if req.ArchiveFlag != nil {
		filters = append(filters, " AND t.archive_flag = ?")
		args = append(args, *req.ArchiveFlag)
	}

	if req.Search != "" {
		filters = append(filters, " AND (LOWER(t.title) LIKE LOWER(?) OR LOWER(t.tn) LIKE LOWER(?))")
		args = append(args, "%"+req.Search+"%", "%"+req.Search+"%")
	}

	if req.StartDate != nil {
		filters = append(filters, " AND t.create_time >= ?")
		args = append(args, *req.StartDate)
	}

	if req.EndDate != nil {
		filters = append(filters, " AND t.create_time <= ?")
		args = append(args, *req.EndDate)
	}

	// Filter by accessible queue IDs (permission filtering)
	if len(req.AccessibleQueueIDs) > 0 {
		placeholders := make([]string, len(req.AccessibleQueueIDs))
		for i, qid := range req.AccessibleQueueIDs {
			placeholders[i] = "?"
			args = append(args, qid)
		}
		filters = append(filters, " AND t.queue_id IN ("+strings.Join(placeholders, ",")+")")
	}

	filterString := strings.Join(filters, "")

	// Get total count
	var total int
	// Convert placeholders for MySQL compatibility
	countQueryConverted := database.ConvertPlaceholders(countQuery + filterString)
	err := r.db.QueryRow(countQueryConverted, args...).Scan(&total)
	if err != nil {
		return nil, err
	}

	// Add sorting
	orderBy := " ORDER BY "
	switch req.SortBy {
	case "tn":
		orderBy += "t.tn"
	case "title":
		orderBy += "t.title"
	case "state":
		orderBy += "t.ticket_state_id"
	case "priority":
		orderBy += "t.ticket_priority_id"
	case "queue":
		orderBy += "t.queue_id"
	case "created":
		orderBy += "t.create_time"
	case "updated":
		orderBy += "t.change_time"
	default:
		orderBy += "t.create_time"
	}

	if req.SortOrder == "asc" {
		orderBy += " ASC"
	} else {
		orderBy += " DESC"
	}

	// Add pagination
	offset := (req.Page - 1) * req.PerPage
	limitClause := fmt.Sprintf(" LIMIT %d OFFSET %d", req.PerPage, offset)

	// Execute query
	// Convert placeholders for MySQL compatibility
	fullQuery := database.ConvertPlaceholders(selectQuery + filterString + orderBy + limitClause)
	rows, err := r.db.Query(fullQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Parse results
	var tickets []models.Ticket
	for rows.Next() {
		ticket, err := models.ScanTicket(rows)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, *ticket)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Calculate total pages
	totalPages := total / req.PerPage
	if total%req.PerPage > 0 {
		totalPages++
	}

	return &models.TicketListResponse{
		Tickets:    tickets,
		Total:      total,
		Page:       req.Page,
		PerPage:    req.PerPage,
		TotalPages: totalPages,
	}, nil
}

// GetTicketsByCustomer retrieves all tickets for a specific customer.
func (r *TicketRepository) GetTicketsByCustomer(customerID uint, includeArchived bool) ([]models.Ticket, error) {
	query := `
		SELECT
			t.id, t.tn, t.title, t.queue_id, t.ticket_lock_id, t.type_id,
			t.service_id, t.sla_id, t.user_id, t.responsible_user_id,
			t.customer_id, t.customer_user_id, t.ticket_state_id,
			t.ticket_priority_id, t.until_time, t.escalation_time,
			t.escalation_update_time, t.escalation_response_time,
			t.escalation_solution_time, t.archive_flag,
			t.create_time, t.create_by, t.change_time, t.change_by
		FROM ticket t
		WHERE t.customer_id = ?`

	if !includeArchived {
		query += " AND t.archive_flag = 0"
	}

	query += " ORDER BY t.create_time DESC"

	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)

	rows, err := r.db.Query(query, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tickets []models.Ticket
	for rows.Next() {
		ticket, err := models.ScanTicket(rows)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, *ticket)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tickets, nil
}

// GetTicketsByOwner retrieves all tickets assigned to a specific user.
func (r *TicketRepository) GetTicketsByOwner(ownerID uint, includeArchived bool) ([]models.Ticket, error) {
	query := `
		SELECT
			t.id, t.tn, t.title, t.queue_id, t.ticket_lock_id, t.type_id,
			t.service_id, t.sla_id, t.user_id, t.responsible_user_id,
			t.customer_id, t.customer_user_id, t.ticket_state_id,
			t.ticket_priority_id, t.until_time, t.escalation_time,
			t.escalation_update_time, t.escalation_response_time,
			t.escalation_solution_time, t.archive_flag,
			t.create_time, t.create_by, t.change_time, t.change_by
		FROM ticket t
		WHERE (t.user_id = ? OR t.responsible_user_id = ?)`

	if !includeArchived {
		query += " AND t.archive_flag = 0"
	}

	query += " ORDER BY t.create_time DESC"

	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)

	rows, err := r.db.Query(query, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tickets []models.Ticket
	for rows.Next() {
		ticket, err := models.ScanTicket(rows)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, *ticket)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tickets, nil
}

// GetTicketWithRelations retrieves a ticket with all related data.
func (r *TicketRepository) GetTicketWithRelations(id uint) (*models.Ticket, error) {
	query := `
		SELECT
			t.id, t.tn, t.title, t.queue_id, t.ticket_lock_id, t.type_id,
			t.service_id, t.sla_id, t.user_id, t.responsible_user_id,
			t.customer_id, t.customer_user_id, t.ticket_state_id,
			t.ticket_priority_id, t.until_time, t.escalation_time,
			t.escalation_update_time, t.escalation_response_time,
			t.escalation_solution_time, t.archive_flag,
			t.create_time, t.create_by, t.change_time, t.change_by,
			q.id, q.name, q.group_id, q.comment,
			ts.id, ts.name, ts.type_id,
			tp.id, tp.name
		FROM ticket t
		LEFT JOIN queue q ON t.queue_id = q.id
		LEFT JOIN ticket_state ts ON t.ticket_state_id = ts.id
		LEFT JOIN ticket_priority tp ON t.ticket_priority_id = tp.id
		WHERE t.id = ?`

	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)

	var ticket models.Ticket
	var queue models.Queue
	var state models.TicketState
	var priority models.TicketPriority

	err := r.db.QueryRow(query, id).Scan(
		&ticket.ID,
		&ticket.TicketNumber,
		&ticket.Title,
		&ticket.QueueID,
		&ticket.TicketLockID,
		&ticket.TypeID,
		&ticket.ServiceID,
		&ticket.SLAID,
		&ticket.UserID,
		&ticket.ResponsibleUserID,
		&ticket.CustomerID,
		&ticket.CustomerUserID,
		&ticket.TicketStateID,
		&ticket.TicketPriorityID,
		&ticket.UntilTime,
		&ticket.EscalationTime,
		&ticket.EscalationUpdateTime,
		&ticket.EscalationResponseTime,
		&ticket.EscalationSolutionTime,
		&ticket.ArchiveFlag,
		&ticket.CreateTime,
		&ticket.CreateBy,
		&ticket.ChangeTime,
		&ticket.ChangeBy,
		&queue.ID,
		&queue.Name,
		&queue.GroupID,
		&queue.Comment,
		&state.ID,
		&state.Name,
		&state.TypeID,
		&priority.ID,
		&priority.Name,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("ticket not found")
	}

	if err != nil {
		return nil, err
	}

	ticket.Queue = &queue
	ticket.State = &state
	ticket.Priority = &priority

	return &ticket, nil
}

// LockTicket locks a ticket for a specific user.
func (r *TicketRepository) LockTicket(ticketID uint, userID uint, lockType int) error {
	query := `
		UPDATE ticket
		SET ticket_lock_id = ?, user_id = ?, change_time = ?, change_by = ?
		WHERE id = ? AND ticket_lock_id = ?`

	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)

	result, err := r.db.Exec(
		query,
		ticketID,
		lockType,
		userID,
		time.Now(),
		userID,
		models.TicketUnlocked,
	)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("ticket is already locked or not found")
	}

	return nil
}

// UnlockTicket unlocks a ticket.
func (r *TicketRepository) UnlockTicket(ticketID uint, userID uint) error {
	query := `
		UPDATE ticket
		SET ticket_lock_id = ?, change_time = ?, change_by = ?
		WHERE id = ?`

	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)

	_, err := r.db.Exec(
		query,
		ticketID,
		models.TicketUnlocked,
		time.Now(),
		userID,
	)

	return err
}

// ArchiveTicket archives a ticket.
func (r *TicketRepository) ArchiveTicket(ticketID uint, userID uint) error {
	query := `
		UPDATE ticket
		SET archive_flag = 1, change_time = ?, change_by = ?
		WHERE id = ?`

	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)

	_, err := r.db.Exec(query, ticketID, time.Now(), userID)
	return err
}

// RestoreTicket restores an archived ticket.
func (r *TicketRepository) RestoreTicket(ticketID uint, userID uint) error {
	query := `
		UPDATE ticket
		SET archive_flag = 0, change_time = ?, change_by = ?
		WHERE id = ?`

	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)

	_, err := r.db.Exec(query, ticketID, time.Now(), userID)
	return err
}

// UpdateStatus updates the status of a ticket.
func (r *TicketRepository) UpdateStatus(ticketID uint, stateID uint, userID uint) error {
	query := `
		UPDATE ticket
		SET ticket_state_id = ?, change_time = ?, change_by = ?
		WHERE id = ?`

	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)

	_, err := r.db.Exec(query, ticketID, stateID, time.Now(), userID)
	return err
}

// UpdatePriority updates the priority of a ticket.
func (r *TicketRepository) UpdatePriority(ticketID uint, priorityID uint, userID uint) error {
	query := `
		UPDATE ticket
		SET ticket_priority_id = ?, change_time = ?, change_by = ?
		WHERE id = ?`

	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)

	_, err := r.db.Exec(query, ticketID, priorityID, time.Now(), userID)
	return err
}

// UpdateQueue transfers a ticket to a different queue.
func (r *TicketRepository) UpdateQueue(ticketID uint, queueID uint, userID uint) error {
	query := `
		UPDATE ticket
		SET queue_id = ?, change_time = ?, change_by = ?
		WHERE id = ?`

	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)

	_, err := r.db.Exec(query, ticketID, queueID, time.Now(), userID)
	return err
}

// GetQueues retrieves all active queues.
func (r *TicketRepository) GetQueues() ([]models.Queue, error) {
	query := `
		SELECT id, name, group_id, comment, unlock_timeout,
		       follow_up_id, follow_up_lock, valid_id,
		       create_time, create_by, change_time, change_by
		FROM queue
		WHERE valid_id = 1
		ORDER BY name`

	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var queues []models.Queue
	for rows.Next() {
		var q models.Queue
		err := rows.Scan(
			&q.ID,
			&q.Name,
			&q.GroupID,
			&q.Comment,
			&q.UnlockTimeout,
			&q.FollowUpID,
			&q.FollowUpLock,
			&q.ValidID,
			&q.CreateTime,
			&q.CreateBy,
			&q.ChangeTime,
			&q.ChangeBy,
		)
		if err != nil {
			return nil, err
		}
		queues = append(queues, q)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return queues, nil
}

// GetTicketStates retrieves all active ticket states.
func (r *TicketRepository) GetTicketStates() ([]models.TicketState, error) {
	query := `
		SELECT id, name, type_id, valid_id,
		       create_time, create_by, change_time, change_by
		FROM ticket_state
		WHERE valid_id = 1
		ORDER BY name`

	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var states []models.TicketState
	for rows.Next() {
		var s models.TicketState
		err := rows.Scan(
			&s.ID,
			&s.Name,
			&s.TypeID,
			&s.ValidID,
			&s.CreateTime,
			&s.CreateBy,
			&s.ChangeTime,
			&s.ChangeBy,
		)
		if err != nil {
			return nil, err
		}
		states = append(states, s)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return states, nil
}

// GetTicketPriorities retrieves all active ticket priorities.
func (r *TicketRepository) GetTicketPriorities() ([]models.TicketPriority, error) {
	query := `
		SELECT id, name, valid_id,
		       create_time, create_by, change_time, change_by
		FROM ticket_priority
		WHERE valid_id = 1
		ORDER BY id`

	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var priorities []models.TicketPriority
	for rows.Next() {
		var p models.TicketPriority
		err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.ValidID,
			&p.CreateTime,
			&p.CreateBy,
			&p.ChangeTime,
			&p.ChangeBy,
		)
		if err != nil {
			return nil, err
		}
		priorities = append(priorities, p)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return priorities, nil
}

// GetByTicketNumber retrieves a ticket by its ticket number.
func (r *TicketRepository) GetByTicketNumber(ticketNumber string) (*models.Ticket, error) {
	var ticket models.Ticket
	query := `
		SELECT
			id, tn, title, queue_id, ticket_lock_id, type_id,
			service_id, sla_id, user_id, responsible_user_id, customer_id,
			customer_user_id, ticket_state_id, ticket_priority_id, until_time,
			escalation_time, escalation_update_time, escalation_response_time,
			escalation_solution_time, archive_flag, create_time, create_by,
			change_time, change_by
		FROM ticket
		WHERE tn = ?
	`

	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)

	err := r.db.QueryRow(query, ticketNumber).Scan(
		&ticket.ID,
		&ticket.TicketNumber,
		&ticket.Title,
		&ticket.QueueID,
		&ticket.TicketLockID,
		&ticket.TypeID,
		&ticket.ServiceID,
		&ticket.SLAID,
		&ticket.UserID,
		&ticket.ResponsibleUserID,
		&ticket.CustomerID,
		&ticket.CustomerUserID,
		&ticket.TicketStateID,
		&ticket.TicketPriorityID,
		&ticket.UntilTime,
		&ticket.EscalationTime,
		&ticket.EscalationUpdateTime,
		&ticket.EscalationResponseTime,
		&ticket.EscalationSolutionTime,
		&ticket.ArchiveFlag,
		&ticket.CreateTime,
		&ticket.CreateBy,
		&ticket.ChangeTime,
		&ticket.ChangeBy,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("ticket not found")
	}

	return &ticket, err
}

// Count returns the total number of tickets.
func (r *TicketRepository) Count() (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM ticket`
	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)
	err := r.db.QueryRow(query).Scan(&count)
	return count, err
}

// CountByStatus returns the number of tickets with a specific status.
func (r *TicketRepository) CountByStatus(status string) (int, error) {
	var count int
	query := `
		SELECT COUNT(*)
		FROM ticket t
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		WHERE ts.name = ?
	`
	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)
	err := r.db.QueryRow(query, status).Scan(&count)
	return count, err
}

// CountByStateID returns the number of tickets with a specific state ID.
func (r *TicketRepository) CountByStateID(stateID int) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM ticket WHERE ticket_state_id = ?`
	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)
	err := r.db.QueryRow(query, stateID).Scan(&count)
	return count, err
}

// CountClosedToday returns the number of tickets closed today.
func (r *TicketRepository) CountClosedToday() (int, error) {
	var count int
	query := `
		SELECT COUNT(*) FROM ticket
		WHERE ticket_state_id = 3
		AND change_time >= CURRENT_DATE
	`
	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)
	err := r.db.QueryRow(query).Scan(&count)
	return count, err
}

// AddTicketHistoryEntry persists a ticket_history row for the provided ticket snapshot.
// The exec parameter accepts interface{} to satisfy the history.HistoryInserter interface,
// but must be either nil or implement ExecContext.
func (r *TicketRepository) AddTicketHistoryEntry(ctx context.Context, exec interface{}, entry models.TicketHistoryInsert) error {
	if r == nil || r.db == nil {
		return errors.New("ticket repository not initialized")
	}
	if entry.TicketID <= 0 {
		return errors.New("ticket id required")
	}
	entry.HistoryType = strings.TrimSpace(entry.HistoryType)
	if entry.HistoryType == "" {
		return errors.New("history type required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	historyTypeID, err := r.getHistoryTypeID(ctx, entry.HistoryType)
	if err != nil {
		return err
	}

	// Type assert exec to ExecContext if provided, otherwise use default db
	var executor ExecContext
	if exec != nil {
		var ok bool
		executor, ok = exec.(ExecContext)
		if !ok {
			return errors.New("exec must implement ExecContext")
		}
	} else {
		executor = r.db
	}

	now := entry.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}

	typeID := entry.TypeID
	if typeID < 0 {
		typeID = 0
	}

	var article interface{}
	if entry.ArticleID != nil && *entry.ArticleID > 0 {
		article = *entry.ArticleID
	} else {
		article = nil
	}

	query := fmt.Sprintf(`
		INSERT INTO ticket_history (
			name, history_type_id, ticket_id, article_id, %s, queue_id, owner_id,
			priority_id, state_id, create_time, create_by, change_time, change_by
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
	`, database.TicketTypeColumn())

	_, err = executor.ExecContext(ctx, database.ConvertPlaceholders(query),
		entry.Name,
		historyTypeID,
		entry.TicketID,
		article,
		typeID,
		entry.QueueID,
		entry.OwnerID,
		entry.PriorityID,
		entry.StateID,
		now,
		entry.CreatedBy,
		now,
		entry.CreatedBy,
	)
	return err
}

func (r *TicketRepository) getHistoryTypeID(ctx context.Context, name string) (int, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("ticket repository not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return 0, errors.New("history type name required")
	}

	r.historyMu.RLock()
	if id, ok := r.historyTypeCache[trimmed]; ok {
		r.historyMu.RUnlock()
		return id, nil
	}
	r.historyMu.RUnlock()

	query := database.ConvertPlaceholders(`SELECT id FROM ticket_history_type WHERE name = ?`)
	var id int
	if err := r.db.QueryRowContext(ctx, query, trimmed).Scan(&id); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return 0, err
		}

		createdID, createErr := r.ensureHistoryType(ctx, trimmed)
		if createErr != nil {
			return 0, createErr
		}
		id = createdID
	}

	r.historyMu.Lock()
	if r.historyTypeCache == nil {
		r.historyTypeCache = make(map[string]int)
	}
	r.historyTypeCache[trimmed] = id
	r.historyMu.Unlock()

	return id, nil
}

func (r *TicketRepository) ensureHistoryType(ctx context.Context, name string) (int, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("ticket repository not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	adapter := database.GetAdapter()
	now := time.Now().UTC()
	query := `
		INSERT INTO ticket_history_type (
			name, valid_id, create_time, create_by, change_time, change_by
		) VALUES (?,?,?,?,?,?)
		RETURNING id
	`

	id64, err := adapter.InsertWithReturning(r.db, database.ConvertPlaceholders(query), name, 1, now, 1, now, 1)
	if err != nil {
		lookup := database.ConvertPlaceholders(`SELECT id FROM ticket_history_type WHERE name = ?`)
		var existing int
		if scanErr := r.db.QueryRowContext(ctx, lookup, name).Scan(&existing); scanErr == nil {
			return existing, nil
		}
		return 0, err
	}

	return int(id64), nil
}

// GetTicketHistoryEntries returns recent history entries for a ticket.
func (r *TicketRepository) GetTicketHistoryEntries(ticketID uint, limit int) ([]models.TicketHistoryEntry, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT
			th.id,
			COALESCE(tht.name, ''),
			COALESCE(th.name, ''),
			th.create_time,
			COALESCE(u.login, ''),
			COALESCE(u.first_name, ''),
			COALESCE(u.last_name, ''),
			COALESCE(adm.a_subject, ''),
			COALESCE(q.name, ''),
			COALESCE(ts.name, ''),
			COALESCE(tp.name, '')
		FROM ticket_history th
		LEFT JOIN ticket_history_type tht ON tht.id = th.history_type_id
		LEFT JOIN users u ON u.id = th.create_by
		LEFT JOIN article a ON a.id = th.article_id
		LEFT JOIN article_data_mime adm ON adm.article_id = a.id
		LEFT JOIN queue q ON q.id = th.queue_id
		LEFT JOIN ticket_state ts ON ts.id = th.state_id
		LEFT JOIN ticket_priority tp ON tp.id = th.priority_id
		WHERE th.ticket_id = ?
		ORDER BY th.create_time DESC
		LIMIT ?
	`

	rows, err := r.db.Query(database.ConvertQuery(query), ticketID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]models.TicketHistoryEntry, 0)
	for rows.Next() {
		var (
			id       int64
			typeName string
			name     string
			created  time.Time
			login    string
			first    string
			last     string
			subject  string
			queue    string
			state    string
			priority string
		)

		if err := rows.Scan(&id, &typeName, &name, &created, &login, &first, &last, &subject, &queue, &state, &priority); err != nil {
			return nil, err
		}

		entry := models.TicketHistoryEntry{
			ID:              uint(id),
			HistoryType:     typeName,
			Name:            name,
			CreatorLogin:    login,
			CreatorFullName: strings.TrimSpace(fmt.Sprintf("%s %s", first, last)),
			CreatedAt:       created,
			ArticleSubject:  subject,
			QueueName:       queue,
			StateName:       state,
			PriorityName:    priority,
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}

// GetTicketLinks returns linked tickets for a ticket.
func (r *TicketRepository) GetTicketLinks(ticketID uint, limit int) ([]models.TicketLink, error) {
	if limit <= 0 {
		limit = 50
	}
	ticketKey := fmt.Sprintf("%d", ticketID)

	query := `
		SELECT
			lr.source_key,
			lr.target_key,
			COALESCE(lt.name, ''),
			COALESCE(ls.name, ''),
			lr.create_time,
			COALESCE(u.login, ''),
			COALESCE(u.first_name, ''),
			COALESCE(u.last_name, ''),
			src.id,
			COALESCE(src.tn, ''),
			COALESCE(src.title, ''),
			dst.id,
			COALESCE(dst.tn, ''),
			COALESCE(dst.title, '')
		FROM link_relation lr
		JOIN link_object src_obj ON src_obj.id = lr.source_object_id
		JOIN link_object dst_obj ON dst_obj.id = lr.target_object_id
		LEFT JOIN link_type lt ON lt.id = lr.type_id
		LEFT JOIN link_state ls ON ls.id = lr.state_id
		LEFT JOIN users u ON u.id = lr.create_by
		LEFT JOIN ticket src ON src.id = NULLIF(lr.source_key, '')::integer
		LEFT JOIN ticket dst ON dst.id = NULLIF(lr.target_key, '')::integer
		WHERE (src_obj.name = 'Ticket' AND lr.source_key = ?)
		   OR (dst_obj.name = 'Ticket' AND lr.target_key = ?)
		ORDER BY lr.create_time DESC
		LIMIT ?
	`

	args := []interface{}{ticketKey, limit}
	if database.IsMySQL() {
		args = []interface{}{ticketKey, ticketKey, limit}
	}

	rows, err := r.db.Query(database.ConvertQuery(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	links := make([]models.TicketLink, 0)
	for rows.Next() {
		var (
			sourceKey string
			targetKey string
			typeName  string
			stateName string
			created   time.Time
			login     string
			first     string
			last      string
			srcID     sql.NullInt64
			srcTN     string
			srcTitle  string
			dstID     sql.NullInt64
			dstTN     string
			dstTitle  string
		)

		if err := rows.Scan(&sourceKey, &targetKey, &typeName, &stateName, &created, &login, &first, &last, &srcID, &srcTN, &srcTitle, &dstID, &dstTN, &dstTitle); err != nil {
			return nil, err
		}

		direction := "outbound"
		relatedID := dstID
		relatedTN := dstTN
		relatedTitle := dstTitle
		if sourceKey != ticketKey {
			direction = "inbound"
			relatedID = srcID
			relatedTN = srcTN
			relatedTitle = srcTitle
		}

		if !relatedID.Valid || relatedID.Int64 <= 0 {
			continue
		}

		link := models.TicketLink{
			RelatedTicketID:    uint(relatedID.Int64),
			RelatedTicketTN:    relatedTN,
			RelatedTicketTitle: relatedTitle,
			LinkType:           typeName,
			LinkState:          stateName,
			Direction:          direction,
			CreatorLogin:       login,
			CreatorFullName:    strings.TrimSpace(fmt.Sprintf("%s %s", first, last)),
			CreatedAt:          created,
		}
		links = append(links, link)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return links, nil
}
