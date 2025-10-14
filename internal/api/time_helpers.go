package api

import (
    "database/sql"

    "github.com/gotrs-io/gotrs-ce/internal/models"
    "github.com/gotrs-io/gotrs-ce/internal/repository"
)

// saveTimeEntry persists a time accounting entry if inputs are valid.
func saveTimeEntry(db *sql.DB, ticketID int, articleID *int, minutes int, userID int) error {
    if db == nil || minutes <= 0 || ticketID <= 0 { return nil }
    taRepo := repository.NewTimeAccountingRepository(db)
    _, err := taRepo.Create(&models.TimeAccounting{
        TicketID:  ticketID,
        ArticleID: articleID,
        TimeUnit:  minutes,
        CreateBy:  userID,
        ChangeBy:  userID,
    })
    return err
}
