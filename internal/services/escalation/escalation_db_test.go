package escalation

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	if os.Getenv("TEST_DB_HOST") == "" {
		t.Skip("escalation database tests need a test database (TEST_DB_HOST)")
	}
	require.NoError(t, database.InitTestDB())
	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)
	return db
}

var fixtureSeq atomic.Int64

func uniq(prefix string) string {
	return fmt.Sprintf("%s%d-%d", prefix, time.Now().UnixNano(), fixtureSeq.Add(1))
}

func exec(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	_, err := db.Exec(database.ConvertPlaceholders(q), args...)
	require.NoError(t, err, q)
}

func insertID(t *testing.T, db *sql.DB, q string, args ...any) int {
	t.Helper()
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(q+" RETURNING id"), args...)
	require.NoError(t, err, q)
	return int(id)
}

// setting defines a sysconfig setting for the test's duration.
func setting(t *testing.T, db *sql.DB, name, value string) {
	t.Helper()
	now := time.Now().UTC()
	exec(t, db, `DELETE FROM sysconfig_modified WHERE name = ?`, name)
	exec(t, db, `DELETE FROM sysconfig_default WHERE name = ?`, name)
	exec(t, db, `
		INSERT INTO sysconfig_default (
			name, description, navigation, is_invisible, is_readonly, is_required,
			is_valid, has_configlevel, user_modification_possible, user_modification_active,
			xml_content_raw, xml_content_parsed, xml_filename, effective_value,
			is_dirty, exclusive_lock_guid, create_time, create_by, change_time, change_by
		) VALUES (?, 'escalation test', 'Core::Time', 0, 0, 0,
			1, 0, 0, 0, '', '', 'Test.xml', ?, 0, '', ?, 1, ?, 1)`, name, value, now, now)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM sysconfig_default WHERE name = ?`), name)
	})
}

// Calendar 8: office hours in UTC. Calendar 9: Saturday 10:00-12:00 in Berlin.
func defineTestCalendars(t *testing.T, db *sql.DB) {
	t.Helper()
	setting(t, db, "TimeWorkingHours::Calendar8", officeHours)
	setting(t, db, "TimeVacationDays::Calendar8", "---\n'12':\n  '25': Christmas\n")
	setting(t, db, "TimeWorkingHours::Calendar9", "---\nSat:\n- '10'\n- '11'\n")
	setting(t, db, "TimeZone::Calendar9", "--- Europe/Berlin\n")
}

type fixture struct {
	db      *sql.DB
	queueID int
	stateID map[string]int // by state type name
}

// newFixture creates a queue whose escalation settings are given in minutes,
// using calendar 8.
func newFixture(t *testing.T, db *sql.DB, firstResponse, firstResponseNotify, update, solution int) *fixture {
	t.Helper()
	defineTestCalendars(t, db)
	var groupID, sysAddr, salutation, signature, followUp int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`
		SELECT group_id, system_address_id, salutation_id, signature_id, follow_up_id FROM queue WHERE id = 1`)).
		Scan(&groupID, &sysAddr, &salutation, &signature, &followUp))
	now := time.Now().UTC()
	f := &fixture{db: db, stateID: map[string]int{}}
	f.queueID = insertID(t, db, `
		INSERT INTO queue (name, group_id, first_response_time, first_response_notify, update_time, update_notify,
			solution_time, solution_notify, system_address_id, calendar_name, salutation_id, signature_id,
			follow_up_id, follow_up_lock, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, ?, 0, ?, 0, ?, '8', ?, ?, ?, 0, 1, ?, 1, ?, 1)`,
		uniq("EscQ"), groupID, firstResponse, firstResponseNotify, update, solution, sysAddr, salutation, signature,
		followUp, now, now)
	t.Cleanup(func() {
		for _, q := range []string{
			`DELETE FROM ticket_history WHERE ticket_id IN (SELECT id FROM ticket WHERE queue_id = ?)`,
			`DELETE FROM article WHERE ticket_id IN (SELECT id FROM ticket WHERE queue_id = ?)`,
			`DELETE FROM ticket WHERE queue_id = ?`,
			`DELETE FROM queue WHERE id = ?`,
		} {
			_, _ = db.Exec(database.ConvertPlaceholders(q), f.queueID)
		}
	})
	for _, st := range []string{"open", "pending reminder", "closed"} {
		var id int
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`
			SELECT s.id FROM ticket_state s JOIN ticket_state_type st ON st.id = s.type_id
			WHERE st.name = ? ORDER BY s.id`), st).Scan(&id))
		f.stateID[st] = id
	}
	return f
}

func (f *fixture) sla(t *testing.T, firstResponse, update, solution int, calendar string) int {
	t.Helper()
	now := time.Now().UTC()
	id := insertID(t, f.db, `
		INSERT INTO sla (name, calendar_name, first_response_time, first_response_notify, update_time, update_notify,
			solution_time, solution_notify, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, 0, ?, 0, ?, 0, 1, ?, 1, ?, 1)`, uniq("EscSLA"), calendar, firstResponse, update, solution, now, now)
	t.Cleanup(func() {
		_, _ = f.db.Exec(database.ConvertPlaceholders(`UPDATE ticket SET sla_id = NULL WHERE sla_id = ?`), id)
		_, _ = f.db.Exec(database.ConvertPlaceholders(`DELETE FROM sla WHERE id = ?`), id)
	})
	return id
}

func (f *fixture) ticket(t *testing.T, created time.Time, slaID any) int {
	t.Helper()
	return insertID(t, f.db, `
		INSERT INTO ticket (tn, title, queue_id, ticket_lock_id, type_id, sla_id, user_id, responsible_user_id,
			ticket_priority_id, ticket_state_id, timeout, until_time, escalation_time, escalation_update_time,
			escalation_response_time, escalation_solution_time, archive_flag, create_time, create_by, change_time, change_by)
		VALUES (?, 'escalation test', ?, 1, 1, ?, 1, 1, 3, ?, 0, 0, 0, 0, 0, 0, 0, ?, 1, ?, 1)`,
		uniq("ESC"), f.queueID, slaID, f.stateID["open"], created.UTC(), created.UTC())
}

// article adds an article from sender (agent, system, customer) at the given time.
func (f *fixture) article(t *testing.T, ticketID int, sender string, visible bool, at time.Time, by int) {
	t.Helper()
	vis := 0
	if visible {
		vis = 1
	}
	insertID(t, f.db, `
		INSERT INTO article (ticket_id, article_sender_type_id, communication_channel_id, is_visible_for_customer,
			create_time, create_by, change_time, change_by)
		SELECT ?, id, 1, ?, ?, ?, ?, ? FROM article_sender_type WHERE name = ?`,
		ticketID, vis, at.UTC(), by, at.UTC(), by, sender)
}

// setState moves the ticket to a state of the given type and records the
// StateUpdate history row, as the ticket handlers do.
func (f *fixture) setState(t *testing.T, ticketID int, stateType string, at time.Time) {
	t.Helper()
	exec(t, f.db, `UPDATE ticket SET ticket_state_id = ? WHERE id = ?`, f.stateID[stateType], ticketID)
	exec(t, f.db, `
		INSERT INTO ticket_history (name, history_type_id, ticket_id, article_id, type_id, queue_id, owner_id,
			priority_id, state_id, create_time, create_by, change_time, change_by)
		SELECT '%%StateUpdate', id, ?, NULL, 1, ?, 1, 3, ?, ?, 1, ?, 1 FROM ticket_history_type WHERE name = 'StateUpdate'`,
		ticketID, f.queueID, f.stateID[stateType], at.UTC(), at.UTC())
}

func indexOf(t *testing.T, db *sql.DB, ticketID int) Index {
	t.Helper()
	var ix Index
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`
		SELECT escalation_time, escalation_response_time, escalation_update_time, escalation_solution_time
		FROM ticket WHERE id = ?`), ticketID).Scan(&ix.Escalation, &ix.Response, &ix.Update, &ix.Solution))
	return ix
}

// events returns the ticket's escalation history rows as "Type by user".
func events(t *testing.T, db *sql.DB, ticketID int) []string {
	t.Helper()
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT tht.name, th.name, th.create_by FROM ticket_history th
		JOIN ticket_history_type tht ON tht.id = th.history_type_id
		WHERE th.ticket_id = ? AND tht.name LIKE 'Escalation%' ORDER BY th.id`), ticketID)
	require.NoError(t, err)
	defer rows.Close()
	var out []string
	for rows.Next() {
		var typ, name string
		var by int
		require.NoError(t, rows.Scan(&typ, &name, &by))
		require.Equal(t, "%%"+typ+"%%triggered", name)
		out = append(out, fmt.Sprintf("%s by %d", typ, by))
	}
	require.NoError(t, rows.Err())
	return out
}

