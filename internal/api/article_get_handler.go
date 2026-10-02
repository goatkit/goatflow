package api

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// HandleGetArticleAPI handles GET /api/v1/tickets/:ticket_id/articles/:id.
//
//	@Summary		Get article by ID
//	@Description	Retrieve a single article by its ID
//	@Tags			Articles
//	@Accept			json
//	@Produce		json
//	@Param			ticket_id			path		int		true	"Ticket ID"
//	@Param			id					path		int		true	"Article ID"
//	@Param			include_attachments	query		bool	false	"Include attachment metadata"
//	@Success		200					{object}	map[string]interface{}	"Article details"
//	@Failure		401					{object}	map[string]interface{}	"Unauthorized"
//	@Failure		404					{object}	map[string]interface{}	"Article not found"
//	@Security		BearerAuth
//	@Router			/tickets/{ticket_id}/articles/{id} [get]
func HandleGetArticleAPI(c *gin.Context) {
	if _, exists := c.Get("user_id"); !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
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

	req, ok := authorizeTicketArticles(c, db, ticketID, "ro")
	if !ok {
		return
	}

	// The article must belong to the ticket in the URL (access was checked on
	// that ticket); customers only see customer-visible articles.
	response, _, err := scanArticleAPIRow(db.QueryRow(database.ConvertPlaceholders(articleAPIColumns+`
		WHERE a.id = ? AND a.ticket_id = ? AND (a.is_visible_for_customer = 1 OR ? = 0)`),
		articleID, ticketID, boolToInt(req.customer)))
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Article not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch article"})
		return
	}

	if c.Query("include_attachments") == "true" {
		attachments, err := loadArticleAttachments(c.Request.Context(), db, articleID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch attachments"})
			return
		}
		response["attachments"] = attachments
	}

	c.JSON(http.StatusOK, response)
}
