package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/ticketutil"
)

// FindDuePendingReminders returns tickets in pending reminder states whose deadline has passed.
// This includes tickets with no pending date set (legacy/migrated data) - these are treated
// as due immediately to ensure they get processed.
func (r *TicketRepository) FindDuePendingReminders(ctx context.Context, now time.Time, limit int) ([]*models.PendingReminder, error) {
	if limit <= 0 {
		limit = 50
	}

	// Query includes:
	// 1. Tickets with a set deadline that has passed (until_time > 0 AND until_time <= now)
	// 2. Tickets with no deadline set (until_time = 0) - these are legacy/migrated data
	query := `SELECT t.id, t.tn, t.title, t.queue_id, COALESCE(q.name, '') AS queue_name,
       t.responsible_user_id, t.user_id, t.until_time, ts.name AS state_name
FROM ticket t
JOIN ticket_state ts ON ts.id = t.ticket_state_id
LEFT JOIN queue q ON q.id = t.queue_id
WHERE ts.type_id = 4
  AND ((t.until_time > 0 AND t.until_time <= ?) OR t.until_time = 0)
  AND t.archive_flag = 0
ORDER BY CASE WHEN t.until_time = 0 THEN 0 ELSE t.until_time END ASC
LIMIT ?`

	rows, err := r.db.QueryContext(ctx, database.ConvertPlaceholders(query), now.Unix(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reminders := make([]*models.PendingReminder, 0)
	for rows.Next() {
		var (
			ticketID     int
			ticketNumber string
			title        string
			queueID      int
			queueName    string
			responsible  sql.NullInt64
			owner        sql.NullInt64
			until        int64
			stateName    string
		)

		if err := rows.Scan(&ticketID, &ticketNumber, &title, &queueID, &queueName, &responsible, &owner, &until, &stateName); err != nil {
			return nil, err
		}

		reminder := &models.PendingReminder{
			TicketID:     ticketID,
			TicketNumber: ticketNumber,
			Title:        title,
			QueueID:      queueID,
			QueueName:    queueName,
			PendingUntil: ticketutil.GetEffectivePendingTime(int(until)),
			StateName:    stateName,
		}
		if responsible.Valid {
			v := int(responsible.Int64)
			reminder.ResponsibleUserID = &v
		}
		if owner.Valid {
			v := int(owner.Int64)
			reminder.OwnerUserID = &v
		}
		reminders = append(reminders, reminder)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return reminders, nil
}
