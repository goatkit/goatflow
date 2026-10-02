package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// HandleUpdateArticleAPI handles PUT /api/v1/tickets/:ticket_id/articles/:id.
//
//	@Summary		Update article
//	@Description	Update the subject and/or body of an existing article
//	@Tags			Articles
//	@Accept			json
//	@Produce		json
//	@Param			ticket_id	path		int		true	"Ticket ID"
//	@Param			id			path		int		true	"Article ID"
//	@Param			article		body		object	true	"Article update data (subject, body)"
//	@Success		200			{object}	map[string]interface{}	"Updated article"
//	@Failure		400			{object}	map[string]interface{}	"Invalid request"
//	@Failure		401			{object}	map[string]interface{}	"Unauthorized"
//	@Failure		404			{object}	map[string]interface{}	"Article not found"
//	@Security		BearerAuth
//	@Router			/tickets/{ticket_id}/articles/{id} [put]
func HandleUpdateArticleAPI(c *gin.Context) {
	userID, ok := auditUserID(c)
	if !ok {
		return
	}

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

	var req struct {
		Subject *string `json:"subject"`
		Body    *string `json:"body"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Subject == nil && req.Body == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "subject or body is required"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	if _, ok := authorizeTicketArticles(c, db, ticketID, "rw"); !ok {
		return
	}

	// Subject and body live in article_data_mime; the article row only carries
	// the change stamp.
	var subject, body sql.NullString
	err = db.QueryRow(database.ConvertPlaceholders(`
		SELECT m.a_subject, m.a_body
		FROM article a
		INNER JOIN article_data_mime m ON m.article_id = a.id
		WHERE a.id = ? AND a.ticket_id = ?`), articleID, ticketID).Scan(&subject, &body)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Article not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch article"})
		return
	}
	if req.Subject != nil {
		subject = sql.NullString{String: *req.Subject, Valid: true}
	}
	if req.Body != nil {
		body = sql.NullString{String: *req.Body, Valid: true}
	}

	if err := updateArticleContent(db, ticketID, articleID, userID, subject, body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update article"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":        articleID,
		"ticket_id": ticketID,
		"subject":   subject.String,
		"body":      body.String,
	})
}

func updateArticleContent(db *sql.DB, ticketID, articleID, userID int, subject, body sql.NullString) error {
	now := time.Now()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(database.ConvertPlaceholders(`
		UPDATE article_data_mime SET a_subject = ?, a_body = ?, change_time = ?, change_by = ?
		WHERE article_id = ?`), subject, body, now, userID, articleID); err != nil {
		return err
	}
	if _, err := tx.Exec(database.ConvertPlaceholders(`
		UPDATE article SET change_time = ?, change_by = ? WHERE id = ?`), now, userID, articleID); err != nil {
		return err
	}
	if _, err := tx.Exec(database.ConvertPlaceholders(`
		UPDATE ticket SET change_time = ?, change_by = ? WHERE id = ?`), now, userID, ticketID); err != nil {
		return err
	}
	return tx.Commit()
}
