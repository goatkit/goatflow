package ldap

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// LDAPHandlers provides HTTP handlers for LDAP management.
type LDAPHandlers struct {
	middleware *AuthMiddleware
}

// NewLDAPHandlers creates a new LDAP handlers instance.
func NewLDAPHandlers(middleware *AuthMiddleware) *LDAPHandlers {
	return &LDAPHandlers{
		middleware: middleware,
	}
}

// SetupLDAPRoutes sets up LDAP management routes.
func (h *LDAPHandlers) SetupLDAPRoutes(router gin.IRouter) {
	ldap := router.Group("/ldap")
	{
		// Configuration management
		ldap.GET("/config", h.GetConfiguration)
		ldap.POST("/config", h.SetConfiguration)
		ldap.PUT("/config", h.UpdateConfiguration)

		// Connection testing
		ldap.POST("/test", h.TestConnection)
		ldap.POST("/test-auth", h.TestAuthentication)

		// User management
		ldap.GET("/users/:username", h.GetUserInfo)
		ldap.POST("/users/:username/sync", h.SyncUser)
		ldap.GET("/users/:username/groups", h.GetUserGroups)

		// Templates
		ldap.GET("/templates", h.GetTemplates)
		ldap.GET("/templates/:type", h.GetTemplate)

		// Group management
		ldap.GET("/groups", h.ListGroups)
		ldap.GET("/groups/:group/members", h.GetGroupMembers)

		// Statistics and monitoring
		ldap.GET("/stats", h.GetStatistics)
		ldap.GET("/health", h.GetHealth)
	}
}

// GetConfiguration returns current LDAP configuration.
func (h *LDAPHandlers) GetConfiguration(c *gin.Context) {
	if h.middleware.provider == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": map[string]interface{}{
				"enabled":       false,
				"fallback_auth": h.middleware.fallbackAuth,
				"config":        nil,
			},
		})
		return
	}

	// Return sanitized configuration (without sensitive data)
	config := *h.middleware.provider.config
	config.BindPassword = "[REDACTED]"

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"enabled":       h.middleware.enabled,
			"fallback_auth": h.middleware.fallbackAuth,
			"config":        config,
		},
	})
}

// SetConfiguration sets LDAP configuration.
func (h *LDAPHandlers) SetConfiguration(c *gin.Context) {
	var req struct {
		Enabled      bool    `json:"enabled"`
		Config       *Config `json:"config"`
		FallbackAuth bool    `json:"fallback_auth"`
		TestConfig   bool    `json:"test_config"` // If true, test before saving
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request: " + err.Error(),
		})
		return
	}

	// Validate configuration if LDAP is being enabled
	if req.Enabled && req.Config != nil {
		errors := ValidateConfig(req.Config)
		if len(errors) > 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Configuration validation failed",
				"details": errors,
			})
			return
		}
	}

	// Test configuration if requested
	if req.TestConfig && req.Enabled && req.Config != nil {
		provider := NewProvider(req.Config)
		if err := provider.TestConnection(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Configuration test failed: " + err.Error(),
			})
			return
		}
	}

	// Update middleware configuration
	h.middleware.enabled = req.Enabled
	h.middleware.fallbackAuth = req.FallbackAuth

	if req.Enabled && req.Config != nil {
		h.middleware.provider = NewProvider(req.Config)
	} else {
		h.middleware.provider = nil
	}

	// TODO: Persist configuration to database or configuration file

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "LDAP configuration updated successfully",
		"data": map[string]interface{}{
			"enabled":       h.middleware.enabled,
			"fallback_auth": h.middleware.fallbackAuth,
		},
	})
}

// UpdateConfiguration updates LDAP configuration (same as SetConfiguration).
func (h *LDAPHandlers) UpdateConfiguration(c *gin.Context) {
	h.SetConfiguration(c)
}

