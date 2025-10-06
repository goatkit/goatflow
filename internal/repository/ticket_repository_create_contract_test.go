package repository

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// fakeGen implements ticketnumber.Generator subset needed for Create
// We reference interface methods used: Name(), IsDateBased(), Next()

type fakeCounterStore struct{}

func (f fakeCounterStore) NextCounter(scope string, date string) (int64, error) { return 1, nil }
func (f fakeCounterStore) IncrementCounter(scope string, date string) (int64, error) { return 1, nil }

// minimal generator fulfilling methods used in repository

type fakeGenerator struct{ name string; seq int }

func (g *fakeGenerator) Name() string { return g.name }
func (g *fakeGenerator) IsDateBased() bool { return true }
func (g *fakeGenerator) Next(ctx interface{}, store interface{}) (string, error) {
	g.seq++
	return time.Now().Format("20060102150405") + "00", nil
}

// TestTicketRepositoryCreate_UsesGeneratorAndInserts ensures Create() calls generator and inserts row.
func TestTicketRepositoryCreate_UsesGeneratorAndInserts(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil { t.Fatalf("sqlmock: %v", err) }
	defer mockDB.Close()

	gen := &fakeGenerator{name: "Date"}
	store := fakeCounterStore{}
	SetTicketNumberGenerator(gen, store)
	r := NewTicketRepository(mockDB)

	// Expect insert with placeholder converted pattern; we match on INSERT INTO ticket and returning clause removed by adapter path; allow any args
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO ticket ("))
		.WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))

	t := &models.Ticket{Title: "Alpha", QueueID: 1, TicketLockID: 1, TicketStateID: 1, TicketPriorityID: 3, CreateBy: 1, ChangeBy: 1}
	err = r.Create(t)
	if err != nil { t.Fatalf("Create returned error: %v", err) }
	if t.ID != 42 { t.Fatalf("expected id 42 got %d", t.ID) }
	if t.TicketNumber == "" { t.Fatalf("expected ticket number assigned") }
	if gen.seq != 1 { t.Fatalf("expected generator called once got %d", gen.seq) }

	if err := mock.ExpectationsWereMet(); err != nil { t.Fatalf("unmet expectations: %v", err) }
}
