package api

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/gotrs-io/gotrs-ce/internal/services/adapter"
)

// Admin Users CRUD Handlers

// HandleAdminUsersCreate handles POST /admin/users.
func HandleAdminUsersCreate(c *gin.Context) {
	var req struct {
		Login     string   `json:"login" form:"login"`
		FirstName string   `json:"first_name" form:"first_name"`
		LastName  string   `json:"last_name" form:"last_name"`
		Title     string   `json:"title" form:"title"`
		Email     string   `json:"email" form:"email"`
		Password  string   `json:"password" form:"password"`
		ValidID   int      `json:"valid_id" form:"valid_id"`
		Groups    []string `json:"groups" form:"groups"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	login := strings.TrimSpace(req.Login)
	password := strings.TrimSpace(req.Password)
	firstName := strings.TrimSpace(req.FirstName)
	lastName := strings.TrimSpace(req.LastName)
	title := strings.TrimSpace(req.Title)
	if login == "" || password == "" || firstName == "" || lastName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required fields"})
		return
	}

	validID := req.ValidID
	if validID == 0 {
		validID = 1
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Check for existing user with same login
	var exists bool
	existsQuery := "SELECT EXISTS(SELECT 1 FROM users WHERE login = ?)"
	if err := db.QueryRow(database.ConvertPlaceholders(existsQuery), login).Scan(&exists); err == nil && exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User already exists"})
		return
	}

	user := &models.User{
		Login:      login,
		Email:      login,
		Password:   "",
		Title:      title,
		FirstName:  firstName,
		LastName:   lastName,
		ValidID:    validID,
		CreateBy:   1,
		ChangeBy:   1,
		CreateTime: time.Now(),
		ChangeTime: time.Now(),
	}

	if err := user.SetPassword(password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
		return
	}

	userRepo := repository.NewUserRepository(db)
	if err := userRepo.Create(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create user: %v", err)})
		return
	}

	groupIDs := req.Groups
	if len(groupIDs) == 0 {
		groupIDs = c.PostFormArray("groups")
	}

	if len(groupIDs) > 0 {
		groupRepo := repository.NewGroupRepository(db)
		for _, rawID := range groupIDs {
			idStr := strings.TrimSpace(rawID)
			if idStr == "" {
				continue
			}
			groupID, convErr := strconv.Atoi(idStr)
			if convErr != nil {
				// Cleanup on error - log but don't fail on cleanup errors
				if _, err := db.Exec(database.ConvertPlaceholders("DELETE FROM group_user WHERE user_id = ?"), user.ID); err != nil {
					log.Printf("cleanup error: %v", err)
				}
				if _, err := db.Exec(database.ConvertPlaceholders("DELETE FROM users WHERE id = ?"), user.ID); err != nil {
					log.Printf("cleanup error: %v", err)
				}
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
				return
			}
			if err := groupRepo.AddUserToGroup(user.ID, uint(groupID)); err != nil {
				// Cleanup on error - log but don't fail on cleanup errors
				if _, err := db.Exec(database.ConvertPlaceholders("DELETE FROM group_user WHERE user_id = ?"), user.ID); err != nil {
					log.Printf("cleanup error: %v", err)
				}
				if _, err := db.Exec(database.ConvertPlaceholders("DELETE FROM users WHERE id = ?"), user.ID); err != nil {
					log.Printf("cleanup error: %v", err)
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign groups"})
				return
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "User created", "user_id": user.ID})
}

// HandleAdminUsersUpdate handles PUT /admin/users/:id.
func HandleAdminUsersUpdate(c *gin.Context) {
	userID := c.Param("id")
	id, err := strconv.Atoi(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req struct {
		Login     string `json:"login" form:"login"`
		FirstName string `json:"first_name" form:"first_name"`
		LastName  string `json:"last_name" form:"last_name"`
		Email     string `json:"email" form:"email"`
		ValidID   int    `json:"valid_id" form:"valid_id"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// TODO: Update user
	_ = db
	_ = id

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "User updated"})
}

// HandleAdminUsersDelete handles DELETE /admin/users/:id.
func HandleAdminUsersDelete(c *gin.Context) {
	userID := c.Param("id")
	id, err := strconv.Atoi(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Soft delete - set valid_id = 2
	_, err = db.Exec(database.ConvertPlaceholders("UPDATE users SET valid_id = 2 WHERE id = ?"), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "User deleted"})
}

// HandleAdminUsersStatus is implemented in admin_users_handlers.go

