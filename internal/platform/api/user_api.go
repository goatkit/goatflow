package api

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/shared"
)

// HandleUserMeAPI returns the current authenticated user's information.
//
//	@Summary		Get current user
//	@Description	Get the authenticated user's profile
//	@Tags			Users
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}	"User profile"
//	@Failure		401	{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/users/me [get]
func HandleUserMeAPI(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	_, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
		})
		return
	}

	// Convert user ID to int. JWT/session middleware commonly stores this
	// as uint, while tests and API-token paths may use int.
	userID := shared.GetUserIDFromCtx(c, 0)
	if userID == 0 {
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

	// Query user information. The `users` table holds agents/admins
	// and has no email column — only `customer_user` carries email.
	// We surface the login as the email field for response shape
	// continuity (downstream UI/clients expect `email` to be present).
	// Selecting a non-existent column here was returning a generic
	// "Database error" 500 to every authenticated /users/me caller
	// because the underlying error was being swallowed.
	var user struct {
		ID         int          `json:"id"`
		Login      string       `json:"login"`
		FirstName  string       `json:"first_name"`
		LastName   string       `json:"last_name"`
		ValidID    int          `json:"valid_id"`
		CreateTime sql.NullTime `json:"create_time"`
		ChangeTime sql.NullTime `json:"change_time"`
	}

	query := database.ConvertPlaceholders(`
		SELECT id, login, first_name, last_name, valid_id, create_time, change_time
		FROM users
		WHERE id = ?
	`)

	err = db.QueryRow(query, userID).Scan(
		&user.ID,
		&user.Login,
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

	// `users` has no email column; surface the login under the email
	// field so callers (incl. the MCP get_user_me tool) keep working.
	emailStr := user.Login

	// Get user's groups
	groupQuery := database.ConvertPlaceholders(`
		SELECT g.id, g.name
		FROM groups g
		JOIN group_user gu ON g.id = gu.group_id
		WHERE gu.user_id = ? AND g.valid_id = 1
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
