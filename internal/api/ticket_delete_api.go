package api

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/constants"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/lookups"
	"github.com/goatkit/goatflow/internal/services"
)

// HandleDeleteTicketAPI handles DELETE /api/v1/tickets/:id.
// In OTRS, tickets are never hard deleted, only archived.
//
//	@Summary		Delete (archive) ticket
//	@Description	Archive a ticket (soft delete - tickets are never hard deleted)
//	@Tags			Tickets
//	@Accept			json
//	@Produce		json
//	@Param			id	path		int	true	"Ticket ID"
//	@Success		200	{object}	map[string]interface{}	"Ticket archived"
//	@Failure		401	{object}	map[string]interface{}	"Unauthorized"
//	@Failure		404	{object}	map[string]interface{}	"Ticket not found"
//	@Security		BearerAuth
//	@Router			/tickets/{id} [delete]
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
	userID, ok := auditUserID(c)
	if !ok {
		return
	}

	// Get database connection
	db, err := database.GetDB()
	if err != nil || db == nil {
		log.Printf("HandleDeleteTicketAPI: database unavailable: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database unavailable"})
		return
	}

	// Check if ticket exists and get current state
	var currentStateID int
	var queueID int
	err = db.QueryRow(database.ConvertPlaceholders(
		"SELECT ticket_state_id, queue_id FROM ticket WHERE id = ?",
	), ticketID).Scan(&currentStateID, &queueID)

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

	// Check agent has write permission on the ticket's queue
	permSvc := services.NewPermissionService(db)
	canWrite, err := permSvc.CanWriteQueue(userID, queueID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to check permissions",
		})
		return
	}
	// Security: return 404 (not 403) to avoid revealing ticket existence
	if !canWrite {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Ticket not found",
		})
		return
	}

	// Check if ticket is already archived/closed (state type closed/removed/merged)
	ctx := c.Request.Context()
	currentTypeName, err := lookups.StateTypeNameOfState(ctx, db, currentStateID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to resolve ticket state",
		})
		return
	}
	if lookups.IsClosedStateType(currentTypeName) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Ticket is already closed",
		})
		return
	}

	closedStateID, err := lookups.ID(ctx, db, lookups.StateLookup, lookups.StateClosedSuccessful)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to resolve closed state",
		})
		return
	}

	// Archive the ticket by setting state to "closed successful" and archive_flag to 1
	updateQuery := database.ConvertPlaceholders(`
		UPDATE ticket 
		SET ticket_state_id = ?,
		    archive_flag = 1,
		    change_time = NOW(),
		    change_by = ?
		WHERE id = ?
	`)

	result, err := db.Exec(updateQuery, closedStateID, userID, ticketID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   internalDBError(c, "archive ticket", err),
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
			?, ?, ?, 0, 0, NOW(), ?, NOW(), ?
		) RETURNING id
	`)

	articleID, err := database.GetAdapter().InsertWithReturning(db, insertArticleQuery, ticketID,
		constants.ArticleSenderAgent, constants.CommunicationChannelEmail, userID, userID)
	if err == nil {
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
