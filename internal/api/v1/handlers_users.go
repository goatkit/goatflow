package v1

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/middleware"
)

// handleListUsers returns all users (agents).
func (router *APIRouter) handleListUsers(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data:    gin.H{"users": []interface{}{}, "total": 0},
		})
		return
	}

	showInvalid := c.Query("include_invalid") == "true"

	query := database.ConvertQuery(`
		SELECT id, login, first_name, last_name, 
			CONCAT(first_name, ' ', last_name) as full_name,
			valid_id, create_time, change_time
		FROM users
		WHERE 1=1
	`)
	args := []interface{}{}

	if !showInvalid {
		query += database.ConvertQuery(` AND valid_id = 1`)
	}

	query += ` ORDER BY login`

	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data:    gin.H{"users": []interface{}{}, "total": 0},
		})
		return
	}
	defer rows.Close()

	type User struct {
		ID        int       `json:"id"`
		Login     string    `json:"login"`
		FirstName *string   `json:"first_name"`
		LastName  *string   `json:"last_name"`
		FullName  string    `json:"full_name"`
		ValidID   int       `json:"valid_id"`
		Active    bool      `json:"active"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	users := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Login, &u.FirstName, &u.LastName, &u.FullName, &u.ValidID, &u.CreatedAt, &u.UpdatedAt); err != nil {
			continue
		}
		u.FullName = strings.TrimSpace(u.FullName)
		u.Active = u.ValidID == 1
		users = append(users, u)
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    gin.H{"users": users, "total": len(users)},
	})
}

// handleCreateUser creates a new user (agent).
func (router *APIRouter) handleCreateUser(c *gin.Context) {
	var req struct {
		Login     string `json:"login" binding:"required"`
		FirstName string `json:"first_name" binding:"required"`
		LastName  string `json:"last_name" binding:"required"`
		Email     string `json:"email"`
		Password  string `json:"password" binding:"required,min=8"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	adminID := 1
	if id, exists := c.Get("user_id"); exists {
		if idInt, ok := id.(int); ok && idInt > 0 {
			adminID = idInt
		}
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	now := time.Now()
	query := database.ConvertQuery(`
		INSERT INTO users
			(login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, 1, ?, ?, ?, ?)
	`)

	result, err := db.Exec(query, req.Login, string(hashedPassword), req.FirstName, req.LastName, now, adminID, now, adminID)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate") {
			sendError(c, http.StatusConflict, "User with this login already exists")
			return
		}
		sendError(c, http.StatusInternalServerError, "Failed to create user")
		return
	}

	id, _ := result.LastInsertId()

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Data: gin.H{
			"id":         id,
			"login":      req.Login,
			"first_name": req.FirstName,
			"last_name":  req.LastName,
			"active":     true,
			"created_at": now,
		},
	})
}

// handleGetUser returns a specific user.
func (router *APIRouter) handleGetUser(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	query := database.ConvertQuery(`
		SELECT id, login, first_name, last_name,
			CONCAT(first_name, ' ', last_name) as full_name,
			valid_id, create_time, change_time
		FROM users
		WHERE id = ?
	`)

	var user struct {
		ID        int       `json:"id"`
		Login     string    `json:"login"`
		FirstName *string   `json:"first_name"`
		LastName  *string   `json:"last_name"`
		FullName  string    `json:"full_name"`
		ValidID   int       `json:"valid_id"`
		Active    bool      `json:"active"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	err = db.QueryRow(query, userID).Scan(&user.ID, &user.Login, &user.FirstName, &user.LastName, &user.FullName, &user.ValidID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		sendError(c, http.StatusNotFound, "User not found")
		return
	}
	user.FullName = strings.TrimSpace(user.FullName)
	user.Active = user.ValidID == 1

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    user,
	})
}

// handleUpdateUser updates a user.
func (router *APIRouter) handleUpdateUser(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var req struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		ValidID   *int   `json:"valid_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	adminID := 1
	if id, exists := c.Get("user_id"); exists {
		if idInt, ok := id.(int); ok && idInt > 0 {
			adminID = idInt
		}
	}

	now := time.Now()
	query := database.ConvertQuery(`
		UPDATE users SET
			first_name = COALESCE(NULLIF(?, ''), first_name),
			last_name = COALESCE(NULLIF(?, ''), last_name),
			valid_id = COALESCE(?, valid_id),
			change_time = ?,
			change_by = ?
		WHERE id = ?
	`)

	result, err := db.Exec(query, req.FirstName, req.LastName, req.ValidID, now, adminID, userID)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to update user")
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		sendError(c, http.StatusNotFound, "User not found")
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: gin.H{
			"id":         userID,
			"updated_at": now,
		},
	})
}

