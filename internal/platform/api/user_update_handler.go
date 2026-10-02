package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/database"
)

// UpdateUserRequest represents the request to update a user.
type UpdateUserRequest struct {
	Email     *string `json:"email"`
	FirstName *string `json:"first_name"`
	LastName  *string `json:"last_name"`
	Password  *string `json:"password"`
	ValidID   *int    `json:"valid_id"`
}

// HandleUpdateUserAPI handles PUT /api/v1/users/:id.
//
//	@Summary		Update user
//	@Description	Update an existing user's properties
//	@Tags			Users
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int		true	"User ID"
//	@Param			user	body		object	true	"User update data"
//	@Success		200		{object}	map[string]interface{}	"Updated user"
//	@Failure		400		{object}	map[string]interface{}	"Invalid request"
//	@Failure		401		{object}	map[string]interface{}	"Unauthorized"
//	@Failure		404		{object}	map[string]interface{}	"User not found"
//	@Security		BearerAuth
//	@Router			/users/{id} [put]
func HandleUpdateUserAPI(c *gin.Context) {
	// Check authentication
	currentUserID, exists := c.Get("user_id")
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

	// Parse request once; the raw map detects attempts to change the login.
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Failed to read request body",
		})
		return
	}
	var req UpdateUserRequest
	var rawBody map[string]json.RawMessage
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}
	if err := json.Unmarshal(body, &rawBody); err == nil {
		if _, hasLogin := rawBody["login"]; hasLogin {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Login cannot be changed",
			})
			return
		}
	}

	db, err := database.GetDB()
	if err == nil && db == nil {
		err = errors.New("database connection is nil")
	}
	if err != nil {
		log.Printf("HandleUpdateUserAPI: database unavailable: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database unavailable",
		})
		return
	}

	// Check if user exists
	var existingLogin string
	err = db.QueryRow(database.ConvertPlaceholders(`SELECT login FROM users WHERE id = ?`), userID).Scan(&existingLogin)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found",
		})
		return
	}
	if err != nil {
		log.Printf("HandleUpdateUserAPI: load user %d: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to load user",
		})
		return
	}

	updates := []string{}
	args := []interface{}{}

	var email string
	if req.Email != nil {
		email = strings.TrimSpace(*req.Email)
		if email == "" || !strings.Contains(email, "@") {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid email address",
			})
			return
		}
		// The OTRS users table has no email column; the agent's address is
		// the UserEmail preference, which must stay unique among agents.
		var taken int
		err = db.QueryRow(database.ConvertPlaceholders(`
			SELECT COUNT(*) FROM user_preferences
			WHERE preferences_key = 'UserEmail' AND preferences_value = ? AND user_id <> ?
		`), []byte(email), userID).Scan(&taken)
		if err != nil {
			log.Printf("HandleUpdateUserAPI: email uniqueness check: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to check email",
			})
			return
		}
		if taken > 0 {
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"error":   "Email already in use",
			})
			return
		}
	}

	// first_name and last_name are NOT NULL in the users table.
	if req.FirstName != nil {
		if strings.TrimSpace(*req.FirstName) == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "First name cannot be empty",
			})
			return
		}
		updates = append(updates, "first_name = ?")
		args = append(args, *req.FirstName)
	}

	if req.LastName != nil {
		if strings.TrimSpace(*req.LastName) == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Last name cannot be empty",
			})
			return
		}
		updates = append(updates, "last_name = ?")
		args = append(args, *req.LastName)
	}

	if req.Password != nil && *req.Password != "" {
		// Validate password length
		if len(*req.Password) < 8 {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Password must be at least 8 characters",
			})
			return
		}

		// Hash the new password
		hashedPassword, err := auth.NewPasswordHasher().HashPassword(*req.Password)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, auth.ErrPasswordTooLong) {
				status = http.StatusBadRequest
			}
			c.JSON(status, gin.H{
				"success": false,
				"error":   "Failed to process password: " + err.Error(),
			})
			return
		}

		updates = append(updates, "pw = ?")
		args = append(args, hashedPassword)
	}

	if req.ValidID != nil {
		// Don't allow invalidating user ID 1 (admin)
		if userID == 1 && *req.ValidID != 1 {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"error":   "Cannot invalidate system admin user",
			})
			return
		}

		updates = append(updates, "valid_id = ?")
		args = append(args, *req.ValidID)
	}

	if len(updates) == 0 && req.Email == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "No valid fields to update",
		})
		return
	}

	// Change tracking is written even for email-only updates.
	updates = append(updates, "change_time = ?", "change_by = ?")
	args = append(args, time.Now(), currentUserID, userID)

	tx, err := db.Begin()
	if err != nil {
		log.Printf("HandleUpdateUserAPI: begin: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to update user",
		})
		return
	}
	defer func() { _ = tx.Rollback() }()

	updateQuery := database.ConvertPlaceholders("UPDATE users SET " + strings.Join(updates, ", ") + " WHERE id = ?")
	if _, err := tx.Exec(updateQuery, args...); err != nil {
		log.Printf("HandleUpdateUserAPI: update user %d: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to update user",
		})
		return
	}

	if req.Email != nil {
		if err := setUserEmailPreference(tx, userID, email); err != nil {
			log.Printf("HandleUpdateUserAPI: update email of user %d: %v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to update user",
			})
			return
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("HandleUpdateUserAPI: commit: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to update user",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User updated successfully",
		"data": gin.H{
			"id": userID,
		},
	})
}

// setUserEmailPreference replaces the agent's UserEmail preference.
func setUserEmailPreference(tx *sql.Tx, userID int, email string) error {
	if _, err := tx.Exec(database.ConvertPlaceholders(
		`DELETE FROM user_preferences WHERE user_id = ? AND preferences_key = 'UserEmail'`), userID); err != nil {
		return err
	}
	_, err := tx.Exec(database.ConvertPlaceholders(
		`INSERT INTO user_preferences (user_id, preferences_key, preferences_value) VALUES (?, 'UserEmail', ?)`),
		userID, []byte(email))
	return err
}
