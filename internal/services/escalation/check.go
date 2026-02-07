package escalation

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/goatkit/goatflow/internal/database"
)

// CheckService handles escalation event triggering, matching OTRS EscalationCheck.
type CheckService struct {
	db              *sql.DB
	calendarService *CalendarService
	logger          *log.Logger
	decayTime       int // Minutes between repeat notifications (0 = no decay)
}

// EscalationEvent represents an escalation event to trigger.
type EscalationEvent struct {
	TicketID  int
	EventName string
}

// NewCheckService creates a new escalation check service.
func NewCheckService(db *sql.DB, calendarService *CalendarService, logger *log.Logger) *CheckService {
	if logger == nil {
		logger = log.Default()
	}
	return &CheckService{
		db:              db,
		calendarService: calendarService,
		logger:          logger,
		decayTime:       0, // Can be configured via sysconfig
	}
}

// SetDecayTime sets the decay time (minutes between repeat notifications).
func (s *CheckService) SetDecayTime(minutes int) {
	s.decayTime = minutes
}

// CheckEscalations finds tickets that are escalating and triggers events.
// This matches OTRS Maint::Ticket::EscalationCheck.
func (s *CheckService) CheckEscalations(ctx context.Context) ([]EscalationEvent, error) {
	// Find tickets escalating within next 5 days
	fiveDaysMinutes := 5 * 24 * 60
	tickets, err := s.findEscalatingTickets(ctx, fiveDaysMinutes)
	if err != nil {
		return nil, fmt.Errorf("failed to find escalating tickets: %w", err)
	}

	var events []EscalationEvent

	for _, ticket := range tickets {
		// Check if we're in business hours for this ticket's calendar
		calendar := s.getTicketCalendar(ctx, ticket.ID)
		if !s.isInBusinessHours(calendar) {
			continue
		}

		// Get escalation info for this ticket
		escalationInfo, err := s.getTicketEscalationInfo(ctx, ticket.ID)
		if err != nil {
			s.logger.Printf("Error getting escalation info for ticket %d: %v", ticket.ID, err)
			continue
		}

		// Check each escalation type
		ticketEvents := s.checkTicketEscalations(ctx, ticket.ID, escalationInfo)
		events = append(events, ticketEvents...)
	}

	return events, nil
}

// EscalatingTicket holds basic ticket info for escalation check.
type EscalatingTicket struct {
	ID           int
	TicketNumber string
}

