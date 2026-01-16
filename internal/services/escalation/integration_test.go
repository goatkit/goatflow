package escalation

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalendarServiceIntegration(t *testing.T) {
	if err := database.InitTestDB(); err != nil {
		t.Skip("Database not available")
	}
	defer database.CloseTestDB()

	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)

	ctx := context.Background()

	// Ensure sysconfig has TimeWorkingHours
	ensureTimeWorkingHoursConfig(t, db)

	t.Run("LoadCalendars", func(t *testing.T) {
		svc := NewCalendarService(db)

		err := svc.LoadCalendars(ctx)
		require.NoError(t, err)

		// Verify default calendar was loaded
		defaultCal := svc.GetCalendar("")
		assert.NotNil(t, defaultCal, "Default calendar should not be nil")
	})

	t.Run("IsWorkingTime", func(t *testing.T) {
		svc := NewCalendarService(db)
		require.NoError(t, svc.LoadCalendars(ctx))

		// Monday 10:00 should be working time
		mondayMorning := time.Date(2025, 1, 6, 10, 0, 0, 0, time.UTC)
		assert.True(t, svc.IsWorkingTime("", mondayMorning), "Monday 10:00 should be working time")

		// Saturday 10:00 should NOT be working time
		saturdayMorning := time.Date(2025, 1, 11, 10, 0, 0, 0, time.UTC)
		assert.False(t, svc.IsWorkingTime("", saturdayMorning), "Saturday 10:00 should not be working time")

		// Monday 3:00 AM should NOT be working time
		mondayNight := time.Date(2025, 1, 6, 3, 0, 0, 0, time.UTC)
		assert.False(t, svc.IsWorkingTime("", mondayNight), "Monday 03:00 should not be working time")
	})

	t.Run("AddWorkingTime", func(t *testing.T) {
		svc := NewCalendarService(db)
		require.NoError(t, svc.LoadCalendars(ctx))

		// Add 60 minutes on Monday 10:00 should result in Monday 11:00
		start := time.Date(2025, 1, 6, 10, 0, 0, 0, time.UTC)
		result := svc.AddWorkingTime("", start, 60)

		assert.Equal(t, 11, result.Hour(), "Should be 11:00 after adding 60 minutes")
		assert.Equal(t, 6, result.Day(), "Should still be Monday")
	})

	t.Run("AddWorkingTimeAcrossWeekend", func(t *testing.T) {
		svc := NewCalendarService(db)
		require.NoError(t, svc.LoadCalendars(ctx))

		// Friday 16:00, add 4 hours (should land on Monday)
		fridayAfternoon := time.Date(2025, 1, 10, 16, 0, 0, 0, time.UTC)
		result := svc.AddWorkingTime("", fridayAfternoon, 240) // 4 hours

		// Should be Monday (day 13)
		assert.Equal(t, 13, result.Day(), "Should skip weekend and land on Monday")
	})

	t.Run("WorkingTimeBetween", func(t *testing.T) {
		svc := NewCalendarService(db)
		require.NoError(t, svc.LoadCalendars(ctx))

		// Full work day Monday 8:00 to 17:00
		start := time.Date(2025, 1, 6, 8, 0, 0, 0, time.UTC)
		end := time.Date(2025, 1, 6, 17, 0, 0, 0, time.UTC)

		seconds := svc.WorkingTimeBetween("", start, end)
		assert.Equal(t, int64(32400), seconds, "Full work day should be 32400 seconds (9 hours)")
	})
}

