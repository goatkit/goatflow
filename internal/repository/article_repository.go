// Package repository provides data access repositories for domain entities.
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"mime"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/core"
	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/constants"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/storage"
)

// ArticleRepository handles database operations for articles.
type ArticleRepository struct {
	db *sql.DB
}

// NewArticleRepository creates a new article repository.
func NewArticleRepository(db *sql.DB) *ArticleRepository {
	return &ArticleRepository{db: db}
}

// Create creates a new article in the database (OTRS schema compatible).
func (r *ArticleRepository) Create(article *models.Article) error {
	now := time.Now()

	// Set defaults
	if article.ArticleTypeID == 0 {
		article.ArticleTypeID = constants.ArticleTypeEmailExternal
	}
	if article.SenderTypeID == 0 {
		article.SenderTypeID = constants.ArticleSenderCustomer
	}
	// The article type is not stored; it only selects the channel when the
	// caller did not pick one. is_visible_for_customer is taken as given: 0 is
	// a meaningful value (internal article), never a "not set" marker.
	if article.CommunicationChannelID == 0 {
		article.CommunicationChannelID = core.MapCommunicationChannel(article.ArticleTypeID)
	}
	if article.CreateBy == 0 {
		article.CreateBy = 1
	}
	if article.ChangeBy == 0 {
		article.ChangeBy = article.CreateBy
	}

	// Begin transaction
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	articleQuery := database.ConvertPlaceholders(`
		INSERT INTO article (
			ticket_id, article_sender_type_id, communication_channel_id, is_visible_for_customer,
			create_time, create_by, change_time, change_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`)
	args := []any{
		article.TicketID, article.SenderTypeID, article.CommunicationChannelID, article.IsVisibleForCustomer,
		now, article.CreateBy, now, article.ChangeBy,
	}

	// Use adapter for database-specific handling
	adapter := database.GetAdapter()
	var articleID64 int64
	articleID64, err = adapter.InsertWithReturningTx(tx, articleQuery, args...)

	if err != nil {
		return err
	}

	article.ID = int(articleID64)

	// Insert into article_data_mime table
	mimeQuery := database.ConvertPlaceholders(`
		INSERT INTO article_data_mime (
			article_id, a_subject, a_body, a_content_type, content_path,
			incoming_time, create_time, create_by, change_time, change_by
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)`)

	// Normalize body to string for MySQL TEXT column compatibility
	var bodyStr string
	if str, ok := article.Body.(string); ok {
		bodyStr = str
	} else if bytes, ok := article.Body.([]byte); ok {
		bodyStr = string(bytes)
	} else if article.Body != nil {
		bodyStr = fmt.Sprintf("%v", article.Body)
	}

	contentType := "text/plain; charset=utf-8"
	if article.MimeType != "" {
		contentType = article.MimeType
		if article.Charset != "" {
			contentType += "; charset=" + article.Charset
		}
	}

	// Handle HTML content securely like OTRS - store HTML in attachment
	if strings.Contains(contentType, "text/html") && bodyStr != "" {
		_, err = storage.ForDB(r.db).WithTx(tx).WriteAttachment(context.Background(), articleID64, storage.NewAttachment{
			Filename:    storage.HTMLBodyFilename,
			ContentType: "text/html; charset=utf-8",
			Disposition: "inline",
			Content:     []byte(bodyStr),
			CreateBy:    article.CreateBy,
		})
		if err != nil {
			return fmt.Errorf("failed to create HTML body attachment: %w", err)
		}

		// For HTML content, store a placeholder in the main body
		bodyStr = "[HTML content - see attachment]"
		contentType = "text/plain; charset=utf-8"
	}

	_, err = tx.Exec(
		mimeQuery,
		articleID64,
		article.Subject,
		bodyStr,
		contentType,
		storage.ContentPath(now),
		int(now.Unix()),
		now,
		article.CreateBy,
		now,
		article.ChangeBy,
	)

	if err != nil {
		// Surface the DB error to logs to aid debugging
		fmt.Printf("ERROR: article_data_mime insert failed for ticket %d, article %d: %v\n", article.TicketID, articleID64, err)
		return err
	}

	// Update ticket's change_time when an article is added
	// Use left-to-right placeholders so MySQL '?' binding matches arg order
	updateTicketQuery := database.ConvertPlaceholders(`
		UPDATE ticket
		SET change_time = ?, change_by = ?
		WHERE id = ?`)

	_, err = tx.Exec(updateTicketQuery, now, article.CreateBy, article.TicketID)
	if err != nil {
		fmt.Printf("ERROR: ticket change_time update failed for ticket %d: %v\n", article.TicketID, err)
		return err
	}

	// Commit transaction
	return tx.Commit()
}

