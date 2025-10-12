package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// ArticleRepository handles database operations for articles
type ArticleRepository struct {
	db *sql.DB
	hasArticleTypeID *bool
	hasCommunicationChannelID *bool
}

// NewArticleRepository creates a new article repository
func NewArticleRepository(db *sql.DB) *ArticleRepository {
	return &ArticleRepository{db: db}
}

// Create creates a new article in the database (OTRS schema compatible)
func (r *ArticleRepository) Create(article *models.Article) error {
	now := time.Now()
	fmt.Printf("DEBUG: Creating article for ticket ID %d\n", article.TicketID)

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
	if article.CreateBy == 0 { article.CreateBy = 1 }
	if article.ChangeBy == 0 { article.ChangeBy = article.CreateBy }

	// Begin transaction
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

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

	// Build placeholders according to current DB
	placeholders := make([]string, len(cols))
	for i := range cols {
		if database.IsMySQL() { placeholders[i] = "?" } else { placeholders[i] = fmt.Sprintf("$%d", i+1) }
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
			$1, $2, $3, $4, $5, $6, $7, $8, $9
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
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
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
			nil, // content_id
			"",  // content_alternative
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
		SET change_time = $1, change_by = $2
		WHERE id = $3`)

	_, err = tx.Exec(updateTicketQuery, now, article.CreateBy, article.TicketID)
	if err != nil {
		fmt.Printf("ERROR: ticket change_time update failed for ticket %d: %v\n", article.TicketID, err)
		return err
	}

	// Commit transaction
	return tx.Commit()
}

// ensureArticleTypeColumn checks once whether article.article_type_id exists
func (r *ArticleRepository) ensureArticleTypeColumn() (bool, error) {
	if r.hasArticleTypeID != nil {
		return *r.hasArticleTypeID, nil
	}
	// Default false
	has := false
	// MySQL/MariaDB needs TABLE_SCHEMA = DATABASE()
	if database.IsMySQL() {
		row := r.db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'article' AND COLUMN_NAME = 'article_type_id'`)
		var cnt int
		if err := row.Scan(&cnt); err != nil {
			return false, err
		}
		has = cnt > 0
	} else {
		row := r.db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'article' AND column_name = 'article_type_id'`)
		var cnt int
		if err := row.Scan(&cnt); err != nil {
			return false, err
		}
		has = cnt > 0
	}
	r.hasArticleTypeID = &has
	return has, nil
}

// ensureCommunicationChannelColumn checks once whether article.communication_channel_id exists
func (r *ArticleRepository) ensureCommunicationChannelColumn() (bool, error) {
	if r.hasCommunicationChannelID != nil { return *r.hasCommunicationChannelID, nil }
	has := false
	if database.IsMySQL() {
		row := r.db.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'article' AND COLUMN_NAME = 'communication_channel_id'`)
		var cnt int
		if err := row.Scan(&cnt); err != nil { return false, err }
		has = cnt > 0
	} else {
		row := r.db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'article' AND column_name = 'communication_channel_id'`)
		var cnt int
		if err := row.Scan(&cnt); err != nil { return false, err }
		has = cnt > 0
	}
	r.hasCommunicationChannelID = &has
	return has, nil
}

// GetByID retrieves an article by its ID (joins MIME content)
func (r *ArticleRepository) GetByID(id uint) (*models.Article, error) {
	query := database.ConvertPlaceholders(`
		SELECT 
			a.id, a.ticket_id, a.article_sender_type_id,
			a.communication_channel_id, a.is_visible_for_customer,
			adm.a_subject, adm.a_body, adm.a_content_type, adm.content_path,
			a.create_time, a.create_by, a.change_time, a.change_by
		FROM article a
		LEFT JOIN article_data_mime adm ON a.id = adm.article_id
		WHERE a.id = $1`)

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

// GetHTMLBodyAttachmentID finds the HTML body attachment for an article (OTRS-style)
func (r *ArticleRepository) GetHTMLBodyAttachmentID(articleID uint) (*uint, error) {
	query := database.ConvertPlaceholders(`
		SELECT id FROM article_data_mime_attachment
		WHERE article_id = $1
		AND filename = 'html-body.html'
		AND content_type LIKE 'text/html%'
		AND disposition = 'inline'
		ORDER BY id LIMIT 1`)

	var attachmentID uint
	err := r.db.QueryRow(query, articleID).Scan(&attachmentID)
	if err == sql.ErrNoRows {
		return nil, nil // No HTML body attachment found
	}
	if err != nil {
		return nil, err
	}

	return &attachmentID, nil
}

// GetHTMLBodyContent retrieves the HTML body content for an article
func (r *ArticleRepository) GetHTMLBodyContent(articleID uint) (string, error) {
	query := database.ConvertPlaceholders(`
		SELECT content FROM article_data_mime_attachment
		WHERE article_id = $1
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

// GetByTicketID retrieves all articles for a specific ticket
func (r *ArticleRepository) GetByTicketID(ticketID uint, includeInternal bool) ([]models.Article, error) {
	fmt.Printf("DEBUG: GetByTicketID called with ticketID=%d, includeInternal=%v\n", ticketID, includeInternal)
	
	query := database.ConvertPlaceholders(`
		SELECT 
			a.id, a.ticket_id, a.article_sender_type_id,
			a.communication_channel_id, a.is_visible_for_customer,
			adm.a_subject, adm.a_body, adm.a_content_type,
			a.create_time, a.create_by, a.change_time, a.change_by
		FROM article a
		LEFT JOIN article_data_mime adm ON a.id = adm.article_id
		WHERE a.ticket_id = $1`)

	if !includeInternal {
		query += " AND a.is_visible_for_customer = 1"
	}

	query += " ORDER BY a.create_time ASC"

	fmt.Printf("DEBUG: Executing query: %s with ticketID=%d\n", query, ticketID)
	rows, err := r.db.Query(query, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []models.Article
	for rows.Next() {
		var article models.Article
		var subject, contentType sql.NullString
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
			fmt.Printf("DEBUG: Article ID %d has body: %q\n", article.ID, article.Body)
		} else {
			fmt.Printf("DEBUG: Article ID %d has nil bodyBytes\n", article.ID)
		}
		if contentType.Valid {
			article.MimeType = contentType.String
		}

		articles = append(articles, article)
	}

	fmt.Printf("DEBUG: Found %d articles for ticket %d\n", len(articles), ticketID)
	return articles, nil
}

// Update updates an article in the database
func (r *ArticleRepository) Update(article *models.Article) error {
	article.ChangeTime = time.Now()

	query := database.ConvertPlaceholders(`
		UPDATE article SET
			article_type_id = $2,
			article_sender_type_id = $3,
			communication_channel_id = $4,
			is_visible_for_customer = $5,
			subject = $6,
			body = $7,
			body_type = $8,
			charset = $9,
			mime_type = $10,
			content_path = $11,
			valid_id = $12,
			change_time = $13,
			change_by = $14
		WHERE id = $1`)

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
		SET change_time = $1, change_by = $2
		WHERE id = $3`)

	_, err = r.db.Exec(updateTicketQuery, time.Now(), article.ChangeBy, article.TicketID)

	return err
}

// Delete soft deletes an article by setting valid_id to 0
func (r *ArticleRepository) Delete(id uint, userID uint) error {
	// First get the ticket ID for updating change_time
	var ticketID uint
	getTicketQuery := database.ConvertPlaceholders(`SELECT ticket_id FROM article WHERE id = $1`)
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
		SET valid_id = 0, change_time = $2, change_by = $3
		WHERE id = $1`)

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
		SET change_time = $2, change_by = $3
		WHERE id = $1`)

	_, err = r.db.Exec(updateTicketQuery, ticketID, time.Now(), userID)

	return err
}

// GetVisibleArticlesForCustomer retrieves all customer-visible articles for a ticket
func (r *ArticleRepository) GetVisibleArticlesForCustomer(ticketID uint) ([]models.Article, error) {
	return r.GetByTicketID(ticketID, false)
}

// GetLatestArticleForTicket retrieves the most recent article for a ticket
func (r *ArticleRepository) GetLatestArticleForTicket(ticketID uint) (*models.Article, error) {
	query := database.ConvertPlaceholders(`
		SELECT 
			id, ticket_id, article_type_id, article_sender_type_id,
			communication_channel_id, is_visible_for_customer,
			subject, body, body_type, charset, mime_type,
			content_path, valid_id,
			create_time, create_by, change_time, change_by
		FROM article
		WHERE ticket_id = $1 AND valid_id = 1
		ORDER BY create_time DESC
		LIMIT 1`)

	var article models.Article
	err := r.db.QueryRow(query, ticketID).Scan(
		&article.ID,
		&article.TicketID,
		&article.ArticleTypeID,
		&article.SenderTypeID,
		&article.CommunicationChannelID,
		&article.IsVisibleForCustomer,
		&article.Subject,
		&article.Body,
		&article.BodyType,
		&article.Charset,
		&article.MimeType,
		&article.ContentPath,
		&article.ValidID,
		&article.CreateTime,
		&article.CreateBy,
		&article.ChangeTime,
		&article.ChangeBy,
	)

	if err == sql.ErrNoRows {
		return nil, nil // No articles yet
	}

	return &article, err
}

// CountArticlesForTicket counts the number of articles for a ticket
func (r *ArticleRepository) CountArticlesForTicket(ticketID uint, includeInternal bool) (int, error) {
	query := database.ConvertPlaceholders(`
		SELECT COUNT(*) 
		FROM article 
		WHERE ticket_id = $1 AND valid_id = 1`)

	if !includeInternal {
		query += " AND is_visible_for_customer = 1"
	}

	var count int
	err := r.db.QueryRow(query, ticketID).Scan(&count)
	return count, err
}

// CreateAttachment creates a new attachment for an article
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
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
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

// GetAttachmentsByArticleID retrieves all attachments for an article
func (r *ArticleRepository) GetAttachmentsByArticleID(articleID uint) ([]models.Attachment, error) {
	query := database.ConvertPlaceholders(`
		SELECT 
			id, article_id, filename, content_type, content_size,
			content_id, content_alternative, disposition, content,
			create_time, create_by, change_time, change_by
		FROM article_attachments
		WHERE article_id = $1
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

	return attachments, nil
}

// GetAttachmentByID retrieves a specific attachment
func (r *ArticleRepository) GetAttachmentByID(id uint) (*models.Attachment, error) {
	query := database.ConvertPlaceholders(`
		SELECT 
			id, article_id, filename, content_type, content_size,
			content_id, content_alternative, disposition, content,
			create_time, create_by, change_time, change_by
		FROM article_attachments
		WHERE id = $1`)

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

// DeleteAttachment removes an attachment
func (r *ArticleRepository) DeleteAttachment(id uint) error {
	query := database.ConvertPlaceholders(`DELETE FROM article_attachments WHERE id = $1`)
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

// GetArticleWithAttachments retrieves an article with all its attachments
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
