package repository_test

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/history"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
)

func TestAddTicketHistoryEntryInsertsRow(t *testing.T) {
	t.Setenv("DB_DRIVER", "postgres")

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewTicketRepository(db)

	entry := models.TicketHistoryInsert{
		TicketID:    42,
		TypeID:      2,
		QueueID:     3,
		OwnerID:     4,
		PriorityID:  5,
		StateID:     6,
		CreatedBy:   10,
		HistoryType: history.TypeAddNote,
		Name:        "Agent added note",
	}

	mock.ExpectQuery("SELECT id FROM ticket_history_type").
		WithArgs(history.TypeAddNote).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(15))

	mock.ExpectExec("INSERT INTO ticket_history").
		WithArgs(
			entry.Name,
			15,
			entry.TicketID,
			nil,
			entry.TypeID,
			entry.QueueID,
			entry.OwnerID,
			entry.PriorityID,
			entry.StateID,
			sqlmock.AnyArg(),
			entry.CreatedBy,
			sqlmock.AnyArg(),
			entry.CreatedBy,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = repo.AddTicketHistoryEntry(context.Background(), nil, entry)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAddTicketHistoryEntryCachesTypeLookup(t *testing.T) {
	t.Setenv("DB_DRIVER", "postgres")

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewTicketRepository(db)

	entry := models.TicketHistoryInsert{
		TicketID:    7,
		TypeID:      1,
		QueueID:     2,
		OwnerID:     3,
		PriorityID:  4,
		StateID:     5,
		CreatedBy:   9,
		HistoryType: history.TypePriorityUpdate,
		Name:        "Priority bumped",
	}

	mock.ExpectQuery("SELECT id FROM ticket_history_type").
		WithArgs(history.TypePriorityUpdate).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(22))

	mock.ExpectExec("INSERT INTO ticket_history").
		WithArgs(
			entry.Name,
			22,
			entry.TicketID,
			nil,
			entry.TypeID,
			entry.QueueID,
			entry.OwnerID,
			entry.PriorityID,
			entry.StateID,
			sqlmock.AnyArg(),
			entry.CreatedBy,
			sqlmock.AnyArg(),
			entry.CreatedBy,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	require.NoError(t, repo.AddTicketHistoryEntry(context.Background(), nil, entry))

	// Second call should reuse cached history type id (no additional SELECT expected)
	mock.ExpectExec("INSERT INTO ticket_history").
		WithArgs(
			entry.Name,
			22,
			entry.TicketID,
			nil,
			entry.TypeID,
			entry.QueueID,
			entry.OwnerID,
			entry.PriorityID,
			entry.StateID,
			sqlmock.AnyArg(),
			entry.CreatedBy,
			sqlmock.AnyArg(),
			entry.CreatedBy,
		).
		WillReturnResult(sqlmock.NewResult(2, 1))

	err = repo.AddTicketHistoryEntry(context.Background(), nil, entry)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