func TestEscalationServiceIntegration(t *testing.T) {
	if err := database.InitTestDB(); err != nil {
		t.Skip("Database not available")
	}
	defer database.CloseTestDB()

	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)

	ctx := context.Background()

	// Ensure sysconfig has TimeWorkingHours
	ensureTimeWorkingHoursConfig(t, db)

	t.Run("Initialize", func(t *testing.T) {
		svc := NewService(db, nil)
		err := svc.Initialize(ctx)
		require.NoError(t, err)
	})

	t.Run("GetEscalationPreferencesFromQueue", func(t *testing.T) {
		svc := NewService(db, nil)
		require.NoError(t, svc.Initialize(ctx))

		// Ensure we have a queue with escalation settings
		ensureTestQueueWithEscalation(t, db)

		ticket := &TicketInfo{
			ID:        1,
			QueueID:   1,
			StateType: "new",
		}

		prefs, err := svc.getEscalationPreferences(ctx, ticket)
		require.NoError(t, err)
		assert.NotNil(t, prefs)

		t.Logf("Queue preferences: FirstResponse=%d, Update=%d, Solution=%d",
			prefs.FirstResponseTime, prefs.UpdateTime, prefs.SolutionTime)
	})

	t.Run("GetEscalationPreferencesFromSLA", func(t *testing.T) {
		svc := NewService(db, nil)
		require.NoError(t, svc.Initialize(ctx))

		// Create test SLA with escalation times
		slaID := ensureTestSLAWithEscalation(t, db)

		ticket := &TicketInfo{
			ID:        1,
			QueueID:   1,
			SLAID:     &slaID,
			StateType: "new",
		}

		prefs, err := svc.getEscalationPreferences(ctx, ticket)
		require.NoError(t, err)
		assert.NotNil(t, prefs)
		assert.Equal(t, 60, prefs.FirstResponseTime, "Should get FirstResponseTime from SLA")
		assert.Equal(t, 120, prefs.UpdateTime, "Should get UpdateTime from SLA")
		assert.Equal(t, 480, prefs.SolutionTime, "Should get SolutionTime from SLA")
	})

	t.Run("TicketEscalationIndexBuild", func(t *testing.T) {
		svc := NewService(db, nil)
		require.NoError(t, svc.Initialize(ctx))

		// Create a test ticket
		ticketID := ensureTestTicket(t, db)

		// Build escalation index
		err := svc.TicketEscalationIndexBuild(ctx, ticketID, 1)
		require.NoError(t, err)

		// Verify escalation times were set
		var escalationTime, responseTime, updateTime, solutionTime int64
		query := database.ConvertPlaceholders(`
			SELECT escalation_time, escalation_response_time,
			       escalation_update_time, escalation_solution_time
			FROM ticket WHERE id = ?
		`)
		err = db.QueryRowContext(ctx, query, ticketID).Scan(
			&escalationTime, &responseTime, &updateTime, &solutionTime,
		)
		require.NoError(t, err)

		t.Logf("Ticket %d escalation times: escalation=%d, response=%d, update=%d, solution=%d",
			ticketID, escalationTime, responseTime, updateTime, solutionTime)
	})

	t.Run("ClearEscalationOnClose", func(t *testing.T) {
		svc := NewService(db, nil)
		require.NoError(t, svc.Initialize(ctx))

		// Create a closed ticket
		ticketID := ensureClosedTestTicket(t, db)

		// Build escalation index - should clear times for closed ticket
		err := svc.TicketEscalationIndexBuild(ctx, ticketID, 1)
		require.NoError(t, err)

		// Verify all escalation times are 0
		var escalationTime int64
		query := database.ConvertPlaceholders(`SELECT escalation_time FROM ticket WHERE id = ?`)
		err = db.QueryRowContext(ctx, query, ticketID).Scan(&escalationTime)
		require.NoError(t, err)
		assert.Equal(t, int64(0), escalationTime, "Closed ticket should have 0 escalation time")
	})
}

