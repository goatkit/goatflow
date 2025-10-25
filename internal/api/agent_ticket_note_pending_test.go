package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/config"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/stretchr/testify/require"
)

func TestHandleAgentTicketNotePendingReminderPersistsPendingUntil(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DB_DRIVER", "postgres")

	_, filename, _, _ := runtime.Caller(0)
	baseDir := filepath.Dir(filename)

	if err := config.LoadFromFile(filepath.Join(baseDir, "testdata", "config_timezone.yaml")); err != nil {
		t.Fatalf("failed to load test config: %v", err)
	}
	t.Cleanup(func() {
		if err := config.LoadFromFile(filepath.Join(baseDir, "testdata", "config_timezone_utc.yaml")); err != nil {
			t.Fatalf("failed to restore config: %v", err)
		}
	})

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock setup failed: %v", err)
	}
	defer mockDB.Close()

	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	pendingLocal := time.Date(2025, time.October, 18, 9, 30, 0, 0, loc)
	expectedUnix := pendingLocal.UTC().Unix()
	prevChange := time.Date(2025, time.October, 17, 14, 15, 0, 0, time.UTC)
	updatedChange := time.Date(2025, time.October, 17, 15, 0, 0, 0, time.UTC)
	createTime := prevChange.Add(-24 * time.Hour)

	ticketColumns := []string{
		"id", "tn", "title", "queue_id", "ticket_lock_id", "type_id", "service_id", "sla_id",
		"user_id", "responsible_user_id", "customer_id", "customer_user_id", "ticket_state_id",
		"ticket_priority_id", "until_time", "escalation_time", "escalation_update_time",
		"escalation_response_time", "escalation_solution_time", "archive_flag", "create_time",
		"create_by", "change_time", "change_by",
	}

	preTicketRows := sqlmock.NewRows(ticketColumns).AddRow(
		123,
		"2025100100001",
		"Sample ticket",
		1,
		1,
		nil,
		nil,
		nil,
		99,
		nil,
		nil,
		nil,
		2,
		1,
		expectedUnix,
		0,
		0,
		0,
		0,
		0,
		createTime,
		1,
		prevChange,
		1,
	)

	postTicketRows := sqlmock.NewRows(ticketColumns).AddRow(
		123,
		"2025100100001",
		"Sample ticket",
		1,
		1,
		nil,
		nil,
		nil,
		99,
		nil,
		nil,
		nil,
		4,
		1,
		expectedUnix,
		0,
		0,
		0,
		0,
		0,
		createTime,
		1,
		updatedChange,
		1,
	)

	ticketSnapshotQuery := fmt.Sprintf(`SELECT
		t.id, t.tn, t.title, t.queue_id, t.ticket_lock_id, %s AS type_id,
		t.service_id, t.sla_id, t.user_id, t.responsible_user_id,
		t.customer_id, t.customer_user_id, t.ticket_state_id,
		t.ticket_priority_id, t.until_time, t.escalation_time,
		t.escalation_update_time, t.escalation_response_time,
		t.escalation_solution_time, t.archive_flag,
		t.create_time, t.create_by, t.change_time, t.change_by
	FROM ticket t
	WHERE t.id = $1`, database.QualifiedTicketTypeColumn("t"))

	mock.ExpectQuery(regexp.QuoteMeta(ticketSnapshotQuery)).
		WithArgs(uint(123)).
		WillReturnRows(preTicketRows)

	stateRows := sqlmock.NewRows([]string{"id", "name", "type_id", "comments", "valid_id", "create_time", "create_by", "change_time", "change_by"}).
		AddRow(4, "pending reminder", 4, "", 1, time.Now(), 1, time.Now(), 1)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, name, type_id, comments, valid_id,
	       create_time, create_by, change_time, change_by
	FROM ticket_state
	WHERE id = $1`)).
		WithArgs(uint(4)).
		WillReturnRows(stateRows)

	mock.ExpectBegin()

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO article (ticket_id, article_sender_type_id, communication_channel_id, is_visible_for_customer,
                                create_time, create_by, change_time, change_by)
            VALUES ($1, 1, $2, $3, CURRENT_TIMESTAMP, $4, CURRENT_TIMESTAMP, $4)
            RETURNING id`)).
		WithArgs(123, 3, 0, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(555))

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO article_data_mime (article_id, a_from, a_subject, a_body, a_content_type, incoming_time, create_time, create_by, change_time, change_by)
            VALUES ($1, 'Agent', $2, $3, $4, $5, CURRENT_TIMESTAMP, $6, CURRENT_TIMESTAMP, $6)`)).
		WithArgs(int64(555), "Internal Note", "Pending reminder set", "text/plain", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE ticket
                SET ticket_state_id = $1, until_time = $2, change_time = CURRENT_TIMESTAMP, change_by = $3
                WHERE id = $4`)).
		WithArgs(4, expectedUnix, sqlmock.AnyArg(), 123).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectCommit()

	mock.ExpectQuery(regexp.QuoteMeta(ticketSnapshotQuery)).
		WithArgs(uint(123)).
		WillReturnRows(postTicketRows)

	// History expectations
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM ticket_history_type WHERE name = $1`)).
		WithArgs("AddNote").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(15))

	noteMessage := "Internal note added — Pending reminder set"
	historyInsert := fmt.Sprintf(`INSERT INTO ticket_history (
		name, history_type_id, ticket_id, article_id, %s, queue_id, owner_id,
		priority_id, state_id, create_time, create_by, change_time, change_by
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, database.TicketTypeColumn())

	mock.ExpectExec(regexp.QuoteMeta(historyInsert)).
		WithArgs(noteMessage, 15, 123, 555, 0, 1, 99, 1, 4, sqlmock.AnyArg(), 99, sqlmock.AnyArg(), 99).
		WillReturnResult(sqlmock.NewResult(1, 1))

	prevStateRows := sqlmock.NewRows([]string{"id", "name", "type_id", "valid_id", "create_time", "create_by", "change_time", "change_by"}).
		AddRow(2, "open", 2, 1, time.Now(), 1, time.Now(), 1)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, name, type_id, valid_id,
		       create_time, create_by, change_time, change_by
		FROM ticket_state
		WHERE id = $1`)).
		WithArgs(2).
		WillReturnRows(prevStateRows)

	stateLookupRows := sqlmock.NewRows([]string{"id", "name", "type_id", "valid_id", "create_time", "create_by", "change_time", "change_by"}).
		AddRow(4, "pending reminder", 4, 1, time.Now(), 1, time.Now(), 1)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, name, type_id, valid_id,
		       create_time, create_by, change_time, change_by
		FROM ticket_state
		WHERE id = $1`)).
		WithArgs(4).
		WillReturnRows(stateLookupRows)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM ticket_history_type WHERE name = $1`)).
		WithArgs("StateUpdate").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(16))

	stateMessage := "State changed from open to pending reminder"
	mock.ExpectExec(regexp.QuoteMeta(historyInsert)).
		WithArgs(stateMessage, 16, 123, 555, 0, 1, 99, 1, 4, sqlmock.AnyArg(), 99, sqlmock.AnyArg(), 99).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM ticket_history_type WHERE name = $1`)).
		WithArgs("SetPendingTime").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(17))

	pendingMessage := fmt.Sprintf("Pending until %s", time.Unix(expectedUnix, 0).In(time.Local).Format("02 Jan 2006 15:04"))
	mock.ExpectExec(regexp.QuoteMeta(historyInsert)).
		WithArgs(pendingMessage, 17, 123, 555, 0, 1, 99, 1, 4, sqlmock.AnyArg(), 99, sqlmock.AnyArg(), 99).
		WillReturnResult(sqlmock.NewResult(1, 1))

	handler := handleAgentTicketNote(mockDB)

	form := url.Values{}
	form.Set("body", "Pending reminder set")
	form.Set("next_state_id", "4")
	form.Set("pending_until", "2025-10-18T09:30")

	req := httptest.NewRequest(http.MethodPost, "/agent/tickets/123/note", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = []gin.Param{{Key: "id", Value: "123"}}
	c.Set("user_id", uint(99))

	handler(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}
