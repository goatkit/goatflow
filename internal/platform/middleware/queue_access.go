// Package middleware provides HTTP middleware for authentication and authorization.
package middleware

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/convert"
	"github.com/goatkit/goatflow/internal/platform/database"
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
		if IsCustomerPrincipal(c) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Agent access required"})
			c.Abort()
			return
		}

		userIDUint := getQueueAccessUserIDFromCtxUint(c, 0)
		if userIDUint == 0 {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
			c.Abort()
			return
		}

		// Collect the queue ID from every place a handler may read it (path,
		// query, form, JSON body). They must agree: otherwise the check could
		// pass on one value while the handler acts on another.
		var candidates []string
		pathID := c.Param("queue_id")
		if pathID == "" {
			pathID = c.Param("id")
		}
		candidates = append(candidates, pathID, c.Query("queue_id"), c.PostForm("queue_id"))
		if c.Request.Body != nil && c.ContentType() == "application/json" {
			bodyBytes, err := io.ReadAll(c.Request.Body)
			if err == nil && len(bodyBytes) > 0 {
				// Restore the body for downstream handlers
				c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				var jsonBody map[string]interface{}
				if json.Unmarshal(bodyBytes, &jsonBody) == nil {
					switch v := jsonBody["queue_id"].(type) {
					case string:
						candidates = append(candidates, v)
					case float64:
						candidates = append(candidates, strconv.FormatInt(int64(v), 10))
					}
				}
			}
		}
		queueIDStr := ""
		for _, s := range candidates {
			if s == "" {
				continue
			}
			if queueIDStr != "" && s != queueIDStr {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Conflicting queue IDs in request"})
				c.Abort()
				return
			}
			queueIDStr = s
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
		if queueAccessCheckerFactory == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Queue access service unavailable"})
			c.Abort()
			return
		}
		queueAccessSvc := queueAccessCheckerFactory(db)

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
		if IsCustomerPrincipal(c) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Agent access required"})
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

		// Resolve ticket to queue ID via injected resolver
		if ticketQueueResolverFactory == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ticket resolver unavailable"})
			c.Abort()
			return
		}
		resolver := ticketQueueResolverFactory()
		tickets, err := resolver.ResolveTickets(db, ticketIDStr)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
			c.Abort()
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch ticket"})
			c.Abort()
			return
		}
		queueID, ticketID := tickets[0].QueueID, tickets[0].ID

		// Create queue access service
		if queueAccessCheckerFactory == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Queue access service unavailable"})
			c.Abort()
			return
		}
		queueAccessSvc := queueAccessCheckerFactory(db)

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

		// Check the required permission on the queue of every ticket the
		// identifier names (a tn can equal another ticket's id).
		for _, t := range tickets {
			hasAccess, err := queueAccessSvc.HasQueueAccess(c.Request.Context(), userIDUint, t.QueueID, permType)
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

		// Queue permissions are agent permissions. Customers reach their own
		// tickets through customer routes (or RequireCustomerOrAnyQueueAccess).
		if IsCustomerPrincipal(c) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Agent access required"})
			c.Abort()
			return
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
		if queueAccessCheckerFactory == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Queue access service unavailable"})
			c.Abort()
			return
		}
		queueAccessSvc := queueAccessCheckerFactory(db)

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

// RequireCustomerOrAnyQueueAccess is RequireAnyQueueAccess for routes that
// customers may also call: customers pass through and the handler must limit
// them to their own tickets; agents need permType on at least one queue.
func RequireCustomerOrAnyQueueAccess(permType string) gin.HandlerFunc {
	agent := RequireAnyQueueAccess(permType)
	return func(c *gin.Context) {
		if _, exists := c.Get("user_id"); exists && IsCustomerPrincipal(c) {
			c.Next()
			return
		}
		agent(c)
	}
}

// RequireTicketReadOrCustomerOwner admits agents with "ro" on the queue of
// the ticket in the path (as RequireQueueAccessFromTicket("ro")) and customers
// whose login is the ticket's customer_user_id. Other customers get 404, so
// the existence of foreign tickets is not revealed. The handler must still
// hide agent-only content (internal articles) from customers.
func RequireTicketReadOrCustomerOwner() gin.HandlerFunc {
	agent := RequireQueueAccessFromTicket("ro")
	return func(c *gin.Context) {
		if _, exists := c.Get("user_id"); !exists || !IsCustomerPrincipal(c) {
			agent(c)
			return
		}
		ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if p := c.Param("ticket_id"); p != "" {
			ticketID, err = strconv.ParseInt(p, 10, 64)
		}
		if err != nil || ticketID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
			c.Abort()
			return
		}
		login := c.GetString("customer_login")
		db, err := database.GetDB()
		if err != nil || db == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
			c.Abort()
			return
		}
		var owner sql.NullString
		err = db.QueryRowContext(c.Request.Context(), database.ConvertPlaceholders(
			`SELECT customer_user_id FROM ticket WHERE id = ?`), ticketID).Scan(&owner)
		if err != nil && err != sql.ErrNoRows {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch ticket"})
			c.Abort()
			return
		}
		if err == sql.ErrNoRows || login == "" || !owner.Valid || owner.String != login {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
			c.Abort()
			return
		}
		c.Set("ticket_id", uint64(ticketID))
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