// otherAgent returns an existing user other than admin (id 1).
func otherAgent(t *testing.T, db *sql.DB) int {
	t.Helper()
	var id int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		`SELECT id FROM users WHERE id <> 1 ORDER BY id`)).Scan(&id))
	return id
}

func clockAt(at *time.Time) Option { return WithClock(func() time.Time { return *at }) }

func unix(y int, m time.Month, d, h, min int) int64 { return utc(y, m, d, h, min).Unix() }

func TestIndexBuild_QueueSettingsAndWorkingHours(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	// First response 3h, update 4h, solution 10h, in calendar 8 (Mon-Fri 08-18 UTC).
	f := newFixture(t, db, 180, 0, 240, 600)
	now := utc(2026, 1, 9, 16, 0)
	svc := NewService(db, clockAt(&now))

	// Created Friday 16:00: two working hours are left on Friday.
	id := f.ticket(t, utc(2026, 1, 9, 16, 0), nil)
	require.NoError(t, svc.TicketEscalationIndexBuild(ctx, id, 1))
	assert.Equal(t, Index{
		Escalation: unix(2026, 1, 12, 9, 0),
		Response:   unix(2026, 1, 12, 9, 0),  // Fri 16-18 + Mon 08-09
		Update:     0,                        // no article yet
		Solution:   unix(2026, 1, 12, 16, 0), // Fri 2h + Mon 8h
	}, indexOf(t, db, id))

	// The customer's message starts the update clock; an internal agent note
	// is no response.
	f.article(t, id, "customer", true, utc(2026, 1, 9, 16, 0), 1)
	f.article(t, id, "agent", false, utc(2026, 1, 9, 16, 30), 1)
	require.NoError(t, svc.TicketEscalationIndexBuild(ctx, id, 1))
	ix := indexOf(t, db, id)
	assert.Equal(t, unix(2026, 1, 12, 9, 0), ix.Response)
	assert.Equal(t, unix(2026, 1, 12, 10, 0), ix.Update, "Fri 16:00 + 4 working hours")
	assert.Equal(t, unix(2026, 1, 12, 9, 0), ix.Escalation)

	// The agent's visible reply ends the first response escalation and
	// restarts the update clock.
	f.article(t, id, "agent", true, utc(2026, 1, 12, 9, 30), 1)
	require.NoError(t, svc.TicketEscalationIndexBuild(ctx, id, 1))
	ix = indexOf(t, db, id)
	assert.Zero(t, ix.Response)
	assert.Equal(t, unix(2026, 1, 12, 13, 30), ix.Update)
	assert.Equal(t, unix(2026, 1, 12, 13, 30), ix.Escalation)

	// Several customer messages in a row: the clock runs from the first one.
	f.article(t, id, "customer", true, utc(2026, 1, 12, 10, 0), 1)
	f.article(t, id, "customer", true, utc(2026, 1, 12, 11, 0), 1)
	f.article(t, id, "system", true, utc(2026, 1, 12, 11, 5), 1)
	require.NoError(t, svc.TicketEscalationIndexBuild(ctx, id, 1))
	assert.Equal(t, unix(2026, 1, 12, 14, 0), indexOf(t, db, id).Update)

	// Pending states stop the update escalation only.
	f.setState(t, id, "pending reminder", utc(2026, 1, 12, 11, 10))
	require.NoError(t, svc.TicketEscalationIndexBuild(ctx, id, 1))
	assert.Equal(t, Index{Escalation: unix(2026, 1, 12, 16, 0), Solution: unix(2026, 1, 12, 16, 0)}, indexOf(t, db, id))

	// Closing clears the index; reopening keeps the solution done.
	f.setState(t, id, "closed", utc(2026, 1, 12, 11, 20))
	require.NoError(t, svc.TicketEscalationIndexBuild(ctx, id, 1))
	assert.Equal(t, Index{}, indexOf(t, db, id))
	f.setState(t, id, "open", utc(2026, 1, 12, 11, 30))
	require.NoError(t, svc.TicketEscalationIndexBuild(ctx, id, 1))
	assert.Equal(t, Index{Escalation: unix(2026, 1, 12, 14, 0), Update: unix(2026, 1, 12, 14, 0)}, indexOf(t, db, id))
}

