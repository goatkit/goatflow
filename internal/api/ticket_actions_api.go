package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleCloseTicketAPI handles ticket closure via API.
func HandleCloseTicketAPI(c *gin.Context) {
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

	var closeRequest struct {
		Resolution string `json:"resolution" binding:"required"`
		Comment    string `json:"comment"`
	}

	if err := c.ShouldBindJSON(&closeRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid close request: " + err.Error(),
		})
		return
	}

	// Get user ID from context
	userID := 1 // Default for testing
	if id, exists := c.Get("user_id"); exists {
		if intID, ok := id.(int); ok {
			userID = intID
		}
	}

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "Database unavailable",
		})
		return
	}

	// Check if ticket exists and get current state
	var currentStateID int
	var title string
	err = db.QueryRow(database.ConvertPlaceholders(
		"SELECT ticket_state_id, title FROM ticket WHERE id = ?",
	), ticketID).Scan(&currentStateID, &title)

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

	// Check if ticket is already closed
	if currentStateID == 2 || currentStateID == 3 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Ticket is already closed",
		})
		return
	}

	// Determine close state based on resolution
	var newStateID int
	if strings.ToLower(closeRequest.Resolution) == "successful" ||
		strings.ToLower(closeRequest.Resolution) == "resolved" ||
		strings.ToLower(closeRequest.Resolution) == "fixed" {
		newStateID = 2 // closed successful
	} else {
		newStateID = 3 // closed unsuccessful
	}

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to start transaction",
		})
		return
	}
	defer func() { _ = tx.Rollback() }()

	// Update ticket state
	updateQuery := database.ConvertPlaceholders(`
		UPDATE ticket 
		SET ticket_state_id = ?,
		    change_time = NOW(),
		    change_by = ?
		WHERE id = ?
	`)

	_, err = tx.Exec(updateQuery, newStateID, userID, ticketID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to close ticket",
		})
		return
	}

	// Add article documenting the closure
	if closeRequest.Comment != "" {
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
				?, 1, 1, 1, 0, NOW(), ?, NOW(), ?
			)
		`)

		articleResult, err := tx.Exec(insertArticleQuery, ticketID, userID, userID)
		if err == nil {
			articleID, _ := articleResult.LastInsertId() //nolint:errcheck // Best effort

			// Insert article content
			subject := fmt.Sprintf("Ticket Closed: %s", closeRequest.Resolution)
			body := closeRequest.Comment
			if body == "" {
				body = fmt.Sprintf("Ticket has been closed as %s.", closeRequest.Resolution)
			}

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
					?, ?, ?, 'text/plain', 
					?, NOW(), ?, NOW(), ?
				)
			`)

			_, _ = tx.Exec(insertMimeQuery, articleID, subject, body, time.Now().Unix(), userID, userID) //nolint:errcheck // Best effort
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to commit transaction",
		})
		return
	}

	// Return success response
	stateName := "closed successful"
	if newStateID == 3 {
		stateName = "closed unsuccessful"
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"id":         ticketID,
		"state_id":   newStateID,
		"state":      stateName,
		"resolution": closeRequest.Resolution,
		"comment":    closeRequest.Comment,
		"closed_at":  time.Now().UTC(),
	})
}

// HandleReopenTicketAPI handles ticket reopening via API.
func HandleReopenTicketAPI(c *gin.Context) {
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

	var reopenRequest struct {
		Reason string `json:"reason" binding:"required"`
	}

	if err := c.ShouldBindJSON(&reopenRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid reopen request: " + err.Error(),
		})
		return
	}

	// Get user ID from context
	userID := 1 // Default for testing
	if id, exists := c.Get("user_id"); exists {
		if intID, ok := id.(int); ok {
			userID = intID
		}
	}

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "Database unavailable",
		})
		return
	}

	// Check if ticket exists and get current state
	var currentStateID int
	var title string
	err = db.QueryRow(database.ConvertPlaceholders(
		"SELECT ticket_state_id, title FROM ticket WHERE id = ?",
	), ticketID).Scan(&currentStateID, &title)

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

	// Check if ticket is not closed
	if currentStateID != 2 && currentStateID != 3 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Ticket is not closed",
		})
		return
	}

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to start transaction",
		})
		return
	}
	defer func() { _ = tx.Rollback() }()

	// Update ticket state to open
	updateQuery := database.ConvertPlaceholders(`
		UPDATE ticket 
		SET ticket_state_id = 4,
		    archive_flag = 0,
		    change_time = NOW(),
		    change_by = ?
		WHERE id = ?
	`)

	_, err = tx.Exec(updateQuery, userID, ticketID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to reopen ticket",
		})
		return
	}

	// Add article documenting the reopening
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
			?, 1, 1, 1, 0, NOW(), ?, NOW(), ?
		)
	`)

	articleResult, err := tx.Exec(insertArticleQuery, ticketID, userID, userID)
	if err == nil {
		articleID, _ := articleResult.LastInsertId() //nolint:errcheck // Best effort

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
				?, 'Ticket Reopened', ?, 'text/plain', 
				?, NOW(), ?, NOW(), ?
			)
		`)

		body := fmt.Sprintf("Ticket has been reopened. Reason: %s", reopenRequest.Reason)
		_, _ = tx.Exec(insertMimeQuery, articleID, body, time.Now().Unix(), userID, userID) //nolint:errcheck // Best effort
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to commit transaction",
		})
		return
	}

	// Return success response
	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"id":          ticketID,
		"state_id":    4,
		"state":       "open",
		"reason":      reopenRequest.Reason,
		"reopened_at": time.Now().UTC(),
	})
}