func deriveBodyMeta(contentType string) (string, string) {
	bodyType := "text/plain"
	charset := "utf-8"
	if contentType == "" {
		return bodyType, charset
	}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err == nil && mediaType != "" {
		bodyType = mediaType
	}
	if params != nil {
		if v, ok := params["charset"]; ok && v != "" {
			charset = v
		}
	}
	return bodyType, charset
}

// GetByID retrieves an article by its ID (joins MIME content).
func (r *ArticleRepository) GetByID(id uint) (*models.Article, error) {
	query := database.ConvertPlaceholders(`
		SELECT
			a.id, a.ticket_id, a.article_sender_type_id,
			a.communication_channel_id, a.is_visible_for_customer,
			adm.a_subject, adm.a_body, adm.a_content_type, adm.content_path,
			a.create_time, a.create_by, a.change_time, a.change_by
		FROM article a
		LEFT JOIN article_data_mime adm ON a.id = adm.article_id
		WHERE a.id = ?`)

	var article models.Article
	var subject sql.NullString
	var bodyBytes []byte
	var contentType sql.NullString
	var contentPath sql.NullString

	err := r.db.QueryRow(query, id).Scan(
		&article.ID,
		&article.TicketID,
		&article.SenderTypeID,
		&article.CommunicationChannelID,
		&article.IsVisibleForCustomer,
		&subject,
		&bodyBytes,
		&contentType,
		&contentPath,
		&article.CreateTime,
		&article.CreateBy,
		&article.ChangeTime,
		&article.ChangeBy,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("article not found")
	}
	article.ArticleTypeID = core.ArticleTypeFromStorage(article.CommunicationChannelID, article.IsVisibleForCustomer == 1)

	if subject.Valid {
		article.Subject = subject.String
	}
	if bodyBytes != nil {
		article.Body = string(bodyBytes)
	}
	if contentType.Valid {
		article.MimeType = contentType.String
	}
	if contentPath.Valid {
		cp := contentPath.String
		article.ContentPath = &cp
	}

	return &article, err
}

// GetHTMLBodyContent returns the article's HTML body (its last HTML body part,
// see storage.IsHTMLBody), or "" when it has none.
func (r *ArticleRepository) GetHTMLBodyContent(articleID uint) (string, error) {
	ctx := context.Background()
	store := storage.ForDB(r.db)
	atts, err := store.ListAttachments(ctx, int64(articleID))
	if err != nil {
		return "", err
	}
	for i := len(atts) - 1; i >= 0; i-- {
		if !storage.IsHTMLBody(atts[i]) {
			continue
		}
		_, content, err := store.GetAttachment(ctx, int64(articleID), atts[i].FileID)
		if err != nil {
			return "", err
		}
		return string(content), nil
	}
	return "", nil
}

