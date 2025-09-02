package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleGetArticleAPI handles GET /api/v1/tickets/:ticket_id/articles/:id
func HandleGetArticleAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	_ = userID // Will use for permission checks later

    // Parse IDs (accept both :ticket_id and :id, article :article_id or :id)
    ticketParam := c.Param("ticket_id")
    if ticketParam == "" {
        ticketParam = c.Param("id")
    }
    ticketID, err := strconv.Atoi(ticketParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

    articleParam := c.Param("article_id")
    if articleParam == "" {
        articleParam = c.Param("id")
    }
    articleID, err := strconv.Atoi(articleParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid article ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Get article details
	var article struct {
		ID                   int
		TicketID             int
		ArticleTypeID        int
		ArticleSenderTypeID  int
		FromEmail            string
		ToEmail              string
		CC                   *string
		Subject              string
		Body                 string
		CreateTime           string
		CreateBy             int
		ChangeTime           string
		ChangeBy             int
	}

	query := database.ConvertPlaceholders(`
		SELECT id, ticket_id, article_type_id, article_sender_type_id,
			from_email, to_email, cc, subject, body,
			create_time, create_by, change_time, change_by
		FROM article
		WHERE id = $1 AND ticket_id = $2
	`)

	err = db.QueryRow(query, articleID, ticketID).Scan(
		&article.ID, &article.TicketID, &article.ArticleTypeID, &article.ArticleSenderTypeID,
		&article.FromEmail, &article.ToEmail, &article.CC, &article.Subject, &article.Body,
		&article.CreateTime, &article.CreateBy, &article.ChangeTime, &article.ChangeBy,
	)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Article not found"})
		return
	}

	response := gin.H{
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
		"change_time":            article.ChangeTime,
		"change_by":              article.ChangeBy,
	}

	// Add optional CC field
	if article.CC != nil {
		response["cc"] = *article.CC
	}

	// Include attachments if requested
	if c.Query("include_attachments") == "true" {
		attachQuery := database.ConvertPlaceholders(`
			SELECT id, filename, content_type, content_size
			FROM article_attachment
			WHERE article_id = $1
		`)
		
		rows, err := db.Query(attachQuery, articleID)
		if err == nil {
			defer rows.Close()
			attachments := []gin.H{}
			for rows.Next() {
				var attachment struct {
					ID          int
					Filename    string
					ContentType string
					Size        int
				}
				if err := rows.Scan(&attachment.ID, &attachment.Filename, &attachment.ContentType, &attachment.Size); err == nil {
					attachments = append(attachments, gin.H{
						"id":           attachment.ID,
						"filename":     attachment.Filename,
						"content_type": attachment.ContentType,
						"size":         attachment.Size,
					})
				}
			}
			response["attachments"] = attachments
		}
	}

	c.JSON(http.StatusOK, response)
}