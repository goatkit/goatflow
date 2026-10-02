package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/goatkit/goatflow/internal/history"
	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/config"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/repository"
)

// saveTimeEntry persists a time accounting entry if inputs are valid.
// Nothing to record (no minutes / no ticket) is not an error; a missing
// database is, so callers report the entry as not saved.
func saveTimeEntry(db *sql.DB, ticketID int, articleID *int, minutes int, userID int) error {
	if minutes <= 0 || ticketID <= 0 {
		return nil
	}
	if db == nil {
		return errors.New("time accounting: database unavailable")
	}
	taRepo := repository.NewTimeAccountingRepository(db)
	_, err := taRepo.Create(&models.TimeAccounting{
		TicketID:  ticketID,
		ArticleID: articleID,
		TimeUnit:  minutes,
		CreateBy:  userID,
		ChangeBy:  userID,
	})
	if err != nil {
		return err
	}

	ticketRepo := repository.NewTicketRepository(db)
	recorder := history.NewRecorder(ticketRepo)
	unit := "minutes"
	if minutes == 1 {
		unit = "minute"
	}
	message := fmt.Sprintf("Logged %d %s", minutes, unit)
	if recErr := recorder.RecordByTicketID(
		context.Background(), nil, ticketID, articleID, history.TypeTimeAccounting, message, userID); recErr != nil {
		log.Printf("time accounting history insert failed: %v", recErr)
	}

	return nil
}

// isTimeUnitsRequired reports whether notes must carry a time entry. A sysconfig
// value (modified, then default) wins; with no sysconfig row the static config
// default applies. A missing database or failed lookup is returned as an error.
func isTimeUnitsRequired(db *sql.DB) (bool, error) {
	required, ok, err := sysconfigBool(db, "Ticket::Frontend::AgentTicketNote###RequiredTimeUnits")
	if err != nil {
		return false, err
	}
	if ok {
		return required, nil
	}
	if cfg := config.Get(); cfg != nil {
		return cfg.Ticket.Frontend.AgentTicketNote.RequiredTimeUnits, nil
	}
	return false, nil
}

// sysconfigBool reads a boolean sysconfig setting. ok is false when no row
// holds a value; a stored value that is not a boolean is an error.
func sysconfigBool(db *sql.DB, name string) (value bool, ok bool, err error) {
	raw, ok, err := sysconfigValue(db, name)
	if err != nil || !ok {
		return false, false, err
	}
	trimmed := strings.Trim(strings.TrimSpace(raw), "\"'")
	parsed, perr := strconv.ParseBool(trimmed)
	if perr != nil {
		return false, false, fmt.Errorf("sysconfig %q: value %q is not a boolean", name, raw)
	}
	return parsed, true, nil
}

// sysconfigValue returns the effective value of a sysconfig setting, preferring
// the newest valid sysconfig_modified row over sysconfig_default. ok is false
// when neither table holds a value for name; a missing database or a failed
// query is returned as an error.
func sysconfigValue(db *sql.DB, name string) (string, bool, error) {
	if strings.TrimSpace(name) == "" {
		return "", false, errors.New("sysconfig: empty setting name")
	}
	if db == nil {
		return "", false, fmt.Errorf("sysconfig %q: database unavailable", name)
	}
	var value sql.NullString

	// Prefer modified values.
	query := database.ConvertPlaceholders(`
        SELECT effective_value
        FROM sysconfig_modified
        WHERE name = ? AND is_valid = 1
        ORDER BY change_time DESC
        LIMIT 1
    `)
	err := db.QueryRow(query, name).Scan(&value)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", false, fmt.Errorf("sysconfig %q: modified lookup: %w", name, err)
	}
	if err == nil && value.Valid {
		return value.String, true, nil
	}

	query = database.ConvertPlaceholders(`
        SELECT effective_value
        FROM sysconfig_default
        WHERE name = ?
    `)
	err = db.QueryRow(query, name).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("sysconfig %q: default lookup: %w", name, err)
	}
	if !value.Valid {
		return "", false, nil
	}
	return value.String, true, nil
}
