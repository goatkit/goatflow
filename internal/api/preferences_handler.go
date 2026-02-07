package api

import (
	"log"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/auth"
	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/i18n"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/service"
	"github.com/goatkit/goatflow/internal/sysconfig"
)

// HandleGetAvailableLanguages returns the list of supported languages (public, no auth required).
// Used on login pages to allow language selection before authentication.
func HandleGetAvailableLanguages(c *gin.Context) {
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

	// Get current language from cookie if set
	currentLang, _ := c.Cookie("goatflow_lang")

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"current":   currentLang,
		"available": languageList,
	})
}

// HandleSetPreLoginLanguage sets the language preference cookie (public, no auth required).
// This is used on login pages before the user authenticates.
func HandleSetPreLoginLanguage(c *gin.Context) {
	var request struct {
		Value string `json:"value"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	// Validate language is supported (empty is allowed - means browser default)
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

	// Set cookie (30 day expiry)
	if request.Value != "" {
		c.SetCookie("goatflow_lang", request.Value, 86400*30, "/", "", false, false)
	} else {
		c.SetCookie("goatflow_lang", "", -1, "/", "", false, false)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Language preference set",
	})
}

// HandleGetSessionTimeout retrieves the user's session timeout preference.
func HandleGetSessionTimeout(c *gin.Context) {
	// Get user ID from context (middleware sets "user_id" not "userID")
	if _, exists := c.Get("user_id"); !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
		})
		return
	}

	userID := GetUserIDFromCtx(c, 0)
	if userID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID",
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
	if _, exists := c.Get("user_id"); !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
		})
		return
	}

	userID := GetUserIDFromCtx(c, 0)
	if userID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID",
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
	if _, exists := c.Get("user_id"); !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
		})
		return
	}

	userID := GetUserIDFromCtx(c, 0)
	if userID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID",
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
	if _, exists := c.Get("user_id"); !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
		})
		return
	}

	userID := GetUserIDFromCtx(c, 0)
	if userID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID",
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
	if _, exists := c.Get("user_id"); !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
		})
		return
	}

	userID := GetUserIDFromCtxUint(c, 0)
	if userID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID",
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
	if _, exists := c.Get("user_id"); !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
		})
		return
	}

	userID := GetUserIDFromCtxUint(c, 0)
	if userID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID",
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

// getAvailableThemes discovers themes by scanning theme package directories.
// Checks both builtin/ (shipped themes) and .cache/ (extracted community themes).
func getAvailableThemes() []string {
	var themes []string

	// Scan builtin themes (shipped with GoatFlow)
	builtinDir := "static/themes/builtin"
	if entries, err := os.ReadDir(builtinDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				// Check for theme.css or theme.yaml to validate it's a theme
				themePath := builtinDir + "/" + entry.Name()
				if _, err := os.Stat(themePath + "/theme.css"); err == nil {
					themes = append(themes, entry.Name())
				}
			}
		}
	} else {
		log.Printf("Warning: could not read builtin themes directory: %v", err)
	}

	// Scan community themes (extracted from ZIP packages)
	cacheDir := "static/themes/.cache"
	if entries, err := os.ReadDir(cacheDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				themePath := cacheDir + "/" + entry.Name()
				if _, err := os.Stat(themePath + "/theme.css"); err == nil {
					themes = append(themes, entry.Name())
				}
			}
		}
	}
	// No warning for .cache - it may not exist yet

	// Fallback to known themes if nothing found
	if len(themes) == 0 {
		log.Printf("Warning: no themes found in theme directories, using defaults")
		return []string{"synthwave", "goatflow-classic", "seventies-vibes", "nineties-vibe"}
	}

	// Sort for consistent ordering
	sort.Strings(themes)
	return themes
}

// HandleGetAvailableThemes returns the list of available themes (public, no auth required).
// Used on login pages to allow theme selection before authentication.
func HandleGetAvailableThemes(c *gin.Context) {
	themes := getAvailableThemes()
	themeList := make([]gin.H, 0, len(themes))
	for _, theme := range themes {
		themeList = append(themeList, gin.H{
			"id":   theme,
			"name": theme,
		})
	}

	// Get current theme and mode from cookies if set
	currentTheme, _ := c.Cookie("goatflow_theme")
	currentMode, _ := c.Cookie("goatflow_mode")

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"current_theme": currentTheme,
		"current_mode":  currentMode,
		"available":     themeList,
	})
}

// HandleSetPreLoginTheme sets the theme and mode preference cookies (public, no auth required).
// This is used on login pages before the user authenticates.
func HandleSetPreLoginTheme(c *gin.Context) {
	var request struct {
		Theme string `json:"theme"`
		Mode  string `json:"mode"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	// Validate theme if provided
	if request.Theme != "" {
		valid := false
		for _, t := range getAvailableThemes() {
			if t == request.Theme {
				valid = true
				break
			}
		}
		if !valid {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Unsupported theme: " + request.Theme,
			})
			return
		}
	}

	// Validate mode if provided
	if request.Mode != "" && request.Mode != "light" && request.Mode != "dark" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Mode must be 'light' or 'dark'",
		})
		return
	}

	// Set theme cookie (30 day expiry)
	if request.Theme != "" {
		c.SetCookie("goatflow_theme", request.Theme, 86400*30, "/", "", false, false)
	}

	// Set mode cookie (30 day expiry)
	if request.Mode != "" {
		c.SetCookie("goatflow_mode", request.Mode, 86400*30, "/", "", false, false)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Theme preference set",
	})
}

