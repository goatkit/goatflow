package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/ticketnumber"
)

// fakeGen implements ticketnumber.Generator subset needed for Create
// We reference interface methods used: Name(), IsDateBased(), Next()

type fakeCounterStore struct{}

func (f fakeCounterStore) Add(ctx context.Context, dateScoped bool, offset int64) (int64, error) {
	return 1, nil
}

// minimal generator fulfilling methods used in repository

type fakeGenerator struct {
	name string
	seq  int
}

func (g *fakeGenerator) Name() string      { return g.name }
func (g *fakeGenerator) IsDateBased() bool { return true }
func (g *fakeGenerator) Next(ctx context.Context, store ticketnumber.CounterStore) (string, error) {
	g.seq++
	return time.Now().Format("20060102150405") + "00", nil
}

// TestTicketRepositoryCreate_UsesGeneratorAndInserts ensures Create() calls generator and inserts row.
func TestTicketRepositoryCreate_UsesGeneratorAndInserts(t *testing.T) {
	t.Setenv("TEST_DB_DRIVER", "postgres")
	database.ResetAdapterForTest()
	database.SetAdapter(&database.PostgreSQLAdapter{})
	t.Cleanup(database.ResetAdapterForTest)

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer mockDB.Close()

	gen := &fakeGenerator{name: "Date"}
	store := fakeCounterStore{}
	SetTicketNumberGenerator(gen, store)
	r := NewTicketRepository(mockDB)

	// Expect insert with placeholder converted pattern; we match on INSERT INTO ticket and returning clause removed by adapter path; allow any args
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO ticket (")).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))

	ticket := &models.Ticket{Title: "Alpha", QueueID: 1, TicketLockID: 1, TicketStateID: 1, TicketPriorityID: 3, CreateBy: 1, ChangeBy: 1}
	err = r.Create(ticket)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if ticket.ID != 42 {
		t.Fatalf("expected id 42 got %d", ticket.ID)
	}
	if ticket.TicketNumber == "" {
		t.Fatalf("expected ticket number assigned")
	}
	if gen.seq != 1 {
		t.Fatalf("expected generator called once got %d", gen.seq)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
