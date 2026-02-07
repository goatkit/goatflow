package v1

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/ticketnumber"
)

var (
	integrationSeedOnce sync.Once
	integrationSeedErr  error
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
		integrationSeedErr = seedLookupTables(db)
	})

	if integrationSeedErr != nil {
		t.Skipf("skipping integration test: %v", integrationSeedErr)
	}

	gen := &sequentialTicketNumber{}
	repository.SetTicketNumberGenerator(gen, testCounterStore{})
	t.Cleanup(func() { repository.SetTicketNumberGenerator(nil, nil) })
}

type sequentialTicketNumber struct {
	mu  sync.Mutex
	seq int64
}

func (g *sequentialTicketNumber) Name() string      { return "TestSequential" }
func (g *sequentialTicketNumber) IsDateBased() bool { return true }
func (g *sequentialTicketNumber) Next(ctx context.Context, store ticketnumber.CounterStore) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.seq++
	return fmt.Sprintf("990000000%04d", g.seq), nil
}

type testCounterStore struct{}

func (testCounterStore) Add(ctx context.Context, dateScoped bool, offset int64) (int64, error) {
	return offset, nil
}

func seedLookupTables(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	exec := func(query string, args ...interface{}) error {
		_, execErr := tx.Exec(database.ConvertPlaceholders(query), args...)
		return execErr
	}

	if err := exec(`DELETE FROM system_address WHERE id = ? OR queue_id = ?`, 1, 1); err != nil {
		return err
	}
	if err := exec(`DELETE FROM queue WHERE id = ? OR name = ?`, 1, "Raw"); err != nil {
		return err
	}
	if err := exec(`DELETE FROM groups WHERE id = ? OR name = ?`, 1, "Raw"); err != nil {
		return err
	}
	if err := exec(`DELETE FROM salutation WHERE id = ? OR name = ?`, 1, "Standard"); err != nil {
		return err
	}
	if err := exec(`DELETE FROM signature WHERE id = ? OR name = ?`, 1, "Default"); err != nil {
		return err
	}
	if err := exec(`DELETE FROM follow_up_possible WHERE id = ? OR name = ?`, 1, "yes"); err != nil {
		return err
	}
	if err := exec(`DELETE FROM ticket_type WHERE id = ? OR name = ?`, 1, "Incident"); err != nil {
		return err
	}
	if err := exec(`DELETE FROM ticket_priority WHERE id IN (1,2,3,4,5)`); err != nil {
		return err
	}
	if err := exec(`DELETE FROM ticket_state WHERE id IN (1,2,3,4)`); err != nil {
		return err
	}
	if err := exec(`DELETE FROM ticket_state_type WHERE id IN (1,2,3,4,5)`); err != nil {
		return err
	}

	if err := exec(`
		INSERT INTO groups (id, name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 1, NOW(), 1, NOW(), 1)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			valid_id = EXCLUDED.valid_id
	`, 1, "Raw"); err != nil {
		return err
	}

	if err := exec(`
        INSERT INTO follow_up_possible (id, name, valid_id, create_time, create_by, change_time, change_by)
        VALUES (?, ?, 1, NOW(), 1, NOW(), 1)
        ON CONFLICT (id) DO UPDATE SET
            name = EXCLUDED.name,
            valid_id = EXCLUDED.valid_id
    `, 1, "yes"); err != nil {
		return err
	}

	if err := exec(`
        INSERT INTO salutation (id, name, text, content_type, comments, valid_id, create_time, create_by, change_time, change_by)
        VALUES (?, ?, ?, 'text/plain', ?, 1, NOW(), 1, NOW(), 1)
        ON CONFLICT (id) DO UPDATE SET
            name = EXCLUDED.name,
            text = EXCLUDED.text
    `, 1, "Standard", "Hello", "Test salutation"); err != nil {
		return err
	}

	if err := exec(`
        INSERT INTO signature (id, name, text, content_type, comments, valid_id, create_time, create_by, change_time, change_by)
        VALUES (?, ?, ?, 'text/plain', ?, 1, NOW(), 1, NOW(), 1)
        ON CONFLICT (id) DO UPDATE SET
            name = EXCLUDED.name,
            text = EXCLUDED.text
    `, 1, "Default", "--\nThanks", "Test signature"); err != nil {
		return err
	}

	if err := exec(`
        INSERT INTO queue (id, name, group_id, unlock_timeout, first_response_time, first_response_notify, update_time, update_notify,
                           solution_time, solution_notify, system_address_id, calendar_name, default_sign_key, salutation_id,
                           signature_id, follow_up_id, follow_up_lock, comments, valid_id, create_time, create_by, change_time, change_by)
        VALUES (?, ?, 1, 0, 0, 0, 0, 0, 0, 0, 1, NULL, NULL, 1, 1, 1, 1, ?, 1, NOW(), 1, NOW(), 1)
        ON CONFLICT (id) DO UPDATE SET
            name = EXCLUDED.name,
            group_id = EXCLUDED.group_id,
            valid_id = EXCLUDED.valid_id
    `, 1, "Raw", "Primary support queue"); err != nil {
		return err
	}

	if err := exec(`
        INSERT INTO system_address (id, value0, value1, value2, value3, queue_id, comments, valid_id, create_time, create_by, change_time, change_by)
        VALUES (?, ?, ?, NULL, NULL, ?, ?, 1, NOW(), 1, NOW(), 1)
        ON CONFLICT (id) DO UPDATE SET
            value0 = EXCLUDED.value0,
            value1 = EXCLUDED.value1,
            queue_id = EXCLUDED.queue_id
    `, 1, "support@example.com", "Support", 1, "Primary address"); err != nil {
		return err
	}

	stateTypes := []struct {
		id   int
		name string
	}{
		{1, "new"},
		{2, "open"},
		{3, "closed"},
		{4, "removed"},
		{5, "pending reminder"},
	}
	for _, st := range stateTypes {
		if err := exec(`
            INSERT INTO ticket_state_type (id, name, comments, create_time, create_by, change_time, change_by)
            VALUES (?, ?, ?, NOW(), 1, NOW(), 1)
            ON CONFLICT (id) DO UPDATE SET
                name = EXCLUDED.name
        `, st.id, st.name, st.name); err != nil {
			return err
		}
	}

	states := []struct {
		id     int
		name   string
		typeID int
	}{
		{1, "new", 1},
		{2, "closed", 3},
		{3, "pending reminder", 5},
		{4, "open", 2},
	}
	for _, s := range states {
		if err := exec(`
            INSERT INTO ticket_state (id, name, comments, type_id, valid_id, create_time, create_by, change_time, change_by)
            VALUES (?, ?, ?, ?, 1, NOW(), 1, NOW(), 1)
            ON CONFLICT (id) DO UPDATE SET
                name = EXCLUDED.name,
                type_id = EXCLUDED.type_id,
                valid_id = EXCLUDED.valid_id
        `, s.id, s.name, s.name, s.typeID); err != nil {
			return err
		}
	}

	priorities := []struct {
		id    int
		name  string
		color string
	}{
		{1, "1 very low", "#03c4f0"},
		{2, "2 low", "#83bfc8"},
		{3, "3 normal", "#cdcdcd"},
		{4, "4 high", "#ffaaaa"},
		{5, "5 very high", "#ff505e"},
	}
	for _, p := range priorities {
		if err := exec(`
			INSERT INTO ticket_priority (id, name, valid_id, color, create_time, create_by, change_time, change_by)
			VALUES (?, ?, 1, ?, NOW(), 1, NOW(), 1)
            ON CONFLICT (id) DO UPDATE SET
                name = EXCLUDED.name,
				valid_id = EXCLUDED.valid_id,
				color = EXCLUDED.color
		`, p.id, p.name, p.color); err != nil {
			return err
		}
	}

	if err := exec(`
        INSERT INTO ticket_type (id, name, valid_id, create_time, create_by, change_time, change_by)
        VALUES (?, ?, 1, NOW(), 1, NOW(), 1)
        ON CONFLICT (id) DO UPDATE SET
            name = EXCLUDED.name,
            valid_id = EXCLUDED.valid_id
    `, 1, "Incident"); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}