// HandleGetTheme retrieves the user's theme preferences.
// Uses UserPreferencesService for agents/admins, CustomerPreferencesService for customers.
func HandleGetTheme(c *gin.Context) {
	if _, exists := c.Get("user_id"); !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
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

	var theme, mode string

	// Check if user is a customer (uses different preferences table)
	userRole := c.GetString("user_role")
	if userRole == "Customer" {
		username := c.GetString("username")
		if username == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Customer username not found",
			})
			return
		}
		customerPrefService := service.NewCustomerPreferencesService(db)
		theme = customerPrefService.GetTheme(username)
		mode = customerPrefService.GetThemeMode(username)
	} else {
		// Agent/Admin user - uses numeric ID
		userID := GetUserIDFromCtx(c, 0)
		if userID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid user ID",
			})
			return
		}
		userPrefService := service.NewUserPreferencesService(db)
		theme = userPrefService.GetTheme(userID)
		mode = userPrefService.GetThemeMode(userID)
	}

	// Build list of available themes for the UI
	themes := getAvailableThemes()
	themeList := make([]gin.H, 0, len(themes))
	for _, t := range themes {
		themeList = append(themeList, gin.H{
			"id":   t,
			"name": t,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"theme":     theme,
		"mode":      mode,
		"available": themeList,
	})
}

