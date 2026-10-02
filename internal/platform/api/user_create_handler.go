package api

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/database"
)

// CreateUserRequest represents the request to create a new user.
type CreateUserRequest struct {
	Login     string `json:"login" binding:"required"`
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=8"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	ValidID   int    `json:"valid_id"`
	Groups    []int  `json:"groups"` // Optional group IDs to assign
}

// HandleCreateUserAPI handles POST /api/v1/users.
//
//	@Summary		Create user
//	@Description	Create a new agent user
//	@Tags			Users
//	@Accept			json
//	@Produce		json
//	@Param			user	body		object	true	"User data (login, email, first_name, last_name, password)"
//	@Success		201		{object}	map[string]interface{}	"Created user"
//	@Failure		400		{object}	map[string]interface{}	"Invalid request"
//	@Failure		401		{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/users [post]
func HandleCreateUserAPI(c *gin.Context) {
	// Check authentication
	currentUserID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Authentication required",
		})
		return
	}

	// Parse request
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Validate login doesn't contain special characters
	if strings.ContainsAny(req.Login, " @#$%^&*()+=[]{}|\\:;\"'<>,.?/") {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Login contains invalid characters",
		})
		return
	}

	// Default valid_id to 1 (valid)
	if req.ValidID == 0 {
		req.ValidID = 1
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

	// Check if login already exists
	var existingID int
	checkQuery := database.ConvertPlaceholders(`
		SELECT id FROM users WHERE login = ?
	`)
	err = db.QueryRow(checkQuery, req.Login).Scan(&existingID)
	if err != sql.ErrNoRows {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"error":   "Login already exists",
		})
		return
	}

	// Every requested group must exist before anything is written; a bad id is a
	// client error, not a reason to silently drop the membership.
	for _, groupID := range req.Groups {
		var found int
		err = db.QueryRow(database.ConvertPlaceholders(`SELECT id FROM groups WHERE id = ?`), groupID).Scan(&found)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   fmt.Sprintf("Group %d does not exist", groupID),
			})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to validate groups",
			})
			return
		}
	}

	// Hash the password
	hashedPassword, err := auth.NewPasswordHasher().HashPassword(req.Password)
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

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to start transaction",
		})
		return
	}
	defer func() { _ = tx.Rollback() }()

	// Insert user. The OTRS users table has no email column; the agent's address
	// is the UserEmail preference.
	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO users (
			login, pw, first_name, last_name, valid_id,
			create_time, create_by, change_time, change_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`)

	now := time.Now()
	adapter := database.GetAdapter()
	newUserID64, err := adapter.InsertWithReturningTx(
		tx,
		insertQuery,
		req.Login,
		hashedPassword,
		req.FirstName,
		req.LastName,
		req.ValidID,
		now,
		currentUserID,
		now,
		currentUserID,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to create user",
		})
		return
	}

	newUserID := int(newUserID64)

	if _, err = tx.Exec(database.ConvertPlaceholders(`
		INSERT INTO user_preferences (user_id, preferences_key, preferences_value) VALUES (?, 'UserEmail', ?)
	`), newUserID, []byte(req.Email)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to create user",
		})
		return
	}

	// Membership lives in group_user; one row per permission_key. 'rw' implies
	// every other OTRS permission.
	groupInsertQuery := database.ConvertPlaceholders(`
		INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'rw', ?, ?, ?, ?)
	`)
	groupIDs := make([]int, 0, len(req.Groups))
	seen := make(map[int]bool, len(req.Groups))
	for _, groupID := range req.Groups {
		if seen[groupID] {
			continue
		}
		seen[groupID] = true
		if _, err = tx.Exec(groupInsertQuery, newUserID, groupID, now, currentUserID, now, currentUserID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to assign groups",
			})
			return
		}
		groupIDs = append(groupIDs, groupID)
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to complete user creation",
		})
		return
	}

	// Return created user (without password)
	response := gin.H{
		"id":         newUserID,
		"login":      req.Login,
		"email":      req.Email,
		"valid_id":   req.ValidID,
		"valid":      req.ValidID == 1,
		"groups":     groupIDs,
		"created_at": now.Format("2006-01-02T15:04:05Z"),
	}

	if req.FirstName != "" {
		response["first_name"] = req.FirstName
	}
	if req.LastName != "" {
		response["last_name"] = req.LastName
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    response,
		"message": "User created successfully",
	})
}
