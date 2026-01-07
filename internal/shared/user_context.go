package shared

import (
	"database/sql"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// UserContext represents user information for templates.
type UserContext struct {
	ID              int    `json:"id"`
	Login           string `json:"login"`
	FirstName       string `json:"first_name"`
	LastName        string `json:"last_name"`
	Email           string `json:"email"`
	IsInAdminGroup  bool   `json:"is_in_admin_group"`
	GroupIDs        []int  `json:"group_ids"`
	RoleIDs         []int  `json:"role_ids"`
	CustomerCompany string `json:"customer_company"`
}

// GetUserMapForTemplate builds a user context map for template rendering.
func GetUserMapForTemplate(c *gin.Context) map[string]interface{} {
	userMap := make(map[string]interface{})

	// Get user ID from session or token
	userID := 0
	if cookie, err := c.Cookie("user_id"); err == nil {
		if id, err := strconv.Atoi(cookie); err == nil {
			userID = id
		}
	}

	if userID == 0 {
		// Check Authorization header for API calls
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			// For API calls, user ID might be in a different format
			// This is a simplified version - actual implementation would parse JWT
			userMap["ID"] = 0
			userMap["Login"] = ""
			userMap["IsInAdminGroup"] = false
			return userMap
		}
	}

	if userID > 0 {
		db, err := database.GetDB()
		if err == nil && db != nil {
			var login, firstName, lastName, email sql.NullString
			var isAdmin bool

			err := db.QueryRow(`
				SELECT u.login, u.first_name, u.last_name, u.email,
				       EXISTS(SELECT 1 FROM user_group ug 
				              JOIN groups g ON ug.group_id = g.id 
				              WHERE ug.user_id = u.id AND g.name = 'admin') as is_admin
				FROM users u 
				WHERE u.id = ?
			`, userID).Scan(&login, &firstName, &lastName, &email, &isAdmin)

			if err == nil {
				userMap["ID"] = userID
				if login.Valid {
					userMap["Login"] = login.String
				}
				if firstName.Valid {
					userMap["FirstName"] = firstName.String
				}
				if lastName.Valid {
					userMap["LastName"] = lastName.String
				}
				if email.Valid {
					userMap["Email"] = email.String
				}
				userMap["IsInAdminGroup"] = isAdmin
			}
		}
	}

	// Set defaults if not found
	if _, exists := userMap["ID"]; !exists {
		userMap["ID"] = 0
		userMap["Login"] = ""
		userMap["IsInAdminGroup"] = false
	}

	return userMap
}

// GetUserID extracts user ID from request context.
func GetUserID(c *gin.Context) int {
	// Check cookie first
	if cookie, err := c.Cookie("user_id"); err == nil {
		if id, err := strconv.Atoi(cookie); err == nil {
			return id
		}
	}

	// Check Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		// Simplified - actual implementation would parse JWT
		if userMap, ok := GetUserMapForTemplate(c)["ID"].(int); ok {
			return userMap
		}
	}

	return 0
}

// IsAuthenticated checks if user is authenticated.
func IsAuthenticated(c *gin.Context) bool {
	return GetUserID(c) > 0
}

// IsAdmin checks if current user is admin.
func IsAdmin(c *gin.Context) bool {
	userMap := GetUserMapForTemplate(c)
	if isAdmin, ok := userMap["IsInAdminGroup"].(bool); ok {
		return isAdmin
	}
	return false
}
