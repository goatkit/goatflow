// Package middleware provides HTTP middleware for authentication and authorization.
package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/convert"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/service"
)

// RequireQueueAccess checks if the user has the specified permission for the queue.
// The queue ID is extracted from the URL parameter "queue_id" or query parameter "queue_id".
// Permission types: ro, rw, create, move_into, note, owner, priority
func RequireQueueAccess(permType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context (set by auth middleware)
		if _, exists := c.Get("user_id"); !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		userIDUint := getQueueAccessUserIDFromCtxUint(c, 0)
		if userIDUint == 0 {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
			c.Abort()
			return
		}

		// Extract queue ID from URL param or query param
		queueIDStr := c.Param("queue_id")
		if queueIDStr == "" {
			queueIDStr = c.Param("id") // Also check :id param
		}
		if queueIDStr == "" {
			queueIDStr = c.Query("queue_id")
		}
		if queueIDStr == "" {
			// Try to get from form data
			queueIDStr = c.PostForm("queue_id")
		}

		if queueIDStr == "" {
			// Try to get from JSON body - read body and restore it for downstream handlers
			if c.Request.Body != nil && c.ContentType() == "application/json" {
				bodyBytes, err := io.ReadAll(c.Request.Body)
				if err == nil && len(bodyBytes) > 0 {
					// Restore the body for downstream handlers
					c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

					// Parse JSON to extract queue_id
					var jsonBody map[string]interface{}
					if json.Unmarshal(bodyBytes, &jsonBody) == nil {
						if queueVal, ok := jsonBody["queue_id"]; ok && queueVal != nil {
							switch v := queueVal.(type) {
							case string:
								queueIDStr = v
							case float64:
								queueIDStr = strconv.FormatInt(int64(v), 10)
							case int:
								queueIDStr = strconv.Itoa(v)
							case int64:
								queueIDStr = strconv.FormatInt(v, 10)
							}
						}
					}
				}
			}
		}

		if queueIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Queue ID is required"})
			c.Abort()
			return
		}

		queueID, err := strconv.ParseUint(queueIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid queue ID"})
			c.Abort()
			return
		}

		// Get database connection
		db, err := database.GetDB()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
			c.Abort()
			return
		}

		// Create queue access service
		queueAccessSvc := service.NewQueueAccessService(db)

		// Check if user is admin (bypass permission check)
		isAdmin, err := queueAccessSvc.IsAdmin(c.Request.Context(), userIDUint)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
			c.Abort()
			return
		}

		if isAdmin {
			// Admin users have full access - set context for downstream handlers
			c.Set("is_queue_admin", true)
			c.Set("queue_id", uint(queueID))
			c.Next()
			return
		}

		// Check if user has the required permission for this queue
		hasAccess, err := queueAccessSvc.HasQueueAccess(c.Request.Context(), userIDUint, uint(queueID), permType)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check queue permissions"})
			c.Abort()
			return
		}

		if !hasAccess {
			c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to access this queue"})
			c.Abort()
			return
		}

		// User has access - set context for downstream handlers
		c.Set("is_queue_admin", false)
		c.Set("queue_id", uint(queueID))
		c.Next()
	}
}

