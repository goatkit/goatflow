package repository

import (
	"sync"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gotrs-io/gotrs-ce/internal/models"
)

type seqGen struct{ mu sync.Mutex; i int }
func (g *seqGen) Name() string { return "Increment" }
func (g *seqGen) IsDateBased() bool { return false }
func (g *seqGen) Next(ctx interface{}, store interface{}) (string, error) { g.mu.Lock(); defer g.mu.Unlock(); g.i++; return fmt.Sprintf("TN%05d", g.i), nil }

type noopStore struct{}
func (noopStore) NextCounter(scope, date string) (int64, error) { return 1, nil }
func (noopStore) IncrementCounter(scope, date string) (int64, error) { return 1, nil }

func TestTicketRepository_Create_ConcurrencyUnique(t *testing.T) {
	mockDB, mock, err := sqlmock.New(); if err != nil { t.Fatalf("sqlmock: %v", err) }
	defer mockDB.Close()
	gen := &seqGen{}
	store := noopStore{}
	SetTicketNumberGenerator(gen, store)
	r := NewTicketRepository(mockDB)

	workers := 20
	inserts := 20
	// Expect that many INSERT queries; we can use a loose expectation by repeating
	for i := 0; i < inserts; i++ {
		mock.ExpectQuery("INSERT INTO ticket (").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(i+1))
	}

	var wg sync.WaitGroup
	seen := sync.Map{}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m := &models.Ticket{Title:"C", QueueID:1, TicketLockID:1, TicketStateID:1, TicketPriorityID:3, CreateBy:1, ChangeBy:1}
			if err := r.Create(m); err != nil { t.Errorf("create err: %v", err); return }
			if _, loaded := seen.LoadOrStore(m.TicketNumber, struct{}{}); loaded { t.Errorf("duplicate ticket number %s", m.TicketNumber) }
		}()
	}
	wg.Wait()
	if err := mock.ExpectationsWereMet(); err != nil { t.Fatalf("unmet expectations: %v", err) }
	if gen.i != inserts { t.Fatalf("expected %d generations got %d", inserts, gen.i) }
}