// HandleAdminUsersList handles GET /admin/users (JSON API).
func HandleAdminUsersList(c *gin.Context) {
	db, err := adapter.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT id, login, first_name, last_name, valid_id
		FROM users 
		WHERE valid_id = 1
		ORDER BY login
	`))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
		return
	}
	defer rows.Close()

	var users []gin.H
	for rows.Next() {
		var user struct {
			ID        int    `json:"id"`
			Login     string `json:"login"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			ValidID   int    `json:"valid_id"`
		}
		if err := rows.Scan(&user.ID, &user.Login, &user.FirstName, &user.LastName, &user.ValidID); err != nil {
			continue
		}
		users = append(users, gin.H{
			"id":         user.ID,
			"login":      user.Login,
			"first_name": user.FirstName,
			"last_name":  user.LastName,
			"valid_id":   user.ValidID,
		})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error iterating users"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "users": users})
}

// Admin Groups CRUD Handlers

// HandleAdminGroupsCreate handles POST /admin/groups.
func HandleAdminGroupsCreate(c *gin.Context) {
	var req struct {
		Name     string `json:"name" form:"name"`
		Comments string `json:"comments" form:"comments"`
		ValidID  int    `json:"valid_id" form:"valid_id"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// TODO: Create group
	_ = db

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Group created"})
}

// HandleAdminGroupsUpdate handles PUT /admin/groups/:id.
func HandleAdminGroupsUpdate(c *gin.Context) {
	groupID := c.Param("id")
	id, err := strconv.Atoi(groupID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	var req struct {
		Name     string `json:"name" form:"name"`
		Comments string `json:"comments" form:"comments"`
		ValidID  int    `json:"valid_id" form:"valid_id"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// TODO: Update group
	_ = db
	_ = id

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Group updated"})
}

// HandleAdminGroupsDelete handles DELETE /admin/groups/:id.
func HandleAdminGroupsDelete(c *gin.Context) {
	groupID := c.Param("id")
	id, err := strconv.Atoi(groupID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Soft delete - set valid_id = 2
	_, err = db.Exec(database.ConvertPlaceholders("UPDATE groups SET valid_id = 2 WHERE id = ?"), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete group"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Group deleted"})
}

// HandleAdminGroupsUsers handles GET /admin/groups/:id/users.
func HandleAdminGroupsUsers(c *gin.Context) {
	groupID := c.Param("id")
	id, err := strconv.Atoi(groupID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	db, err := adapter.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT u.id, u.login, u.first_name, u.last_name, u.login as email
		FROM users u
		JOIN group_user gu ON u.id = gu.user_id
		WHERE gu.group_id = ? AND u.valid_id = 1
		ORDER BY u.login
	`), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
		return
	}
	defer rows.Close()

	var users []gin.H
	for rows.Next() {
		var user struct {
			ID        int    `json:"id"`
			Login     string `json:"login"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Email     string `json:"email"`
		}
		if err := rows.Scan(&user.ID, &user.Login, &user.FirstName, &user.LastName, &user.Email); err != nil {
			continue
		}
		users = append(users, gin.H{
			"id":         user.ID,
			"login":      user.Login,
			"first_name": user.FirstName,
			"last_name":  user.LastName,
			"email":      user.Email,
		})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error iterating users"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "users": users})
}

// HandleAdminGroupsAddUser handles POST /admin/groups/:id/users.
//
//nolint:dupl // Similar boilerplate to queue_api_handlers.HandleAPIQueueStatus but different logic
func HandleAdminGroupsAddUser(c *gin.Context) {
	groupID := c.Param("id")
	id, err := strconv.Atoi(groupID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	var req struct {
		UserID int `json:"user_id" form:"user_id"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	db, err := adapter.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Add user to group with default 'rw' permission (OTRS schema)
	_, err = db.Exec(database.ConvertPlaceholders(`
		INSERT IGNORE INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'rw', NOW(), 1, NOW(), 1)
	`), req.UserID, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add user to group"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "User added to group"})
}

// HandleAdminGroupsRemoveUser handles DELETE /admin/groups/:id/users/:userId.
func HandleAdminGroupsRemoveUser(c *gin.Context) {
	groupID := c.Param("id")
	userID := c.Param("userId")

	gid, err := strconv.Atoi(groupID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	uid, err := strconv.Atoi(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	dbService, err := adapter.GetDatabase()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}
	db := dbService.GetDB()

	// Remove user from group
	_, err = db.Exec(database.ConvertPlaceholders("DELETE FROM group_user WHERE user_id = ? AND group_id = ?"), uid, gid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove user from group"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "User removed from group"})
}