// findEscalatingTickets finds tickets that will escalate within the given minutes.
func (s *CheckService) findEscalatingTickets(ctx context.Context, withinMinutes int) ([]EscalatingTicket, error) {
	// Calculate cutoff time (now + withinMinutes)
	cutoff := time.Now().Add(time.Duration(withinMinutes) * time.Minute).Unix()

	query := database.ConvertPlaceholders(`
		SELECT id, tn FROM ticket
		WHERE escalation_time > 0 AND escalation_time < ?
		ORDER BY escalation_time ASC
		LIMIT 1000
	`)

	rows, err := s.db.QueryContext(ctx, query, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tickets []EscalatingTicket
	for rows.Next() {
		var t EscalatingTicket
		if err := rows.Scan(&t.ID, &t.TicketNumber); err != nil {
			return nil, err
		}
		tickets = append(tickets, t)
	}

	return tickets, nil
}

// getTicketCalendar gets the calendar name for a ticket (from SLA or Queue).
func (s *CheckService) getTicketCalendar(ctx context.Context, ticketID int) string {
	// Try SLA first
	query := database.ConvertPlaceholders(`
		SELECT COALESCE(sla.calendar_name, '')
		FROM ticket t
		LEFT JOIN sla ON t.sla_id = sla.id
		WHERE t.id = ?
	`)
	var calendar string
	if err := s.db.QueryRowContext(ctx, query, ticketID).Scan(&calendar); err == nil && calendar != "" {
		return calendar
	}

	// Fall back to queue
	query = database.ConvertPlaceholders(`
		SELECT COALESCE(q.calendar_name, '')
		FROM ticket t
		JOIN queue q ON t.queue_id = q.id
		WHERE t.id = ?
	`)
	if err := s.db.QueryRowContext(ctx, query, ticketID).Scan(&calendar); err == nil {
		return calendar
	}

	return "" // Default calendar
}

// isInBusinessHours checks if current time is within business hours.
func (s *CheckService) isInBusinessHours(calendarName string) bool {
	if s.calendarService == nil {
		return true // No calendar service, assume always working
	}
	return s.calendarService.IsWorkingTime(calendarName, time.Now())
}

// EscalationInfo holds ticket escalation state.
type EscalationInfo struct {
	FirstResponseTimeEscalation   bool
	FirstResponseTimeNotification bool
	UpdateTimeEscalation          bool
	UpdateTimeNotification        bool
	SolutionTimeEscalation        bool
	SolutionTimeNotification      bool
}

// getTicketEscalationInfo calculates current escalation state for a ticket.
func (s *CheckService) getTicketEscalationInfo(ctx context.Context, ticketID int) (*EscalationInfo, error) {
	// Get escalation times and preferences
	query := database.ConvertPlaceholders(`
		SELECT t.escalation_response_time, t.escalation_update_time, t.escalation_solution_time,
		       COALESCE(sla.first_response_notify, q.first_response_notify, 0),
		       COALESCE(sla.update_notify, q.update_notify, 0),
		       COALESCE(sla.solution_notify, q.solution_notify, 0)
		FROM ticket t
		LEFT JOIN sla ON t.sla_id = sla.id
		JOIN queue q ON t.queue_id = q.id
		WHERE t.id = ?
	`)

	var responseTime, updateTime, solutionTime int64
	var responseNotify, updateNotify, solutionNotify int
	err := s.db.QueryRowContext(ctx, query, ticketID).Scan(
		&responseTime, &updateTime, &solutionTime,
		&responseNotify, &updateNotify, &solutionNotify,
	)
	if err != nil {
		return nil, err
	}

	now := time.Now().Unix()
	info := &EscalationInfo{}

	// Check first response escalation
	if responseTime > 0 {
		if now >= responseTime {
			info.FirstResponseTimeEscalation = true
		} else if responseNotify > 0 {
			// Check if we're past the notify threshold
			info.FirstResponseTimeNotification = s.isPastNotifyThreshold(responseTime, responseNotify)
		}
	}

	// Check update escalation
	if updateTime > 0 {
		if now >= updateTime {
			info.UpdateTimeEscalation = true
		} else if updateNotify > 0 {
			info.UpdateTimeNotification = s.isPastNotifyThreshold(updateTime, updateNotify)
		}
	}

	// Check solution escalation
	if solutionTime > 0 {
		if now >= solutionTime {
			info.SolutionTimeEscalation = true
		} else if solutionNotify > 0 {
			info.SolutionTimeNotification = s.isPastNotifyThreshold(solutionTime, solutionNotify)
		}
	}

	return info, nil
}

// isPastNotifyThreshold checks if we're past the notification threshold percentage.
func (s *CheckService) isPastNotifyThreshold(destTime int64, notifyPercent int) bool {
	now := time.Now().Unix()

	// If already escalated, no notification needed
	if now >= destTime {
		return false
	}

	// Calculate time elapsed vs total time
	// We don't have the start time here, so we check if remaining time is less than (100-notify)%
	// This is a simplification - OTRS calculates from ticket creation
	remaining := destTime - now

	// If less than (100-notifyPercent)% of time remaining, notify
	// E.g., if notify=80, we notify when 20% or less time remains
	thresholdPercent := 100 - notifyPercent

	// We estimate total time as 2x remaining (rough estimate when 50% done)
	// Better implementation would store/calculate actual total time
	return remaining > 0 && float64(remaining)/float64(destTime-now+remaining)*100 <= float64(thresholdPercent)
}

// checkTicketEscalations checks escalation conditions and returns events to trigger.
func (s *CheckService) checkTicketEscalations(ctx context.Context, ticketID int, info *EscalationInfo) []EscalationEvent {
	var events []EscalationEvent

	// Map escalation types to event names
	type escalationType struct {
		condition bool
		eventName string
	}

	escalations := []escalationType{
		{info.FirstResponseTimeEscalation, "EscalationResponseTimeStart"},
		{info.UpdateTimeEscalation, "EscalationUpdateTimeStart"},
		{info.SolutionTimeEscalation, "EscalationSolutionTimeStart"},
		{info.FirstResponseTimeNotification, "EscalationResponseTimeNotifyBefore"},
		{info.UpdateTimeNotification, "EscalationUpdateTimeNotifyBefore"},
		{info.SolutionTimeNotification, "EscalationSolutionTimeNotifyBefore"},
	}

	for _, esc := range escalations {
		if !esc.condition {
			continue
		}

		// Check decay time - don't repeat events too frequently
		if s.decayTime > 0 && s.wasEventTriggeredRecently(ctx, ticketID, esc.eventName) {
			continue
		}

		events = append(events, EscalationEvent{
			TicketID:  ticketID,
			EventName: esc.eventName,
		})

		// Record event in history
		s.recordEscalationEvent(ctx, ticketID, esc.eventName)
	}

	// Add notification meta-events (for notification system)
	hasEscalation := info.FirstResponseTimeEscalation || info.UpdateTimeEscalation || info.SolutionTimeEscalation
	hasNotification := info.FirstResponseTimeNotification || info.UpdateTimeNotification || info.SolutionTimeNotification

	if hasEscalation {
		events = append(events, EscalationEvent{
			TicketID:  ticketID,
			EventName: "NotificationEscalation",
		})
	} else if hasNotification {
		events = append(events, EscalationEvent{
			TicketID:  ticketID,
			EventName: "NotificationEscalationNotifyBefore",
		})
	}

	return events
}

// wasEventTriggeredRecently checks if event was triggered within decay time.
func (s *CheckService) wasEventTriggeredRecently(ctx context.Context, ticketID int, eventName string) bool {
	if s.decayTime <= 0 {
		return false
	}

	cutoff := time.Now().Add(-time.Duration(s.decayTime) * time.Minute)

	query := database.ConvertPlaceholders(`
		SELECT 1 FROM ticket_history th
		JOIN ticket_history_type tht ON th.history_type_id = tht.id
		WHERE th.ticket_id = ? AND tht.name = ? AND th.create_time > ?
		LIMIT 1
	`)

	var exists int
	err := s.db.QueryRowContext(ctx, query, ticketID, eventName, cutoff).Scan(&exists)
	return err == nil
}

// recordEscalationEvent records an escalation event in ticket history.
func (s *CheckService) recordEscalationEvent(ctx context.Context, ticketID int, eventName string) {
	// Get history type ID
	query := database.ConvertPlaceholders(`
		SELECT id FROM ticket_history_type WHERE name = ?
	`)
	var historyTypeID int
	if err := s.db.QueryRowContext(ctx, query, eventName).Scan(&historyTypeID); err != nil {
		s.logger.Printf("History type not found: %s", eventName)
		return
	}

	// Get ticket's current state for required history fields
	query = database.ConvertPlaceholders(`
		SELECT type_id, queue_id, user_id, ticket_priority_id, ticket_state_id
		FROM ticket WHERE id = ?
	`)
	var typeID, queueID, ownerID, priorityID, stateID int
	if err := s.db.QueryRowContext(ctx, query, ticketID).Scan(&typeID, &queueID, &ownerID, &priorityID, &stateID); err != nil {
		s.logger.Printf("Failed to get ticket %d for history: %v", ticketID, err)
		return
	}

	// Insert history record with all required fields
	query = database.ConvertPlaceholders(`
		INSERT INTO ticket_history (ticket_id, article_id, history_type_id, name, type_id, queue_id, owner_id, priority_id, state_id, create_time, create_by, change_time, change_by)
		VALUES (?, NULL, ?, ?, ?, ?, ?, ?, ?, NOW(), 1, NOW(), 1)
	`)
	historyName := fmt.Sprintf("%%%%%s%%%%triggered", eventName)
	if _, err := s.db.ExecContext(ctx, query, ticketID, historyTypeID, historyName, typeID, queueID, ownerID, priorityID, stateID); err != nil {
		s.logger.Printf("Failed to record history for ticket %d: %v", ticketID, err)
	}
}