// HandleAssignTicketAPI handles ticket assignment via API.
func HandleAssignTicketAPI(c *gin.Context) {
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

	var assignRequest struct {
		AssignedTo int    `json:"assigned_to" binding:"required"`
		Comment    string `json:"comment"`
	}

	if err := c.ShouldBindJSON(&assignRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid assign request: " + err.Error(),
		})
		return
	}

	// Get user ID from context
	userID := 1 // Default for testing
	if id, exists := c.Get("user_id"); exists {
		if intID, ok := id.(int); ok {
			userID = intID
		}
	}

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "Database unavailable",
		})
		return
	}

	// Check if ticket exists
	var currentResponsibleID sql.NullInt32
	var title string
	err = db.QueryRow(database.ConvertPlaceholders(
		"SELECT responsible_user_id, title FROM ticket WHERE id = ?",
	), ticketID).Scan(&currentResponsibleID, &title)

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

	// Check if the user to assign to exists
	var assigneeLogin string
	err = db.QueryRow(database.ConvertPlaceholders(
		"SELECT login FROM users WHERE id = ? AND valid_id = 1",
	), assignRequest.AssignedTo).Scan(&assigneeLogin)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "User not found or inactive",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Database error",
			})
		}
		return
	}

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to start transaction",
		})
		return
	}
	defer func() { _ = tx.Rollback() }()

	// Update ticket with new responsible user
	updateQuery := database.ConvertPlaceholders(`
		UPDATE ticket 
		SET responsible_user_id = ?,
		    change_time = NOW(),
		    change_by = ?
		WHERE id = ?
	`)

	_, err = tx.Exec(updateQuery, assignRequest.AssignedTo, userID, ticketID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to assign ticket",
		})
		return
	}

	// Add article documenting the assignment
	if assignRequest.Comment != "" || true { // Always document assignment
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

		articleResult, err := tx.Exec(insertArticleQuery, ticketID, userID, userID)
		if err == nil {
			articleID, _ := articleResult.LastInsertId() //nolint:errcheck // Best effort

			// Build assignment message
			var previousAssignee string
			if currentResponsibleID.Valid {
				_ = db.QueryRow(database.ConvertPlaceholders( //nolint:errcheck // Defaults to empty
					"SELECT login FROM users WHERE id = ?",
				), currentResponsibleID.Int32).Scan(&previousAssignee)
			}

			var body string
			if previousAssignee != "" {
				body = fmt.Sprintf("Ticket reassigned from %s to %s.", previousAssignee, assigneeLogin)
			} else {
				body = fmt.Sprintf("Ticket assigned to %s.", assigneeLogin)
			}

			if assignRequest.Comment != "" {
				body += "\n\nComment: " + assignRequest.Comment
			}

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
					?, 'Ticket Assignment', ?, 'text/plain', 
					?, NOW(), ?, NOW(), ?
				)
			`)

			_, _ = tx.Exec(insertMimeQuery, articleID, body, time.Now().Unix(), userID, userID) //nolint:errcheck // Best effort
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to commit transaction",
		})
		return
	}

	// Return success response
	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"id":          ticketID,
		"assigned_to": assignRequest.AssignedTo,
		"assignee":    assigneeLogin,
		"comment":     assignRequest.Comment,
		"assigned_at": time.Now().UTC(),
	})
}
