package api

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/storage"
)

// HandleDeleteArticleAPI handles DELETE /api/v1/tickets/:ticket_id/articles/:id.
//
//	@Summary		Delete article
//	@Description	Delete an article, its MIME data and attachments from a ticket
//	@Tags			Articles
//	@Accept			json
//	@Produce		json
//	@Param			ticket_id	path	int	true	"Ticket ID"
//	@Param			id			path	int	true	"Article ID"
//	@Success		200			{object}	map[string]interface{}	"Article deleted"
//	@Failure		401			{object}	map[string]interface{}	"Unauthorized"
//	@Failure		404			{object}	map[string]interface{}	"Article not found"
//	@Security		BearerAuth
//	@Router			/tickets/{ticket_id}/articles/{id} [delete]
func HandleDeleteArticleAPI(c *gin.Context) {
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

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	if _, ok := authorizeTicketArticles(c, db, ticketID, "rw"); !ok {
		return
	}

	var exists int
	err = db.QueryRow(database.ConvertPlaceholders(`
		SELECT 1 FROM article WHERE id = ? AND ticket_id = ?`), articleID, ticketID).Scan(&exists)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Article not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch article"})
		return
	}

	if err := deleteArticle(c.Request.Context(), db, ticketID, articleID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete article"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Article deleted successfully",
		"id":      articleID,
	})
}

// deleteArticle removes an article and everything that references it, in one
// transaction. Attachments and the raw email go through the article content
// store (DB rows or OTRS ArticleStorageFS files). Every article_id foreign key
// in the schema is RESTRICT, so the dependent rows go first. History and time
// accounting are kept and only detached from the article (their article_id is
// nullable).
func deleteArticle(ctx context.Context, db *sql.DB, ticketID, articleID, userID int) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := storage.ForDB(db).WithTx(tx).DeleteArticle(ctx, int64(articleID)); err != nil {
		return err
	}

	run := func(_ sql.Result, err error) error { return err }
	steps := []func() error{
		func() error {
			return run(tx.ExecContext(ctx, database.ConvertPlaceholders(`DELETE FROM article_data_mime_send_error WHERE article_id = ?`), articleID))
		},
		func() error {
			return run(tx.ExecContext(ctx, database.ConvertPlaceholders(`DELETE FROM article_data_mime WHERE article_id = ?`), articleID))
		},
		func() error {
			return run(tx.ExecContext(ctx, database.ConvertPlaceholders(`DELETE FROM article_data_otrs_chat WHERE article_id = ?`), articleID))
		},
		func() error {
			return run(tx.ExecContext(ctx, database.ConvertPlaceholders(`DELETE FROM article_flag WHERE article_id = ?`), articleID))
		},
		func() error {
			return run(tx.ExecContext(ctx, database.ConvertPlaceholders(`DELETE FROM article_search_index WHERE article_id = ?`), articleID))
		},
		func() error {
			return run(tx.ExecContext(ctx, database.ConvertPlaceholders(`DELETE FROM mail_queue WHERE article_id = ?`), articleID))
		},
		func() error {
			return run(tx.ExecContext(ctx, database.ConvertPlaceholders(`
				DELETE FROM dynamic_field_value
				WHERE object_id = ?
				  AND field_id IN (SELECT id FROM dynamic_field WHERE object_type = 'Article')`), articleID))
		},
		func() error {
			return run(tx.ExecContext(ctx, database.ConvertPlaceholders(`UPDATE ticket_history SET article_id = NULL WHERE article_id = ?`), articleID))
		},
		func() error {
			return run(tx.ExecContext(ctx, database.ConvertPlaceholders(`UPDATE time_accounting SET article_id = NULL WHERE article_id = ?`), articleID))
		},
		func() error {
			return run(tx.ExecContext(ctx, database.ConvertPlaceholders(`DELETE FROM article WHERE id = ? AND ticket_id = ?`), articleID, ticketID))
		},
		func() error {
			return run(tx.ExecContext(ctx, database.ConvertPlaceholders(`UPDATE ticket SET change_time = ?, change_by = ? WHERE id = ?`), time.Now(), userID, ticketID))
		},
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return tx.Commit()
}
