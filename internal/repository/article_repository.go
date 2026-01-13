// Package repository provides data access repositories for domain entities.
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"mime"
	"strings"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// ArticleRepository handles database operations for articles.
type ArticleRepository struct {
	db                        *sql.DB
	hasArticleTypeID          *bool
	hasCommunicationChannelID *bool
	articleColumnCache        map[string]bool
}

func (r *ArticleRepository) articleColumnExpressions() (string, string, error) {
	articleTypeExpr := "article_type_id"
	commChannelExpr := "communication_channel_id"

	hasType, err := r.ensureArticleTypeColumn()
	if err != nil {
		return "", "", err
	}
	if !hasType {
		articleTypeExpr = "0"
	}

	hasComm, err := r.ensureCommunicationChannelColumn()
	if err != nil {
		return "", "", err
	}
	if !hasComm {
		commChannelExpr = "0"
	}

	return articleTypeExpr, commChannelExpr, nil
}

func (r *ArticleRepository) hasArticleColumn(column string) (bool, error) {
	if r.articleColumnCache == nil {
		r.articleColumnCache = make(map[string]bool)
	}

	if val, ok := r.articleColumnCache[column]; ok {
		return val, nil
	}

	var cnt int
	if database.IsMySQL() {
		row := r.db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'article' AND COLUMN_NAME = ?`, column)
		if err := row.Scan(&cnt); err != nil {
			return false, err
		}
	} else {
		query := database.ConvertPlaceholders(`
			SELECT COUNT(*) FROM information_schema.columns
			WHERE table_name = 'article' AND column_name = ?`)
		row := r.db.QueryRow(query, column)
		if err := row.Scan(&cnt); err != nil {
			return false, err
		}
	}

	has := cnt > 0
	r.articleColumnCache[column] = has
	return has, nil
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
		article.ArticleTypeID = 1 // external email default
	}
	if article.SenderTypeID == 0 {
		article.SenderTypeID = 3 // Customer
	}
	if article.CommunicationChannelID == 0 {
		article.CommunicationChannelID = 1 // Email
	}
	if article.IsVisibleForCustomer == 0 {
		article.IsVisibleForCustomer = 1 // Visible by default
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

	// Determine if schema has article_type_id; build INSERT accordingly
	hasType, derr := r.ensureArticleTypeColumn()
	if derr != nil {
		return derr
	}
	// Determine if schema has communication_channel_id
	hasComm, derr2 := r.ensureCommunicationChannelColumn()
	if derr2 != nil {
		return derr2
	}

	// Build column list and args dynamically to fit actual schema
	cols := []string{"ticket_id"}
	args := []interface{}{article.TicketID}
	if hasType {
		cols = append(cols, "article_type_id")
		args = append(args, article.ArticleTypeID)
	}
	cols = append(cols, "article_sender_type_id")
	args = append(args, article.SenderTypeID)
	if hasComm {
		cols = append(cols, "communication_channel_id")
		args = append(args, article.CommunicationChannelID)
	}
	cols = append(cols, "is_visible_for_customer", "create_time", "create_by", "change_time", "change_by")
	args = append(args, article.IsVisibleForCustomer, now, article.CreateBy, now, article.ChangeBy)

	// Build placeholders - use ? and let ConvertPlaceholders handle DB-specific conversion
	placeholders := make([]string, len(cols))
	for i := range cols {
		placeholders[i] = "?"
	}
	articleQuery := fmt.Sprintf("INSERT INTO article (%s) VALUES (%s) RETURNING id", strings.Join(cols, ", "), strings.Join(placeholders, ", "))
	articleQuery = database.ConvertPlaceholders(articleQuery)

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
			article_id, a_subject, a_body, a_content_type,
			incoming_time, create_time, create_by, change_time, change_by
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?
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
		// Create HTML body attachment
		attachmentQuery := database.ConvertPlaceholders(`
			INSERT INTO article_data_mime_attachment (
				article_id, filename, content_type, content_size, content,
				content_id, content_alternative, disposition,
				create_time, create_by, change_time, change_by
			) VALUES (
				?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
			) RETURNING id`)

		contentSize := len(bodyStr)
		var attachmentID64 int64
		err = tx.QueryRow(
			attachmentQuery,
			articleID64,
			"html-body.html", // Special filename for HTML body like OTRS
			"text/html; charset=utf-8",
			contentSize,
			bodyStr,
			nil,      // content_id
			"",       // content_alternative
			"inline", // disposition - inline for HTML body
			now,
			article.CreateBy,
			now,
			article.ChangeBy,
		).Scan(&attachmentID64)

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

// ensureArticleTypeColumn checks once whether article.article_type_id exists.
func (r *ArticleRepository) ensureArticleTypeColumn() (bool, error) {
	if r.hasArticleTypeID != nil {
		return *r.hasArticleTypeID, nil
	}
	has, err := r.hasArticleColumn("article_type_id")
	if err != nil {
		return false, err
	}
	r.hasArticleTypeID = &has
	return has, nil
}

// ensureCommunicationChannelColumn checks once whether article.communication_channel_id exists.
func (r *ArticleRepository) ensureCommunicationChannelColumn() (bool, error) {
	if r.hasCommunicationChannelID != nil {
		return *r.hasCommunicationChannelID, nil
	}
	has, err := r.hasArticleColumn("communication_channel_id")
	if err != nil {
		return false, err
	}
	r.hasCommunicationChannelID = &has
	return has, nil
}

func (r *ArticleRepository) articleValidExpressions(alias string) (string, string, error) {
	hasValid, err := r.hasArticleColumn("valid_id")
	if err != nil {
		return "", "", err
	}
	if hasValid {
		return fmt.Sprintf("COALESCE(%s.valid_id, 1)", alias), fmt.Sprintf("%s.valid_id = 1", alias), nil
	}
	return "1", "", nil
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

// GetHTMLBodyAttachmentID finds the HTML body attachment for an article (OTRS-style).
func (r *ArticleRepository) GetHTMLBodyAttachmentID(articleID uint) (*uint, error) {
	query := database.ConvertPlaceholders(`
		SELECT id FROM article_data_mime_attachment
		WHERE article_id = ?
		AND filename = 'html-body.html'
		AND content_type LIKE 'text/html%'
		AND disposition = 'inline'
		ORDER BY id LIMIT 1`)

	var attachmentID uint
	err := r.db.QueryRow(query, articleID).Scan(&attachmentID)
	if err == sql.ErrNoRows {
		return nil, nil //nolint:nilnil // No HTML body attachment found
	}
	if err != nil {
		return nil, err
	}

	return &attachmentID, nil
}

// GetHTMLBodyContent retrieves the HTML body content for an article.
func (r *ArticleRepository) GetHTMLBodyContent(articleID uint) (string, error) {
	query := database.ConvertPlaceholders(`
		SELECT content FROM article_data_mime_attachment
		WHERE article_id = ?
		AND content_type LIKE 'text/html%'
		ORDER BY id DESC LIMIT 1`)

	var content []byte
	err := r.db.QueryRow(query, articleID).Scan(&content)
	if err == sql.ErrNoRows {
		return "", nil // No HTML body attachment found
	}
	if err != nil {
		return "", err
	}

	return string(content), nil
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

// Update updates an article in the database.
func (r *ArticleRepository) Update(article *models.Article) error {
	article.ChangeTime = time.Now()

	query := database.ConvertPlaceholders(`
		UPDATE article SET
			article_type_id = ?,
			article_sender_type_id = ?,
			communication_channel_id = ?,
			is_visible_for_customer = ?,
			subject = ?,
			body = ?,
			body_type = ?,
			charset = ?,
			mime_type = ?,
			content_path = ?,
			valid_id = ?,
			change_time = ?,
			change_by = ?
		WHERE id = ?`)

	result, err := r.db.Exec(
		query,
		article.ID,
		article.ArticleTypeID,
		article.SenderTypeID,
		article.CommunicationChannelID,
		article.IsVisibleForCustomer,
		article.Subject,
		article.Body,
		article.BodyType,
		article.Charset,
		article.MimeType,
		article.ContentPath,
		article.ValidID,
		article.ChangeTime,
		article.ChangeBy,
	)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("article not found")
	}

	// Update ticket's change_time when an article is updated
	// Use left-to-right placeholders so MySQL '?' binding matches arg order
	updateTicketQuery := database.ConvertPlaceholders(`
		UPDATE ticket
		SET change_time = ?, change_by = ?
		WHERE id = ?`)

	_, err = r.db.Exec(updateTicketQuery, time.Now(), article.ChangeBy, article.TicketID)

	return err
}

// Delete soft deletes an article by setting valid_id to 0.
func (r *ArticleRepository) Delete(id uint, userID uint) error {
	// First get the ticket ID for updating change_time
	var ticketID uint
	getTicketQuery := database.ConvertPlaceholders(`SELECT ticket_id FROM article WHERE id = ?`)
	err := r.db.QueryRow(getTicketQuery, id).Scan(&ticketID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("article not found")
		}
		return err
	}

	// Soft delete the article
	query := database.ConvertPlaceholders(`
		UPDATE article
		SET valid_id = 0, change_time = ?, change_by = ?
		WHERE id = ?`)

	result, err := r.db.Exec(query, id, time.Now(), userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("article not found")
	}

	// Update ticket's change_time
	updateTicketQuery := database.ConvertPlaceholders(`
		UPDATE ticket
		SET change_time = ?, change_by = ?
		WHERE id = ?`)

	_, err = r.db.Exec(updateTicketQuery, ticketID, time.Now(), userID)

	return err
}

// GetVisibleArticlesForCustomer retrieves all customer-visible articles for a ticket.
func (r *ArticleRepository) GetVisibleArticlesForCustomer(ticketID uint) ([]models.Article, error) {
	return r.GetByTicketID(ticketID, false)
}

// GetLatestArticleForTicket retrieves the most recent article for a ticket.
func (r *ArticleRepository) GetLatestArticleForTicket(ticketID uint) (*models.Article, error) {
	articleTypeExpr, commChannelExpr, err := r.articleColumnExpressions()
	if err != nil {
		return nil, err
	}
	validSelect, validPredicate, err := r.articleValidExpressions("a")
	if err != nil {
		return nil, err
	}
	selectValid := fmt.Sprintf("%s AS valid_id", validSelect)
	whereParts := []string{"a.ticket_id = ?"}
	if validPredicate != "" {
		whereParts = append(whereParts, validPredicate)
	}
	//nolint:gosec // articleTypeExpr, commChannelExpr, selectValid are from schema detection, not user input
	query := fmt.Sprintf(`
		SELECT
			a.id, a.ticket_id, %s AS article_type_id, a.article_sender_type_id,
			%s AS communication_channel_id, a.is_visible_for_customer,
			COALESCE(adm.a_subject, '') AS subject,
			COALESCE(adm.a_body, '') AS body,
			COALESCE(adm.a_content_type, '') AS content_type,
			adm.content_path,
			%s,
			a.create_time, a.create_by, a.change_time, a.change_by
		FROM article a
		LEFT JOIN article_data_mime adm ON a.id = adm.article_id
		WHERE %s
		ORDER BY a.create_time DESC, a.id DESC
		LIMIT 1`, articleTypeExpr, commChannelExpr, selectValid, strings.Join(whereParts, " AND "))
	query = database.ConvertPlaceholders(query)

	var article models.Article
	var subject, body, contentType sql.NullString
	var contentPath sql.NullString
	err = r.db.QueryRow(query, ticketID).Scan(
		&article.ID,
		&article.TicketID,
		&article.ArticleTypeID,
		&article.SenderTypeID,
		&article.CommunicationChannelID,
		&article.IsVisibleForCustomer,
		&subject,
		&body,
		&contentType,
		&contentPath,
		&article.ValidID,
		&article.CreateTime,
		&article.CreateBy,
		&article.ChangeTime,
		&article.ChangeBy,
	)

	if err == sql.ErrNoRows {
		return nil, nil //nolint:nilnil // No articles yet
	}
	if err != nil {
		return nil, err
	}

	if subject.Valid {
		article.Subject = subject.String
	}
	if body.Valid {
		article.Body = body.String
	}
	if contentType.Valid {
		article.MimeType = contentType.String
	}
	if contentPath.Valid {
		cp := contentPath.String
		article.ContentPath = &cp
	}
	bodyType, charset := deriveBodyMeta(article.MimeType)
	article.BodyType = bodyType
	article.Charset = charset
	return &article, nil
}

// GetLatestCustomerArticleForTicket gets the most recent customer article for a ticket.
func (r *ArticleRepository) GetLatestCustomerArticleForTicket(ticketID uint) (*models.Article, error) {
	articleTypeExpr, commChannelExpr, err := r.articleColumnExpressions()
	if err != nil {
		return nil, err
	}
	validSelect, validPredicate, err := r.articleValidExpressions("a")
	if err != nil {
		return nil, err
	}
	selectValid := fmt.Sprintf("%s AS valid_id", validSelect)
	whereParts := []string{"a.ticket_id = ?", "a.article_sender_type_id = 3"}
	if validPredicate != "" {
		whereParts = append(whereParts, validPredicate)
	}
	//nolint:gosec // articleTypeExpr, commChannelExpr, selectValid are from schema detection, not user input
	query := fmt.Sprintf(`
		SELECT
			a.id, a.ticket_id, %s AS article_type_id, a.article_sender_type_id,
			%s AS communication_channel_id, a.is_visible_for_customer,
			COALESCE(adm.a_subject, '') AS subject,
			COALESCE(adm.a_body, '') AS body,
			COALESCE(adm.a_content_type, '') AS content_type,
			adm.content_path,
			adm.a_message_id,
			adm.a_in_reply_to,
			adm.a_references,
			%s,
			a.create_time, a.create_by, a.change_time, a.change_by
		FROM article a
		LEFT JOIN article_data_mime adm ON a.id = adm.article_id
		WHERE %s
		ORDER BY a.create_time DESC, a.id DESC
		LIMIT 1`, articleTypeExpr, commChannelExpr, selectValid, strings.Join(whereParts, " AND "))
	query = database.ConvertPlaceholders(query)

	var article models.Article
	var subject, body, contentType sql.NullString
	var contentPath, messageID, inReplyTo, references sql.NullString
	err = r.db.QueryRow(query, ticketID).Scan(
		&article.ID,
		&article.TicketID,
		&article.ArticleTypeID,
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
		&article.ValidID,
		&article.CreateTime,
		&article.CreateBy,
		&article.ChangeTime,
		&article.ChangeBy,
	)

	if err == sql.ErrNoRows {
		return nil, nil //nolint:nilnil // No customer articles yet
	}
	if err != nil {
		return nil, err
	}

	if subject.Valid {
		article.Subject = subject.String
	}
	if body.Valid {
		article.Body = body.String
	}
	if contentType.Valid {
		article.MimeType = contentType.String
	}
	if contentPath.Valid {
		cp := contentPath.String
		article.ContentPath = &cp
	}
	if messageID.Valid {
		article.MessageID = messageID.String
	}
	if inReplyTo.Valid {
		article.InReplyTo = inReplyTo.String
	}
	if references.Valid {
		article.References = references.String
	}
	bodyType, charset := deriveBodyMeta(article.MimeType)
	article.BodyType = bodyType
	article.Charset = charset

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

// CountArticlesForTicket counts the number of articles for a ticket.
func (r *ArticleRepository) CountArticlesForTicket(ticketID uint, includeInternal bool) (int, error) {
	query := database.ConvertPlaceholders(`
		SELECT COUNT(*)
		FROM article
		WHERE ticket_id = ? AND valid_id = 1`)

	if !includeInternal {
		query += " AND is_visible_for_customer = 1"
	}

	var count int
	err := r.db.QueryRow(query, ticketID).Scan(&count)
	return count, err
}

// CreateAttachment creates a new attachment for an article.
func (r *ArticleRepository) CreateAttachment(attachment *models.Attachment) error {
	attachment.CreateTime = time.Now()
	attachment.ChangeTime = time.Now()

	if attachment.Disposition == "" {
		attachment.Disposition = "attachment"
	}

	query := database.ConvertPlaceholders(`
		INSERT INTO article_attachments (
			article_id, filename, content_type, content_size,
			content_id, content_alternative, disposition, content,
			create_time, create_by, change_time, change_by
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		) RETURNING id`)

	err := r.db.QueryRow(
		query,
		attachment.ArticleID,
		attachment.Filename,
		attachment.ContentType,
		attachment.ContentSize,
		attachment.ContentID,
		attachment.ContentAlternative,
		attachment.Disposition,
		attachment.Content,
		attachment.CreateTime,
		attachment.CreateBy,
		attachment.ChangeTime,
		attachment.ChangeBy,
	).Scan(&attachment.ID)

	return err
}

// GetAttachmentsByArticleID retrieves all attachments for an article.
func (r *ArticleRepository) GetAttachmentsByArticleID(articleID uint) ([]models.Attachment, error) {
	query := database.ConvertPlaceholders(`
		SELECT
			id, article_id, filename, content_type, content_size,
			content_id, content_alternative, disposition, content,
			create_time, create_by, change_time, change_by
		FROM article_attachments
		WHERE article_id = ?
		ORDER BY create_time ASC`)

	rows, err := r.db.Query(query, articleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attachments []models.Attachment
	for rows.Next() {
		var attachment models.Attachment
		err := rows.Scan(
			&attachment.ID,
			&attachment.ArticleID,
			&attachment.Filename,
			&attachment.ContentType,
			&attachment.ContentSize,
			&attachment.ContentID,
			&attachment.ContentAlternative,
			&attachment.Disposition,
			&attachment.Content,
			&attachment.CreateTime,
			&attachment.CreateBy,
			&attachment.ChangeTime,
			&attachment.ChangeBy,
		)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, attachment)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return attachments, nil
}

// GetAttachmentByID retrieves a specific attachment.
func (r *ArticleRepository) GetAttachmentByID(id uint) (*models.Attachment, error) {
	query := database.ConvertPlaceholders(`
		SELECT
			id, article_id, filename, content_type, content_size,
			content_id, content_alternative, disposition, content,
			create_time, create_by, change_time, change_by
		FROM article_attachments
		WHERE id = ?`)

	var attachment models.Attachment
	err := r.db.QueryRow(query, id).Scan(
		&attachment.ID,
		&attachment.ArticleID,
		&attachment.Filename,
		&attachment.ContentType,
		&attachment.ContentSize,
		&attachment.ContentID,
		&attachment.ContentAlternative,
		&attachment.Disposition,
		&attachment.Content,
		&attachment.CreateTime,
		&attachment.CreateBy,
		&attachment.ChangeTime,
		&attachment.ChangeBy,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("attachment not found")
	}

	return &attachment, err
}

// DeleteAttachment removes an attachment.
func (r *ArticleRepository) DeleteAttachment(id uint) error {
	query := database.ConvertPlaceholders(`DELETE FROM article_attachments WHERE id = ?`)
	result, err := r.db.Exec(query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("attachment not found")
	}

	return nil
}

// GetArticleWithAttachments retrieves an article with all its attachments.
func (r *ArticleRepository) GetArticleWithAttachments(id uint) (*models.Article, error) {
	article, err := r.GetByID(id)
	if err != nil {
		return nil, err
	}

	attachments, err := r.GetAttachmentsByArticleID(id)
	if err != nil {
		return nil, err
	}

	article.Attachments = attachments
	return article, nil
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