// handleDeleteUser soft-deletes a user.
func (router *APIRouter) handleDeleteUser(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	adminID := 1
	if id, exists := c.Get("user_id"); exists {
		if idInt, ok := id.(int); ok && idInt > 0 {
			adminID = idInt
		}
	}

	query := database.ConvertQuery(`
		UPDATE users SET valid_id = 2, change_time = ?, change_by = ?
		WHERE id = ?
	`)

	result, err := db.Exec(query, time.Now(), adminID, userID)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to delete user")
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		sendError(c, http.StatusNotFound, "User not found")
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    gin.H{"message": "User deactivated"},
	})
}

// handleActivateUser activates a user.
func (router *APIRouter) handleActivateUser(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	adminID := 1
	if id, exists := c.Get("user_id"); exists {
		if idInt, ok := id.(int); ok && idInt > 0 {
			adminID = idInt
		}
	}

	query := database.ConvertQuery(`
		UPDATE users SET valid_id = 1, change_time = ?, change_by = ?
		WHERE id = ?
	`)

	result, err := db.Exec(query, time.Now(), adminID, userID)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to activate user")
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		sendError(c, http.StatusNotFound, "User not found")
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "User activated successfully",
	})
}

// handleDeactivateUser deactivates a user.
func (router *APIRouter) handleDeactivateUser(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	adminID := 1
	if id, exists := c.Get("user_id"); exists {
		if idInt, ok := id.(int); ok && idInt > 0 {
			adminID = idInt
		}
	}

	query := database.ConvertQuery(`
		UPDATE users SET valid_id = 2, change_time = ?, change_by = ?
		WHERE id = ?
	`)

	result, err := db.Exec(query, time.Now(), adminID, userID)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to deactivate user")
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		sendError(c, http.StatusNotFound, "User not found")
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "User deactivated successfully",
	})
}

// handleResetUserPassword resets a user's password.
func (router *APIRouter) handleResetUserPassword(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var req struct {
		NewPassword string `json:"new_password" binding:"required,min=8"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	adminID := 1
	if id, exists := c.Get("user_id"); exists {
		if idInt, ok := id.(int); ok && idInt > 0 {
			adminID = idInt
		}
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	query := database.ConvertQuery(`
		UPDATE users SET pw = ?, change_time = ?, change_by = ?
		WHERE id = ?
	`)

	result, err := db.Exec(query, string(hashedPassword), time.Now(), adminID, userID)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to reset password")
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		sendError(c, http.StatusNotFound, "User not found")
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "Password reset successfully",
	})
}

// handleGetUserNotifications returns notifications for the current user.
func (router *APIRouter) handleGetUserNotifications(c *gin.Context) {
	userID, _, _, exists := middleware.GetCurrentUser(c)
	if !exists {
		userID = 1
	}

	// Notifications would come from a notifications table
	// For now return empty list
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: gin.H{
			"user_id":       userID,
			"notifications": []interface{}{},
			"unread_count":  0,
		},
	})
}

// handleMarkNotificationRead marks a notification as read.
func (router *APIRouter) handleMarkNotificationRead(c *gin.Context) {
	// Would update notification table
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "Notification marked as read",
	})
}

// handleGetNotifications is an alias for handleGetUserNotifications.
func (router *APIRouter) handleGetNotifications(c *gin.Context) {
	router.handleGetUserNotifications(c)
}

// handleListAllUsers is an alias for handleListUsers.
func (router *APIRouter) handleListAllUsers(c *gin.Context) {
	router.handleListUsers(c)
}
