//go:build db

package api

import (
	"database/sql/driver"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/stretchr/testify/require"
)

func TestHandleCreateTicket_PendingStateWithDueDate(t *testing.T) {
	t.Setenv("APP_ENV", "unit-real")
	t.Setenv("DB_DRIVER", "postgres")
	t.Setenv("TEST_DB_DRIVER", "postgres")
	t.Setenv("SKIP_TICKET_SIDE_EFFECTS", "1")
	database.ResetAdapterForTest()
	t.Cleanup(database.ResetAdapterForTest)
	gin.SetMode(gin.TestMode)

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock setup failed: %v", err)
	}
	t.Cleanup(func() { _ = mockDB.Close() })
	database.SetDB(mockDB)
	t.Cleanup(database.ResetDB)

	repository.SetTicketNumberGenerator(stubGen{n: "202510050001"}, stubStore{})
	t.Cleanup(func() { repository.SetTicketNumberGenerator(nil, nil) })

	pendingStateID := 42
	pendingUntil := "2030-05-20T14:30"
	expectedPending, perr := time.Parse("2006-01-02T15:04", pendingUntil)
	require.NoError(t, perr)

	stateRow := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"id", "name", "type_id", "valid_id", "create_time", "create_by", "change_time", "change_by"}).
			AddRow(pendingStateID, "pending auto close", 5, 1, time.Now(), 1, time.Now(), 1)
	}

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, type_id, valid_id")).
		WithArgs(pendingStateID).
		WillReturnRows(stateRow())

	queueExistsQuery := database.ConvertPlaceholders("SELECT EXISTS(SELECT 1 FROM queue WHERE id = $1)")
	mock.ExpectQuery(regexp.QuoteMeta(queueExistsQuery)).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, type_id, valid_id")).
		WithArgs(pendingStateID).
		WillReturnRows(stateRow())

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO ticket (")).
		WithArgs(
			"202510050001",
			"Pending Example",
			int64(1),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			int64(pendingStateID),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			int64(expectedPending.Unix()),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(555))

	historyTypeQuery := database.ConvertPlaceholders(`SELECT id FROM ticket_history_type WHERE name = $1`)
	mock.ExpectQuery(regexp.QuoteMeta(historyTypeQuery)).
		WithArgs("NewTicket").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(30))

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO ticket_history (")).
		WithArgs(
			"Ticket created (202510050001)",
			30,
			555,
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectBegin()

	mock.ExpectQuery(regexp.QuoteMeta(articleColumnCheckQuery())).
		WithArgs("article_type_id").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(regexp.QuoteMeta(articleColumnCheckQuery())).
		WithArgs("communication_channel_id").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	articleArgs := []driver.Value{
		555,
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
	}
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO article (")).
		WithArgs(articleArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(777))

	mock.ExpectExec("INSERT INTO article_data_mime").
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectExec("UPDATE ticket ").
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectCommit()

	router := gin.New()
	router.POST("/api/tickets", handleCreateTicket)

	form := url.Values{
		"subject":        {"Pending Example"},
		"description":    {"Body"},
		"customer_email": {"customer@example.com"},
		"queue_id":       {"1"},
		"next_state":     {"pending-auto-close"},
		"next_state_id":  {"42"},
		"pending_until":  {pendingUntil},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/tickets", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code, "unexpected status: %s", w.Body.String())

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, float64(555), resp["id"])
	require.NotEmpty(t, resp["ticket_number"])
	require.Equal(t, float64(1), resp["queue_id"])
	require.Equal(t, "normal", resp["priority"])
	require.Equal(t, "Ticket created successfully", resp["message"])
	require.Equal(t, float64(1), resp["type_id"])

	time.Sleep(10 * time.Millisecond)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestHandleCreateTicket_PendingStateWithoutDueDateFails(t *testing.T) {
	t.Setenv("APP_ENV", "unit-real")
	t.Setenv("DB_DRIVER", "postgres")
	t.Setenv("TEST_DB_DRIVER", "postgres")
	t.Setenv("SKIP_TICKET_SIDE_EFFECTS", "1")
	database.ResetAdapterForTest()
	t.Cleanup(database.ResetAdapterForTest)
	gin.SetMode(gin.TestMode)

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock setup failed: %v", err)
	}
	t.Cleanup(func() { _ = mockDB.Close() })
	database.SetDB(mockDB)
	t.Cleanup(database.ResetDB)

	repository.SetTicketNumberGenerator(stubGen{n: "202510050001"}, stubStore{})
	t.Cleanup(func() { repository.SetTicketNumberGenerator(nil, nil) })

	pendingStateID := 42

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, type_id, valid_id")).
		WithArgs(pendingStateID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type_id", "valid_id", "create_time", "create_by", "change_time", "change_by"}).
			AddRow(pendingStateID, "pending auto close", 5, 1, time.Now(), 1, time.Now(), 1))

	router := gin.New()
	router.POST("/api/tickets", handleCreateTicket)

	form := url.Values{
		"subject":        {"Pending Example"},
		"description":    {"Body"},
		"customer_email": {"customer@example.com"},
		"queue_id":       {"1"},
		"next_state_id":  {"42"},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/tickets", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Contains(t, strings.ToLower(resp["error"].(string)), "pending_until is required")

	time.Sleep(10 * time.Millisecond)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
