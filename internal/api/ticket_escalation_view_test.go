package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/services/escalation"
)

// An agent sees a ticket's SLA escalation in the ticket view and in the
// ticket list, and the agent's reply ends the first response escalation.
func TestTicketEscalation_ShownInViewsAndStoppedByReply(t *testing.T) {
	db := getTestDB(t)
	ctx := context.Background()
	exec := func(q string, args ...any) {
		t.Helper()
		_, err := db.Exec(database.ConvertPlaceholders(q), args...)
		require.NoError(t, err, q)
	}
	now := time.Now().UTC()

	// Calendar 5 works around the clock, so destinations are plain create time + SLA time.
	all := "[0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23]"
	exec(`DELETE FROM sysconfig_default WHERE name = 'TimeWorkingHours::Calendar5'`)
	exec(`INSERT INTO sysconfig_default (name, description, navigation, is_invisible, is_readonly, is_required,
			is_valid, has_configlevel, user_modification_possible, user_modification_active,
			xml_content_raw, xml_content_parsed, xml_filename, effective_value,
			is_dirty, exclusive_lock_guid, create_time, create_by, change_time, change_by)
		VALUES ('TimeWorkingHours::Calendar5', 'test', 'Core::Time', 0, 0, 0, 1, 0, 0, 0, '', '', 'Test.xml', ?,
			0, '', ?, 1, ?, 1)`,
		fmt.Sprintf("{Mon: %[1]s, Tue: %[1]s, Wed: %[1]s, Thu: %[1]s, Fri: %[1]s, Sat: %[1]s, Sun: %[1]s}", all), now, now)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM sysconfig_default WHERE name = 'TimeWorkingHours::Calendar5'`))
	})
	slaName := fmt.Sprintf("Gold %d", now.UnixNano())
	slaID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO sla (name, calendar_name, first_response_time, first_response_notify, update_time, update_notify,
			solution_time, solution_notify, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, '5', 60, 0, 0, 0, 480, 0, 1, ?, 1, ?, 1) RETURNING id`), slaName, now, now)
	require.NoError(t, err)

	ticketID := createWriteTestTicket(t, db, nil)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`UPDATE ticket SET sla_id = NULL WHERE id = ?`), ticketID)
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM sla WHERE id = ?`), slaID)
	})
	created := now.Add(-2 * time.Hour).Truncate(time.Second)
	exec(`UPDATE ticket SET sla_id = ?, create_time = ? WHERE id = ?`, slaID, created, ticketID)
	var tn string
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT tn FROM ticket WHERE id = ?`), ticketID).Scan(&tn))

	indexer := escalation.NewIndexer(escalation.NewService(db))
	_, err = indexer.RunOnce(ctx)
	require.NoError(t, err)

	router := NewSimpleRouterWithDB(db)
	token := GetTestAuthToken(t)
	get := func(path string) string {
		t.Helper()
		w := companyPageGet(t, router, token, path)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		return w.Body.String()
	}

	page := get(fmt.Sprintf("/ticket/%d", ticketID))
	assert.Contains(t, page, slaName, "the ticket's SLA is shown")
	response := card(t, page, `data-escalation="first_response"`)
	assert.Contains(t, response, `data-escalated="true"`)
	assert.Contains(t, response, created.Add(time.Hour).Format("2006-01-02 15:04"))
	assert.Contains(t, response, "Overdue by")
	solution := card(t, page, `data-escalation="solution"`)
	assert.NotContains(t, solution, `data-escalated`)
	assert.Contains(t, solution, created.Add(8*time.Hour).Format("2006-01-02 15:04"))
	assert.Contains(t, solution, "Due in")

	list := get("/agent/tickets?status=escalated&per_page=100")
	assert.Contains(t, list, tn, "escalated filter lists the ticket")

	// The agent replies: the first response is done and its escalation stops.
	w := serveWriteTest(http.MethodPost, "/api/tickets/:id/reply", fmt.Sprintf("/api/tickets/%d/reply", ticketID),
		"application/x-www-form-urlencoded", "reply=On+it", map[string]any{"user_id": 1}, handleTicketReply)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	_, err = indexer.RunOnce(ctx)
	require.NoError(t, err)

	assert.Equal(t, 1, countRows(t, db, `SELECT COUNT(*) FROM ticket_history th
		JOIN ticket_history_type tht ON tht.id = th.history_type_id
		WHERE th.ticket_id = ? AND tht.name = 'EscalationResponseTimeStop'`, ticketID))
	page = get(fmt.Sprintf("/ticket/%d", ticketID))
	assert.NotContains(t, page, `data-escalation="first_response"`)
	assert.Contains(t, page, `data-escalation="solution"`)
	assert.NotContains(t, get("/agent/tickets?status=escalated&per_page=100"), tn)
}

// card returns the element of the ticket view whose opening tag contains marker.
func card(t *testing.T, page, marker string) string {
	t.Helper()
	i := strings.Index(page, marker)
	require.GreaterOrEqual(t, i, 0, "%s not on the page", marker)
	start := strings.LastIndex(page[:i], "<div")
	end := strings.Index(page[i:], "</span>")
	require.GreaterOrEqual(t, end, 0)
	return page[start : i+end]
}
