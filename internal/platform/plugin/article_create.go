package plugin

import (
	"context"
	"fmt"
	"strings"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// sanitizeArticleText strips runes outside the Basic Multilingual Plane
// (>U+FFFF), which require 4 bytes in UTF-8 and cannot be stored in the
// utf8mb3 article_data_mime columns. Emoji are the common case; all BMP text
// is preserved verbatim. This mirrors the schema constraint so plugin-supplied
// bodies never fail the insert.
func sanitizeArticleText(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r <= 0xFFFF {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// CreateArticle creates an article (note/transcript/deliverable) on a ticket
// with the platform's OTRS invariants: an article row (agent sender, internal
// channel, is_visible_for_customer per visibleToCustomer) plus its
// article_data_mime row (subject/body, text/plain, utf8mb3-safe). The article
// and its mime data are committed atomically. createdBy must be a valid
// users.id (the acting user). This lets plugins create articles through the
// platform (article + mime + status/visibility invariants) instead of
// hand-writing article rows via raw SQL.
func (h *ProdHostAPI) CreateArticle(ctx context.Context, ticketID, createdBy int64, subject, body string, visibleToCustomer bool) (int64, error) {
	db, err := h.getDB("")
	if err != nil {
		return 0, err
	}
	if ticketID <= 0 {
		return 0, fmt.Errorf("invalid ticket_id %d", ticketID)
	}
	if createdBy <= 0 {
		return 0, fmt.Errorf("invalid created_by %d", createdBy)
	}

	var exists int
	if err := db.QueryRowContext(ctx, database.ConvertPlaceholders(
		`SELECT 1 FROM ticket WHERE id = ?`), ticketID).Scan(&exists); err != nil {
		return 0, fmt.Errorf("ticket %d not found: %w", ticketID, err)
	}

	visible := 0
	if visibleToCustomer {
		visible = 1
	}
	subject = sanitizeArticleText(subject)
	body = sanitizeArticleText(body)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Do NOT call ConvertPlaceholders here — GetAdapter's InsertWithReturningTx
	// expands repeated ? placeholders (create_by/change_by) per driver.
	id, err := database.GetAdapter().InsertWithReturningTx(tx, `
		INSERT INTO article (ticket_id, article_sender_type_id, communication_channel_id,
			is_visible_for_customer, create_time, create_by, change_time, change_by)
		VALUES (?, 1, 3, ?, CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP, ?)
		RETURNING id`,
		ticketID, visible, createdBy, createdBy)
	if err != nil {
		return 0, fmt.Errorf("insert article: %w", err)
	}

	if _, err := database.GetAdapter().ExecTx(tx, `
		INSERT INTO article_data_mime (article_id, a_from, a_subject, a_body, a_content_type,
			incoming_time, create_time, create_by, change_time, change_by)
		VALUES (?, '', ?, ?, 'text/plain', 0, CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP, ?)`,
		id, subject, body, createdBy, createdBy); err != nil {
		return 0, fmt.Errorf("insert article mime data: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit article: %w", err)
	}
	return id, nil
}