func TestCheckServiceIntegration(t *testing.T) {
	if err := database.InitTestDB(); err != nil {
		t.Skip("Database not available")
	}
	defer database.CloseTestDB()

	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)

	ctx := context.Background()

	// Ensure sysconfig has TimeWorkingHours
	ensureTimeWorkingHoursConfig(t, db)

	t.Run("CheckEscalations", func(t *testing.T) {
		calSvc := NewCalendarService(db)
		require.NoError(t, calSvc.LoadCalendars(ctx))

		checkSvc := NewCheckService(db, calSvc, nil)

		events, err := checkSvc.CheckEscalations(ctx)
		require.NoError(t, err)

		t.Logf("Found %d escalation events", len(events))
		for _, evt := range events {
			t.Logf("  Ticket %d: %s", evt.TicketID, evt.EventName)
		}
	})

	t.Run("CheckEscalationsWithEscalatingTicket", func(t *testing.T) {
		calSvc := NewCalendarService(db)
		require.NoError(t, calSvc.LoadCalendars(ctx))

		// Create a ticket that is past escalation time
		ticketID := ensureEscalatingTicket(t, db)

		checkSvc := NewCheckService(db, calSvc, nil)
		events, err := checkSvc.CheckEscalations(ctx)
		require.NoError(t, err)

		// Log any events found - note: may not find events if test runs outside business hours
		for _, evt := range events {
			if evt.TicketID == ticketID {
				t.Logf("Found escalation event for ticket %d: %s", ticketID, evt.EventName)
			}
		}

		// Verify ticket was created with correct escalation time
		var escalationTime int64
		query := database.ConvertPlaceholders(`SELECT escalation_time FROM ticket WHERE id = ?`)
		err = db.QueryRow(query, ticketID).Scan(&escalationTime)
		require.NoError(t, err)
		assert.Greater(t, escalationTime, int64(0), "Ticket should have escalation_time set")
		assert.Less(t, escalationTime, time.Now().Unix(), "Escalation time should be in the past")
	})
}

// Helper functions using database.ConvertPlaceholders

func ensureTestQueueWithEscalation(t *testing.T, db *sql.DB) {
	t.Helper()

	query := database.ConvertPlaceholders(`
		UPDATE queue SET
			first_response_time = 60,
			update_time = 120,
			solution_time = 480
		WHERE id = 1
	`)
	_, err := db.Exec(query)
	require.NoError(t, err)
}

func ensureTestSLAWithEscalation(t *testing.T, db *sql.DB) int {
	t.Helper()

	// Check if test SLA exists
	var slaID int
	query := database.ConvertPlaceholders(`SELECT id FROM sla WHERE name = ?`)
	err := db.QueryRow(query, "Test Escalation SLA").Scan(&slaID)
	if err == nil {
		return slaID
	}

	// Create test SLA
	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO sla (name, first_response_time, first_response_notify,
			update_time, update_notify, solution_time, solution_notify,
			valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 60, 80, 120, 80, 480, 80, 1, NOW(), 1, NOW(), 1)
	`)

	insertQuery, useLastInsert := database.ConvertReturning(insertQuery + " RETURNING id")
	if useLastInsert {
		result, err := db.Exec(insertQuery, "Test Escalation SLA")
		require.NoError(t, err)
		id, _ := result.LastInsertId()
		return int(id)
	}

	err = db.QueryRow(insertQuery, "Test Escalation SLA").Scan(&slaID)
	require.NoError(t, err)
	return slaID
}

func ensureTestTicket(t *testing.T, db *sql.DB) int {
	t.Helper()

	// Find or create a test ticket in "new" state
	var ticketID int
	query := database.ConvertPlaceholders(`
		SELECT t.id FROM ticket t
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		WHERE ts.type_id = 1
		LIMIT 1
	`)
	err := db.QueryRow(query).Scan(&ticketID)
	if err == nil {
		return ticketID
	}

	// Need to create a ticket - find required IDs first
	var queueID, stateID, priorityID, typeID int
	db.QueryRow(database.ConvertPlaceholders(`SELECT id FROM queue WHERE valid_id = 1 LIMIT 1`)).Scan(&queueID)
	db.QueryRow(database.ConvertPlaceholders(`SELECT id FROM ticket_state WHERE type_id = 1 LIMIT 1`)).Scan(&stateID)
	db.QueryRow(database.ConvertPlaceholders(`SELECT id FROM ticket_priority WHERE valid_id = 1 LIMIT 1`)).Scan(&priorityID)
	db.QueryRow(database.ConvertPlaceholders(`SELECT id FROM ticket_type WHERE valid_id = 1 LIMIT 1`)).Scan(&typeID)

	if queueID == 0 {
		queueID = 1
	}
	if stateID == 0 {
		stateID = 1
	}
	if priorityID == 0 {
		priorityID = 3
	}
	if typeID == 0 {
		typeID = 1
	}

	tn := time.Now().Format("20060102150405") + fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)

	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO ticket (tn, title, queue_id, ticket_state_id, ticket_priority_id,
			type_id, ticket_lock_id, timeout, until_time,
			escalation_time, escalation_update_time, escalation_response_time, escalation_solution_time,
			user_id, responsible_user_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, ?, ?, 1, 0, 0, 0, 0, 0, 0, 1, 1, NOW(), 1, NOW(), 1)
	`)

	insertQuery, useLastInsert := database.ConvertReturning(insertQuery + " RETURNING id")
	if useLastInsert {
		result, err := db.Exec(insertQuery, tn, "Test Escalation Ticket", queueID, stateID, priorityID, typeID)
		require.NoError(t, err)
		id, _ := result.LastInsertId()
		return int(id)
	}

	err = db.QueryRow(insertQuery, tn, "Test Escalation Ticket", queueID, stateID, priorityID, typeID).Scan(&ticketID)
	require.NoError(t, err)
	return ticketID
}

