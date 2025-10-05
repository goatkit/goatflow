package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/ticketnumber"
)

// TicketRepository handles database operations for tickets
type TicketRepository struct {
	db        *sql.DB
	generator ticketnumber.Generator
	store     ticketnumber.CounterStore
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
	if defaultTicketNumberGen == nil { return "", false }
	return defaultTicketNumberGen.Name(), defaultTicketNumberGen.IsDateBased()
}

// NewTicketRepository creates a new ticket repository
func NewTicketRepository(db *sql.DB) *TicketRepository {
	return &TicketRepository{db: db, generator: defaultTicketNumberGen, store: defaultTicketNumberStore}
}

// GetDB returns the database connection
func (r *TicketRepository) GetDB() *sql.DB {
	return r.db
}

// Create creates a new ticket in the database
func (r *TicketRepository) Create(ticket *models.Ticket) error {
	log.Println("DEBUG: SENTINEL TicketRepository.Create entered")
	if (r.generator == nil || r.store == nil) && (defaultTicketNumberGen != nil && defaultTicketNumberStore != nil) {
		r.generator = defaultTicketNumberGen
		r.store = defaultTicketNumberStore
		log.Printf("DEBUG: late-binding ticket number generator=%s", r.generator.Name())
	}
	if r.generator == nil || r.store == nil { return fmt.Errorf("ticket number generator not initialized") }

	const randomRetries = 5
	try := 0
	for {
		try++
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		n, err := r.generator.Next(ctx, r.store)
		cancel()
		if err != nil { return fmt.Errorf("ticket number generation failed: %w", err) }
		ticket.TicketNumber = n
		ticket.CreateTime = time.Now()
		ticket.ChangeTime = time.Now()

		err = r.insertTicket(ticket)
		if err == nil { return nil }

		if r.generator.Name() == "Random" && isUniqueTNError(err) && try < randomRetries {
			log.Printf("⚠️  Random TN collision on %s (attempt %d) retrying", n, try)
			continue
		}
		return err
	}
}

// insertTicket performs the actual INSERT returning ticket id.
func (r *TicketRepository) insertTicket(ticket *models.Ticket) error {

	query := `
		INSERT INTO ticket (
			tn, title, queue_id, ticket_lock_id, type_id,
			service_id, sla_id, user_id, responsible_user_id,
			customer_id, customer_user_id, ticket_state_id,
			ticket_priority_id, timeout, until_time, escalation_time,
			escalation_update_time, escalation_response_time,
			escalation_solution_time, archive_flag,
			create_time, create_by, change_time, change_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24
		) RETURNING id`

	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)

	// Use adapter for database-specific handling
	adapter := database.GetAdapter()
	ticketID, err := adapter.InsertWithReturning(
		r.db,
		query,
		ticket.TicketNumber,
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
	)
	if err != nil { return err }
	ticket.ID = int(ticketID)
	return nil
}

// isUniqueTNError detects a unique constraint violation on the ticket number.
func isUniqueTNError(err error) bool {
	if err == nil { return false }
	msg := err.Error()
	if strings.Contains(msg, "unique") && strings.Contains(msg, "tn") { return true }
	if errors.Is(err, sql.ErrNoRows) { return false }
	return false
}

// GetByID retrieves a ticket by its ID
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
		WHERE t.id = $1`

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

// GetByTN retrieves a ticket by its ticket number
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
		WHERE t.tn = $1`

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

