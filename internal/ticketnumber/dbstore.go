package ticketnumber

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// DBStore keeps exactly one ticket_number_counter row per counter_uid and
// increments it atomically: the row is created on first use, then the
// increment and the read-back happen in one transaction under the row lock,
// so concurrent callers always see distinct values on every driver.
//
// dateScoped controls daily UID suffix (YYYYMMDD) for date-based generators.
type DBStore struct {
	db       *sql.DB
	systemID string
	clock    func() time.Time
}

func NewDBStore(db *sql.DB, systemID string) *DBStore {
	return &DBStore{db: db, systemID: systemID, clock: time.Now}
}

// Add implements CounterStore. offset currently only supports 1. Larger offsets map to repeated increments.
func (s *DBStore) Add(ctx context.Context, dateScoped bool, offset int64) (int64, error) {
	if offset < 1 {
		return 0, errors.New("bad offset")
	}
	uid := s.systemID
	if dateScoped {
		now := s.clock().UTC()
		uid = fmt.Sprintf("%s_%04d%02d%02d", s.systemID, now.Year(), int(now.Month()), now.Day())
	}

	// Create the counter row on first use; a concurrent creator's row is kept.
	if _, err := s.db.ExecContext(ctx, database.ConvertUpsert(`
		INSERT INTO ticket_number_counter (counter, counter_uid, create_time)
		VALUES (0, ?, NOW())
		ON DUPLICATE KEY UPDATE counter = ticket_number_counter.counter`, "counter_uid"), uid); err != nil {
		return 0, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, database.ConvertPlaceholders(
		`UPDATE ticket_number_counter SET counter = counter + ? WHERE counter_uid = ?`), offset, uid); err != nil {
		return 0, err
	}
	var c int64
	if err := tx.QueryRowContext(ctx, database.ConvertPlaceholders(
		`SELECT counter FROM ticket_number_counter WHERE counter_uid = ?`), uid).Scan(&c); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return c, nil
}
