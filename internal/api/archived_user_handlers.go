// ARCHIVED: Old user management handlers - replaced by dynamic module system
// Kept for reference only - DO NOT USE
// The /admin/users routes now use the dynamic module system via internal/components/dynamic/

package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// ARCHIVED - replaced by dynamic module
// handleCreateUser creates a new user
func handleCreateUser_ARCHIVED(c *gin.Context) {
	var user models.User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Set default values
	if user.ValidID == 0 {
		user.ValidID = 1 // Active by default
	}
	user.CreateTime = time.Now()
	user.ChangeTime = time.Now()
	user.CreateBy = 1 // System user
	user.ChangeBy = 1

	// Insert user
	query := `
		INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, change_time, create_by, change_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id`

	err = db.QueryRow(query,
		user.Login,
		user.Password, // In production, this should be hashed
		user.FirstName,
		user.LastName,
		user.ValidID,
		user.CreateTime,
		user.ChangeTime,
		user.CreateBy,
		user.ChangeBy,
	).Scan(&user.ID)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			c.JSON(http.StatusConflict, gin.H{"error": "User with this login already exists"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    user,
	})
}

// ARCHIVED - replaced by dynamic module
// handleGetUser returns user details
func handleGetUser_ARCHIVED(c *gin.Context) {
	userID := c.Param("id")

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Query user details
	query := `
		SELECT id, login, first_name, last_name, valid_id, create_time, change_time
		FROM users
		WHERE id = $1`

	var user models.User
	var firstName, lastName sql.NullString
	err = db.QueryRow(query, userID).Scan(
		&user.ID,
		&user.Login,
		&firstName,
		&lastName,
		&user.ValidID,
		&user.CreateTime,
		&user.ChangeTime,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	// Handle nullable fields
	if firstName.Valid {
		user.FirstName = firstName.String
	}
	if lastName.Valid {
		user.LastName = lastName.String
	}

	// Get user's groups
	groupQuery := `
		SELECT g.id, g.name
		FROM groups g
		JOIN user_groups ug ON g.id = ug.group_id
		WHERE ug.user_id = $1`

	rows, err := db.Query(groupQuery, userID)
	if err == nil {
		defer rows.Close()
		groups := []gin.H{}
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
		// user.Groups = groups // Type mismatch - commented out in archived code
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    user,
	})
}

// ARCHIVED - replaced by dynamic module
// handleUpdateUser updates a user
func handleUpdateUser_ARCHIVED(c *gin.Context) {
	userID := c.Param("id")
	
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Build update query dynamically
	setClauses := []string{"change_time = CURRENT_TIMESTAMP", "change_by = 1"}
	args := []interface{}{}
	argCount := 1

	for field, value := range updates {
		switch field {
		case "login":
			setClauses = append(setClauses, fmt.Sprintf("login = $%d", argCount))
			args = append(args, value)
			argCount++
		case "first_name":
			setClauses = append(setClauses, fmt.Sprintf("first_name = $%d", argCount))
			args = append(args, value)
			argCount++
		case "last_name":
			setClauses = append(setClauses, fmt.Sprintf("last_name = $%d", argCount))
			args = append(args, value)
			argCount++
		case "valid_id":
			setClauses = append(setClauses, fmt.Sprintf("valid_id = $%d", argCount))
			args = append(args, value)
			argCount++
		case "pw", "password":
			// In production, this should be hashed
			setClauses = append(setClauses, fmt.Sprintf("pw = $%d", argCount))
			args = append(args, value)
			argCount++
		}
	}

	args = append(args, userID)
	query := fmt.Sprintf("UPDATE users SET %s WHERE id = $%d",
		strings.Join(setClauses, ", "), argCount)

	result, err := db.Exec(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User updated successfully",
	})
}

// ARCHIVED - replaced by dynamic module
// handleDeleteUser deletes a user
func handleDeleteUser_ARCHIVED(c *gin.Context) {
	userID := c.Param("id")

	// Don't allow deletion of system users
	if userID == "1" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot delete system user"})
		return
	}

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Soft delete by setting valid_id to 2 (invalid)
	query := "UPDATE users SET valid_id = 2, change_time = CURRENT_TIMESTAMP WHERE id = $1"
	result, err := db.Exec(query, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User deleted successfully",
	})
}

// ARCHIVED - replaced by dynamic module
// handleUpdateUserStatus updates a user's active/inactive status
func handleUpdateUserStatus_ARCHIVED(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req struct {
		Active bool `json:"active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Update valid_id: 1 = active, 2 = inactive
	validID := 2
	if req.Active {
		validID = 1
	}

	query := "UPDATE users SET valid_id = $1, change_time = CURRENT_TIMESTAMP WHERE id = $2"
	result, err := db.Exec(query, validID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("User %s successfully", map[bool]string{true: "activated", false: "deactivated"}[req.Active]),
	})
}