// A ticket's SLA replaces its queue's settings, including the calendar (here
// calendar 9: Saturdays 10:00-12:00 Berlin time).
func TestIndexBuild_SLAWithOwnCalendarAndTimeZone(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	f := newFixture(t, db, 120, 0, 240, 600)
	slaID := f.sla(t, 90, 0, 0, "9")
	now := utc(2026, 1, 9, 16, 0)
	svc := NewService(db, clockAt(&now))

	// Fri 16:00 UTC: next working time is Sat 10:00 CET = 09:00 UTC; 2h on
	// Saturday, so 90 minutes end Sat 10:30 UTC.
	id := f.ticket(t, utc(2026, 1, 9, 16, 0), slaID)
	require.NoError(t, svc.TicketEscalationIndexBuild(ctx, id, 1))
	assert.Equal(t, Index{Escalation: unix(2026, 1, 10, 10, 30), Response: unix(2026, 1, 10, 10, 30)}, indexOf(t, db, id),
		"SLA has no update or solution time, so the queue's do not apply")
}

func TestIndexBuild_StopEventWhenAChangeEndsAStartedEscalation(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	f := newFixture(t, db, 60, 0, 0, 0)
	now := utc(2026, 1, 12, 8, 0)
	svc := NewService(db, clockAt(&now))
	id := f.ticket(t, utc(2026, 1, 12, 8, 0), nil)
	require.NoError(t, svc.TicketEscalationIndexBuild(ctx, id, 1))
	require.Equal(t, unix(2026, 1, 12, 9, 0), indexOf(t, db, id).Response)

	// Answered before the destination: nothing had started, no stop event.
	other := f.ticket(t, utc(2026, 1, 12, 8, 0), nil)
	require.NoError(t, svc.TicketEscalationIndexBuild(ctx, other, 1))
	agent := otherAgent(t, db)
	f.article(t, other, "agent", true, utc(2026, 1, 12, 8, 30), agent)
	now = utc(2026, 1, 12, 8, 30)
	require.NoError(t, svc.TicketEscalationIndexBuild(ctx, other, agent))
	assert.Zero(t, indexOf(t, db, other).Response)
	assert.Empty(t, events(t, db, other))

	// Answered after the destination: the response escalation stops.
	now = utc(2026, 1, 12, 10, 0)
	f.article(t, id, "agent", true, now, agent)
	require.NoError(t, svc.TicketEscalationIndexBuild(ctx, id, agent))
	assert.Equal(t, Index{}, indexOf(t, db, id))
	stopped := []string{fmt.Sprintf("EscalationResponseTimeStop by %d", agent)}
	assert.Equal(t, stopped, events(t, db, id))

	// Rebuilding again changes nothing and raises nothing.
	require.NoError(t, svc.TicketEscalationIndexBuild(ctx, id, agent))
	assert.Equal(t, stopped, events(t, db, id))
}

