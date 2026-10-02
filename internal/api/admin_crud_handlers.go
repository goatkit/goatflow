package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/services/adapter"
)

// Admin Users CRUD Handlers

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
		SELECT DISTINCT u.id, u.login, u.first_name, u.last_name, u.login as email
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

	if req.UserID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}
	actorID, ok := auditUserID(c)
	if !ok {
		return
	}

	// group_user has no unique key, so INSERT IGNORE would add a duplicate row on
	// every call; only grant 'rw' when the user has no 'rw' row in this group yet.
	var existing int
	err = db.QueryRow(database.ConvertPlaceholders(`
		SELECT COUNT(*) FROM group_user WHERE user_id = ? AND group_id = ? AND permission_key = 'rw'
	`), req.UserID, id).Scan(&existing)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add user to group"})
		return
	}
	if existing == 0 {
		_, err = db.Exec(database.ConvertPlaceholders(`
			INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
			VALUES (?, ?, 'rw', CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP, ?)
		`), req.UserID, id, actorID, actorID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add user to group"})
			return
		}
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
