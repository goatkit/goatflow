package escalation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/sysconfig"
)

// DefaultDecayTime is OTRS's OTRSEscalationEvents::DecayTime default: an
// escalation event is raised again for the same ticket only after this long.
const DefaultDecayTime = 24 * time.Hour

// DecayTime returns the OTRS setting OTRSEscalationEvents::DecayTime
// (minutes; 0 repeats events on every check), DefaultDecayTime when unset.
func DecayTime(db *sql.DB) (time.Duration, error) {
	v, ok := sysconfig.Value(db, "OTRSEscalationEvents::DecayTime")
	if !ok {
		return DefaultDecayTime, nil
	}
	minutes, err := strconv.Atoi(scalarSetting(v))
	if err != nil || minutes < 0 {
		return 0, fmt.Errorf("OTRSEscalationEvents::DecayTime: invalid value %q", v)
	}
	return time.Duration(minutes) * time.Minute, nil
}

// State is a ticket's escalation state at one moment (OTRS
// TicketEscalationDateCalculation) for one kind.
type State struct {
	Kind        string
	Destination int64 // unix seconds
	Escalated   bool  // the destination time has passed
	Notify      bool  // the notify-before percentage of the working time is used up
	WorkingTime int64 // working seconds left (negative: overdue)
}

// escalationStates returns the escalation states of t at now, using the
// calendar of its escalation preferences. Finished tickets and tickets
// without escalation settings have none.
func (s *Service) escalationStates(ctx context.Context, t *ticket, now time.Time) ([]State, error) {
	if finished(t.stateType) {
		return nil, nil
	}
	prefs, err := s.preferences(ctx, t)
	if err != nil || !prefs.any() {
		return nil, err
	}
	cal, err := s.calendar(ctx, prefs.Calendar)
	if err != nil {
		return nil, err
	}
	var states []State
	for _, kind := range Kinds {
		dest := t.index.Of(kind)
		if dest == 0 {
			continue
		}
		st := State{Kind: kind, Destination: dest}
		destTime := time.Unix(dest, 0)
		if dest-now.Unix() > 0 {
			if st.WorkingTime, err = cal.WorkingTime(now, destTime); err != nil {
				return nil, err
			}
			minutes, notify := prefs.timeOf(kind)
			if notify > 0 && minutes > 0 {
				reached := 100 - float64(st.WorkingTime)/(float64(minutes)*60/100)
				st.Notify = reached >= float64(notify)
			}
		} else {
			overdue, err := cal.WorkingTime(destTime, now)
			if err != nil {
				return nil, err
			}
			st.WorkingTime = -overdue
			st.Escalated = true
		}
		states = append(states, st)
	}
	return states, nil
}

// Event is one escalation event raised by CheckEscalations.
type Event struct {
	TicketID int
	Name     string // ticket_history_type name, e.g. EscalationResponseTimeStart
}

// CheckEscalations is OTRS Maint::Ticket::EscalationCheck: for up to 1000
// tickets escalating within five days it raises Escalation*TimeStart for
// every started escalation and Escalation*TimeNotifyBefore for every one past
// its notify percentage, by writing the ticket_history row of that type
// (webhooks and notification events read those). Tickets whose calendar had
// no working time in the last ten minutes are skipped, and an event is not
// repeated for a ticket within decay (0 = every run).
func (s *Service) CheckEscalations(ctx context.Context, decay time.Duration) ([]Event, error) {
	now := s.now()
	rows, err := s.db.QueryContext(ctx, database.ConvertPlaceholders(`
		SELECT id FROM ticket
		WHERE escalation_time <> 0 AND escalation_time <= ?
		ORDER BY escalation_time, id
		LIMIT 1000`), now.Add(5*24*time.Hour).Unix())
	if err != nil {
		return nil, fmt.Errorf("find escalating tickets: %w", err)
	}
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	var events []Event
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return events, err
		}
		ev, err := s.checkTicket(ctx, id, now, decay)
		if err != nil {
			s.logger.Printf("escalation: check ticket %d: %v", id, err)
			continue
		}
		events = append(events, ev...)
	}
	return events, nil
}

func (s *Service) checkTicket(ctx context.Context, ticketID int, now time.Time, decay time.Duration) ([]Event, error) {
	t, err := s.loadTicket(ctx, s.db, ticketID)
	if err != nil || t == nil {
		return nil, err
	}

	// OTRS TicketCalendarGet: the SLA's calendar, else the queue's.
	var calName string
	err = s.db.QueryRowContext(ctx, database.ConvertPlaceholders(`
		SELECT COALESCE(NULLIF(sla.calendar_name, ''), q.calendar_name, '')
		FROM ticket t
		JOIN queue q ON q.id = t.queue_id
		LEFT JOIN sla ON sla.id = t.sla_id
		WHERE t.id = ?`), ticketID).Scan(&calName)
	if err != nil {
		return nil, fmt.Errorf("calendar: %w", err)
	}
	cal, err := s.calendar(ctx, calName)
	if err != nil {
		return nil, err
	}
	counted, err := cal.WorkingTime(now.Add(-10*time.Minute), now)
	if err != nil {
		return nil, err
	}
	if counted == 0 {
		return nil, nil // outside business hours
	}

	states, err := s.escalationStates(ctx, t, now)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, st := range states {
		if st.Escalated {
			names = append(names, startEvent[st.Kind])
		}
	}
	for _, st := range states {
		if st.Notify {
			names = append(names, notifyBeforeEvent[st.Kind])
		}
	}

	var events []Event
	for _, name := range names {
		if decay > 0 {
			recent, err := s.triggeredSince(ctx, ticketID, name, now.Add(-decay))
			if err != nil {
				return events, err
			}
			if recent {
				continue
			}
		}
		if err := s.addHistory(ctx, nil, t, name, 1); err != nil {
			return events, err
		}
		events = append(events, Event{TicketID: ticketID, Name: name})
	}
	return events, nil
}

// triggeredSince reports whether the ticket has a history row of the event
// type created at or after since.
func (s *Service) triggeredSince(ctx context.Context, ticketID int, event string, since time.Time) (bool, error) {
	var last sql.NullTime
	err := s.db.QueryRowContext(ctx, database.ConvertPlaceholders(`
		SELECT MAX(th.create_time) FROM ticket_history th
		JOIN ticket_history_type tht ON tht.id = th.history_type_id
		WHERE th.ticket_id = ? AND tht.name = ?`), ticketID, event).Scan(&last)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("last %s: %w", event, err)
	}
	return last.Valid && !last.Time.Before(since), nil
}
