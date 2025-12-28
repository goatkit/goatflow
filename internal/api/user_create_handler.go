package api

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"golang.org/x/crypto/bcrypt"
)

// CreateUserRequest represents the request to create a new user
type CreateUserRequest struct {
	Login     string `json:"login" binding:"required"`
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=8"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	ValidID   int    `json:"valid_id"`
	Groups    []int  `json:"groups"` // Optional group IDs to assign
}

// HandleCreateUserAPI handles POST /api/v1/users
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
		SELECT id FROM users WHERE login = $1
	`)
	err = db.QueryRow(checkQuery, req.Login).Scan(&existingID)
	if err != sql.ErrNoRows {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"error":   "Login already exists",
		})
		return
	}

	// Check if email already exists
	checkEmailQuery := database.ConvertPlaceholders(`
		SELECT id FROM users WHERE email = $1
	`)
	err = db.QueryRow(checkEmailQuery, req.Email).Scan(&existingID)
	if err != sql.ErrNoRows {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"error":   "Email already exists",
		})
		return
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to process password",
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
	defer tx.Rollback()

	// Insert user using adapter for cross-database compatibility
	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO users (
			login, 
			pw, 
			email, 
			first_name, 
			last_name, 
			valid_id,
			create_time,
			create_by,
			change_time,
			change_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`)

	adapter := database.GetAdapter()
	newUserID64, err := adapter.InsertWithReturningTx(
		tx,
		insertQuery,
		req.Login,
		string(hashedPassword),
		req.Email,
		sql.NullString{String: req.FirstName, Valid: req.FirstName != ""},
		sql.NullString{String: req.LastName, Valid: req.LastName != ""},
		req.ValidID,
		time.Now(),
		currentUserID,
		time.Now(),
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

	// Add user to groups if specified
	if len(req.Groups) > 0 {
		for _, groupID := range req.Groups {
			groupInsertQuery := database.ConvertPlaceholders(`
				INSERT INTO user_groups (
					user_id,
					group_id,
					permission_key,
					permission_value,
					create_time,
					create_by,
					change_time,
					change_by
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			`)

			_, err = tx.Exec(
				groupInsertQuery,
				newUserID,
				groupID,
				"rw", // Default permission
				1,    // Default value
				time.Now(),
				currentUserID,
				time.Now(),
				currentUserID,
			)

			if err != nil {
				// Log error but don't fail the whole operation
				// Group assignment is optional
				continue
			}
		}
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
		"created_at": time.Now().Format("2006-01-02T15:04:05Z"),
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