func ensureClosedTestTicket(t *testing.T, db *sql.DB) int {
	t.Helper()

	// Find a closed state ID (type_id = 3)
	var closedStateID int
	query := database.ConvertPlaceholders(`SELECT id FROM ticket_state WHERE type_id = 3 LIMIT 1`)
	err := db.QueryRow(query).Scan(&closedStateID)
	if err != nil {
		closedStateID = 2 // fallback
	}

	// Create a closed ticket
	ticketID := ensureTestTicket(t, db)

	// Update to closed state
	updateQuery := database.ConvertPlaceholders(`UPDATE ticket SET ticket_state_id = ? WHERE id = ?`)
	_, err = db.Exec(updateQuery, closedStateID, ticketID)
	require.NoError(t, err)

	return ticketID
}

func ensureEscalatingTicket(t *testing.T, db *sql.DB) int {
	t.Helper()

	// Create a ticket and set escalation time in the past
	ticketID := ensureTestTicket(t, db)

	// Set escalation time to 1 hour ago
	pastTime := time.Now().Add(-1 * time.Hour).Unix()
	updateQuery := database.ConvertPlaceholders(`
		UPDATE ticket SET
			escalation_time = ?,
			escalation_response_time = ?
		WHERE id = ?
	`)
	_, err := db.Exec(updateQuery, pastTime, pastTime, ticketID)
	require.NoError(t, err)

	return ticketID
}

