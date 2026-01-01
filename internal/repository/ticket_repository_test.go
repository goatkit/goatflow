package repository

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/ticketnumber"
)

// stub generator returning fixed TN without using store.
type stubGen struct{}

func (s stubGen) Name() string      { return "AutoIncrement" }
func (s stubGen) IsDateBased() bool { return false }
func (s stubGen) Next(ctx context.Context, store ticketnumber.CounterStore) (string, error) {
	return "45000042", nil
}

// dummy store (never called because stubGen ignores it).
type dummyStore struct{}

func (d dummyStore) Add(ctx context.Context, dateScoped bool, offset int64) (int64, error) {
	return 0, nil
}

func TestTicketRepositoryCreate_IncrementsTN(t *testing.T) {
	t.Setenv("TEST_DB_DRIVER", "postgres")
	database.ResetAdapterForTest()
	database.SetAdapter(&database.PostgreSQLAdapter{})
	t.Cleanup(database.ResetAdapterForTest)

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	// Inject generator
	SetTicketNumberGenerator(stubGen{}, dummyStore{})

	repo := NewTicketRepository(db)

	ticket := &models.Ticket{
		Title: "Test", QueueID: 1, TicketLockID: 1, TicketStateID: 1, TicketPriorityID: 3,
		UserID: intPtr(1), ResponsibleUserID: intPtr(1), CreateBy: 1, ChangeBy: 1,
	}

	// Expect INSERT ... RETURNING id (Postgres style)
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO ticket")).
		WithArgs(
			"45000042", ticket.Title, ticket.QueueID, ticket.TicketLockID, ticket.TypeID,
			ticket.ServiceID, ticket.SLAID, ticket.UserID, ticket.ResponsibleUserID,
			ticket.CustomerID, ticket.CustomerUserID, ticket.TicketStateID,
			ticket.TicketPriorityID, ticket.Timeout, ticket.UntilTime, ticket.EscalationTime,
			ticket.EscalationUpdateTime, ticket.EscalationResponseTime, ticket.EscalationSolutionTime,
			ticket.ArchiveFlag, sqlmock.AnyArg(), ticket.CreateBy, sqlmock.AnyArg(), ticket.ChangeBy,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(123))

	if err := repo.Create(ticket); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if ticket.ID != 123 {
		t.Fatalf("expected ID=123 got %d", ticket.ID)
	}
	if ticket.TicketNumber != "45000042" {
		t.Fatalf("tn mismatch got %s", ticket.TicketNumber)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func intPtr(v int) *int { return &v }