// GetByTicketID retrieves all articles for a specific ticket.
func (r *ArticleRepository) GetByTicketID(ticketID uint, includeInternal bool) ([]models.Article, error) {
	query := database.ConvertPlaceholders(`
		SELECT
			a.id, a.ticket_id, a.article_sender_type_id,
			a.communication_channel_id, a.is_visible_for_customer,
			adm.a_subject, adm.a_body, adm.a_content_type,
			adm.a_message_id, adm.a_in_reply_to, adm.a_references,
			a.create_time, a.create_by, a.change_time, a.change_by
		FROM article a
		LEFT JOIN article_data_mime adm ON a.id = adm.article_id
		WHERE a.ticket_id = ?`)

	if !includeInternal {
		query += " AND a.is_visible_for_customer = 1"
	}

	query += " ORDER BY a.create_time ASC, a.id ASC"

	rows, err := r.db.Query(query, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []models.Article
	for rows.Next() {
		var article models.Article
		var subject, contentType, messageID, inReplyTo, references sql.NullString
		var bodyBytes []byte

		err := rows.Scan(
			&article.ID,
			&article.TicketID,
			&article.SenderTypeID,
			&article.CommunicationChannelID,
			&article.IsVisibleForCustomer,
			&subject,
			&bodyBytes,
			&contentType,
			&messageID,
			&inReplyTo,
			&references,
			&article.CreateTime,
			&article.CreateBy,
			&article.ChangeTime,
			&article.ChangeBy,
		)
		if err != nil {
			return nil, err
		}

		// Set the subject and body from the joined data
		if subject.Valid {
			article.Subject = subject.String
		}
		if bodyBytes != nil {
			article.Body = string(bodyBytes)
		}
		if contentType.Valid {
			article.MimeType = contentType.String
		}
		article.ArticleTypeID = core.ArticleTypeFromStorage(article.CommunicationChannelID, article.IsVisibleForCustomer == 1)
		if messageID.Valid {
			article.MessageID = messageID.String
		}
		if inReplyTo.Valid {
			article.InReplyTo = inReplyTo.String
		}
		if references.Valid {
			article.References = references.String
		}

		articles = append(articles, article)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return articles, nil
}

// GetLatestArticleForTicket retrieves the most recent article for a ticket.
func (r *ArticleRepository) GetLatestArticleForTicket(ticketID uint) (*models.Article, error) {
	return scanLatestArticle(r.db.QueryRow(database.ConvertPlaceholders(latestArticleColumns+`
		WHERE a.ticket_id = ?
		ORDER BY a.create_time DESC, a.id DESC
		LIMIT 1`), ticketID))
}

// GetLatestCustomerArticleForTicket gets the most recent customer article for a ticket.
func (r *ArticleRepository) GetLatestCustomerArticleForTicket(ticketID uint) (*models.Article, error) {
	return scanLatestArticle(r.db.QueryRow(database.ConvertPlaceholders(latestArticleColumns+`
		WHERE a.ticket_id = ? AND a.article_sender_type_id = ?
		ORDER BY a.create_time DESC, a.id DESC
		LIMIT 1`), ticketID, constants.ArticleSenderCustomer))
}

const latestArticleColumns = `
	SELECT
		a.id, a.ticket_id, a.article_sender_type_id,
		a.communication_channel_id, a.is_visible_for_customer,
		adm.a_subject, adm.a_body, adm.a_content_type, adm.content_path,
		adm.a_message_id, adm.a_in_reply_to, adm.a_references,
		a.create_time, a.create_by, a.change_time, a.change_by
	FROM article a
	LEFT JOIN article_data_mime adm ON a.id = adm.article_id`

// scanLatestArticle reads a latestArticleColumns row; (nil, nil) when no row.
func scanLatestArticle(row *sql.Row) (*models.Article, error) {
	var article models.Article
	var subject, body, contentType, contentPath, messageID, inReplyTo, references sql.NullString
	err := row.Scan(
		&article.ID,
		&article.TicketID,
		&article.SenderTypeID,
		&article.CommunicationChannelID,
		&article.IsVisibleForCustomer,
		&subject,
		&body,
		&contentType,
		&contentPath,
		&messageID,
		&inReplyTo,
		&references,
		&article.CreateTime,
		&article.CreateBy,
		&article.ChangeTime,
		&article.ChangeBy,
	)
	if err == sql.ErrNoRows {
		return nil, nil //nolint:nilnil // No matching article yet
	}
	if err != nil {
		return nil, err
	}

	article.ArticleTypeID = core.ArticleTypeFromStorage(article.CommunicationChannelID, article.IsVisibleForCustomer == 1)
	article.Subject = subject.String
	article.Body = body.String
	article.MimeType = contentType.String
	if contentPath.Valid {
		cp := contentPath.String
		article.ContentPath = &cp
	}
	article.MessageID = messageID.String
	article.InReplyTo = inReplyTo.String
	article.References = references.String
	article.BodyType, article.Charset = deriveBodyMeta(article.MimeType)
	return &article, nil
}

// FindTicketByMessageID resolves the ticket owning the provided Message-ID header.
func (r *ArticleRepository) FindTicketByMessageID(ctx context.Context, messageID string) (*models.Ticket, error) {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return nil, nil //nolint:nilnil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	query := database.ConvertPlaceholders(`
		SELECT t.id, t.tn, t.queue_id
		FROM article_data_mime adm
		INNER JOIN article a ON a.id = adm.article_id
		INNER JOIN ticket t ON t.id = a.ticket_id
		WHERE adm.a_message_id = ?
		ORDER BY a.create_time DESC, a.id DESC
		LIMIT 1`)
	var (
		id           int
		ticketNumber string
		queueID      int
	)
	err := r.db.QueryRowContext(ctx, query, messageID).Scan(&id, &ticketNumber, &queueID)
	if err == sql.ErrNoRows {
		return nil, nil //nolint:nilnil
	}
	if err != nil {
		return nil, err
	}
	return &models.Ticket{ID: id, TicketNumber: ticketNumber, QueueID: queueID}, nil
}

// GetSenderTypeColors returns a map of sender_type_id to hex color.
// Colors are looked up by matching article_sender_type.name to article_color.name.
func (r *ArticleRepository) GetSenderTypeColors() (map[int]string, error) {
	query := database.ConvertPlaceholders(`
		SELECT ast.id, COALESCE(ac.color, '') as color
		FROM article_sender_type ast
		LEFT JOIN article_color ac ON LOWER(ast.name) = LOWER(ac.name)
		WHERE ast.valid_id = 1`)

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	colors := make(map[int]string)
	for rows.Next() {
		var id int
		var color string
		if err := rows.Scan(&id, &color); err != nil {
			return nil, err
		}
		if color != "" {
			colors[id] = color
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return colors, nil
}
