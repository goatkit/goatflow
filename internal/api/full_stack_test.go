package api

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// TestDatabaseIntegrity verifies our database operations maintain referential integrity.
func TestDatabaseIntegrity(t *testing.T) {
	if err := database.InitTestDB(); err != nil {
		t.Skip("Database not available")
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skip("Database not available")
	}

	t.Run("Verify foreign key constraints work", func(t *testing.T) {
		// Try to create an article for non-existent ticket
		_, err := db.Exec(database.ConvertPlaceholders(`
            INSERT INTO article (ticket_id, article_sender_type_id, communication_channel_id, is_visible_for_customer, create_time, create_by, change_time, change_by)
            VALUES (999999, 1, 1, 1, NOW(), 1, NOW(), 1)
        `))

		assert.Error(t, err, "Should fail due to foreign key constraint")
		// Different engines produce different messages; just assert error
	})

	t.Run("Verify ticket with articles cannot be deleted", func(t *testing.T) {
		// article.ticket_id references ticket.id without ON DELETE CASCADE on
		// both drivers, so deleting a ticket that still has articles must fail.
		ticketID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
			INSERT INTO ticket (
				tn, title, queue_id, ticket_state_id, ticket_priority_id, ticket_lock_id,
				user_id, responsible_user_id, timeout, until_time,
				escalation_time, escalation_update_time, escalation_response_time, escalation_solution_time,
				create_time, create_by, change_time, change_by, customer_user_id
			)
			VALUES (?, 'Cascade Test', 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, NOW(), 1, NOW(), 1, 'test@example.com')
			RETURNING id
		`), fmt.Sprintf("FKTEST%d", time.Now().UnixNano()))
		require.NoError(t, err)

		articleID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
            INSERT INTO article (ticket_id, article_sender_type_id, communication_channel_id, is_visible_for_customer, search_index_needs_rebuild, create_time, create_by, change_time, change_by)
            VALUES (?, 1, 1, 1, 1, NOW(), 1, NOW(), 1)
            RETURNING id
        `), ticketID)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM article WHERE id = ?`), articleID)
			_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM ticket WHERE id = ?`), ticketID)
		})

		_, err = db.Exec(database.ConvertPlaceholders(`DELETE FROM ticket WHERE id = ?`), ticketID)
		assert.Error(t, err, "Deleting a ticket that still has articles should violate the foreign key")

		var count int
		err = db.QueryRow(database.ConvertPlaceholders(`SELECT COUNT(*) FROM article WHERE ticket_id = ?`), ticketID).Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 1, count, "Article should still exist")
	})
}

// TestCleanupOldTestData ensures we don't accumulate test data over time.
func TestCleanupOldTestData(t *testing.T) {
	if err := database.InitTestDB(); err != nil {
		t.Skip("Database not available")
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skip("Database not available")
	}

	// Clean up test tickets older than 1 hour
	result, err := db.Exec(database.ConvertPlaceholders(`
		DELETE FROM ticket 
		WHERE (title LIKE 'Test%' OR title LIKE 'Full Stack Test%' OR title LIKE 'Concurrent Test%')
        AND create_time < DATE_SUB(NOW(), INTERVAL 1 HOUR)
    `))

	if err == nil {
		affected, _ := result.RowsAffected()
		t.Logf("Cleaned up %d old test tickets", affected)
	}

	// Clean up test storage files
	// This would clean up old test files from ./internal/api/storage/tickets/
	// Implementation depends on your storage structure
}
