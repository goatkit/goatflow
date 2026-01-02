package api

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleUserMeAPI returns the current authenticated user's information.
func HandleUserMeAPI(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	userIDValue, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
		})
		return
	}

	// Convert user ID to int
	userID, ok := userIDValue.(int)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Invalid user ID in context",
		})
		return
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

	// Query user information
	var user struct {
		ID         int            `json:"id"`
		Login      string         `json:"login"`
		Email      sql.NullString `json:"-"`
		FirstName  string         `json:"first_name"`
		LastName   string         `json:"last_name"`
		ValidID    int            `json:"valid_id"`
		CreateTime sql.NullTime   `json:"create_time"`
		ChangeTime sql.NullTime   `json:"change_time"`
	}

	query := database.ConvertPlaceholders(`
		SELECT id, login, email, first_name, last_name, valid_id, create_time, change_time
		FROM users
		WHERE id = $1
	`)

	err = db.QueryRow(query, userID).Scan(
		&user.ID,
		&user.Login,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.ValidID,
		&user.CreateTime,
		&user.ChangeTime,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "User not found",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Database error",
			})
		}
		return
	}

	// Prepare email for response
	emailStr := user.Login // Default to login if email is null
	if user.Email.Valid {
		emailStr = user.Email.String
	}

	// Get user's groups
	groupQuery := database.ConvertPlaceholders(`
		SELECT g.id, g.name
		FROM groups g
		JOIN group_user gu ON g.id = gu.group_id
		WHERE gu.user_id = $1 AND g.valid_id = 1
	`)

	rows, err := db.Query(groupQuery, userID)
	if err == nil {
		defer rows.Close()
		var groups []gin.H
		for rows.Next() {
			var groupID int
			var groupName string
			if err := rows.Scan(&groupID, &groupName); err == nil {
				groups = append(groups, gin.H{
					"id":   groupID,
					"name": groupName,
				})
			}
		}
		_ = rows.Err() //nolint:errcheck // Check for iteration errors
		// Return user information with groups
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"id":         user.ID,
				"login":      user.Login,
				"email":      emailStr,
				"first_name": user.FirstName,
				"last_name":  user.LastName,
				"active":     user.ValidID == 1,
				"groups":     groups,
			},
		})
	} else {
		// Return user without groups if query fails
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"id":         user.ID,
				"login":      user.Login,
				"email":      emailStr,
				"first_name": user.FirstName,
				"last_name":  user.LastName,
				"active":     user.ValidID == 1,
				"groups":     []gin.H{},
			},
		})
	}
}
