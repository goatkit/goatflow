package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/i18n"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
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

// HandleGetLanguage retrieves the user's language preference.
func HandleGetLanguage(c *gin.Context) {
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

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection error",
		})
		return
	}

	prefService := service.NewUserPreferencesService(db)
	lang := prefService.GetLanguage(userID)

	// Build list of available languages with display names for the UI
	availableLanguages := i18n.GetInstance().GetSupportedLanguages()
	languageList := make([]gin.H, 0, len(availableLanguages))
	for _, code := range availableLanguages {
		if config, exists := i18n.GetLanguageConfig(code); exists {
			languageList = append(languageList, gin.H{
				"code":        code,
				"name":        config.Name,
				"native_name": config.NativeName,
			})
		} else {
			languageList = append(languageList, gin.H{
				"code":        code,
				"name":        code,
				"native_name": code,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"value":     lang,
		"available": languageList,
	})
}

// HandleSetLanguage sets the user's language preference.
func HandleSetLanguage(c *gin.Context) {
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

	var request struct {
		Value string `json:"value"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request format",
		})
		return
	}

	// Validate language is supported (empty is allowed - means system default)
	if request.Value != "" {
		instance := i18n.GetInstance()
		supported := false
		for _, lang := range instance.GetSupportedLanguages() {
			if lang == request.Value {
				supported = true
				break
			}
		}
		if !supported {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Unsupported language: " + request.Value,
			})
			return
		}
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection error",
		})
		return
	}

	prefService := service.NewUserPreferencesService(db)

	if err := prefService.SetLanguage(userID, request.Value); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to save preference",
		})
		return
	}

	// Also set/clear cookie to reflect preference immediately
	if request.Value != "" {
		c.SetCookie("lang", request.Value, 86400*30, "/", "", false, true)
	} else {
		c.SetCookie("lang", "", -1, "/", "", false, true)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Language preference saved successfully",
	})
}

// HandleGetProfile retrieves the current user's profile information.
func HandleGetProfile(c *gin.Context) {
	userIDInterface, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
		})
		return
	}

	var userID uint
	switch v := userIDInterface.(type) {
	case uint:
		userID = v
	case int:
		userID = uint(v)
	case int64:
		userID = uint(v)
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID type",
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection error",
		})
		return
	}

	userRepo := repository.NewUserRepository(db)
	user, err := userRepo.GetByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found",
		})
		return
	}

	// TODO: When LDAP/OAuth integration is added, check user source here
	// to determine if profile fields are editable
	// For now, assume all users can edit their profile
	editable := true

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"profile": gin.H{
			"first_name": user.FirstName,
			"last_name":  user.LastName,
			"title":      user.Title,
			"login":      user.Login,
			"email":      user.Email,
		},
		"editable": editable,
	})
}

// HandleUpdateProfile updates the current user's profile information.
func HandleUpdateProfile(c *gin.Context) {
	userIDInterface, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
		})
		return
	}

	var userID uint
	switch v := userIDInterface.(type) {
	case uint:
		userID = v
	case int:
		userID = uint(v)
	case int64:
		userID = uint(v)
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID type",
		})
		return
	}

	var request struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Title     string `json:"title"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request format",
		})
		return
	}

	// Validate required fields
	if request.FirstName == "" || request.LastName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "First name and last name are required",
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection error",
		})
		return
	}

	userRepo := repository.NewUserRepository(db)

	// TODO: When LDAP/OAuth integration is added, check user source here
	// to prevent edits for externally managed users

	now := time.Now()
	if err := userRepo.UpdateProfile(userID, request.FirstName, request.LastName, request.Title, userID, now); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to update profile",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Profile updated successfully",
	})
}
