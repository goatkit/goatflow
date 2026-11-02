package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/deletion"
	"github.com/goatkit/goatflow/internal/storage"
)

// recycleBinTicket creates a ticket with the rows a real one carries (an
// article with MIME data, an attachment, the raw email, history, time
// accounting and a seen flag), and returns its id and article id.
func recycleBinTicket(t *testing.T, db *sql.DB, tn string) (ticketID, articleID int64) {
	t.Helper()
	ctx := context.Background()
	now := time.Now()
	ticketID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO ticket (tn, title, queue_id, ticket_lock_id, type_id, user_id, responsible_user_id,
			ticket_priority_id, ticket_state_id, customer_id, customer_user_id, timeout, until_time,
			escalation_time, escalation_update_time, escalation_response_time, escalation_solution_time,
			archive_flag, create_time, create_by, change_time, change_by)
		VALUES (?, 'Recycle bin purge test', 1, 1, 1, 1, 1, 3, 1, 'rb-co', 'rb-customer', 0, 0, 0, 0, 0, 0, 1,
			?, 1, ?, 1) RETURNING id`), tn, now, now)
	require.NoError(t, err)
	articleID, err = database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO article (ticket_id, article_sender_type_id, communication_channel_id, is_visible_for_customer,
			search_index_needs_rebuild, create_time, create_by, change_time, change_by)
		VALUES (?, 1, 1, 1, 1, ?, 1, ?, 1) RETURNING id`), ticketID, now, now)
	require.NoError(t, err)
	exec := func(query string, args ...any) {
		t.Helper()
		_, err := db.Exec(database.ConvertPlaceholders(query), args...)
		require.NoError(t, err)
	}
	exec(`INSERT INTO article_data_mime (article_id, a_from, a_subject, a_body, a_content_type, content_path,
			incoming_time, create_time, create_by, change_time, change_by)
		VALUES (?, 'customer@example.com', 'help', 'body', 'text/plain', ?, ?, ?, 1, ?, 1)`,
		articleID, storage.ContentPath(now), now.Unix(), now, now)
	store := storage.ForDB(db)
	_, err = store.WriteAttachment(ctx, articleID, storage.NewAttachment{
		Filename: "notes.txt", ContentType: "text/plain", Disposition: "attachment", Content: []byte("x"), CreateBy: 1,
	})
	require.NoError(t, err)
	require.NoError(t, store.WritePlain(ctx, articleID, []byte("From: customer@example.com\r\n\r\nbody"), 1))
	exec(`INSERT INTO ticket_history (name, history_type_id, ticket_id, article_id, type_id, queue_id, owner_id,
			priority_id, state_id, create_time, create_by, change_time, change_by)
		SELECT 'created', id, ?, ?, 1, 1, 1, 3, 1, ?, 1, ?, 1 FROM ticket_history_type WHERE name = 'NewTicket'`,
		ticketID, articleID, now, now)
	exec(`INSERT INTO time_accounting (ticket_id, article_id, time_unit, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 5, ?, 1, ?, 1)`, ticketID, articleID, now, now)
	exec(`INSERT INTO ticket_flag (ticket_id, ticket_key, ticket_value, create_time, create_by)
		VALUES (?, 'Seen', '1', ?, 1)`, ticketID, now)

	t.Cleanup(func() {
		for _, stmt := range []string{
			`DELETE FROM ticket_flag WHERE ticket_id = ?`,
			`DELETE FROM time_accounting WHERE ticket_id = ?`,
			`DELETE FROM ticket_history WHERE ticket_id = ?`,
		} {
			_, _ = db.Exec(database.ConvertPlaceholders(stmt), ticketID)
		}
		_ = store.DeleteArticle(ctx, articleID)
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM article_data_mime WHERE article_id = ?`), articleID)
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM article WHERE ticket_id = ?`), ticketID)
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM ticket WHERE id = ?`), ticketID)
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM gk_recycle_bin WHERE entity_type = ? AND entity_id = ?`),
			deletion.EntityTicket, ticketID)
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM gk_deletion_log WHERE entity_type = ? AND entity_id = ?`),
			deletion.EntityTicket, ticketID)
	})
	return ticketID, articleID
}

// The retention job permanently deletes a ticket whose recycle bin entry has
// expired, with everything that references it, and leaves one that has not.
func TestRecycleBinPurgeDeletesExpiredTickets(t *testing.T) {
	db := lockTestDB(t)
	deletion.SetArticleContentDeleter(storage.DeleteArticleContent)
	t.Cleanup(func() { deletion.SetArticleContentDeleter(nil) })
	ctx := context.Background()
	stamp := time.Now().UnixNano()
	expiredID, expiredArticle := recycleBinTicket(t, db, fmt.Sprintf("rbx%d", stamp))
	keptID, _ := recycleBinTicket(t, db, fmt.Sprintf("rbk%d", stamp))

	repo := deletion.NewRepositoryWithDB(db)
	for id, expires := range map[int64]time.Time{
		expiredID: time.Now().Add(-time.Hour),
		keptID:    time.Now().Add(24 * time.Hour),
	} {
		_, err := repo.AddToRecycleBin(&deletion.RecycleBinEntry{
			EntityType: deletion.EntityTicket, EntityID: id, DeletedBy: 1,
			DeletedAt: time.Now().AddDate(0, 0, -60), ExpiresAt: &expires,
		})
		require.NoError(t, err)
	}

	svc := NewService(db)
	require.NoError(t, svc.handleRecycleBinPurge(ctx, nil))

	count := func(query string, arg int64) int {
		t.Helper()
		var n int
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(query), arg).Scan(&n))
		return n
	}
	for _, q := range []string{
		`SELECT COUNT(*) FROM ticket WHERE id = ?`,
		`SELECT COUNT(*) FROM article WHERE ticket_id = ?`,
		`SELECT COUNT(*) FROM ticket_history WHERE ticket_id = ?`,
		`SELECT COUNT(*) FROM time_accounting WHERE ticket_id = ?`,
		`SELECT COUNT(*) FROM ticket_flag WHERE ticket_id = ?`,
	} {
		require.Zero(t, count(q, expiredID), q)
		require.Equal(t, 1, count(q, keptID), q)
	}
	for _, q := range []string{
		`SELECT COUNT(*) FROM article_data_mime WHERE article_id = ?`,
		`SELECT COUNT(*) FROM article_data_mime_attachment WHERE article_id = ?`,
		`SELECT COUNT(*) FROM article_data_mime_plain WHERE article_id = ?`,
	} {
		require.Zero(t, count(q, expiredArticle), q)
	}

	bin := `SELECT COUNT(*) FROM gk_recycle_bin WHERE entity_type = 'ticket' AND entity_id = ?`
	require.Zero(t, count(bin, expiredID))
	require.Equal(t, 1, count(bin, keptID))
	require.Equal(t, 1, count(`SELECT COUNT(*) FROM gk_deletion_log
		WHERE entity_type = 'ticket' AND action = 'hard_delete' AND entity_id = ?`, expiredID))
}
