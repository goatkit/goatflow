package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
)

// HandleDeleteUserAPI handles DELETE /api/v1/users/:id.
// This performs a soft delete by setting valid_id = 2 (OTRS pattern).
//
//	@Summary		Delete user
//	@Description	Soft delete a user (deactivate)
//	@Tags			Users
//	@Accept			json
//	@Produce		json
//	@Param			id	path		int	true	"User ID"
//	@Success		200	{object}	map[string]interface{}	"User deleted"
//	@Failure		401	{object}	map[string]interface{}	"Unauthorized"
//	@Failure		404	{object}	map[string]interface{}	"User not found"
//	@Security		BearerAuth
//	@Router			/users/{id} [delete]
func HandleDeleteUserAPI(c *gin.Context) {
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

	// Prevent deletion of system users
	if userID == 1 {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   "Cannot delete system user",
		})
		return
	}

	// Prevent self-deletion
	if currentUID, ok := currentUserID.(uint); ok && int(currentUID) == userID {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   "Cannot delete your own account",
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

	// Check if user exists and is not already deleted
	var currentValidID int
	checkQuery := database.ConvertPlaceholders(`
		SELECT valid_id FROM users WHERE id = ?
	`)
	err = db.QueryRow(checkQuery, userID).Scan(&currentValidID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found",
		})
		return
	}

	if currentValidID == 2 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "User is already deleted",
		})
		return
	}

	// Soft delete by setting valid_id = 2
	updateQuery := database.ConvertPlaceholders(`
		UPDATE users 
		SET valid_id = 2,
		    change_time = ?,
		    change_by = ?
		WHERE id = ?
	`)

	result, err := db.Exec(updateQuery, time.Now(), currentUserID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to delete user",
		})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found",
		})
		return
	}

	// Return 204 No Content on successful deletion
	c.Status(http.StatusNoContent)
}

// Additional helper handlers for user groups

// HandleGetUserGroupsAPI handles GET /api/v1/users/:id/groups.
//
//	@Summary		Get user groups
//	@Description	List groups a user belongs to
//	@Tags			Users
//	@Accept			json
//	@Produce		json
//	@Param			id	path		int	true	"User ID"
//	@Success		200	{object}	map[string]interface{}	"List of groups"
//	@Failure		401	{object}	map[string]interface{}	"Unauthorized"
//	@Failure		404	{object}	map[string]interface{}	"User not found"
//	@Security		BearerAuth
//	@Router			/users/{id}/groups [get]
func HandleGetUserGroupsAPI(c *gin.Context) {
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

	// Get user's groups
	query := database.ConvertPlaceholders(`
		SELECT 
			g.id,
			g.name,
			ug.permission_key,
			ug.permission_value
		FROM groups g
		INNER JOIN user_groups ug ON g.id = ug.group_id
		WHERE ug.user_id = ?
		ORDER BY g.name
	`)

	rows, err := db.Query(query, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to retrieve user groups",
		})
		return
	}
	defer rows.Close()

	groups := []map[string]interface{}{}
	for rows.Next() {
		var groupID int
		var groupName string
		var permKey, permValue interface{}

		if err := rows.Scan(&groupID, &groupName, &permKey, &permValue); err != nil {
			continue
		}

		group := map[string]interface{}{
			"id":   groupID,
			"name": groupName,
		}

		if permKey != nil {
			group["permission_key"] = permKey
		}
		if permValue != nil {
			group["permission_value"] = permValue
		}

		groups = append(groups, group)
	}
	_ = rows.Err() //nolint:errcheck // Check for iteration errors

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    groups,
	})
}

// HandleAddUserToGroupAPI handles POST /api/v1/users/:id/groups.
//
//	@Summary		Add user to group
//	@Description	Add a user to a group
//	@Tags			Users
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int		true	"User ID"
//	@Param			group	body		object	true	"Group data (group_id, permission)"
//	@Success		200		{object}	map[string]interface{}	"User added to group"
//	@Failure		400		{object}	map[string]interface{}	"Invalid request"
//	@Failure		401		{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/users/{id}/groups [post]
func HandleAddUserToGroupAPI(c *gin.Context) {
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

	// Parse request body
	var req struct {
		GroupID     int    `json:"group_id" binding:"required"`
		Permissions string `json:"permissions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Default permissions
	if req.Permissions == "" {
		req.Permissions = "rw"
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

	// Check if association already exists
	var associationExists int
	checkQuery := database.ConvertPlaceholders(`
		SELECT 1 FROM user_groups 
		WHERE user_id = ? AND group_id = ?
	`)
	row := db.QueryRow(checkQuery, userID, req.GroupID)
	_ = row.Scan(&associationExists) //nolint:errcheck // Defaults to 0

	if associationExists == 1 {
		// Update existing association
		updateQuery := database.ConvertPlaceholders(`
			UPDATE user_groups 
			SET permission_key = ?,
			    permission_value = 1,
			    change_time = ?,
			    change_by = ?
			WHERE user_id = ? AND group_id = ?
		`)
		_, err = db.Exec(updateQuery, req.Permissions, time.Now(), currentUserID, userID, req.GroupID)
	} else {
		// Create new association
		insertQuery := database.ConvertPlaceholders(`
			INSERT INTO user_groups (
				user_id, group_id, permission_key, permission_value,
				create_time, create_by, change_time, change_by
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`)
		_, err = db.Exec(insertQuery,
			userID, req.GroupID, req.Permissions, 1,
			time.Now(), currentUserID, time.Now(), currentUserID,
		)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to add user to group",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User added to group successfully",
	})
}

// HandleRemoveUserFromGroupAPI handles DELETE /api/v1/users/:id/groups/:group_id.
//
//	@Summary		Remove user from group
//	@Description	Remove a user from a group
//	@Tags			Users
//	@Accept			json
//	@Produce		json
//	@Param			id			path		int	true	"User ID"
//	@Param			group_id	path		int	true	"Group ID"
//	@Success		200			{object}	map[string]interface{}	"User removed from group"
//	@Failure		401			{object}	map[string]interface{}	"Unauthorized"
//	@Failure		404			{object}	map[string]interface{}	"Not found"
//	@Security		BearerAuth
//	@Router			/users/{id}/groups/{group_id} [delete]
func HandleRemoveUserFromGroupAPI(c *gin.Context) {
	// Check authentication
	_, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Authentication required",
		})
		return
	}

	// Get user ID and group ID from URL
	userIDStr := c.Param("id")
	groupIDStr := c.Param("group_id")

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID",
		})
		return
	}

	groupID, err := strconv.Atoi(groupIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid group ID",
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

	// Delete the association
	deleteQuery := database.ConvertPlaceholders(`
		DELETE FROM user_groups 
		WHERE user_id = ? AND group_id = ?
	`)

	result, err := db.Exec(deleteQuery, userID, groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to remove user from group",
		})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User group association not found",
		})
		return
	}

	c.Status(http.StatusNoContent)
}