// RequireQueueAccessFromTicket checks if the user has the specified permission for the queue
// that the ticket belongs to. The ticket ID is extracted from the URL parameter "ticket_id" or "id".
func RequireQueueAccessFromTicket(permType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context (set by auth middleware)
		if _, exists := c.Get("user_id"); !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		userIDUint := getQueueAccessUserIDFromCtxUint(c, 0)
		if userIDUint == 0 {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
			c.Abort()
			return
		}

		// Extract ticket ID from URL param
		ticketIDStr := c.Param("ticket_id")
		if ticketIDStr == "" {
			ticketIDStr = c.Param("id")
		}
		if ticketIDStr == "" {
			ticketIDStr = c.Query("ticket_id")
		}

		if ticketIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Ticket ID is required"})
			c.Abort()
			return
		}

		// Get database connection
		db, err := database.GetDB()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
			c.Abort()
			return
		}

		// Look up the ticket - try ticket number (tn) first, then numeric ID
		// This is important because ticket numbers like "2025082610000069" are numeric
		// but should be matched against the tn field, not the id field
		var queueID uint
		var ticketID uint64

		// First, try to find by ticket number (tn field)
		query := database.ConvertPlaceholders("SELECT id, queue_id FROM ticket WHERE tn = ?")
		err = db.QueryRow(query, ticketIDStr).Scan(&ticketID, &queueID)

		if err != nil {
			// If not found by tn, try as numeric primary key ID (for backwards compatibility)
			numericID, parseErr := strconv.ParseUint(ticketIDStr, 10, 64)
			if parseErr == nil {
				ticketID = numericID
				query = database.ConvertPlaceholders("SELECT queue_id FROM ticket WHERE id = ?")
				err = db.QueryRow(query, ticketID).Scan(&queueID)
			}
		}

		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
			c.Abort()
			return
		}

		// Create queue access service
		queueAccessSvc := service.NewQueueAccessService(db)

		// Check if user is admin (bypass permission check)
		isAdmin, err := queueAccessSvc.IsAdmin(c.Request.Context(), userIDUint)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
			c.Abort()
			return
		}

		if isAdmin {
			// Admin users have full access - set context for downstream handlers
			c.Set("is_queue_admin", true)
			c.Set("queue_id", queueID)
			c.Set("ticket_id", ticketID)
			c.Next()
			return
		}

		// Check if user has the required permission for this queue
		hasAccess, err := queueAccessSvc.HasQueueAccess(c.Request.Context(), userIDUint, queueID, permType)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check queue permissions"})
			c.Abort()
			return
		}

		if !hasAccess {
			c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to access this queue"})
			c.Abort()
			return
		}

		// Store context values for downstream handlers
		c.Set("is_queue_admin", false)
		c.Set("queue_id", queueID)
		c.Set("ticket_id", ticketID)

		// User has access, continue
		c.Next()
	}
}

// RequireAnyQueueAccess checks if the user has the specified permission for at least one queue.
// This is useful for routes where access to any queue is sufficient (like ticket list pages).
func RequireAnyQueueAccess(permType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context (set by auth middleware)
		if _, exists := c.Get("user_id"); !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		// Customers bypass queue checks - they access tickets via customer_user_id
		if role, exists := c.Get("user_role"); exists {
			if r, ok := role.(string); ok && r == "Customer" {
				c.Set("is_customer", true)
				c.Next()
				return
			}
		}

		userIDUint := getQueueAccessUserIDFromCtxUint(c, 0)
		if userIDUint == 0 {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
			c.Abort()
			return
		}

		// Get database connection
		db, err := database.GetDB()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
			c.Abort()
			return
		}

		// Create queue access service
		queueAccessSvc := service.NewQueueAccessService(db)

		// Check if user is admin (bypass permission check)
		isAdmin, err := queueAccessSvc.IsAdmin(c.Request.Context(), userIDUint)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
			c.Abort()
			return
		}

		if isAdmin {
			// Admin users have full access - set context for downstream handlers
			c.Set("is_queue_admin", true)
			// Admin doesn't need accessible_queue_ids - handlers should check is_queue_admin first
			c.Next()
			return
		}

		// Check if user has access to any queue
		accessibleQueueIDs, err := queueAccessSvc.GetAccessibleQueueIDs(c.Request.Context(), userIDUint, permType)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check queue permissions"})
			c.Abort()
			return
		}

		if len(accessibleQueueIDs) == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "You do not have access to any queues"})
			c.Abort()
			return
		}

		// Store context values for downstream handlers
		c.Set("is_queue_admin", false)
		c.Set("accessible_queue_ids", accessibleQueueIDs)

		// User has access, continue
		c.Next()
	}
}

// getQueueAccessUserIDFromCtxUint extracts the authenticated user's ID from gin context as uint.
func getQueueAccessUserIDFromCtxUint(c *gin.Context, fallback uint) uint {
	v, ok := c.Get("user_id")
	if !ok {
		return fallback
	}
	return convert.ToUint(v, fallback)
}
