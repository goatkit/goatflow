package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/shared"
)

// GetUserIDFromCtx extracts the authenticated user's ID from gin context.
// Handles multiple types since different auth middleware may set different types.
// Returns the fallback value if user_id is not found or cannot be converted.
func GetUserIDFromCtx(c *gin.Context, fallback int) int {
	v, ok := c.Get("user_id")
	if !ok {
		return fallback
	}
	switch id := v.(type) {
	case int:
		return id
	case int64:
		return int(id)
	case uint:
		return int(id)
	case uint64:
		return int(id)
	case float64:
		return int(id)
	case string:
		if n, err := strconv.Atoi(id); err == nil {
			return n
		}
	}
	return fallback
}

// auditUserID returns the authenticated user's ID for create_by/change_by
// audit columns. It answers 401 and returns false when the request carries no
// authenticated user, so writes are never attributed to a made-up account.
func auditUserID(c *gin.Context) (int, bool) {
	id := GetUserIDFromCtx(c, 0)
	if id <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "authentication required"})
		return 0, false
	}
	return id, true
}

// GetUserIDFromCtxUint is like GetUserIDFromCtx but returns uint.
func GetUserIDFromCtxUint(c *gin.Context, fallback uint) uint {
	v, ok := c.Get("user_id")
	if !ok {
		return fallback
	}
	switch id := v.(type) {
	case int:
		return uint(id)
	case int64:
		return uint(id)
	case uint:
		return id
	case uint64:
		return uint(id)
	case float64:
		return uint(id)
	case string:
		if n, err := strconv.Atoi(id); err == nil {
			return uint(n)
		}
	}
	return fallback
}

// formatFileSize formats a file size in bytes to a human-readable string.
func formatFileSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case size >= GB:
		return fmt.Sprintf("%.2f GB", float64(size)/float64(GB))
	case size >= MB:
		return fmt.Sprintf("%.2f MB", float64(size)/float64(MB))
	case size >= KB:
		return fmt.Sprintf("%.2f KB", float64(size)/float64(KB))
	default:
		return fmt.Sprintf("%d B", size)
	}
}

// getUserFromContext returns the authenticated user from the gin context, or
// nil when the request carries no identity. It never invents one: a missing
// user must not render as (or act for) the admin account.
func getUserFromContext(c *gin.Context) *models.User {
	role := c.GetString("user_role")

	userInterface, exists := c.Get("user")
	if !exists {
		return buildUserFromContext(c, role)
	}

	if user, ok := userInterface.(*models.User); ok && user != nil {
		user.Role = role
		return user
	}

	if user, ok := userInterface.(models.User); ok {
		user.Role = role
		return &user
	}

	return buildUserFromContext(c, role)
}

// buildUserFromContext builds the user from the auth middleware's user_id /
// user_email keys; nil when no user id is present.
func buildUserFromContext(c *gin.Context, role string) *models.User {
	raw, ok := c.Get("user_id")
	if !ok {
		return nil
	}
	id := shared.ToUint(raw, 0)
	if id == 0 {
		return nil
	}
	user := &models.User{ID: id, Role: role}
	if email := c.GetString("user_email"); email != "" {
		user.Email = email
		user.Login = email
	}
	return user
}

// sendGuruMeditation sends a detailed error response (similar to VirtualBox's Guru Meditation).
func sendGuruMeditation(c *gin.Context, err error, message string) {
	// Log the full error for debugging
	if err != nil {
		fmt.Printf("Guru Meditation: %s - Error: %v\n", message, err)
	}

	// Send a user-friendly error response
	c.JSON(http.StatusInternalServerError, gin.H{
		"error":   message,
		"details": err.Error(),
		"status":  "error",
	})
}

// getStateID converts a state string to its database ID.
func getStateID(state string) int {
	db, err := database.GetDB()
	if err != nil {
		return 1 // Default to "new" state
	}
	var stateRow struct {
		ID int
	}
	stateQuery := "SELECT id FROM ticket_state WHERE name = ? AND valid_id = 1"
	err = db.QueryRow(database.ConvertPlaceholders(stateQuery), state).Scan(&stateRow.ID)
	if err == nil {
		return stateRow.ID
	}
	return 1 // Default to "new" state
}

// getPriorityID converts a priority string to its database ID.
func getPriorityID(priority string) int {
	db, err := database.GetDB()
	if err != nil {
		return 2 // Default to normal priority
	}
	var priorityRow struct {
		ID int
	}
	priorityQuery := "SELECT id FROM ticket_priority WHERE name = ? AND valid_id = 1"
	err = db.QueryRow(database.ConvertPlaceholders(priorityQuery), priority).Scan(&priorityRow.ID)
	if err == nil {
		return priorityRow.ID
	}
	// Fallback to default priority (normal/medium)
	fallbackQuery := "SELECT id FROM ticket_priority WHERE name IN ('normal', 'medium') AND valid_id = 1 LIMIT 1"
	err = db.QueryRow(database.ConvertPlaceholders(fallbackQuery)).Scan(&priorityRow.ID)
	if err == nil {
		return priorityRow.ID
	}
	return 2 // Ultimate fallback
}
