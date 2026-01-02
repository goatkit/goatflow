package api

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleUpdateArticleAPI handles PUT /api/v1/tickets/:ticket_id/articles/:id.
func HandleUpdateArticleAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
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

	var req struct {
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Check if article exists and belongs to the ticket
	var count int
	checkQuery := database.ConvertPlaceholders(`
		SELECT 1 FROM article
		WHERE id = $1 AND ticket_id = $2
	`)
	_ = db.QueryRow(checkQuery, articleID, ticketID).Scan(&count) //nolint:errcheck // Defaults to 0
	if count != 1 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Article not found"})
		return
	}

	// Update article
	updateQuery := database.ConvertPlaceholders(`
		UPDATE article 
		SET subject = $1, body = $2, change_time = NOW(), change_by = $3
		WHERE id = $4 AND ticket_id = $5
	`)

	result, err := db.Exec(updateQuery, req.Subject, req.Body, userID, articleID, ticketID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update article"})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update article"})
		return
	}

	// Update ticket change time
	updateTicketQuery := database.ConvertPlaceholders(`
		UPDATE ticket 
		SET change_time = NOW(), change_by = $1
		WHERE id = $2
	`)
	// Argument order already matches placeholders left-to-right for MySQL
	if _, err := db.Exec(updateTicketQuery, userID, ticketID); err != nil {
		log.Printf("Failed to update ticket change time: %v", err)
	}

	// Return updated article
	response := gin.H{
		"id":        articleID,
		"ticket_id": ticketID,
		"subject":   req.Subject,
		"body":      req.Body,
	}

	c.JSON(http.StatusOK, response)
}
