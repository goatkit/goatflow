package repository

import (
    "context"
    "regexp"
    "testing"
    "time"

    "github.com/DATA-DOG/go-sqlmock"
)

func TestFindDuePendingReminders(t *testing.T) {
    db, mock, err := sqlmock.New()
    if err != nil {
        t.Fatalf("sqlmock setup failed: %v", err)
    }
    defer db.Close()

    repo := NewTicketRepository(db)

    now := time.Date(2025, 10, 16, 20, 0, 0, 0, time.UTC)
    until := now.Add(-30 * time.Minute)

    rows := sqlmock.NewRows([]string{
        "id", "tn", "title", "queue_id", "queue_name", "responsible_user_id", "user_id", "until_time", "state_name",
    }).
        AddRow(321, "202510161000008", "VPN outage", 5, "Support", 99, 42, until.Unix(), "pending reminder")

    mock.ExpectQuery(regexp.QuoteMeta(`SELECT t.id, t.tn, t.title, t.queue_id, COALESCE(q.name, '') AS queue_name,
       t.responsible_user_id, t.user_id, t.until_time, ts.name AS state_name
FROM ticket t
JOIN ticket_state ts ON ts.id = t.ticket_state_id
LEFT JOIN queue q ON q.id = t.queue_id
WHERE ts.type_id = 4
  AND t.until_time > 0
  AND t.until_time <= $1
  AND t.archive_flag = 0
ORDER BY t.until_time ASC
LIMIT $2`)).
        WithArgs(now.Unix(), 25).
        WillReturnRows(rows)

    reminders, err := repo.FindDuePendingReminders(context.Background(), now, 25)
    if err != nil {
        t.Fatalf("FindDuePendingReminders failed: %v", err)
    }

    if len(reminders) != 1 {
        t.Fatalf("expected 1 reminder, got %d", len(reminders))
    }

    got := reminders[0]
    if got.TicketID != 321 {
        t.Fatalf("unexpected ticket id %d", got.TicketID)
    }
    if got.TicketNumber != "202510161000008" {
        t.Fatalf("unexpected ticket number %s", got.TicketNumber)
    }
    if got.QueueName != "Support" {
        t.Fatalf("unexpected queue name %s", got.QueueName)
    }
    if got.PendingUntil.Unix() != until.Unix() {
        t.Fatalf("unexpected pending until %v", got.PendingUntil)
    }
    if got.StateName != "pending reminder" {
        t.Fatalf("unexpected state %s", got.StateName)
    }
    if got.ResponsibleUserID == nil || *got.ResponsibleUserID != 99 {
        t.Fatalf("unexpected responsible id")
    }
    if got.OwnerUserID == nil || *got.OwnerUserID != 42 {
        t.Fatalf("unexpected owner id")
    }

    if err := mock.ExpectationsWereMet(); err != nil {
        t.Fatalf("unmet expectations: %v", err)
    }
}

func TestFindDuePendingRemindersDefaultsLimit(t *testing.T) {
    db, mock, err := sqlmock.New()
    if err != nil {
        t.Fatalf("sqlmock setup failed: %v", err)
    }
    defer db.Close()

    repo := NewTicketRepository(db)

    now := time.Now().UTC()

    mock.ExpectQuery(regexp.QuoteMeta(`SELECT t.id, t.tn, t.title, t.queue_id, COALESCE(q.name, '') AS queue_name,
       t.responsible_user_id, t.user_id, t.until_time, ts.name AS state_name
FROM ticket t
JOIN ticket_state ts ON ts.id = t.ticket_state_id
LEFT JOIN queue q ON q.id = t.queue_id
WHERE ts.type_id = 4
  AND t.until_time > 0
  AND t.until_time <= $1
  AND t.archive_flag = 0
ORDER BY t.until_time ASC
LIMIT $2`)).
        WithArgs(now.Unix(), 50).
        WillReturnRows(sqlmock.NewRows([]string{
            "id", "tn", "title", "queue_id", "queue_name", "responsible_user_id", "user_id", "until_time", "state_name",
        }))

    if _, err := repo.FindDuePendingReminders(context.Background(), now, 0); err != nil {
        t.Fatalf("FindDuePendingReminders failed: %v", err)
    }

    if err := mock.ExpectationsWereMet(); err != nil {
        t.Fatalf("unmet expectations: %v", err)
    }
}