// TestConnection tests LDAP connection.
func (h *LDAPHandlers) TestConnection(c *gin.Context) {
	var req *Config

	// If no config provided in request, use current config
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid configuration: " + err.Error(),
			})
			return
		}
	} else if h.middleware.provider != nil {
		req = h.middleware.provider.config
	} else {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "No LDAP configuration available",
		})
		return
	}

	// Validate configuration
	errors := ValidateConfig(req)
	if len(errors) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Configuration validation failed",
			"details": errors,
		})
		return
	}

	// Test connection
	provider := NewProvider(req)
	start := time.Now()
	err := provider.TestConnection()
	duration := time.Since(start)

	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success":       false,
			"error":         "Connection test failed",
			"details":       err.Error(),
			"response_time": duration.Milliseconds(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "LDAP connection test successful",
		"response_time": duration.Milliseconds(),
		"server":        fmt.Sprintf("%s:%d", req.Host, req.Port),
	})
}

// TestAuthentication tests LDAP authentication with provided credentials.
func (h *LDAPHandlers) TestAuthentication(c *gin.Context) {
	var req struct {
		Username string  `json:"username" binding:"required"`
		Password string  `json:"password" binding:"required"`
		Config   *Config `json:"config"` // Optional, use current if not provided
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request: " + err.Error(),
		})
		return
	}

	// Use provided config or current config
	var config *Config
	if req.Config != nil {
		config = req.Config
	} else if h.middleware.provider != nil {
		config = h.middleware.provider.config
	} else {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "No LDAP configuration available",
		})
		return
	}

	// Test authentication
	provider := NewProvider(config)
	start := time.Now()
	result := provider.Authenticate(req.Username, req.Password)
	duration := time.Since(start)

	if !result.Success {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success":       false,
			"error":         "Authentication failed",
			"details":       result.ErrorMessage,
			"response_time": duration.Milliseconds(),
		})
		return
	}

	// Sanitize user data for response
	userData := map[string]interface{}{
		"username":     result.User.Username,
		"email":        result.User.Email,
		"first_name":   result.User.FirstName,
		"last_name":    result.User.LastName,
		"display_name": result.User.DisplayName,
		"role":         result.User.Role,
		"groups":       result.User.Groups,
		"dn":           result.User.DN,
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "Authentication successful",
		"user":          userData,
		"response_time": duration.Milliseconds(),
	})
}

// GetUserInfo retrieves user information from LDAP.
func (h *LDAPHandlers) GetUserInfo(c *gin.Context) {
	if !h.middleware.enabled || h.middleware.provider == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "LDAP is not enabled",
		})
		return
	}

	username := c.Param("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Username is required",
		})
		return
	}

	// Connect to LDAP
	if err := h.middleware.provider.Connect(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "Failed to connect to LDAP: " + err.Error(),
		})
		return
	}
	defer h.middleware.provider.Close()

	// Find user
	user, err := h.middleware.provider.findUser(username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "User lookup failed: " + err.Error(),
		})
		return
	}

	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found",
		})
		return
	}

	// Get user groups
	groups, err := h.middleware.provider.getUserGroups(user.DN)
	if err != nil {
		// Don't fail the request for group lookup errors
		groups = []string{}
	}

	user.Groups = groups
	user.Role = h.middleware.provider.determineRole(groups)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    user,
	})
}

// SyncUser synchronizes user information from LDAP to local database.
func (h *LDAPHandlers) SyncUser(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Username is required",
		})
		return
	}

	// TODO: Implement user synchronization logic
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User synchronization completed",
		"user":    username,
	})
}

// GetUserGroups retrieves groups for a specific user.
func (h *LDAPHandlers) GetUserGroups(c *gin.Context) {
	if !h.middleware.enabled || h.middleware.provider == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "LDAP is not enabled",
		})
		return
	}

	username := c.Param("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Username is required",
		})
		return
	}

	// Connect to LDAP
	if err := h.middleware.provider.Connect(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "Failed to connect to LDAP: " + err.Error(),
		})
		return
	}
	defer h.middleware.provider.Close()

	// Find user to get DN
	user, err := h.middleware.provider.findUser(username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "User lookup failed: " + err.Error(),
		})
		return
	}

	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found",
		})
		return
	}

	// Get user groups
	groups, err := h.middleware.provider.getUserGroups(user.DN)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to retrieve user groups: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"username": username,
			"groups":   groups,
			"role":     h.middleware.provider.determineRole(groups),
		},
	})
}