func TestCheckEscalations_StartNotifyDecayAndBusinessHours(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	// First response 4h, notify at 50%.
	f := newFixture(t, db, 240, 50, 0, 0)
	now := utc(2026, 1, 12, 8, 0)
	svc := NewService(db, clockAt(&now))
	id := f.ticket(t, utc(2026, 1, 12, 8, 0), nil)
	require.NoError(t, svc.TicketEscalationIndexBuild(ctx, id, 1))
	require.Equal(t, unix(2026, 1, 12, 12, 0), indexOf(t, db, id).Response)

	mine := func(evs []Event) []string {
		var out []string
		for _, e := range evs {
			if e.TicketID == id {
				out = append(out, e.Name)
			}
		}
		return out
	}
	check := func(decay time.Duration) []string {
		t.Helper()
		evs, err := svc.CheckEscalations(ctx, decay)
		require.NoError(t, err)
		return mine(evs)
	}

	now = utc(2026, 1, 12, 9, 30) // 37.5% used
	assert.Empty(t, check(DefaultDecayTime))

	now = utc(2026, 1, 12, 10, 30) // 62.5% used: notify before
	assert.Equal(t, []string{"EscalationResponseTimeNotifyBefore"}, check(DefaultDecayTime))
	now = utc(2026, 1, 12, 11, 0)
	assert.Empty(t, check(DefaultDecayTime), "within the decay time")

	now = utc(2026, 1, 12, 12, 5) // escalated
	assert.Equal(t, []string{"EscalationResponseTimeStart"}, check(DefaultDecayTime))
	now = utc(2026, 1, 12, 12, 6)
	assert.Empty(t, check(DefaultDecayTime))
	assert.Equal(t, []string{"EscalationResponseTimeStart"}, check(0), "decay 0 repeats every run")

	// Outside the calendar's working time no events are raised.
	now = utc(2026, 1, 13, 20, 0)
	assert.Empty(t, check(0))
	now = utc(2026, 1, 14, 8, 5)
	assert.Equal(t, []string{"EscalationResponseTimeStart"}, check(0))

	assert.Equal(t, []string{
		"EscalationResponseTimeNotifyBefore by 1",
		"EscalationResponseTimeStart by 1",
		"EscalationResponseTimeStart by 1",
		"EscalationResponseTimeStart by 1",
	}, events(t, db, id))
}

