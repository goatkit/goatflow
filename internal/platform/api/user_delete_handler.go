package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/database"
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
