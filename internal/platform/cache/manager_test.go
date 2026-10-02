package cache

import (
	"context"
	"errors"
	"testing"
)

// failingStrategy records deletes and fails those listed in failOn.
type failingStrategy struct {
	CacheStrategy
	failOn  map[string]bool
	deleted []string
}

func (s *failingStrategy) Delete(_ context.Context, key string) error {
	s.deleted = append(s.deleted, key)
	if s.failOn[key] {
		return errors.New("backend down")
	}
	return nil
}

func TestInvalidateTicketReportsRelatedKeyFailures(t *testing.T) {
	s := &failingStrategy{failOn: map[string]bool{"ticket:7:articles": true}}
	m := &Manager{config: &ManagerConfig{}, strategies: map[string]CacheStrategy{"write-through": s}}

	err := m.InvalidateTicket(context.Background(), 7)
	if err == nil {
		t.Fatal("expected error when a related key could not be invalidated")
	}
	want := []string{"ticket:7", "ticket:7:articles", "ticket:7:attachments", "ticket:7:history"}
	if len(s.deleted) != len(want) {
		t.Fatalf("deleted %v, want all of %v attempted", s.deleted, want)
	}
	for i := range want {
		if s.deleted[i] != want[i] {
			t.Fatalf("deleted %v, want %v", s.deleted, want)
		}
	}
}
