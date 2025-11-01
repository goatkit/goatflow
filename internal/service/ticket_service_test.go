package service

import (
	"context"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/gotrs-io/gotrs-ce/internal/ticketnumber"
)

type stubGen struct{ n string }

func (g stubGen) Name() string      { return "Date" }
func (g stubGen) IsDateBased() bool { return true }
func (g stubGen) Next(ctx context.Context, store ticketnumber.CounterStore) (string, error) {
	return g.n, nil
}

type stubStore struct{}

func (stubStore) Add(ctx context.Context, dateScoped bool, offset int64) (int64, error) {
	return 1, nil
}

func TestTicketService_CreateRecordsHistory(t *testing.T) {
	t.Setenv("DB_DRIVER", "postgres")
	database.ResetAdapterForTest()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repository.SetTicketNumberGenerator(stubGen{n: "202510050002"}, stubStore{})
	repo := repository.NewTicketRepository(db)
	svc := NewTicketService(repo)

	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS(SELECT 1 FROM queue")).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, type_id, valid_id,")).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type_id", "valid_id", "create_time", "create_by", "change_time", "change_by"}).
			AddRow(1, "new", 1, 1, now, 1, now, 1))

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO ticket (")).
		WithArgs(
			"202510050002",
			"Alpha",
			1,
			1,
			nil,
			nil,
			nil,
			1,
			1,
			nil,
			nil,
			1,
			3,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			sqlmock.AnyArg(),
			1,
			sqlmock.AnyArg(),
			1,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(88))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM ticket_history_type")).
		WithArgs("NewTicket").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(30))

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO ticket_history (")).
		WithArgs(
			"Ticket created (202510050002)",
			30,
			88,
			nil,
			1,
			1,
			1,
			3,
			1,
			sqlmock.AnyArg(),
			1,
			sqlmock.AnyArg(),
			1,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	result, err := svc.Create(context.Background(), CreateTicketInput{
		Title:   "Alpha",
		QueueID: 1,
		UserID:  1,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "202510050002", result.TicketNumber)
	require.NoError(t, mock.ExpectationsWereMet())
}
