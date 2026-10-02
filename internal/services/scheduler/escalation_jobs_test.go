package scheduler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/database"
)

const allHours = "[0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23]"

// The built-in escalation jobs fill a new ticket's OTRS escalation index from
// its queue's settings and raise the escalation event once it is overdue.
func TestEscalationJobs_IndexNewTicketsAndRaiseEvents(t *testing.T) {
	db := lockTestDB(t)
	ctx := context.Background()
	exec := func(q string, args ...any) {
		t.Helper()
		_, err := db.Exec(database.ConvertPlaceholders(q), args...)
		require.NoError(t, err, q)
	}
	now := time.Now().UTC()

	// Calendar 6: around the clock, so the destination is exactly create time + 60 minutes.
	exec(`DELETE FROM sysconfig_default WHERE name = 'TimeWorkingHours::Calendar6'`)
	exec(`INSERT INTO sysconfig_default (name, description, navigation, is_invisible, is_readonly, is_required,
			is_valid, has_configlevel, user_modification_possible, user_modification_active,
			xml_content_raw, xml_content_parsed, xml_filename, effective_value,
			is_dirty, exclusive_lock_guid, create_time, create_by, change_time, change_by)
		VALUES ('TimeWorkingHours::Calendar6', 'test', 'Core::Time', 0, 0, 0, 1, 0, 0, 0, '', '', 'Test.xml', ?,
			0, '', ?, 1, ?, 1)`,
		fmt.Sprintf("{Mon: %[1]s, Tue: %[1]s, Wed: %[1]s, Thu: %[1]s, Fri: %[1]s, Sat: %[1]s, Sun: %[1]s}", allHours), now, now)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM sysconfig_default WHERE name = 'TimeWorkingHours::Calendar6'`))
	})

	queueID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO queue (name, group_id, first_response_time, first_response_notify, update_time, update_notify,
			solution_time, solution_notify, system_address_id, calendar_name, salutation_id, signature_id,
			follow_up_id, follow_up_lock, valid_id, create_time, create_by, change_time, change_by)
		SELECT ?, group_id, 60, 0, 0, 0, 0, 0, system_address_id, '6', salutation_id, signature_id,
			follow_up_id, 0, 1, ?, 1, ?, 1 FROM queue WHERE id = 1
		RETURNING id`), fmt.Sprintf("EscJobs%d", now.UnixNano()), now, now)
	require.NoError(t, err)
	created := now.Add(-2 * time.Hour).Truncate(time.Second)
	ticketID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO ticket (tn, title, queue_id, ticket_lock_id, type_id, user_id, responsible_user_id,
			ticket_priority_id, ticket_state_id, timeout, until_time, escalation_time, escalation_update_time,
			escalation_response_time, escalation_solution_time, archive_flag, create_time, create_by, change_time, change_by)
		SELECT ?, 'escalation job test', ?, 1, 1, 1, 1, 3, s.id, 0, 0, 0, 0, 0, 0, 0, ?, 1, ?, 1
		FROM ticket_state s JOIN ticket_state_type st ON st.id = s.type_id WHERE st.name = 'open' ORDER BY s.id
		LIMIT 1 RETURNING id`), fmt.Sprintf("EJ%d", now.UnixNano()), queueID, created, created)
	require.NoError(t, err)
	t.Cleanup(func() {
		for _, q := range []string{
			`DELETE FROM ticket_history WHERE ticket_id = ?`,
			`DELETE FROM ticket WHERE id = ?`,
		} {
			_, _ = db.Exec(database.ConvertPlaceholders(q), ticketID)
		}
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM queue WHERE id = ?`), queueID)
	})

	s := NewService(db)
	run := func(slug string) {
		t.Helper()
		var job *models.ScheduledJob
		for _, j := range DefaultJobs() {
			if j.Slug == slug {
				job = j
			}
		}
		require.NotNil(t, job, "default job %s", slug)
		h := s.getHandler(job.Handler)
		require.NotNil(t, h, "handler %s", job.Handler)
		require.NoError(t, h(ctx, job))
	}

	run("escalation-index")
	var response, escalation int64
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		`SELECT escalation_response_time, escalation_time FROM ticket WHERE id = ?`), ticketID).Scan(&response, &escalation))
	assert.Equal(t, created.Add(time.Hour).Unix(), response)
	assert.Equal(t, response, escalation)

	run("escalation-check")
	run("escalation-check") // within OTRSEscalationEvents::DecayTime: no repeat
	var starts int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`
		SELECT COUNT(*) FROM ticket_history th JOIN ticket_history_type tht ON tht.id = th.history_type_id
		WHERE th.ticket_id = ? AND tht.name = 'EscalationResponseTimeStart'`), ticketID).Scan(&starts))
	assert.Equal(t, 1, starts)
}
