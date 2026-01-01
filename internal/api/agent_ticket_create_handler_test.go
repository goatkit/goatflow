package api

import (
	"bytes"
	"context"
	"database/sql/driver"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/gotrs-io/gotrs-ce/internal/ticketnumber"
)

type ticketNumberStub struct{ value string }

func (s ticketNumberStub) Name() string      { return "Date" }
func (s ticketNumberStub) IsDateBased() bool { return true }
func (s ticketNumberStub) Next(ctx context.Context, store ticketnumber.CounterStore) (string, error) {
	return s.value, nil
}

type noopCounterStore struct{}

func (noopCounterStore) Add(ctx context.Context, dateScoped bool, offset int64) (int64, error) {
	return 1, nil
}

type equalsInt struct{ want int }

func (e equalsInt) Match(v driver.Value) bool {
	switch val := v.(type) {
	case int:
		return val == e.want
	case int32:
		return int(val) == e.want
	case int64:
		return int(val) == e.want
	case uint:
		return int(val) == e.want
	case uint32:
		return int(val) == e.want
	case uint64:
		return int(val) == e.want
	case string:
		if len(val) == 0 {
			return e.want == 0
		}
	case nil:
		return e.want == 0
	}
	return false
}

type equalsString struct{ want string }

func (e equalsString) Match(v driver.Value) bool {
	if e.want == "" {
		return v == nil || v == ""
	}
	switch val := v.(type) {
	case string:
		return val == e.want
	case []byte:
		return string(val) == e.want
	default:
		return false
	}
}

func articleColumnCheckQuery() string {
	if database.IsMySQL() {
		return "SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'article' AND COLUMN_NAME = ?"
	}
	return database.ConvertPlaceholders(`
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_name = 'article' AND column_name = $1`)
}