// HandleSetTheme sets the user's theme preferences and persists to database.
// Uses UserPreferencesService for agents/admins, CustomerPreferencesService for customers.
func HandleSetTheme(c *gin.Context) {
	if _, exists := c.Get("user_id"); !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
		})
		return
	}

	var request struct {
		Theme string `json:"theme"`
		Mode  string `json:"mode"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request format",
		})
		return
	}

	// Validate theme if provided
	if request.Theme != "" {
		valid := false
		for _, t := range getAvailableThemes() {
			if t == request.Theme {
				valid = true
				break
			}
		}
		if !valid {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Unsupported theme: " + request.Theme,
			})
			return
		}
	}

	// Validate mode if provided
	if request.Mode != "" && request.Mode != "light" && request.Mode != "dark" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Mode must be 'light' or 'dark'",
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

	// Check if user is a customer (uses different preferences table)
	userRole := c.GetString("user_role")
	if userRole == "Customer" {
		username := c.GetString("username")
		if username == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Customer username not found",
			})
			return
		}

		customerPrefService := service.NewCustomerPreferencesService(db)

		// Save theme preference to customer_preferences table
		if request.Theme != "" {
			if err := customerPrefService.SetTheme(username, request.Theme); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"error":   "Failed to save theme preference",
				})
				return
			}
			c.SetCookie("goatflow_theme", request.Theme, 86400*30, "/", "", false, false)
		}

		// Save mode preference to customer_preferences table
		if request.Mode != "" {
			if err := customerPrefService.SetThemeMode(username, request.Mode); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"error":   "Failed to save theme mode preference",
				})
				return
			}
			c.SetCookie("goatflow_mode", request.Mode, 86400*30, "/", "", false, false)
		}
	} else {
		// Agent/Admin user - uses numeric ID and user_preferences table
		userID := GetUserIDFromCtx(c, 0)
		if userID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid user ID",
			})
			return
		}

		userPrefService := service.NewUserPreferencesService(db)

		// Save theme preference to user_preferences table
		if request.Theme != "" {
			if err := userPrefService.SetTheme(userID, request.Theme); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"error":   "Failed to save theme preference",
				})
				return
			}
			c.SetCookie("goatflow_theme", request.Theme, 86400*30, "/", "", false, false)
		}

		// Save mode preference to user_preferences table
		if request.Mode != "" {
			if err := userPrefService.SetThemeMode(userID, request.Mode); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"error":   "Failed to save theme mode preference",
				})
				return
			}
			c.SetCookie("goatflow_mode", request.Mode, 86400*30, "/", "", false, false)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Theme preferences saved successfully",
	})
}

// HandleAgentPasswordForm renders the agent password change form.
func HandleAgentPasswordForm(c *gin.Context) {
	if _, exists := c.Get("user_id"); !exists {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.String(http.StatusInternalServerError, "Database connection error")
		return
	}

	// Load password policy from sysconfig
	policy, err := sysconfig.LoadAgentPasswordPolicy(db)
	if err != nil {
		log.Printf("Error loading agent password policy: %v", err)
		policy = sysconfig.DefaultAgentPasswordPolicy()
	}

	userID := GetUserIDFromCtxUint(c, 0)

	userRepo := repository.NewUserRepository(db)
	user, err := userRepo.GetByID(userID)
	if err != nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/password_form.pongo2", pongo2.Context{
		"Title":      "Change Password",
		"ActivePage": "profile",
		"Policy":     policy,
		"User": map[string]interface{}{
			"Login":     user.Login,
			"FirstName": user.FirstName,
			"LastName":  user.LastName,
			"Email":     user.Email,
		},
	})
}

// HandleAgentChangePassword processes the agent password change request.
func HandleAgentChangePassword(c *gin.Context) {
	if _, exists := c.Get("user_id"); !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Not authenticated",
		})
		return
	}

	userID := GetUserIDFromCtxUint(c, 0)
	if userID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID",
		})
		return
	}

	var request struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
		ConfirmPassword string `json:"confirm_password"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request format",
		})
		return
	}

	// Validate required fields
	if request.CurrentPassword == "" || request.NewPassword == "" || request.ConfirmPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "All password fields are required",
		})
		return
	}

	// Verify passwords match
	if request.NewPassword != request.ConfirmPassword {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Passwords do not match",
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

	// Get current password hash from database
	var currentHash string
	err = db.QueryRow(database.ConvertPlaceholders(`
		SELECT pw FROM users WHERE id = ? AND valid_id = 1
	`), userID).Scan(&currentHash)

	if err != nil {
		log.Printf("Error getting agent password for user %d: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to verify current password",
		})
		return
	}

	// Verify current password
	hasher := auth.NewPasswordHasher()
	if !hasher.VerifyPassword(request.CurrentPassword, currentHash) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Current password is incorrect",
		})
		return
	}

	// Check that new password is different from current
	if request.NewPassword == request.CurrentPassword {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "New password must be different from current password",
		})
		return
	}

	// Load and validate against password policy
	policy, err := sysconfig.LoadAgentPasswordPolicy(db)
	if err != nil {
		log.Printf("Error loading agent password policy: %v", err)
		policy = sysconfig.DefaultAgentPasswordPolicy()
	}

	if validationErr := policy.ValidatePassword(request.NewPassword); validationErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   validationErr.Message,
			"code":    validationErr.Code,
		})
		return
	}

	// Hash the new password
	newHash, err := hasher.HashPassword(request.NewPassword)
	if err != nil {
		log.Printf("Error hashing new password: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to process new password",
		})
		return
	}

	// Update password in database
	_, err = db.Exec(database.ConvertPlaceholders(`
		UPDATE users SET pw = ?, change_time = CURRENT_TIMESTAMP, change_by = ? WHERE id = ?
	`), newHash, userID, userID)

	if err != nil {
		log.Printf("Error updating agent password: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to update password",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Password changed successfully",
	})
}
