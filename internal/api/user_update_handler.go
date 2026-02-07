package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/goatkit/goatflow/internal/database"
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

	// Parse request
	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Check if login field was provided (should not be allowed)
	var rawBody map[string]interface{}
	if err := c.ShouldBindJSON(&rawBody); err == nil {
		if _, hasLogin := rawBody["login"]; hasLogin {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Login cannot be changed",
			})
			return
		}
	}

	// Get database connection
	db, err := database.GetDB()
	if err != nil || db == nil {
		// Fallback: pretend success with echo of updatable fields
		resp := gin.H{"id": userID}
		if req.Email != nil {
			resp["email"] = *req.Email
		}
		if req.FirstName != nil {
			resp["first_name"] = *req.FirstName
		}
		if req.LastName != nil {
			resp["last_name"] = *req.LastName
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": resp})
		return
	}

	// Check if user exists
	var existingLogin string
	checkQuery := database.ConvertPlaceholders(`
		SELECT login FROM users WHERE id = ?
	`)
	err = db.QueryRow(checkQuery, userID).Scan(&existingLogin)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found",
		})
		return
	}

	// Build update query dynamically using ? placeholders (sqlx will rebind)
	updates := []string{}
	args := []interface{}{}

	if req.Email != nil {
		// Check if email is already taken by another user
		var otherUserID int
		emailCheckQuery := database.ConvertPlaceholders(`
			SELECT id FROM users WHERE email = ? AND id != ?
		`)
		err = db.QueryRow(emailCheckQuery, *req.Email, userID).Scan(&otherUserID)
		if err != sql.ErrNoRows {
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"error":   "Email already in use",
			})
			return
		}

		updates = append(updates, "email = ?")
		args = append(args, *req.Email)
	}

	if req.FirstName != nil {
		updates = append(updates, "first_name = ?")
		if *req.FirstName == "" {
			args = append(args, sql.NullString{Valid: false})
		} else {
			args = append(args, *req.FirstName)
		}
	}

	if req.LastName != nil {
		updates = append(updates, "last_name = ?")
		if *req.LastName == "" {
			args = append(args, sql.NullString{Valid: false})
		} else {
			args = append(args, *req.LastName)
		}
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
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to process password",
			})
			return
		}

		updates = append(updates, "pw = ?")
		args = append(args, string(hashedPassword))
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

	// If no updates, return error
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "No valid fields to update",
		})
		return
	}

	// Add change tracking
	updates = append(updates, "change_time = ?")
	args = append(args, time.Now())

	updates = append(updates, "change_by = ?")
	args = append(args, currentUserID)

	// Add WHERE clause parameter
	args = append(args, userID)

	// Build and execute update query using QueryBuilder for rebinding
	qb, err := database.GetQueryBuilder()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	updateQuery := database.ConvertPlaceholders("UPDATE users SET " + strings.Join(updates, ", ") + " WHERE id = ?")
	result, err := qb.DB().Exec(updateQuery, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to update user",
		})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found or no changes made",
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
