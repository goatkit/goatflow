package history_test

import (
	"context"
	"testing"
	"time"
	"unicode/utf8"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/history"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
)

func TestRecorderRecordCreatesHistoryEntry(t *testing.T) {
	t.Setenv("DB_DRIVER", "postgres")

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewTicketRepository(db)
	recorder := history.NewRecorder(repo)

	changeTime := time.Now().UTC().Truncate(time.Second)

	ownerID := 15
	typeID := 9
	ticket := &models.Ticket{
		ID:               100,
		QueueID:          12,
		TypeID:           &typeID,
		UserID:           &ownerID,
		TicketPriorityID: 8,
		TicketStateID:    3,
		ChangeBy:         15,
		ChangeTime:       changeTime,
	}

	actorID := 15
	message := "Queue changed from Support to Escalation"

	mock.ExpectQuery("SELECT id FROM ticket_history_type").
		WithArgs(history.TypeQueueMove).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(5))

	mock.ExpectExec("INSERT INTO ticket_history").
		WithArgs(
			message,
			5,
			ticket.ID,
			nil,
			typeID,
			ticket.QueueID,
			ownerID,
			ticket.TicketPriorityID,
			ticket.TicketStateID,
			ticket.ChangeTime,
			actorID,
			ticket.ChangeTime,
			actorID,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = recorder.Record(context.Background(), nil, ticket, nil, history.TypeQueueMove, message, actorID)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRecorderChangeMessage(t *testing.T) {
	msg := history.ChangeMessage("State", "Open", "Closed")
	require.Equal(t, "State changed from Open to Closed", msg)
}

func TestRecorderExcerpt(t *testing.T) {
	longText := "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Vestibulum viverra leo sed." // 100+ chars
	excerpt := history.Excerpt(longText, 50)
	require.LessOrEqual(t, utf8.RuneCountInString(excerpt), 51)
	require.Contains(t, excerpt, "\u2026")
}
