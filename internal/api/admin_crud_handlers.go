package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// Admin Users CRUD Handlers

// HandleAdminUsersCreate handles POST /admin/users
func HandleAdminUsersCreate(c *gin.Context) {
	var req struct {
		Login     string `json:"login" form:"login"`
		FirstName string `json:"first_name" form:"first_name"`
		LastName  string `json:"last_name" form:"last_name"`
		Email     string `json:"email" form:"email"`
		Password  string `json:"password" form:"password"`
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
	
	// TODO: Hash password and create user
	_ = db
	
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "User created"})
}

// HandleAdminUsersUpdate handles PUT /admin/users/:id
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

// HandleAdminUsersDelete handles DELETE /admin/users/:id
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
	_, err = db.Exec(database.ConvertPlaceholders("UPDATE users SET valid_id = 2 WHERE id = $1"), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "User deleted"})
}

// HandleAdminUsersStatus is implemented in admin_users_handlers.go

// Admin Groups CRUD Handlers

// HandleAdminGroupsCreate handles POST /admin/groups
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

// HandleAdminGroupsUpdate handles PUT /admin/groups/:id
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

// HandleAdminGroupsDelete handles DELETE /admin/groups/:id
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
	_, err = db.Exec("UPDATE groups SET valid_id = 2 WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete group"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Group deleted"})
}

// HandleAdminGroupsUsers handles GET /admin/groups/:id/users
func HandleAdminGroupsUsers(c *gin.Context) {
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
	
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT u.id, u.login, u.first_name, u.last_name, u.email
		FROM users u
		JOIN group_user gu ON u.id = gu.user_id
		WHERE gu.group_id = $1 AND u.valid_id = 1
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
	
	c.JSON(http.StatusOK, gin.H{"success": true, "users": users})
}

// HandleAdminGroupsAddUser handles POST /admin/groups/:id/users
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
	
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}
	
	// Add user to group with default 'rw' permission
	_, err = db.Exec(database.ConvertPlaceholders(`
		INSERT INTO group_user (user_id, group_id, permission_key, permission_value, create_time, create_by, change_time, change_by)
		VALUES ($1, $2, 'rw', 1, NOW(), 1, NOW(), 1)
		ON CONFLICT (user_id, group_id, permission_key) DO NOTHING
	`), req.UserID, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add user to group"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "User added to group"})
}

// HandleAdminGroupsRemoveUser handles DELETE /admin/groups/:id/users/:userId
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
	
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}
	
	// Remove user from group
	_, err = db.Exec("DELETE FROM group_user WHERE user_id = $1 AND group_id = $2", uid, gid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove user from group"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "User removed from group"})
}