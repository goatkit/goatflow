package api

// Admin email queue management handlers.
// Split from admin_htmx_handlers.go for maintainability.

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/routing"
)

func init() {
	routing.RegisterHandler("handleAdminEmailQueue", handleAdminEmailQueue)
	routing.RegisterHandler("handleAdminEmailQueueRetry", handleAdminEmailQueueRetry)
	routing.RegisterHandler("handleAdminEmailQueueDelete", handleAdminEmailQueueDelete)
	routing.RegisterHandler("handleAdminEmailQueueRetryAll", handleAdminEmailQueueRetryAll)
}

// handleAdminEmailQueue shows the admin email queue management page.
func handleAdminEmailQueue(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database connection failed")
		return
	}

	// Get email queue items from database
	rows, err := db.Query(`
		SELECT id, insert_fingerprint, article_id, attempts, sender, recipient,
			   due_time, last_smtp_code, last_smtp_message, create_time
		FROM mail_queue
		ORDER BY create_time DESC
		LIMIT 100
	`)
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to fetch email queue")
		return
	}
	defer rows.Close()

	var emails []gin.H
	for rows.Next() {
		var email gin.H
		var id int64
		var insertFingerprint, sender, recipient sql.NullString
		var articleID sql.NullInt64
		var attempts int
		var dueTime sql.NullTime
		var lastSMTPCode sql.NullInt32
		var lastSMTPMessage sql.NullString
		var createTime time.Time

		err := rows.Scan(&id, &insertFingerprint, &articleID, &attempts, &sender, &recipient,
			&dueTime, &lastSMTPCode, &lastSMTPMessage, &createTime)
		if err != nil {
			continue
		}

		email = gin.H{
			"ID":         id,
			"Attempts":   attempts,
			"Recipient":  recipient.String,
			"CreateTime": createTime,
			"Status":     "pending",
		}

		if insertFingerprint.Valid {
			email["InsertFingerprint"] = insertFingerprint.String
		}
		if articleID.Valid {
			email["ArticleID"] = articleID.Int64
		}
		if sender.Valid {
			email["Sender"] = sender.String
		}
		if dueTime.Valid {
			email["DueTime"] = dueTime.Time
		}
		if lastSMTPCode.Valid {
			email["LastSMTPCode"] = lastSMTPCode.Int32
			if lastSMTPCode.Int32 == 0 {
				email["Status"] = "sent"
			} else {
				email["Status"] = "failed"
			}
		}
		if lastSMTPMessage.Valid {
			email["LastSMTPMessage"] = lastSMTPMessage.String
		}

		emails = append(emails, email)
	}
	if err := rows.Err(); err != nil {
		log.Printf("error iterating email queue: %v", err)
	}

	// Get queue statistics - defaults to 0 on error
	var totalEmails, pendingEmails, failedEmails int
	_ = db.QueryRow("SELECT COUNT(*) FROM mail_queue").Scan(&totalEmails)                                                           //nolint:errcheck
	_ = db.QueryRow("SELECT COUNT(*) FROM mail_queue WHERE (due_time IS NULL OR due_time <= NOW())").Scan(&pendingEmails)           //nolint:errcheck
	_ = db.QueryRow("SELECT COUNT(*) FROM mail_queue WHERE last_smtp_code IS NOT NULL AND last_smtp_code != 0").Scan(&failedEmails) //nolint:errcheck

	processedEmails := totalEmails - pendingEmails

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/email_queue.pongo2", pongo2.Context{
		"Emails":          emails,
		"TotalEmails":     totalEmails,
		"PendingEmails":   pendingEmails,
		"FailedEmails":    failedEmails,
		"ProcessedEmails": processedEmails,
		"User":            getUserMapForTemplate(c),
		"ActivePage":      "admin",
	})
}

// handleAdminEmailQueueRetry retries sending a specific email from the queue.
func handleAdminEmailQueueRetry(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid email ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	// Reset the email for retry by clearing due_time and last_smtp_code/message
	_, err = db.Exec(`
		UPDATE mail_queue
		SET due_time = NULL, last_smtp_code = NULL, last_smtp_message = NULL
		WHERE id = ?
	`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to retry email"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Email queued for retry"})
}

// handleAdminEmailQueueDelete deletes a specific email from the queue.
func handleAdminEmailQueueDelete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid email ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	// Delete the email from the queue
	result, err := db.Exec(`DELETE FROM mail_queue WHERE id = ?`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete email"})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		rowsAffected = 0
	}
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Email not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Email deleted from queue"})
}

// handleAdminEmailQueueRetryAll retries all failed emails in the queue.
func handleAdminEmailQueueRetryAll(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	// Reset all failed emails for retry (emails with SMTP errors, attempts > 0, or error messages)
	result, err := db.Exec(`
		UPDATE mail_queue
		SET due_time = NULL, last_smtp_code = NULL, last_smtp_message = NULL
		WHERE last_smtp_code IS NOT NULL AND last_smtp_code != 0
		   OR attempts > 0
		   OR last_smtp_message IS NOT NULL
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to retry all emails"})
		return
	}

	rowsAffected, _ := result.RowsAffected() //nolint:errcheck // OK if this fails
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("%d emails queued for retry", rowsAffected),
	})
}
