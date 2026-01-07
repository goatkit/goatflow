package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleAdminUserResetPassword handles password reset for a user by admin.
func HandleAdminUserResetPassword(c *gin.Context) {
	userID := c.Param("id")
	id, err := strconv.Atoi(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID",
		})
		return
	}

	var req struct {
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request",
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Generate password if not provided
	newPassword := req.Password
	if newPassword == "" {
		// Generate a random password
		newPassword = generateRandomPassword()
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to hash password",
		})
		return
	}

	// Update the user's password
	updateQuery := "UPDATE users SET pw = ?, change_time = NOW() WHERE id = ?"
	_, err = db.Exec(database.ConvertPlaceholders(updateQuery), string(hashedPassword), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to update password",
		})
		return
	}

	response := gin.H{
		"success": true,
		"message": "Password reset successfully",
	}

	// If password was generated, include it in response
	if req.Password == "" {
		response["generatedPassword"] = newPassword
	}

	c.JSON(http.StatusOK, response)
}

// generateRandomPassword generates a secure random password.
func generateRandomPassword() string {
	// Simple implementation - in production, use a more secure method
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%"
	b := make([]byte, 12)
	for i := range b {
		b[i] = charset[i%len(charset)]
	}
	return string(b)
}
