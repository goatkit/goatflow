package api

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
)

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

// getUserIDFromContext gets the user ID from the gin context
// getUserIDFromContext returns user ID from context. Kept for future admin pages.
// Deprecated: prefer extracting from JWT middleware claims.
//
//nolint:unused
func getUserIDFromContext(c *gin.Context) int {
	// Try to get user from context
	userInterface, exists := c.Get("user")
	if !exists {
		return 1 // Default to admin user
	}

	// Try to cast to *models.User
	if user, ok := userInterface.(*models.User); ok && user != nil {
		return int(user.ID)
	}

	// Try to cast to models.User
	if user, ok := userInterface.(models.User); ok {
		return int(user.ID)
	}

	return 1 // Default to admin user
}

// getUserFromContext gets the user from the gin context.
func getUserFromContext(c *gin.Context) *models.User {
	userInterface, exists := c.Get("user")
	if !exists {
		// Create user from context values
		userID, _ := c.Get("user_id")
		userEmail, _ := c.Get("user_email")
		userRole, _ := c.Get("user_role")

		user := &models.User{
			ID:    1,
			Login: "admin",
			Email: "root@localhost",
			Role:  "Admin", // Default role
		}

		// Set values from context if available
		if id, ok := userID.(int); ok {
			user.ID = uint(id)
		}
		if email, ok := userEmail.(string); ok {
			user.Email = email
			if user.Login == "admin" && email != "" {
				user.Login = email
			}
		}
		if role, ok := userRole.(string); ok {
			user.Role = role
		}

		return user
	}

	// Try to cast to *models.User
	if user, ok := userInterface.(*models.User); ok {
		// Also check for role in context
		if userRole, exists := c.Get("user_role"); exists {
			if role, ok := userRole.(string); ok {
				user.Role = role
			}
		}
		return user
	}

	// Try to cast to models.User
	if user, ok := userInterface.(models.User); ok {
		// Also check for role in context
		if userRole, exists := c.Get("user_role"); exists {
			if role, ok := userRole.(string); ok {
				user.Role = role
			}
		}
		return &user
	}

	// Return default user with role from context
	userRole, _ := c.Get("user_role")
	role := "Admin"
	if r, ok := userRole.(string); ok {
		role = r
	}

	return &models.User{
		ID:    1,
		Login: "admin",
		Email: "root@localhost",
		Role:  role,
	}
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
	err = db.QueryRow(database.ConvertPlaceholders("SELECT id FROM ticket_state WHERE name = $1 AND valid_id = 1"), state).Scan(&stateRow.ID)
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
	err = db.QueryRow(database.ConvertPlaceholders("SELECT id FROM ticket_priority WHERE name = $1 AND valid_id = 1"), priority).Scan(&priorityRow.ID)
	if err == nil {
		return priorityRow.ID
	}
	// Fallback to default priority (normal/medium)
	err = db.QueryRow(database.ConvertPlaceholders("SELECT id FROM ticket_priority WHERE name IN ('normal', 'medium') AND valid_id = 1 LIMIT 1")).Scan(&priorityRow.ID)
	if err == nil {
		return priorityRow.ID
	}
	return 2 // Ultimate fallback
}

// loadTemplate loads and parses HTML template files.
func loadTemplate(files ...string) (*template.Template, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("no template files provided")
	}

	// Provide minimal functions expected by templates during tests
	funcMap := template.FuncMap{
		"firstLetter": func(s string) string {
			if len(s) == 0 {
				return ""
			}
			return s[:1]
		},
		"L": func(key string, args ...any) string {
			if len(args) == 0 {
				return key
			}
			return fmt.Sprintf(key, args...)
		},
		"H": func(key string, args ...any) string {
			if len(args) == 0 {
				return key
			}
			return fmt.Sprintf(key, args...)
		},
	}

	// Parse with func map to avoid "function not defined" errors in tests
	tmpl := template.New("base").Funcs(funcMap)
	var err error
	tmpl, err = tmpl.ParseFiles(files...)
	if err != nil {
		return nil, err
	}
	return tmpl, nil
}