// Update updates a ticket in the database
func (r *TicketRepository) Update(ticket *models.Ticket) error {
	ticket.ChangeTime = time.Now()

	query := `
		UPDATE ticket SET
			title = $2,
			queue_id = $3,
			ticket_lock_id = $4,
			type_id = $5,
			service_id = $6,
			sla_id = $7,
			user_id = $8,
			responsible_user_id = $9,
			customer_id = $10,
			customer_user_id = $11,
			ticket_state_id = $12,
			ticket_priority_id = $13,
			until_time = $14,
			escalation_time = $15,
			escalation_update_time = $16,
			escalation_response_time = $17,
			escalation_solution_time = $18,
			archive_flag = $19,
			change_time = $20,
			change_by = $21
		WHERE id = $1`

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

// Delete deletes a ticket from the database
func (r *TicketRepository) Delete(id uint) error {
	query := `DELETE FROM ticket WHERE id = $1`
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

// List retrieves a paginated list of tickets with filters
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
	baseQuery := `FROM ticket t WHERE 1=1`
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
	argCount := 1

	if req.QueueID != nil {
		filters = append(filters, fmt.Sprintf(" AND t.queue_id = $%d", argCount))
		args = append(args, *req.QueueID)
		argCount++
	}

	if req.StateID != nil {
		filters = append(filters, fmt.Sprintf(" AND t.ticket_state_id = $%d", argCount))
		args = append(args, *req.StateID)
		argCount++
	}

	if req.PriorityID != nil {
		filters = append(filters, fmt.Sprintf(" AND t.ticket_priority_id = $%d", argCount))
		args = append(args, *req.PriorityID)
		argCount++
	}

	if req.CustomerID != nil {
		filters = append(filters, fmt.Sprintf(" AND t.customer_id = $%d", argCount))
		args = append(args, *req.CustomerID)
		argCount++
	}

	if req.OwnerID != nil {
		filters = append(filters, fmt.Sprintf(" AND t.user_id = $%d", argCount))
		args = append(args, *req.OwnerID)
		argCount++
	}

	if req.ArchiveFlag != nil {
		filters = append(filters, fmt.Sprintf(" AND t.archive_flag = $%d", argCount))
		args = append(args, *req.ArchiveFlag)
		argCount++
	}

	if req.Search != "" {
		filters = append(filters, fmt.Sprintf(" AND (t.title ILIKE $%d OR t.tn ILIKE $%d)", argCount, argCount))
		args = append(args, "%"+req.Search+"%")
		argCount++
	}

	if req.StartDate != nil {
		filters = append(filters, fmt.Sprintf(" AND t.create_time >= $%d", argCount))
		args = append(args, *req.StartDate)
		argCount++
	}

	if req.EndDate != nil {
		filters = append(filters, fmt.Sprintf(" AND t.create_time <= $%d", argCount))
		args = append(args, *req.EndDate)
		argCount++
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

// GetTicketsByCustomer retrieves all tickets for a specific customer
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
		WHERE t.customer_id = $1`

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

	return tickets, nil
}

// GetTicketsByOwner retrieves all tickets assigned to a specific user
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
		WHERE (t.user_id = $1 OR t.responsible_user_id = $1)`

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

	return tickets, nil
}

// GetTicketWithRelations retrieves a ticket with all related data
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
		WHERE t.id = $1`

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

// LockTicket locks a ticket for a specific user
func (r *TicketRepository) LockTicket(ticketID uint, userID uint, lockType int) error {
	query := `
		UPDATE ticket 
		SET ticket_lock_id = $2, user_id = $3, change_time = $4, change_by = $5
		WHERE id = $1 AND ticket_lock_id = $6`

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

// UnlockTicket unlocks a ticket
func (r *TicketRepository) UnlockTicket(ticketID uint, userID uint) error {
	query := `
		UPDATE ticket 
		SET ticket_lock_id = $2, change_time = $3, change_by = $4
		WHERE id = $1`

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

// ArchiveTicket archives a ticket
func (r *TicketRepository) ArchiveTicket(ticketID uint, userID uint) error {
	query := `
		UPDATE ticket 
		SET archive_flag = 1, change_time = $2, change_by = $3
		WHERE id = $1`

	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)

	_, err := r.db.Exec(query, ticketID, time.Now(), userID)
	return err
}

// RestoreTicket restores an archived ticket
func (r *TicketRepository) RestoreTicket(ticketID uint, userID uint) error {
	query := `
		UPDATE ticket 
		SET archive_flag = 0, change_time = $2, change_by = $3
		WHERE id = $1`

	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)

	_, err := r.db.Exec(query, ticketID, time.Now(), userID)
	return err
}

// UpdateStatus updates the status of a ticket
func (r *TicketRepository) UpdateStatus(ticketID uint, stateID uint, userID uint) error {
	query := `
		UPDATE ticket 
		SET ticket_state_id = $2, change_time = $3, change_by = $4
		WHERE id = $1`

	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)

	_, err := r.db.Exec(query, ticketID, stateID, time.Now(), userID)
	return err
}

// UpdatePriority updates the priority of a ticket
func (r *TicketRepository) UpdatePriority(ticketID uint, priorityID uint, userID uint) error {
	query := `
		UPDATE ticket 
		SET ticket_priority_id = $2, change_time = $3, change_by = $4
		WHERE id = $1`

	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)

	_, err := r.db.Exec(query, ticketID, priorityID, time.Now(), userID)
	return err
}

// UpdateQueue transfers a ticket to a different queue
func (r *TicketRepository) UpdateQueue(ticketID uint, queueID uint, userID uint) error {
	query := `
		UPDATE ticket 
		SET queue_id = $2, change_time = $3, change_by = $4
		WHERE id = $1`

	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)

	_, err := r.db.Exec(query, ticketID, queueID, time.Now(), userID)
	return err
}


// GetQueues retrieves all active queues
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

	return queues, nil
}

// GetTicketStates retrieves all active ticket states
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

	return states, nil
}

// GetTicketPriorities retrieves all active ticket priorities
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

	return priorities, nil
}

// GetByTicketNumber retrieves a ticket by its ticket number
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
		WHERE tn = $1
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

// Count returns the total number of tickets
func (r *TicketRepository) Count() (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM ticket`
	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)
	err := r.db.QueryRow(query).Scan(&count)
	return count, err
}

// CountByStatus returns the number of tickets with a specific status
func (r *TicketRepository) CountByStatus(status string) (int, error) {
	var count int
	query := `
		SELECT COUNT(*) 
		FROM ticket t
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		WHERE ts.name = $1
	`
	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)
	err := r.db.QueryRow(query, status).Scan(&count)
	return count, err
}

// CountByStateID returns the number of tickets with a specific state ID
func (r *TicketRepository) CountByStateID(stateID int) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM ticket WHERE ticket_state_id = $1`
	// Convert placeholders for MySQL compatibility
	query = database.ConvertPlaceholders(query)
	err := r.db.QueryRow(query, stateID).Scan(&count)
	return count, err
}

// CountClosedToday returns the number of tickets closed today
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