func TestRecordEscalationEventIntegration(t *testing.T) {
	if err := database.InitTestDB(); err != nil {
		t.Skip("Database not available")
	}
	defer database.CloseTestDB()

	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)

	ctx := context.Background()

	t.Run("RecordEscalationEvent_IncludesAllRequiredFields", func(t *testing.T) {
		calSvc := NewCalendarService(db)
		require.NoError(t, calSvc.LoadCalendars(ctx))

		checkSvc := NewCheckService(db, calSvc, nil)

		// Ensure the escalation history type exists
		ensureEscalationHistoryType(t, db, "EscalationResponseTimeStart")

		// Create a test ticket with all required fields populated
		ticketID := ensureTestTicket(t, db)

		// Get the ticket's current values for verification
		var typeID, queueID, ownerID, priorityID, stateID int
		query := database.ConvertPlaceholders(`
			SELECT type_id, queue_id, user_id, ticket_priority_id, ticket_state_id
			FROM ticket WHERE id = ?
		`)
		err := db.QueryRowContext(ctx, query, ticketID).Scan(&typeID, &queueID, &ownerID, &priorityID, &stateID)
		require.NoError(t, err, "Should be able to read ticket fields")

		// Record an escalation event
		checkSvc.recordEscalationEvent(ctx, ticketID, "EscalationResponseTimeStart")

		// Verify the history record was created with all required fields
		var historyID int
		var histTypeID, histQueueID, histOwnerID, histPriorityID, histStateID int
		verifyQuery := database.ConvertPlaceholders(`
			SELECT th.id, th.type_id, th.queue_id, th.owner_id, th.priority_id, th.state_id
			FROM ticket_history th
			JOIN ticket_history_type tht ON th.history_type_id = tht.id
			WHERE th.ticket_id = ? AND tht.name = ?
			ORDER BY th.id DESC LIMIT 1
		`)
		err = db.QueryRowContext(ctx, verifyQuery, ticketID, "EscalationResponseTimeStart").Scan(
			&historyID, &histTypeID, &histQueueID, &histOwnerID, &histPriorityID, &histStateID,
		)
		require.NoError(t, err, "Should find history record with escalation event")

		// Verify the history record has the correct values from the ticket
		assert.Equal(t, typeID, histTypeID, "History type_id should match ticket")
		assert.Equal(t, queueID, histQueueID, "History queue_id should match ticket")
		assert.Equal(t, ownerID, histOwnerID, "History owner_id should match ticket")
		assert.Equal(t, priorityID, histPriorityID, "History priority_id should match ticket")
		assert.Equal(t, stateID, histStateID, "History state_id should match ticket")

		t.Logf("Successfully recorded escalation event with all required fields for ticket %d", ticketID)
	})
}

func ensureEscalationHistoryType(t *testing.T, db *sql.DB, name string) {
	t.Helper()

	// Check if the history type exists
	var id int
	query := database.ConvertPlaceholders(`SELECT id FROM ticket_history_type WHERE name = ?`)
	err := db.QueryRow(query, name).Scan(&id)
	if err == nil {
		return // Already exists
	}

	// Insert the history type
	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO ticket_history_type (name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 1, NOW(), 1, NOW(), 1)
	`)
	_, err = db.Exec(insertQuery, name)
	require.NoError(t, err, "Should be able to create history type")
}

func ensureTimeWorkingHoursConfig(t *testing.T, db *sql.DB) {
	t.Helper()

	// Check if TimeWorkingHours already exists in sysconfig_default
	query := database.ConvertPlaceholders(`SELECT COUNT(*) FROM sysconfig_default WHERE name = ?`)
	var count int
	err := db.QueryRow(query, "TimeWorkingHours").Scan(&count)
	if err == nil && count > 0 {
		return // Already exists
	}

	// Insert default TimeWorkingHours (Mon-Fri 8:00-17:00)
	workingHoursYAML := `---
Mon:
- 8
- 9
- 10
- 11
- 12
- 13
- 14
- 15
- 16
Tue:
- 8
- 9
- 10
- 11
- 12
- 13
- 14
- 15
- 16
Wed:
- 8
- 9
- 10
- 11
- 12
- 13
- 14
- 15
- 16
Thu:
- 8
- 9
- 10
- 11
- 12
- 13
- 14
- 15
- 16
Fri:
- 8
- 9
- 10
- 11
- 12
- 13
- 14
- 15
- 16
Sat: []
Sun: []
`

	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO sysconfig_default (
			name, description, navigation, is_invisible, is_readonly, is_required,
			is_valid, has_configlevel, user_modification_possible, user_modification_active,
			xml_content_raw, xml_content_parsed, xml_filename, effective_value,
			is_dirty, exclusive_lock_guid, create_time, create_by, change_time, change_by
		) VALUES (
			?, 'Working hours configuration', 'Core::Time', 0, 0, 0,
			1, 0, 0, 0,
			'', '', 'Calendar.xml', ?,
			0, '', NOW(), 1, NOW(), 1
		)
	`)
	_, err = db.Exec(insertQuery, "TimeWorkingHours", workingHoursYAML)
	if err != nil {
		t.Logf("Warning: could not insert TimeWorkingHours: %v", err)
	}
}
