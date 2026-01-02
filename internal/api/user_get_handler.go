package api

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleGetUserAPI handles GET /api/v1/users/:id.
func HandleGetUserAPI(c *gin.Context) {
	// Check authentication
	_, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Authentication required",
		})
		return
	}

	// Get user ID from URL
	userIDStr := c.Param("id")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID",
		})
		return
	}

	// Get database connection
	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection not available",
		})
		return
	}

	// Query for user details
	query := database.ConvertPlaceholders(`
		SELECT 
			id,
			login,
			email,
			first_name,
			last_name,
			valid_id,
			create_time,
			change_time
		FROM users
		WHERE id = $1
	`)

	var user struct {
		ID         int            `json:"id"`
		Login      string         `json:"login"`
		Email      sql.NullString `json:"-"`
		FirstName  sql.NullString `json:"-"`
		LastName   sql.NullString `json:"-"`
		ValidID    int            `json:"valid_id"`
		CreateTime sql.NullTime   `json:"-"`
		ChangeTime sql.NullTime   `json:"-"`
	}

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

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found",
		})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to retrieve user",
		})
		return
	}

	// Build response
	response := gin.H{
		"id":       user.ID,
		"login":    user.Login,
		"valid_id": user.ValidID,
		"valid":    user.ValidID == 1,
	}

	if user.Email.Valid {
		response["email"] = user.Email.String
	}
	if user.FirstName.Valid {
		response["first_name"] = user.FirstName.String
	}
	if user.LastName.Valid {
		response["last_name"] = user.LastName.String
	}
	if user.CreateTime.Valid {
		response["create_time"] = user.CreateTime.Time.Format("2006-01-02T15:04:05Z")
	}
	if user.ChangeTime.Valid {
		response["change_time"] = user.ChangeTime.Time.Format("2006-01-02T15:04:05Z")
	}

	// Get user's groups with permissions
	groupQuery := database.ConvertPlaceholders(`
		SELECT 
			g.id, 
			g.name,
			ug.permission_key,
			ug.permission_value
		FROM groups g
		INNER JOIN user_groups ug ON g.id = ug.group_id
		WHERE ug.user_id = $1
		ORDER BY g.name
	`)

	rows, err := db.Query(groupQuery, userID)
	if err == nil {
		defer rows.Close()

		groups := []map[string]interface{}{}
		for rows.Next() {
			var groupID int
			var groupName string
			var permKey sql.NullString
			var permValue sql.NullInt32

			if err := rows.Scan(&groupID, &groupName, &permKey, &permValue); err == nil {
				group := map[string]interface{}{
					"id":   groupID,
					"name": groupName,
				}

				// Add permission info if available
				if permKey.Valid {
					group["permission_key"] = permKey.String
				}
				if permValue.Valid {
					group["permission_value"] = permValue.Int32
				}

				groups = append(groups, group)
			}
		}
		_ = rows.Err() //nolint:errcheck // Check for iteration errors
		response["groups"] = groups
	} else {
		// Even if groups query fails, return empty array
		response["groups"] = []interface{}{}
	}

	// Get user preferences (if any)
	prefsQuery := database.ConvertPlaceholders(`
		SELECT 
			preferences_key,
			preferences_value
		FROM user_preferences
		WHERE user_id = $1
	`)

	prefRows, err := db.Query(prefsQuery, userID)
	if err == nil {
		defer prefRows.Close()

		preferences := make(map[string]string)
		for prefRows.Next() {
			var key, value string
			if err := prefRows.Scan(&key, &value); err == nil {
				preferences[key] = value
			}
		}
		_ = prefRows.Err() //nolint:errcheck // Check for iteration errors
		if len(preferences) > 0 {
			response["preferences"] = preferences
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    response,
	})
}
