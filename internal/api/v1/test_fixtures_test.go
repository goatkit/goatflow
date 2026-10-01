package v1

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/ticketnumber"
)

var (
	integrationSeedOnce sync.Once
	integrationSeedErr  error

	// testTicketNumbers is shared by every test so numbers never repeat within
	// a run; the start-time prefix keeps them unique across runs.
	testTicketNumbers = &sequentialTicketNumber{prefix: time.Now().Format("20060102150405")}
)

func ensureTicketFixtures(t *testing.T) {
	t.Helper()
	requireDatabase(t)

	integrationSeedOnce.Do(func() {
		db, err := database.GetDB()
		if err != nil {
			integrationSeedErr = err
			return
		}
		if db == nil {
			integrationSeedErr = fmt.Errorf("integration database not available")
			return
		}
		integrationSeedErr = grantRootGroupPermissions(db)
	})

	if integrationSeedErr != nil {
		t.Skipf("skipping integration test: %v", integrationSeedErr)
	}

	repository.SetTicketNumberGenerator(testTicketNumbers, testCounterStore{})
	t.Cleanup(func() { repository.SetTicketNumberGenerator(nil, nil) })
}

type sequentialTicketNumber struct {
	mu     sync.Mutex
	prefix string
	seq    int64
}

func (g *sequentialTicketNumber) Name() string      { return "TestSequential" }
func (g *sequentialTicketNumber) IsDateBased() bool { return true }
func (g *sequentialTicketNumber) Next(ctx context.Context, store ticketnumber.CounterStore) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.seq++
	return fmt.Sprintf("%s%04d", g.prefix, g.seq), nil
}

type testCounterStore struct{}

func (testCounterStore) Add(ctx context.Context, dateScoped bool, offset int64) (int64, error) {
	return offset, nil
}

// grantRootGroupPermissions grants user 1 (root) every group 1 permission the
// ticket and article handlers check. Queues, states, priorities and types come
// from the migration seeds plus resetTestDatabase. group_user has no unique
// key, so check before inserting to avoid duplicate grants.
func grantRootGroupPermissions(db *sql.DB) error {
	for _, perm := range []string{"rw", "create", "note", "owner", "priority", "move_into"} {
		var count int
		if err := db.QueryRow(database.ConvertPlaceholders(
			"SELECT COUNT(*) FROM group_user WHERE user_id = 1 AND group_id = 1 AND permission_key = ?",
		), perm).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		if _, err := db.Exec(database.ConvertPlaceholders(`
			INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
			VALUES (1, 1, ?, NOW(), 1, NOW(), 1)
		`), perm); err != nil {
			return err
		}
	}
	return nil
}
