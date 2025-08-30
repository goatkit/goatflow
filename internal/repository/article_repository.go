package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// ArticleRepository handles database operations for articles
type ArticleRepository struct {
	db *sql.DB
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
	if article.SenderTypeID == 0 {
		article.SenderTypeID = 3 // Customer
	}
	if article.CommunicationChannelID == 0 {
		article.CommunicationChannelID = 1 // Email
	}
	if article.IsVisibleForCustomer == 0 {
		article.IsVisibleForCustomer = 1 // Visible by default
	}
	
	// Begin transaction
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	
	// Insert into article table
	articleQuery := database.ConvertPlaceholders(`
		INSERT INTO article (
			ticket_id, article_sender_type_id,
			communication_channel_id, is_visible_for_customer,
			create_time, create_by, change_time, change_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		) RETURNING id`)
	
	// Use adapter for database-specific handling
	adapter := database.GetAdapter()
	articleID64, err := adapter.InsertWithReturningTx(
		tx,
		articleQuery,
		article.TicketID,
		article.SenderTypeID,
		article.CommunicationChannelID,
		article.IsVisibleForCustomer,
		now,
		article.CreateBy,
		now,
		article.ChangeBy,
	)
	
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
	
	// Convert body to bytea
	var bodyBytes []byte
	if str, ok := article.Body.(string); ok {
		bodyBytes = []byte(str)
	} else if bytes, ok := article.Body.([]byte); ok {
		bodyBytes = bytes
	} else {
		bodyBytes = []byte(fmt.Sprintf("%v", article.Body))
	}
	
	contentType := "text/plain; charset=utf-8"
	if article.MimeType != "" {
		contentType = article.MimeType
		if article.Charset != "" {
			contentType += "; charset=" + article.Charset
		}
	}
	
	_, err = tx.Exec(
		mimeQuery,
		articleID64,
		article.Subject,
		bodyBytes,
		contentType,
		int(now.Unix()),
		now,
		article.CreateBy,
		now,
		article.ChangeBy,
	)
	
	if err != nil {
		return err
	}
	
	// Update ticket's change_time when an article is added
	updateTicketQuery := database.ConvertPlaceholders(`
		UPDATE ticket 
		SET change_time = $2, change_by = $3
		WHERE id = $1`)
	
	_, err = tx.Exec(updateTicketQuery, article.TicketID, now, article.CreateBy)
	if err != nil {
		return err
	}
	
	// Commit transaction
	return tx.Commit()
}

// GetByID retrieves an article by its ID
func (r *ArticleRepository) GetByID(id uint) (*models.Article, error) {
	query := database.ConvertPlaceholders(`
		SELECT 
			id, ticket_id, article_type_id, sender_type_id,
			communication_channel_id, is_visible_for_customer,
			subject, body, body_type, charset, mime_type,
			content_path, valid_id,
			create_time, create_by, change_time, change_by
		FROM article
		WHERE id = $1`)
	
	var article models.Article
	err := r.db.QueryRow(query, id).Scan(
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
		return nil, fmt.Errorf("article not found")
	}
	
	return &article, err
}

// GetByTicketID retrieves all articles for a specific ticket
func (r *ArticleRepository) GetByTicketID(ticketID uint, includeInternal bool) ([]models.Article, error) {
	query := database.ConvertPlaceholders(`
		SELECT 
			id, ticket_id, article_type_id, sender_type_id,
			communication_channel_id, is_visible_for_customer,
			subject, body, body_type, charset, mime_type,
			content_path, valid_id,
			create_time, create_by, change_time, change_by
		FROM article
		WHERE ticket_id = $1 AND valid_id = 1`)
	
	if !includeInternal {
		query += " AND is_visible_for_customer = 1"
	}
	
	query += " ORDER BY create_time ASC"
	
	rows, err := r.db.Query(query, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var articles []models.Article
	for rows.Next() {
		var article models.Article
		err := rows.Scan(
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
		if err != nil {
			return nil, err
		}
		articles = append(articles, article)
	}
	
	return articles, nil
}

// Update updates an article in the database
func (r *ArticleRepository) Update(article *models.Article) error {
	article.ChangeTime = time.Now()
	
	query := database.ConvertPlaceholders(`
		UPDATE article SET
			article_type_id = $2,
			sender_type_id = $3,
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
	updateTicketQuery := database.ConvertPlaceholders(`
		UPDATE ticket 
		SET change_time = $2, change_by = $3
		WHERE id = $1`)
	
	_, err = r.db.Exec(updateTicketQuery, article.TicketID, time.Now(), article.ChangeBy)
	
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
			id, ticket_id, article_type_id, sender_type_id,
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