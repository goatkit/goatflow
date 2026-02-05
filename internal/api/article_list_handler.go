package api

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// loadArticleAttachments fetches attachments for an article and returns them as a slice of maps.
func loadArticleAttachments(db *sql.DB, articleID int) []gin.H {
	query := database.ConvertPlaceholders(`
		SELECT id, filename, content_type, content_size
		FROM article_attachment
		WHERE article_id = ?
	`)
	rows, err := db.Query(query, articleID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var attachments []gin.H
	for rows.Next() {
		var id, size int
		var filename, contentType string
		if err := rows.Scan(&id, &filename, &contentType, &size); err == nil {
			attachments = append(attachments, gin.H{
				"id": id, "filename": filename, "content_type": contentType, "size": size,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil
	}
	return attachments
}

// HandleListArticlesAPI handles GET /api/v1/tickets/:ticket_id/articles.
//
//	@Summary		List ticket articles
//	@Description	Retrieve all articles for a ticket
//	@Tags			Articles
//	@Accept			json
//	@Produce		json
//	@Param			ticket_id	path		int		true	"Ticket ID"
//	@Param			page		query		int		false	"Page number"				default(1)
//	@Param			per_page	query		int		false	"Items per page"			default(20)
//	@Param			limit		query		int		false	"Alias for per_page"
//	@Param			offset		query		int		false	"Offset for pagination"
//	@Success		200			{object}	map[string]interface{}	"List of articles"
//	@Failure		401			{object}	map[string]interface{}	"Unauthorized"
//	@Failure		404			{object}	map[string]interface{}	"Ticket not found"
//	@Security		BearerAuth
//	@Router			/tickets/{ticket_id}/articles [get]
func HandleListArticlesAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	_ = userID // Will use for permission checks later

	// Parse ticket ID (accept both :ticket_id and :id)
	ticketParam := c.Param("ticket_id")
	if ticketParam == "" {
		ticketParam = c.Param("id")
	}
	ticketID, err := strconv.Atoi(ticketParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Check if ticket exists (OTRS uses singular table names)
	var ticketExists int
	checkQuery := database.ConvertPlaceholders(`
		SELECT 1 FROM ticket WHERE id = ?
	`)
	_ = db.QueryRow(checkQuery, ticketID).Scan(&ticketExists) //nolint:errcheck // Defaults to 0
	if ticketExists != 1 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	// Get articles for the ticket (OTRS tables are singular: article)
	query := database.ConvertPlaceholders(`
		SELECT a.id, a.ticket_id, a.article_type_id, a.article_sender_type_id,
			a.from_email, a.to_email, a.cc, a.subject, a.body,
			a.create_time, a.create_by, 
			at.name as article_type, ast.name as sender_type
        FROM article a
		LEFT JOIN article_type at ON a.article_type_id = at.id
		LEFT JOIN article_sender_type ast ON a.article_sender_type_id = ast.id
		WHERE a.ticket_id = ?
		ORDER BY a.create_time DESC
	`)

	rows, err := db.Query(query, ticketID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch articles"})
		return
	}
	defer rows.Close()

	articles := []gin.H{}
	for rows.Next() {
		var article struct {
			ID                  int
			TicketID            int
			ArticleTypeID       int
			ArticleSenderTypeID int
			FromEmail           string
			ToEmail             string
			CC                  *string
			Subject             string
			Body                string
			CreateTime          string
			CreateBy            int
			ArticleType         *string
			SenderType          *string
		}

		err := rows.Scan(
			&article.ID, &article.TicketID, &article.ArticleTypeID, &article.ArticleSenderTypeID,
			&article.FromEmail, &article.ToEmail, &article.CC, &article.Subject, &article.Body,
			&article.CreateTime, &article.CreateBy,
			&article.ArticleType, &article.SenderType,
		)
		if err != nil {
			continue
		}

		articleData := gin.H{
			"id":                     article.ID,
			"ticket_id":              article.TicketID,
			"article_type_id":        article.ArticleTypeID,
			"article_sender_type_id": article.ArticleSenderTypeID,
			"from_email":             article.FromEmail,
			"to_email":               article.ToEmail,
			"subject":                article.Subject,
			"body":                   article.Body,
			"create_time":            article.CreateTime,
			"create_by":              article.CreateBy,
		}

		// Add optional fields
		if article.CC != nil {
			articleData["cc"] = *article.CC
		}
		if article.ArticleType != nil {
			articleData["article_type"] = *article.ArticleType
		}
		if article.SenderType != nil {
			articleData["sender_type"] = *article.SenderType
		}

		if c.Query("include_attachments") == "true" {
			if attachments := loadArticleAttachments(db, article.ID); len(attachments) > 0 {
				articleData["attachments"] = attachments
			}
		}

		articles = append(articles, articleData)
	}
	_ = rows.Err() //nolint:errcheck // Iteration errors don't affect response

	c.JSON(http.StatusOK, gin.H{
		"articles": articles,
		"total":    len(articles),
	})
}
