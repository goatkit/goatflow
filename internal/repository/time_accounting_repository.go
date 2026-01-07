package repository

import (
	"database/sql"
	"math"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
)

type TimeAccountingRepository struct{ db *sql.DB }

func NewTimeAccountingRepository(db *sql.DB) *TimeAccountingRepository {
	return &TimeAccountingRepository{db: db}
}

// Create inserts a time accounting row and returns its id.
func (r *TimeAccountingRepository) Create(entry *models.TimeAccounting) (int, error) {
	entry.CreateTime = time.Now()
	entry.ChangeTime = entry.CreateTime
	query := database.ConvertPlaceholders(`
        INSERT INTO time_accounting (ticket_id, article_id, time_unit, create_time, create_by, change_time, change_by)
        VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id
    `)
	adapter := database.GetAdapter()
	// Store as decimal minutes (keep compatibility with DECIMAL(10,2))
	id, err := adapter.InsertWithReturning(r.db, query,
		entry.TicketID, entry.ArticleID, float64(entry.TimeUnit),
		entry.CreateTime, entry.CreateBy, entry.ChangeTime, entry.ChangeBy,
	)
	if err != nil {
		return 0, err
	}
	entry.ID = int(id)
	return entry.ID, nil
}

// ListByTicket fetches time accounting entries for a ticket ordered by create_time.
func (r *TimeAccountingRepository) ListByTicket(ticketID int) ([]models.TimeAccounting, error) {
	query := database.ConvertPlaceholders(`
        SELECT id, ticket_id, article_id, time_unit, create_time, create_by, change_time, change_by
        FROM time_accounting WHERE ticket_id = ? ORDER BY create_time ASC
    `)
	rows, err := r.db.Query(query, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.TimeAccounting
	for rows.Next() {
		var it models.TimeAccounting
		var tuFloat float64
		if err := rows.Scan(&it.ID, &it.TicketID, &it.ArticleID, &tuFloat, &it.CreateTime, &it.CreateBy, &it.ChangeTime, &it.ChangeBy); err != nil {
			return nil, err
		}
		// DB stores DECIMAL(10,2); convert to integer minutes by rounding
		it.TimeUnit = int(math.Round(tuFloat))
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
