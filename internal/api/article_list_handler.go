package api

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/core"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/storage"
)

// articleAPIColumns is the SELECT list shared by the article get and list
// endpoints; scanArticleAPIRow reads it. Subject, body and addresses live in
// article_data_mime, the article type is derived from channel + visibility.
const articleAPIColumns = `
	SELECT a.id, a.ticket_id, a.article_sender_type_id, ast.name,
		a.communication_channel_id, a.is_visible_for_customer,
		m.a_from, m.a_to, m.a_cc, m.a_subject, m.a_body, m.a_content_type,
		a.create_time, a.create_by, a.change_time, a.change_by
	FROM article a
	LEFT JOIN article_data_mime m ON m.article_id = a.id
	LEFT JOIN article_sender_type ast ON ast.id = a.article_sender_type_id`

func scanArticleAPIRow(row interface{ Scan(...any) error }) (gin.H, int, error) {
	var (
		id, ticketID, senderTypeID, channelID, visible, createBy, changeBy int
		senderType, from, to, cc, subject, body, contentType               sql.NullString
		createTime, changeTime                                             time.Time
	)
	if err := row.Scan(&id, &ticketID, &senderTypeID, &senderType, &channelID, &visible,
		&from, &to, &cc, &subject, &body, &contentType,
		&createTime, &createBy, &changeTime, &changeBy); err != nil {
		return nil, 0, err
	}
	return gin.H{
		"id":                       id,
		"ticket_id":                ticketID,
		"article_sender_type_id":   senderTypeID,
		"sender_type":              senderType.String,
		"communication_channel_id": channelID,
		"is_visible_for_customer":  visible == 1,
		"article_type":             core.ArticleTypeName(core.ArticleTypeFromStorage(channelID, visible == 1)),
		"from":                     from.String,
		"to":                       to.String,
		"cc":                       cc.String,
		"subject":                  subject.String,
		"body":                     body.String,
		"content_type":             contentType.String,
		"create_time":              createTime,
		"create_by":                createBy,
		"change_time":              changeTime,
		"change_by":                changeBy,
	}, id, nil
}

// loadArticleAttachments returns attachment metadata for an article from the
// configured article content store. "id" is the attachment's file id within
// the article.
func loadArticleAttachments(ctx context.Context, db *sql.DB, articleID int) ([]gin.H, error) {
	atts, err := storage.ForDB(db).ListAttachments(ctx, int64(articleID))
	if err != nil {
		return nil, err
	}
	out := make([]gin.H, 0, len(atts))
	for _, a := range atts {
		if storage.IsHTMLBody(a) {
			continue
		}
		out = append(out, gin.H{
			"id":           a.FileID,
			"filename":     a.Filename,
			"content_type": a.ContentType,
			"size":         a.Size,
			"disposition":  a.Disposition,
		})
	}
	return out, nil
}

// HandleListArticlesAPI handles GET /api/v1/tickets/:ticket_id/articles.
//
//	@Summary		List ticket articles
//	@Description	Retrieve all articles for a ticket, newest first
//	@Tags			Articles
//	@Accept			json
//	@Produce		json
//	@Param			ticket_id			path		int		true	"Ticket ID"
//	@Param			include_attachments	query		bool	false	"Include attachment metadata"
//	@Success		200					{object}	map[string]interface{}	"List of articles"
//	@Failure		401					{object}	map[string]interface{}	"Unauthorized"
//	@Failure		404					{object}	map[string]interface{}	"Ticket not found"
//	@Security		BearerAuth
//	@Router			/tickets/{ticket_id}/articles [get]
func HandleListArticlesAPI(c *gin.Context) {
	if _, exists := c.Get("user_id"); !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

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

	req, ok := authorizeTicketArticles(c, db, ticketID, "ro")
	if !ok {
		return
	}

	rows, err := db.Query(database.ConvertPlaceholders(articleAPIColumns+`
		WHERE a.ticket_id = ? AND (a.is_visible_for_customer = 1 OR ? = 0)
		ORDER BY a.create_time DESC, a.id DESC`), ticketID, boolToInt(req.customer))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch articles"})
		return
	}
	defer rows.Close()

	includeAttachments := c.Query("include_attachments") == "true"
	articles := []gin.H{}
	for rows.Next() {
		article, articleID, err := scanArticleAPIRow(rows)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read articles"})
			return
		}
		if includeAttachments {
			attachments, err := loadArticleAttachments(c.Request.Context(), db, articleID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch attachments"})
				return
			}
			article["attachments"] = attachments
		}
		articles = append(articles, article)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read articles"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"articles": articles,
		"total":    len(articles),
	})
}