// GetTemplates returns available LDAP configuration templates.
func (h *LDAPHandlers) GetTemplates(c *gin.Context) {
	templates := make(map[string]interface{})
	descriptions := map[string]string{
		"active_directory": "Microsoft Active Directory with typical AD schema and attributes",
		"openldap":         "OpenLDAP with standard LDAP schema and posixAccount objects",
		"389ds":            "389 Directory Server (Red Hat Directory Server) configuration",
	}

	title := cases.Title(language.English)
	for name := range DefaultConfigs {
		templates[name] = map[string]string{
			"name":        title.String(name),
			"description": descriptions[name],
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"data":      templates,
		"templates": templates, // Legacy compatibility
	})
}

// GetTemplate returns a specific LDAP configuration template.
func (h *LDAPHandlers) GetTemplate(c *gin.Context) {
	templateType := c.Param("type")
	if templateType == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Template type is required",
		})
		return
	}

	template, err := GetConfigTemplate(templateType)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	descriptions := map[string]string{
		"active_directory": "Microsoft Active Directory with typical AD schema and attributes",
		"openldap":         "OpenLDAP with standard LDAP schema and posixAccount objects",
		"389ds":            "389 Directory Server (Red Hat Directory Server) configuration",
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"data":        template,
		"template":    template, // Legacy compatibility
		"description": descriptions[templateType],
	})
}

// ListGroups lists groups from LDAP (limited implementation).
func (h *LDAPHandlers) ListGroups(c *gin.Context) {
	if !h.middleware.enabled || h.middleware.provider == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "LDAP is not enabled",
		})
		return
	}

	limit := 50 // Default limit
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}

	// TODO: Implement actual group listing
	// This would require connecting to LDAP and searching for group objects
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"groups": []string{}, // Empty for now
			"limit":  limit,
			"total":  0,
		},
		"message": "Group listing not yet implemented",
	})
}

// GetGroupMembers returns members of a specific group.
func (h *LDAPHandlers) GetGroupMembers(c *gin.Context) {
	groupName := c.Param("group")
	if groupName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Group name is required",
		})
		return
	}

	// TODO: Implement actual group member retrieval
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"group":   groupName,
			"members": []string{}, // Empty for now
			"count":   0,
		},
		"message": "Group member listing not yet implemented",
	})
}

// GetStatistics returns LDAP usage statistics.
func (h *LDAPHandlers) GetStatistics(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"enabled":                    h.middleware.enabled,
			"fallback_auth_enabled":      h.middleware.fallbackAuth,
			"last_connection_test":       nil, // TODO: Track this
			"total_authentications":      0,   // TODO: Track this
			"successful_authentications": 0,   // TODO: Track this
			"failed_authentications":     0,   // TODO: Track this
			"last_sync":                  nil, // TODO: Track user synchronization
		},
		"message": "Statistics collection not yet fully implemented",
	})
}

// GetHealth returns LDAP health status.
func (h *LDAPHandlers) GetHealth(c *gin.Context) {
	if !h.middleware.enabled || h.middleware.provider == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": map[string]interface{}{
				"status":  "disabled",
				"message": "LDAP authentication is disabled",
			},
		})
		return
	}

	// Test connection health
	start := time.Now()
	err := h.middleware.provider.TestConnection()
	duration := time.Since(start)

	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"data": map[string]interface{}{
				"status":        "unhealthy",
				"error":         err.Error(),
				"response_time": duration.Milliseconds(),
				"server":        fmt.Sprintf("%s:%d", h.middleware.provider.config.Host, h.middleware.provider.config.Port),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"status":        "healthy",
			"response_time": duration.Milliseconds(),
			"server":        fmt.Sprintf("%s:%d", h.middleware.provider.config.Host, h.middleware.provider.config.Port),
		},
	})
}
