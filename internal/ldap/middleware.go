package ldap

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware provides LDAP authentication middleware
type AuthMiddleware struct {
	provider   *Provider
	enabled    bool
	fallbackAuth bool // If true, fall back to local auth when LDAP fails
}

// NewAuthMiddleware creates a new LDAP authentication middleware
func NewAuthMiddleware(config *Config, enabled, fallbackAuth bool) *AuthMiddleware {
	var provider *Provider
	if enabled && config != nil {
		provider = NewProvider(config)
	}
	
	return &AuthMiddleware{
		provider:     provider,
		enabled:      enabled,
		fallbackAuth: fallbackAuth,
	}
}

// AuthenticateUser authenticates a user with LDAP
func (m *AuthMiddleware) AuthenticateUser(username, password string) (*User, error) {
	if !m.enabled || m.provider == nil {
		return nil, nil // Not enabled, fall back to local auth
	}
	
	result := m.provider.Authenticate(username, password)
	if !result.Success {
		if m.fallbackAuth {
			log.Printf("LDAP authentication failed for %s: %s, falling back to local auth", 
				username, result.ErrorMessage)
			return nil, nil // Allow fallback to local auth
		}
		return nil, &AuthError{Message: result.ErrorMessage}
	}
	
	return result.User, nil
}

// AuthError represents an LDAP authentication error
type AuthError struct {
	Message string
}

func (e *AuthError) Error() string {
	return e.Message
}

// HandleLogin handles LDAP login requests
func (m *AuthMiddleware) HandleLogin(c *gin.Context) {
	var loginRequest struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&loginRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid login request: " + err.Error(),
		})
		return
	}
	
	// Try LDAP authentication first
	user, err := m.AuthenticateUser(loginRequest.Username, loginRequest.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Authentication failed: " + err.Error(),
		})
		return
	}
	
	if user != nil {
		// LDAP authentication successful
		// TODO: Create or update user in local database
		// TODO: Generate JWT token
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"user":    user,
			"source":  "ldap",
		})
		return
	}
	
	// Fall back to local authentication if enabled
	if m.fallbackAuth {
		// TODO: Call local authentication handler
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Local authentication not implemented",
		})
		return
	}
	
	c.JSON(http.StatusUnauthorized, gin.H{
		"error": "Authentication failed",
	})
}

// SyncUserMiddleware synchronizes LDAP users with local database
func (m *AuthMiddleware) SyncUserMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		if !m.enabled {
			c.Next()
			return
		}
		
		// Get current user from context (set by JWT middleware)
		userInterface, exists := c.Get("user")
		if !exists {
			c.Next()
			return
		}
		
		// Check if user came from LDAP
		sourceInterface, hasSource := c.Get("auth_source")
		if !hasSource || sourceInterface != "ldap" {
			c.Next()
			return
		}
		
		// TODO: Implement user synchronization logic
		// This would update user information from LDAP if it has changed
		
		c.Next()
	})
}

// TestConnectionHandler provides an endpoint to test LDAP connection
func (m *AuthMiddleware) TestConnectionHandler(c *gin.Context) {
	if !m.enabled || m.provider == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "LDAP is not enabled",
		})
		return
	}
	
	err := m.provider.TestConnection()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "LDAP connection test failed",
			"details": err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "LDAP connection test successful",
	})
}

// GetUserInfoHandler retrieves user information from LDAP
func (m *AuthMiddleware) GetUserInfoHandler(c *gin.Context) {
	if !m.enabled || m.provider == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "LDAP is not enabled",
		})
		return
	}
	
	username := c.Param("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Username is required",
		})
		return
	}
	
	// Connect to LDAP
	if err := m.provider.Connect(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "Failed to connect to LDAP",
			"details": err.Error(),
		})
		return
	}
	defer m.provider.Close()
	
	// Find user
	user, err := m.provider.findUser(username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "User lookup failed",
			"details": err.Error(),
		})
		return
	}
	
	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "User not found",
		})
		return
	}
	
	// Get user groups
	groups, err := m.provider.getUserGroups(user.DN)
	if err != nil {
		log.Printf("Failed to get groups for user %s: %v", username, err)
		groups = []string{}
	}
	
	user.Groups = groups
	user.Role = m.provider.determineRole(groups)
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"user":    user,
	})
}

// ConfigurationHandler handles LDAP configuration endpoints
func (m *AuthMiddleware) ConfigurationHandler(c *gin.Context) {
	switch c.Request.Method {
	case "GET":
		m.getConfiguration(c)
	case "POST":
		m.setConfiguration(c)
	case "PUT":
		m.updateConfiguration(c)
	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{
			"error": "Method not allowed",
		})
	}
}

func (m *AuthMiddleware) getConfiguration(c *gin.Context) {
	if m.provider == nil {
		c.JSON(http.StatusOK, gin.H{
			"enabled": false,
			"config":  nil,
		})
		return
	}
	
	// Return sanitized configuration (without sensitive data)
	config := *m.provider.config
	config.BindPassword = "" // Don't return password
	
	c.JSON(http.StatusOK, gin.H{
		"enabled": m.enabled,
		"config":  config,
	})
}

func (m *AuthMiddleware) setConfiguration(c *gin.Context) {
	var req struct {
		Enabled      bool    `json:"enabled"`
		Config       *Config `json:"config"`
		FallbackAuth bool    `json:"fallback_auth"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid configuration: " + err.Error(),
		})
		return
	}
	
	// Validate configuration if LDAP is being enabled
	if req.Enabled && req.Config != nil {
		errors := ValidateConfig(req.Config)
		if len(errors) > 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":  "Configuration validation failed",
				"errors": errors,
			})
			return
		}
	}
	
	// Update middleware configuration
	m.enabled = req.Enabled
	m.fallbackAuth = req.FallbackAuth
	
	if req.Enabled && req.Config != nil {
		m.provider = NewProvider(req.Config)
		
		// Test the configuration
		if err := m.provider.TestConnection(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":   "LDAP configuration test failed",
				"details": err.Error(),
			})
			return
		}
	} else {
		m.provider = nil
	}
	
	// TODO: Save configuration to database or file
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "LDAP configuration updated successfully",
	})
}

func (m *AuthMiddleware) updateConfiguration(c *gin.Context) {
	// For now, treat PUT the same as POST
	m.setConfiguration(c)
}

// TemplateHandler provides LDAP configuration templates
func (m *AuthMiddleware) TemplateHandler(c *gin.Context) {
	ldapType := c.Param("type")
	if ldapType == "" {
		// Return available templates
		templates := make(map[string]interface{})
		for name := range DefaultConfigs {
			templates[name] = map[string]string{
				"name":        strings.Title(name),
				"description": getTemplateDescription(name),
			}
		}
		
		c.JSON(http.StatusOK, gin.H{
			"templates": templates,
		})
		return
	}
	
	template, err := GetConfigTemplate(ldapType)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"template":    template,
		"description": getTemplateDescription(ldapType),
	})
}

func getTemplateDescription(ldapType string) string {
	descriptions := map[string]string{
		"active_directory": "Microsoft Active Directory with typical AD schema and attributes",
		"openldap":        "OpenLDAP with standard LDAP schema and posixAccount objects",
		"389ds":           "389 Directory Server (Red Hat Directory Server) configuration",
	}
	
	if desc, exists := descriptions[ldapType]; exists {
		return desc
	}
	
	return "Generic LDAP configuration template"
}