package ticketnumber

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// We maintain exactly one row per counter_uid and atomically increment using dialect specific UPSERT:
//
//	Postgres: INSERT ... ON CONFLICT(counter_uid) DO UPDATE SET counter = ticket_number_counter.counter + EXCLUDED.counter RETURNING counter
//	MySQL: INSERT ... ON DUPLICATE KEY UPDATE counter = LAST_INSERT_ID(counter + VALUES(counter)); SELECT LAST_INSERT_ID()
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
	// Dialect specific atomic increment
	if database.IsPostgreSQL() {
		// Use upsert with RETURNING to atomically add offset
		q := `INSERT INTO ticket_number_counter (counter, counter_uid, create_time)
              VALUES ($2, $1, NOW())
              ON CONFLICT (counter_uid) DO UPDATE SET counter = ticket_number_counter.counter + EXCLUDED.counter
              RETURNING counter`
		var c int64
		if err := s.db.QueryRowContext(ctx, q, uid, offset).Scan(&c); err != nil {
			return 0, err
		}
		return c, nil
	}
	if database.IsMySQL() {
		// Use MySQL LAST_INSERT_ID trick and read it from Exec result to stay on the same session/connection.
		// This avoids relying on a subsequent SELECT LAST_INSERT_ID() that might hit a different pooled connection.
		q := `INSERT INTO ticket_number_counter (counter, counter_uid, create_time)
              VALUES (?, ?, NOW())
              ON DUPLICATE KEY UPDATE counter = LAST_INSERT_ID(counter + VALUES(counter))`
		res, err := s.db.ExecContext(ctx, q, offset, uid)
		if err != nil {
			return 0, err
		}
		c, err := res.LastInsertId()
		if err != nil {
			return 0, err
		}
		return c, nil
	}
	// Generic fallback (rare path): emulate with transaction + SELECT FOR UPDATE
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	var current int64
	row := tx.QueryRowContext(ctx, database.ConvertPlaceholders(`SELECT counter FROM ticket_number_counter WHERE counter_uid = ? FOR UPDATE`), uid)
	scanErr := row.Scan(&current)
	switch scanErr {
	case nil:
		newVal := current + offset
		if _, err = tx.ExecContext(ctx, database.ConvertPlaceholders(`UPDATE ticket_number_counter SET counter=? WHERE counter_uid=?`), newVal, uid); err != nil {
			return 0, err
		}
		if err = tx.Commit(); err != nil {
			return 0, err
		}
		return newVal, nil
	case sql.ErrNoRows:
		if _, err = tx.ExecContext(ctx, database.ConvertPlaceholders(`INSERT INTO ticket_number_counter (counter, counter_uid, create_time) VALUES (?, ?, NOW())`), offset, uid); err != nil {
			return 0, err
		}
		if err = tx.Commit(); err != nil {
			return 0, err
		}
		return offset, nil
	default:
		return 0, scanErr
	}
}
