// Package escalation provides SLA escalation calculation and management.
package escalation

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// Service handles ticket escalation index calculation matching OTRS behavior.
type Service struct {
	db              *sql.DB
	calendarService *CalendarService
	logger          *log.Logger
}

// EscalationPreferences holds escalation settings from SLA or Queue.
type EscalationPreferences struct {
	FirstResponseTime   int    // Minutes until first response required
	FirstResponseNotify int    // Percentage for warning notification
	UpdateTime          int    // Minutes between updates required
	UpdateNotify        int    // Percentage for warning notification
	SolutionTime        int    // Minutes until solution required
	SolutionNotify      int    // Percentage for warning notification
	Calendar            string // Calendar name (empty = default, "1"-"9" = named)
}

// TicketInfo holds ticket data needed for escalation calculation.
type TicketInfo struct {
	ID         int
	QueueID    int
	SLAID      *int
	StateType  string // new, open, closed, removed, pending, merged
	Created    time.Time
	UserID     int
}

// NewService creates a new escalation service.
func NewService(db *sql.DB, logger *log.Logger) *Service {
	if logger == nil {
		logger = log.Default()
	}
	return &Service{
		db:              db,
		calendarService: NewCalendarService(db),
		logger:          logger,
	}
}

// Initialize loads calendars from sysconfig.
func (s *Service) Initialize(ctx context.Context) error {
	return s.calendarService.LoadCalendars(ctx)
}

// TicketEscalationIndexBuild rebuilds escalation index for a ticket.
// This matches OTRS Kernel::System::Ticket::TicketEscalationIndexBuild.
func (s *Service) TicketEscalationIndexBuild(ctx context.Context, ticketID, userID int) error {
	// Get ticket info
	ticket, err := s.getTicketInfo(ctx, ticketID)
	if err != nil {
		return fmt.Errorf("failed to get ticket info: %w", err)
	}
	if ticket == nil {
		return nil // Ticket was deleted
	}

	// Do no escalations on merge|close|remove tickets
	if ticket.StateType == "merged" || ticket.StateType == "closed" || ticket.StateType == "removed" {
		return s.clearEscalationTimes(ctx, ticketID, userID)
	}

	// Get escalation preferences from SLA or Queue
	prefs, err := s.getEscalationPreferences(ctx, ticket)
	if err != nil {
		return fmt.Errorf("failed to get escalation preferences: %w", err)
	}

	var escalationTime int64 = 0

	// Calculate first response escalation
	responseTime, err := s.calculateFirstResponseTime(ctx, ticket, prefs)
	if err != nil {
		return fmt.Errorf("failed to calculate first response time: %w", err)
	}
	if responseTime > 0 && (escalationTime == 0 || responseTime < escalationTime) {
		escalationTime = responseTime
	}

	// Calculate update escalation (skip if pending state)
	updateTime, err := s.calculateUpdateTime(ctx, ticket, prefs)
	if err != nil {
		return fmt.Errorf("failed to calculate update time: %w", err)
	}
	if updateTime > 0 && (escalationTime == 0 || updateTime < escalationTime) {
		escalationTime = updateTime
	}

	// Calculate solution escalation
	solutionTime, err := s.calculateSolutionTime(ctx, ticket, prefs)
	if err != nil {
		return fmt.Errorf("failed to calculate solution time: %w", err)
	}
	if solutionTime > 0 && (escalationTime == 0 || solutionTime < escalationTime) {
		escalationTime = solutionTime
	}

	// Update ticket escalation times
	return s.updateEscalationTimes(ctx, ticketID, userID, escalationTime, responseTime, updateTime, solutionTime)
}