func TestDecayTimeSetting(t *testing.T) {
	db := testDB(t)
	exec(t, db, `DELETE FROM sysconfig_modified WHERE name = 'OTRSEscalationEvents::DecayTime'`)
	exec(t, db, `DELETE FROM sysconfig_default WHERE name = 'OTRSEscalationEvents::DecayTime'`)
	d, err := DecayTime(db)
	require.NoError(t, err)
	assert.Equal(t, 24*time.Hour, d)

	setting(t, db, "OTRSEscalationEvents::DecayTime", "--- '30'\n")
	d, err = DecayTime(db)
	require.NoError(t, err)
	assert.Equal(t, 30*time.Minute, d)

	setting(t, db, "OTRSEscalationEvents::DecayTime", "soon")
	_, err = DecayTime(db)
	assert.Error(t, err)
}

func TestIndexer_RebuildsChangedTicketsAndFollowsSettings(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	f := newFixture(t, db, 120, 0, 240, 0)
	now := utc(2026, 1, 12, 8, 0)
	ix := NewIndexer(NewService(db, clockAt(&now)))

	before := f.ticket(t, utc(2026, 1, 12, 8, 0), nil)
	_, err := ix.RunOnce(ctx) // first run: every open ticket
	require.NoError(t, err)
	assert.Equal(t, unix(2026, 1, 12, 10, 0), indexOf(t, db, before).Response)

	// New ticket and new article are picked up by the next run.
	created := f.ticket(t, utc(2026, 1, 12, 9, 0), nil)
	f.article(t, before, "agent", true, utc(2026, 1, 12, 9, 0), otherAgent(t, db))
	n, err := ix.RunOnce(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, 2)
	assert.Equal(t, unix(2026, 1, 12, 11, 0), indexOf(t, db, created).Response)
	assert.Equal(t, Index{Escalation: unix(2026, 1, 12, 13, 0), Update: unix(2026, 1, 12, 13, 0)}, indexOf(t, db, before))

	// A state change is a ticket_history row.
	f.setState(t, created, "closed", utc(2026, 1, 12, 9, 30))
	_, err = ix.RunOnce(ctx)
	require.NoError(t, err)
	assert.Equal(t, Index{}, indexOf(t, db, created))

	// Changing the queue's settings rebuilds its tickets without any ticket change.
	exec(t, db, `UPDATE queue SET first_response_time = 0, update_time = 60, change_time = ? WHERE id = ?`,
		time.Now().UTC().Add(365*24*time.Hour), f.queueID)
	_, err = ix.RunOnce(ctx)
	require.NoError(t, err)
	assert.Equal(t, Index{Escalation: unix(2026, 1, 12, 10, 0), Update: unix(2026, 1, 12, 10, 0)}, indexOf(t, db, before))

	// So does a calendar change: calendar 8 becomes Mondays 08:00-09:00 only.
	setting(t, db, "TimeWorkingHours::Calendar8", "{Mon: [8]}")
	_, err = ix.RunOnce(ctx)
	require.NoError(t, err)
	assert.Equal(t, unix(2026, 1, 19, 9, 0), indexOf(t, db, before).Update, "Mon 09:00 + 1h: next Monday 08:00-09:00")
}
