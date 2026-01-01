// Package middleware provides HTTP middleware for authentication and authorization.
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// RequireAdminGroup checks if the user is in the admin group.
func RequireAdminGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context (set by auth middleware)
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
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

		// Check if user is in admin group
		var count int
		query := `
			SELECT COUNT(*) 
			FROM group_user gu 
			JOIN groups g ON gu.group_id = g.id 
			WHERE gu.user_id = ? AND g.name = 'admin' AND g.valid_id = 1
		`

		err = db.QueryRow(query, userID).Scan(&count)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check admin status"})
			c.Abort()
			return
		}

		if count == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			c.Abort()
			return
		}

		// User is admin, continue
		c.Next()
	}
}
