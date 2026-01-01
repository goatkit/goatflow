package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/service"
)

// HandleGetSessionTimeout retrieves the user's session timeout preference.
func HandleGetSessionTimeout(c *gin.Context) {
	// Get user ID from context (middleware sets "user_id" not "userID")
	userIDInterface, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
		})
		return
	}

	var userID int
	switch v := userIDInterface.(type) {
	case uint:
		userID = int(v)
	case int:
		userID = v
	case string:
		var err error
		userID, err = strconv.Atoi(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid user ID",
			})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID type",
		})
		return
	}

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection error",
		})
		return
	}

	// Get preference service
	prefService := service.NewUserPreferencesService(db)

	// Get session timeout preference
	timeout := prefService.GetSessionTimeout(userID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"value":   timeout,
	})
}

// HandleSetSessionTimeout sets the user's session timeout preference.
func HandleSetSessionTimeout(c *gin.Context) {
	// Get user ID from context (middleware sets "user_id" not "userID")
	userIDInterface, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
		})
		return
	}

	var userID int
	switch v := userIDInterface.(type) {
	case uint:
		userID = int(v)
	case int:
		userID = v
	case string:
		var err error
		userID, err = strconv.Atoi(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid user ID",
			})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID type",
		})
		return
	}

	// Parse request body
	var request struct {
		Value int `json:"value"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request format",
		})
		return
	}

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection error",
		})
		return
	}

	// Get preference service
	prefService := service.NewUserPreferencesService(db)

	// Set session timeout preference
	if err := prefService.SetSessionTimeout(userID, request.Value); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to save preference",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Session timeout preference saved successfully",
	})
}