// getTicketInfo retrieves ticket information needed for escalation.
func (s *Service) getTicketInfo(ctx context.Context, ticketID int) (*TicketInfo, error) {
	query := database.ConvertPlaceholders(`
		SELECT t.id, t.queue_id, t.sla_id, ts.type_id, t.create_time, COALESCE(t.user_id, 1)
		FROM ticket t
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		WHERE t.id = ?
	`)

	var ticket TicketInfo
	var stateTypeID int
	err := s.db.QueryRowContext(ctx, query, ticketID).Scan(
		&ticket.ID, &ticket.QueueID, &ticket.SLAID, &stateTypeID, &ticket.Created, &ticket.UserID,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Map state type ID to name
	ticket.StateType = s.stateTypeIDToName(stateTypeID)
	return &ticket, nil
}

// stateTypeIDToName converts state type ID to name.
func (s *Service) stateTypeIDToName(typeID int) string {
	switch typeID {
	case 1:
		return "new"
	case 2:
		return "open"
	case 3:
		return "closed"
	case 4:
		return "pending reminder"
	case 5:
		return "pending auto"
	case 6:
		return "removed"
	case 7:
		return "merged"
	default:
		return "open"
	}
}

// getEscalationPreferences gets escalation settings from SLA (preferred) or Queue.
func (s *Service) getEscalationPreferences(ctx context.Context, ticket *TicketInfo) (*EscalationPreferences, error) {
	prefs := &EscalationPreferences{}

	// Try SLA first (if ticket has SLA)
	if ticket.SLAID != nil && *ticket.SLAID > 0 {
		query := database.ConvertPlaceholders(`
			SELECT first_response_time, COALESCE(first_response_notify, 0),
			       update_time, COALESCE(update_notify, 0),
			       solution_time, COALESCE(solution_notify, 0),
			       COALESCE(calendar_name, '')
			FROM sla WHERE id = ?
		`)
		err := s.db.QueryRowContext(ctx, query, *ticket.SLAID).Scan(
			&prefs.FirstResponseTime, &prefs.FirstResponseNotify,
			&prefs.UpdateTime, &prefs.UpdateNotify,
			&prefs.SolutionTime, &prefs.SolutionNotify,
			&prefs.Calendar,
		)
		if err == nil {
			return prefs, nil
		}
		// Fall through to queue if SLA not found
	}

	// Fall back to Queue
	query := database.ConvertPlaceholders(`
		SELECT COALESCE(first_response_time, 0), COALESCE(first_response_notify, 0),
		       COALESCE(update_time, 0), COALESCE(update_notify, 0),
		       COALESCE(solution_time, 0), COALESCE(solution_notify, 0),
		       COALESCE(calendar_name, '')
		FROM queue WHERE id = ?
	`)
	err := s.db.QueryRowContext(ctx, query, ticket.QueueID).Scan(
		&prefs.FirstResponseTime, &prefs.FirstResponseNotify,
		&prefs.UpdateTime, &prefs.UpdateNotify,
		&prefs.SolutionTime, &prefs.SolutionNotify,
		&prefs.Calendar,
	)
	if err != nil {
		return nil, err
	}
	return prefs, nil
}

// calculateFirstResponseTime calculates first response escalation time.
// Returns 0 if no escalation needed (already responded or no SLA).
func (s *Service) calculateFirstResponseTime(ctx context.Context, ticket *TicketInfo, prefs *EscalationPreferences) (int64, error) {
	if prefs.FirstResponseTime <= 0 {
		return 0, nil
	}

	// Check if first response already done (agent article exists)
	hasResponse, err := s.hasAgentResponse(ctx, ticket.ID)
	if err != nil {
		return 0, err
	}
	if hasResponse {
		return 0, nil // Already responded
	}

	// Calculate destination time: ticket created + first_response_time (as working time)
	destTime := s.calendarService.AddWorkingTime(prefs.Calendar, ticket.Created, prefs.FirstResponseTime)
	return destTime.Unix(), nil
}

// calculateUpdateTime calculates update escalation time.
// Returns 0 if no escalation needed or ticket is in pending state.
func (s *Service) calculateUpdateTime(ctx context.Context, ticket *TicketInfo, prefs *EscalationPreferences) (int64, error) {
	if prefs.UpdateTime <= 0 {
		return 0, nil
	}

	// Don't escalate update time for pending states
	if ticket.StateType == "pending reminder" || ticket.StateType == "pending auto" {
		return 0, nil
	}

	// Get last sender time and type from article history
	lastSenderTime, lastSenderType, err := s.getLastSenderInfo(ctx, ticket.ID)
	if err != nil {
		return 0, err
	}

	// If last sender was agent, no update escalation needed
	if lastSenderType == "agent" {
		return 0, nil
	}

	// Calculate from last customer message or ticket creation
	baseTime := ticket.Created
	if lastSenderTime != nil {
		baseTime = *lastSenderTime
	}

	destTime := s.calendarService.AddWorkingTime(prefs.Calendar, baseTime, prefs.UpdateTime)
	return destTime.Unix(), nil
}

// calculateSolutionTime calculates solution escalation time.
// Returns 0 if no escalation needed (already solved or no SLA).
func (s *Service) calculateSolutionTime(ctx context.Context, ticket *TicketInfo, prefs *EscalationPreferences) (int64, error) {
	if prefs.SolutionTime <= 0 {
		return 0, nil
	}

	// Check if ticket has been closed
	isClosed, err := s.hasBeenClosed(ctx, ticket.ID)
	if err != nil {
		return 0, err
	}
	if isClosed {
		return 0, nil // Already solved
	}

	// Calculate destination time: ticket created + solution_time (as working time)
	destTime := s.calendarService.AddWorkingTime(prefs.Calendar, ticket.Created, prefs.SolutionTime)
	return destTime.Unix(), nil
}

// hasAgentResponse checks if there's an agent response article.
func (s *Service) hasAgentResponse(ctx context.Context, ticketID int) (bool, error) {
	query := database.ConvertPlaceholders(`
		SELECT 1 FROM article a
		JOIN article_sender_type ast ON a.article_sender_type_id = ast.id
		WHERE a.ticket_id = ? AND ast.name = 'agent' AND a.is_visible_for_customer = 1
		LIMIT 1
	`)
	var exists int
	err := s.db.QueryRowContext(ctx, query, ticketID).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// getLastSenderInfo gets the last article sender info for update escalation.
// Returns the time and type of the last relevant sender.
func (s *Service) getLastSenderInfo(ctx context.Context, ticketID int) (*time.Time, string, error) {
	// Get articles ordered by creation time descending, looking for customer/agent pattern
	query := database.ConvertPlaceholders(`
		SELECT a.create_time, ast.name, a.is_visible_for_customer
		FROM article a
		JOIN article_sender_type ast ON a.article_sender_type_id = ast.id
		WHERE a.ticket_id = ?
		ORDER BY a.create_time DESC
	`)

	rows, err := s.db.QueryContext(ctx, query, ticketID)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var lastCustomerTime *time.Time

	for rows.Next() {
		var createTime time.Time
		var senderType string
		var isVisible int
		if err := rows.Scan(&createTime, &senderType, &isVisible); err != nil {
			return nil, "", err
		}

		// Skip internal articles
		if isVisible == 0 {
			continue
		}

		// Only consider agent and customer
		if senderType != "agent" && senderType != "customer" {
			continue
		}

		// If we find an agent article, escalation starts from there
		if senderType == "agent" {
			return &createTime, "agent", nil
		}

		// Track customer time - we want the most recent customer after any agent
		if senderType == "customer" && lastCustomerTime == nil {
			t := createTime
			lastCustomerTime = &t
		}
	}

	if lastCustomerTime != nil {
		return lastCustomerTime, "customer", nil
	}

	return nil, "", nil
}

// hasBeenClosed checks if ticket has ever been closed (for solution time).
func (s *Service) hasBeenClosed(ctx context.Context, ticketID int) (bool, error) {
	// Check ticket history for close state
	query := database.ConvertPlaceholders(`
		SELECT 1 FROM ticket_history th
		JOIN ticket_history_type tht ON th.history_type_id = tht.id
		WHERE th.ticket_id = ? AND tht.name = 'StateUpdate'
		AND th.state_id IN (SELECT id FROM ticket_state WHERE type_id = 3)
		LIMIT 1
	`)
	var exists int
	err := s.db.QueryRowContext(ctx, query, ticketID).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// clearEscalationTimes sets all escalation times to 0 for closed/merged/removed tickets.
func (s *Service) clearEscalationTimes(ctx context.Context, ticketID, userID int) error {
	query := database.ConvertPlaceholders(`
		UPDATE ticket SET
			escalation_time = 0,
			escalation_response_time = 0,
			escalation_update_time = 0,
			escalation_solution_time = 0,
			change_time = NOW(),
			change_by = ?
		WHERE id = ?
	`)
	_, err := s.db.ExecContext(ctx, query, userID, ticketID)
	return err
}

// updateEscalationTimes updates ticket escalation times.
func (s *Service) updateEscalationTimes(ctx context.Context, ticketID, userID int, escalation, response, update, solution int64) error {
	query := database.ConvertPlaceholders(`
		UPDATE ticket SET
			escalation_time = ?,
			escalation_response_time = ?,
			escalation_update_time = ?,
			escalation_solution_time = ?,
			change_time = NOW(),
			change_by = ?
		WHERE id = ?
	`)
	_, err := s.db.ExecContext(ctx, query, escalation, response, update, solution, userID, ticketID)
	return err
}

// RebuildAllTicketEscalations rebuilds escalation index for all open tickets.
// This is useful for bulk updates after SLA/Queue changes.
func (s *Service) RebuildAllTicketEscalations(ctx context.Context, userID int) error {
	// Get all non-closed tickets
	query := database.ConvertPlaceholders(`
		SELECT t.id FROM ticket t
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		WHERE ts.type_id NOT IN (3, 6, 7)
	`)

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	var ticketIDs []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ticketIDs = append(ticketIDs, id)
	}

	s.logger.Printf("Rebuilding escalation index for %d tickets", len(ticketIDs))

	for _, ticketID := range ticketIDs {
		if err := s.TicketEscalationIndexBuild(ctx, ticketID, userID); err != nil {
			s.logger.Printf("Failed to rebuild escalation for ticket %d: %v", ticketID, err)
		}
	}

	return nil
}