func TestHandleAgentCreateTicket_UsesSelectedNextState(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock setup failed: %v", err)
	}
	t.Cleanup(func() {
		database.ResetDB()
		_ = mockDB.Close()
	})

	repository.SetTicketNumberGenerator(ticketNumberStub{value: "202510050001"}, noopCounterStore{})
	t.Cleanup(func() { repository.SetTicketNumberGenerator(nil, nil) })
	database.SetDB(mockDB)

	stateRows := sqlmock.NewRows([]string{"id", "name", "type_id", "valid_id", "create_time", "create_by", "change_time", "change_by"}).
		AddRow(5, "pending reminder", 4, 1, time.Now(), 1, time.Now(), 1)

	stateQuery := database.ConvertPlaceholders(`
		SELECT id, name, type_id, valid_id,
	       create_time, create_by, change_time, change_by
	FROM ticket_state
	WHERE id = $1`)

	mock.ExpectQuery(regexp.QuoteMeta(stateQuery)).
		WithArgs(5).
		WillReturnRows(stateRows)

	pendingUntil := "2025-10-18T15:30"
	pendingUnix := parsePendingUntil(pendingUntil)

	ticketArgs := []driver.Value{
		"202510050001",
		"Pending state ticket",
		1,
		1,
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		equalsInt{want: 5},
		3,
		sqlmock.AnyArg(),
		equalsInt{want: pendingUnix},
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		equalsInt{want: 42},
		sqlmock.AnyArg(),
		equalsInt{want: 42},
	}
	if database.IsMySQL() {
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO ticket (")).
			WithArgs(ticketArgs...).
			WillReturnResult(sqlmock.NewResult(987, 1))
	} else {
		mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO ticket (")).
			WithArgs(ticketArgs...).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(987))
	}

	mock.ExpectBegin()

	mock.ExpectQuery(regexp.QuoteMeta(articleColumnCheckQuery())).
		WithArgs("article_type_id").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(regexp.QuoteMeta(articleColumnCheckQuery())).
		WithArgs("communication_channel_id").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	articleArgs := []driver.Value{
		987,
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
	}
	if database.IsMySQL() {
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO article (")).
			WithArgs(articleArgs...).
			WillReturnResult(sqlmock.NewResult(321, 1))
	} else {
		mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO article (")).
			WithArgs(articleArgs...).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(321))
	}

	mock.ExpectExec("INSERT INTO article_data_mime").
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectExec("UPDATE ticket ").
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectCommit()

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uint(42))
		c.Next()
	})
	router.POST("/tickets", HandleAgentCreateTicket(mockDB))

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("subject", "Pending state ticket"); err != nil {
		t.Fatalf("write field: %v", err)
	}
	if err := writer.WriteField("body", "Details"); err != nil {
		t.Fatalf("write field: %v", err)
	}
	if err := writer.WriteField("queue_id", "1"); err != nil {
		t.Fatalf("write field: %v", err)
	}
	if err := writer.WriteField("priority", "3"); err != nil {
		t.Fatalf("write field: %v", err)
	}
	if err := writer.WriteField("type_id", "1"); err != nil {
		t.Fatalf("write field: %v", err)
	}
	if err := writer.WriteField("interaction_type", "internal_note"); err != nil {
		t.Fatalf("write field: %v", err)
	}
	if err := writer.WriteField("next_state_id", "5"); err != nil {
		t.Fatalf("write field: %v", err)
	}
	if err := writer.WriteField("pending_until", pendingUntil); err != nil {
		t.Fatalf("write field: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer close: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/tickets", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("unexpected status: got %d want %d", rec.Code, http.StatusSeeOther)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestHandleAgentCreateTicket_PendingStateRequiresPendingUntil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock setup failed: %v", err)
	}
	t.Cleanup(func() {
		database.ResetDB()
		_ = mockDB.Close()
	})

	repository.SetTicketNumberGenerator(ticketNumberStub{value: "202510050002"}, noopCounterStore{})
	t.Cleanup(func() { repository.SetTicketNumberGenerator(nil, nil) })
	database.SetDB(mockDB)

	stateRows := sqlmock.NewRows([]string{"id", "name", "type_id", "valid_id", "create_time", "create_by", "change_time", "change_by"}).
		AddRow(5, "pending auto close+", 5, 1, time.Now(), 1, time.Now(), 1)
	stateQuery := database.ConvertPlaceholders(`
		SELECT id, name, type_id, valid_id,
	       create_time, create_by, change_time, change_by
	FROM ticket_state
	WHERE id = $1`)

	mock.ExpectQuery(regexp.QuoteMeta(stateQuery)).
		WithArgs(5).
		WillReturnRows(stateRows)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uint(99))
		c.Next()
	})
	router.POST("/tickets", HandleAgentCreateTicket(mockDB))

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("subject", "Pending without until"); err != nil {
		t.Fatalf("write field: %v", err)
	}
	if err := writer.WriteField("body", "Body"); err != nil {
		t.Fatalf("write field: %v", err)
	}
	if err := writer.WriteField("queue_id", "1"); err != nil {
		t.Fatalf("write field: %v", err)
	}
	if err := writer.WriteField("priority", "3"); err != nil {
		t.Fatalf("write field: %v", err)
	}
	if err := writer.WriteField("type_id", "1"); err != nil {
		t.Fatalf("write field: %v", err)
	}
	if err := writer.WriteField("next_state_id", "5"); err != nil {
		t.Fatalf("write field: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer close: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/tickets", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when pending_until missing, got %d", rec.Code)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestHandleAgentCreateTicket_PendingStateSetsPendingUntil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock setup failed: %v", err)
	}
	t.Cleanup(func() {
		database.ResetDB()
		_ = mockDB.Close()
	})

	repository.SetTicketNumberGenerator(ticketNumberStub{value: "202510050003"}, noopCounterStore{})
	t.Cleanup(func() { repository.SetTicketNumberGenerator(nil, nil) })
	database.SetDB(mockDB)

	stateRows := sqlmock.NewRows([]string{"id", "name", "type_id", "valid_id", "create_time", "create_by", "change_time", "change_by"}).
		AddRow(5, "pending auto close+", 5, 1, time.Now(), 1, time.Now(), 1)
	stateQuery := database.ConvertPlaceholders(`
		SELECT id, name, type_id, valid_id,
	       create_time, create_by, change_time, change_by
	FROM ticket_state
	WHERE id = $1`)

	mock.ExpectQuery(regexp.QuoteMeta(stateQuery)).
		WithArgs(5).
		WillReturnRows(stateRows)

	pendingUntil := "2025-10-18T15:30"
	pendingUnix := parsePendingUntil(pendingUntil)

	ticketArgs2 := []driver.Value{
		"202510050003",
		"Pending auto-close",
		1,
		1,
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		equalsInt{want: 5},
		3,
		sqlmock.AnyArg(),
		equalsInt{want: pendingUnix},
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		equalsInt{want: 42},
		sqlmock.AnyArg(),
		equalsInt{want: 42},
	}
	if database.IsMySQL() {
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO ticket (")).
			WithArgs(ticketArgs2...).
			WillReturnResult(sqlmock.NewResult(654, 1))
	} else {
		mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO ticket (")).
			WithArgs(ticketArgs2...).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(654))
	}

	mock.ExpectBegin()

	mock.ExpectQuery(regexp.QuoteMeta(articleColumnCheckQuery())).
		WithArgs("article_type_id").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(regexp.QuoteMeta(articleColumnCheckQuery())).
		WithArgs("communication_channel_id").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	articleArgs2 := []driver.Value{
		654,
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
	}
	if database.IsMySQL() {
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO article (")).
			WithArgs(articleArgs2...).
			WillReturnResult(sqlmock.NewResult(777, 1))
	} else {
		mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO article (")).
			WithArgs(articleArgs2...).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(777))
	}

	mock.ExpectExec("INSERT INTO article_data_mime").
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectExec("UPDATE ticket ").
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectCommit()

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uint(42))
		c.Next()
	})
	router.POST("/tickets", HandleAgentCreateTicket(mockDB))

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("subject", "Pending auto-close"); err != nil {
		t.Fatalf("write field: %v", err)
	}
	if err := writer.WriteField("body", "Body"); err != nil {
		t.Fatalf("write field: %v", err)
	}
	if err := writer.WriteField("queue_id", "1"); err != nil {
		t.Fatalf("write field: %v", err)
	}
	if err := writer.WriteField("priority", "3"); err != nil {
		t.Fatalf("write field: %v", err)
	}
	if err := writer.WriteField("type_id", "1"); err != nil {
		t.Fatalf("write field: %v", err)
	}
	if err := writer.WriteField("interaction_type", "internal_note"); err != nil {
		t.Fatalf("write field: %v", err)
	}
	if err := writer.WriteField("next_state_id", "5"); err != nil {
		t.Fatalf("write field: %v", err)
	}
	if err := writer.WriteField("pending_until", pendingUntil); err != nil {
		t.Fatalf("write field: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer close: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/tickets", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("unexpected status: got %d want %d", rec.Code, http.StatusSeeOther)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestHandleAgentCreateTicket_ResolvesCustomerUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock setup failed: %v", err)
	}
	t.Cleanup(func() {
		database.ResetDB()
		_ = mockDB.Close()
	})

	repository.SetTicketNumberGenerator(ticketNumberStub{value: "202510059999"}, noopCounterStore{})
	t.Cleanup(func() { repository.SetTicketNumberGenerator(nil, nil) })
	database.SetDB(mockDB)

	stateRows := sqlmock.NewRows([]string{"id", "name", "type_id", "valid_id", "create_time", "create_by", "change_time", "change_by"}).
		AddRow(1, "new", 1, 1, time.Now(), 1, time.Now(), 1)
	stateQuery := database.ConvertPlaceholders(`
		SELECT id, name, type_id, valid_id,
	       create_time, create_by, change_time, change_by
	FROM ticket_state
	WHERE id = $1`)
	mock.ExpectQuery(regexp.QuoteMeta(stateQuery)).
		WithArgs(1).
		WillReturnRows(stateRows)

	custQuery := database.ConvertPlaceholders(`
		SELECT customer_id, email
	FROM customer_user
	WHERE login = $1 AND valid_id = 1`)
	mock.ExpectQuery(regexp.QuoteMeta(custQuery)).
		WithArgs("acme-user").
		WillReturnRows(sqlmock.NewRows([]string{"customer_id", "email"}).AddRow("acme", "user@acme.test"))

	ticketArgs := []driver.Value{
		"202510059999",
		"Customer selected",
		1,
		1,
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		equalsString{want: "acme"},
		equalsString{want: "acme-user"},
		equalsInt{want: 1},
		3,
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		equalsInt{want: 7},
		sqlmock.AnyArg(),
		equalsInt{want: 7},
	}
	if database.IsMySQL() {
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO ticket (")).
			WithArgs(ticketArgs...).
			WillReturnResult(sqlmock.NewResult(456, 1))
	} else {
		mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO ticket (")).
			WithArgs(ticketArgs...).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(456))
	}

	mock.ExpectBegin()

	mock.ExpectQuery(regexp.QuoteMeta(articleColumnCheckQuery())).
		WithArgs("article_type_id").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(regexp.QuoteMeta(articleColumnCheckQuery())).
		WithArgs("communication_channel_id").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	articleArgs := []driver.Value{
		456,
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
	}
	if database.IsMySQL() {
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO article (")).
			WithArgs(articleArgs...).
			WillReturnResult(sqlmock.NewResult(654, 1))
	} else {
		mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO article (")).
			WithArgs(articleArgs...).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(654))
	}

	mock.ExpectExec("INSERT INTO article_data_mime").
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectExec("UPDATE ticket ").
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectCommit()

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uint(7))
		c.Next()
	})
	router.POST("/tickets", HandleAgentCreateTicket(mockDB))

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields := map[string]string{
		"subject":          "Customer selected",
		"body":             "Body",
		"queue_id":         "1",
		"priority":         "3",
		"type_id":          "1",
		"interaction_type": "phone",
		"next_state_id":    "1",
		"customer_user_id": "acme-user",
	}
	for k, v := range fields {
		if err := writer.WriteField(k, v); err != nil {
			t.Fatalf("write field %s: %v", k, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer close: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/tickets", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("unexpected status: got %d want %d", rec.Code, http.StatusSeeOther)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
