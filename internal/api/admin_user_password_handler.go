package api

import (
	"crypto/rand"
	"errors"
	"math/big"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/sysconfig"
)

// maxGeneratedPasswordAttempts bounds how many random passwords a reset tries
// before giving up on a policy (e.g. a PasswordRegExp) they cannot satisfy.
const maxGeneratedPasswordAttempts = 100

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

	db, ok := adminUsersDB(c)
	if !ok {
		return
	}

	newPassword := req.Password
	if newPassword != "" {
		if !checkAdminSetPassword(c, db, newPassword, nil) {
			return
		}
	} else {
		// Generate one that satisfies the agent password policy.
		policy, err := sysconfig.LoadAgentPasswordPolicy(db)
		if err != nil {
			adminUsersFail(c, "Failed to load password policy", err)
			return
		}
		for range maxGeneratedPasswordAttempts {
			candidate, err := generateRandomPassword()
			if err != nil {
				adminUsersFail(c, "Failed to generate password", err)
				return
			}
			if policy.ValidatePassword(candidate) == nil {
				newPassword = candidate
				break
			}
		}
		if newPassword == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Could not generate a password matching the password policy; enter one",
			})
			return
		}
	}

	hashedPassword, err := auth.NewPasswordHasher().HashPassword(newPassword)
	if err != nil {
		c.JSON(passwordHashErrorStatus(err), gin.H{
			"success": false,
			"error":   "Failed to hash password: " + err.Error(),
		})
		return
	}

	res, err := db.Exec(database.ConvertPlaceholders(
		"UPDATE users SET pw = ?, change_time = CURRENT_TIMESTAMP WHERE id = ?"), hashedPassword, id)
	if err != nil {
		adminUsersFail(c, "Failed to update password", err)
		return
	}
	if n, err := res.RowsAffected(); err != nil {
		adminUsersFail(c, "Failed to update password", err)
		return
	} else if n == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found",
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

// generateRandomPassword returns a 16-character password from crypto/rand.
func generateRandomPassword() (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%"
	limit := big.NewInt(int64(len(charset)))
	b := make([]byte, 16)
	for i := range b {
		n, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", err
		}
		b[i] = charset[n.Int64()]
	}
	return string(b), nil
}

// passwordHashErrorStatus maps an auth.PasswordHasher error to an HTTP status:
// a password the configured algorithm cannot hash is the caller's fault.
func passwordHashErrorStatus(err error) int {
	if errors.Is(err, auth.ErrPasswordTooLong) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}
