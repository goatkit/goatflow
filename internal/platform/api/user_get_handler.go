package api

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// HandleGetUserAPI handles GET /api/v1/users/:id.
//
//	@Summary		Get user by ID
//	@Description	Retrieve a single user by their ID
//	@Tags			Users
//	@Accept			json
//	@Produce		json
//	@Param			id	path		int	true	"User ID"
//	@Success		200	{object}	map[string]interface{}	"User details"
//	@Failure		401	{object}	map[string]interface{}	"Unauthorized"
//	@Failure		404	{object}	map[string]interface{}	"User not found"
//	@Security		BearerAuth
//	@Router			/users/{id} [get]
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

	// Query for user details (OTRS users has no email column; see UserEmail preference below)
	query := database.ConvertPlaceholders(`
		SELECT id, login, first_name, last_name, valid_id, create_time, change_time
		FROM users
		WHERE id = ?
	`)

	var user struct {
		ID         int
		Login      string
		FirstName  sql.NullString
		LastName   sql.NullString
		ValidID    int
		CreateTime sql.NullTime
		ChangeTime sql.NullTime
	}

	err = db.QueryRow(query, userID).Scan(
		&user.ID,
		&user.Login,
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

	groups, err := loadUserGroupPermissions(db, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to retrieve user groups",
		})
		return
	}
	response["groups"] = groups

	// Only display preferences leave the server: user_preferences also holds
	// TOTP secrets, recovery codes and other credentials.
	prefRows, err := db.Query(database.ConvertPlaceholders(`
		SELECT preferences_key, preferences_value
		FROM user_preferences
		WHERE user_id = ? AND preferences_key IN (?, ?, ?, ?, ?, ?, ?)
	`), userID, "UserEmail", "Language", "Theme", "ThemeMode", "SessionTimeout", "RemindersEnabled", "UserTimeZone")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to retrieve user preferences",
		})
		return
	}
	defer prefRows.Close()

	preferences := make(map[string]string)
	for prefRows.Next() {
		var key string
		var value []byte
		if err := prefRows.Scan(&key, &value); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to retrieve user preferences",
			})
			return
		}
		preferences[key] = string(value)
	}
	if err := prefRows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to retrieve user preferences",
		})
		return
	}
	if len(preferences) > 0 {
		response["preferences"] = preferences
	}
	if email, ok := preferences["UserEmail"]; ok {
		response["email"] = email
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    response,
	})
}
