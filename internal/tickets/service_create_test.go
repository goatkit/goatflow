package tickets_test

import (
	context "context"
	"errors"
	"testing"
	"time"
)

// Minimal local interfaces to avoid pulling full repo graph (adjust once real service exists)

type fakeGenerator struct{ n string; err error }
func (f fakeGenerator) Generate(ctx context.Context) (string, error) { return f.n, f.err }

type CreateParams struct {
	Title    string
	QueueID  int
	Priority int
	Body     string
}

type Ticket struct {
	ID     int
	Number string
	Title  string
	QueueID int
	PriorityID int
	CreateTime time.Time
}

type Service struct {
	gen interface{ Generate(context.Context) (string, error) }
	store *memStore
}

type memStore struct { tickets []Ticket }

func newService(g interface{ Generate(context.Context) (string, error) }) *Service { return &Service{gen: g, store: &memStore{}} }

func (s *Service) Create(ctx context.Context, p CreateParams) (Ticket, error) {
	if p.Title == "" { return Ticket{}, errors.New("title required") }
	if len(p.Title) > 200 { return Ticket{}, errors.New("title too long") }
	if p.QueueID <= 0 { return Ticket{}, errors.New("invalid queue") }
	if p.Priority <= 0 { return Ticket{}, errors.New("invalid priority") }
	n, err := s.gen.Generate(ctx)
	if err != nil { return Ticket{}, err }
	t := Ticket{ID: len(s.store.tickets)+1, Number: n, Title: p.Title, QueueID: p.QueueID, PriorityID: p.Priority, CreateTime: time.Now().UTC()}
	s.store.tickets = append(s.store.tickets, t)
	return t, nil
}

func TestServiceCreate_HappyPath(t *testing.T) {
	s := newService(fakeGenerator{n: "TNUM123"})
	ctx := context.Background()
	tk, err := s.Create(ctx, CreateParams{Title: "Alpha", QueueID: 1, Priority: 3})
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if tk.Number != "TNUM123" { t.Fatalf("expected ticket number TNUM123 got %s", tk.Number) }
	if tk.ID != 1 { t.Fatalf("expected ID 1 got %d", tk.ID) }
}

func TestServiceCreate_Validation(t *testing.T) {
	s := newService(fakeGenerator{n: "IGNORED"})
	ctx := context.Background()
	cases := []struct{ name string; p CreateParams; wantErr string }{
		{"empty title", CreateParams{Title: "", QueueID:1, Priority:3}, "title required"},
		{"long title", CreateParams{Title: string(make([]byte, 201)), QueueID:1, Priority:3}, "title too long"},
		{"bad queue", CreateParams{Title: "Ok", QueueID:0, Priority:3}, "invalid queue"},
		{"bad priority", CreateParams{Title: "Ok", QueueID:1, Priority:0}, "invalid priority"},
	}
	for _, cse := range cases {
		_, err := s.Create(ctx, cse.p)
		if err == nil || err.Error() != cse.wantErr { t.Fatalf("%s: expected %s got %v", cse.name, cse.wantErr, err) }
	}
}

func TestServiceCreate_GeneratorError(t *testing.T) {
	s := newService(fakeGenerator{err: errors.New("gen fail")})
	_, err := s.Create(context.Background(), CreateParams{Title: "A", QueueID:1, Priority:3})
	if err == nil || err.Error() != "gen fail" { t.Fatalf("expected gen fail got %v", err) }
}
