package api

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/gotrs-io/gotrs-ce/internal/config"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/history"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
)

// saveTimeEntry persists a time accounting entry if inputs are valid.
func saveTimeEntry(db *sql.DB, ticketID int, articleID *int, minutes int, userID int) error {
	if db == nil || minutes <= 0 || ticketID <= 0 {
		return nil
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

// isTimeUnitsRequired checks configuration (sysconfig or static config) for mandatory time entry on notes.
func isTimeUnitsRequired(db *sql.DB) bool {
	if required, ok := sysconfigBool(db, "Ticket::Frontend::AgentTicketNote###RequiredTimeUnits"); ok {
		return required
	}
	if cfg := config.Get(); cfg != nil {
		return cfg.Ticket.Frontend.AgentTicketNote.RequiredTimeUnits
	}
	return false
}

func sysconfigBool(db *sql.DB, name string) (bool, bool) {
	value, ok := sysconfigValue(db, name)
	if !ok {
		return false, false
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false, true
	}
	if parsed, err := strconv.ParseBool(trimmed); err == nil {
		return parsed, true
	}
	trimmed = strings.Trim(trimmed, "\"'")
	if parsed, err := strconv.ParseBool(trimmed); err == nil {
		return parsed, true
	}
	return false, true
}

func sysconfigValue(db *sql.DB, name string) (string, bool) {
	if db == nil || strings.TrimSpace(name) == "" {
		return "", false
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
	if err != nil && err != sql.ErrNoRows {
		return "", false
	}
	if err == nil && value.Valid {
		return value.String, true
	}

	query = database.ConvertPlaceholders(`
        SELECT effective_value
        FROM sysconfig_default
        WHERE name = ?
    `)
	err = db.QueryRow(query, name).Scan(&value)
	if err != nil || !value.Valid {
		return "", false
	}
	return value.String, true
}
