package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// In OTRS, tickets are never hard deleted, only archived.
func HandleDeleteTicketAPI(c *gin.Context) {
	ticketIDStr := c.Param("id")
	if ticketIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Ticket ID required",
		})
		return
	}

	ticketID, err := strconv.Atoi(ticketIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid ticket ID",
		})
		return
	}

	// Enforce authentication before mutating state
	idValue, exists := c.Get("user_id")
	if !exists {
		if _, authExists := c.Get("is_authenticated"); !authExists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Authentication required",
			})
			return
		}
	}

	userID := 1
	if exists {
		switch v := idValue.(type) {
		case int:
			userID = v
		case uint:
			userID = int(v)
		}
	}

	// Get database connection
	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Authentication required"})
		return
	}

	// Check if ticket exists and get current state
	var currentStateID int
	var customerUserID string
	err = db.QueryRow(database.ConvertPlaceholders(
		"SELECT ticket_state_id, customer_user_id FROM ticket WHERE id = ?",
	), ticketID).Scan(&currentStateID, &customerUserID)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "Ticket not found",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Database error",
			})
		}
		return
	}

	// Check permissions for customer users
	if isCustomer, _ := c.Get("is_customer"); isCustomer == true {
		// Customers cannot delete tickets
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   "Customers cannot delete tickets",
		})
		return
	}

	// Check if ticket is already archived/closed
	// States: 2 = closed successful, 3 = closed unsuccessful, 9 = merged
	if currentStateID == 2 || currentStateID == 3 || currentStateID == 9 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Ticket is already closed",
		})
		return
	}

	// Archive the ticket by setting state to "closed successful" and archive_flag to 1
	updateQuery := database.ConvertPlaceholders(`
		UPDATE ticket 
		SET ticket_state_id = 2,
		    archive_flag = 1,
		    change_time = NOW(),
		    change_by = ?
		WHERE id = ?
	`)

	result, err := db.Exec(updateQuery, userID, ticketID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to archive ticket: " + err.Error(),
		})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		rowsAffected = 0
	}
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Ticket not found",
		})
		return
	}

	// Add a final article noting the ticket was archived
	insertArticleQuery := database.ConvertPlaceholders(`
		INSERT INTO article (
			ticket_id,
			article_sender_type_id,
			communication_channel_id,
			is_visible_for_customer,
			search_index_needs_rebuild,
			create_time,
			create_by,
			change_time,
			change_by
		) VALUES (
			?, 1, 1, 0, 0, NOW(), ?, NOW(), ?
		)
	`)

	articleResult, err := db.Exec(insertArticleQuery, ticketID, userID, userID)
	if err == nil {
		articleID, _ := articleResult.LastInsertId() //nolint:errcheck // Best effort article creation

		// Insert article content
		insertMimeQuery := database.ConvertPlaceholders(`
			INSERT INTO article_data_mime (
				article_id,
				a_subject,
				a_body,
				a_content_type,
				incoming_time,
				create_time,
				create_by,
				change_time,
				change_by
			) VALUES (
				?, 'Ticket Archived', 'This ticket has been archived.', 'text/plain', 
				?, NOW(), ?, NOW(), ?
			)
		`)

		_, _ = db.Exec(insertMimeQuery, articleID, time.Now().Unix(), userID, userID) //nolint:errcheck // Best effort
	}

	// Return 204 No Content as per RESTful standards
	c.Status(http.StatusNoContent)
}
