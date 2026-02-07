package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/repository/memory"
	"github.com/goatkit/goatflow/internal/service"
)

// Global LDAP service instance.
var ldapService *service.LDAPService

// InitializeLDAPService initializes the LDAP service with repositories.
func InitializeLDAPService() {
	userRepo := memory.NewUserRepository()
	roleRepo := memory.NewRoleRepository()
	groupRepo := memory.NewGroupRepository()
	ldapService = service.NewLDAPService(userRepo, roleRepo, groupRepo)
}

// handleLDAPConfigure configures LDAP settings.
func handleLDAPConfigure(c *gin.Context) {
	var config service.LDAPConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid configuration format: " + err.Error(),
		})
		return
	}

	if err := ldapService.ConfigureLDAP(&config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to configure LDAP: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "LDAP configuration successful",
	})
}

// handleLDAPTestConnection tests LDAP connection.
func handleLDAPTestConnection(c *gin.Context) {
	var config service.LDAPConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid configuration format: " + err.Error(),
		})
		return
	}

	if err := ldapService.TestConnection(&config); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"success": false,
			"error":   "Connection test failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "LDAP connection successful",
	})
}

// handleLDAPAuthenticate authenticates a user via LDAP.
func handleLDAPAuthenticate(c *gin.Context) {
	var authRequest struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&authRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid authentication request: " + err.Error(),
		})
		return
	}

	user, err := ldapService.AuthenticateUser(authRequest.Username, authRequest.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Authentication failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    user,
		"message": "Authentication successful",
	})
}

// handleLDAPGetUser retrieves user information from LDAP.
func handleLDAPGetUser(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Username parameter is required",
		})
		return
	}

	user, err := ldapService.GetUser(username)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    user,
	})
}

// handleLDAPGetGroups retrieves LDAP groups.
func handleLDAPGetGroups(c *gin.Context) {
	groups, err := ldapService.GetGroups()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to retrieve groups: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    groups,
		"count":   len(groups),
	})
}

// handleLDAPSyncUsers performs user synchronization.
func handleLDAPSyncUsers(c *gin.Context) {
	result, err := ldapService.SyncUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Synchronization failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
		"message": "User synchronization completed",
	})
}

// handleLDAPImportUsers imports specific users.
func handleLDAPImportUsers(c *gin.Context) {
	var importRequest struct {
		Usernames []string `json:"usernames" binding:"required"`
		DryRun    bool     `json:"dry_run"`
	}

	if err := c.ShouldBindJSON(&importRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid import request: " + err.Error(),
		})
		return
	}

	result, err := ldapService.ImportUsers(importRequest.Usernames, importRequest.DryRun)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Import failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
		"message": "User import completed",
	})
}

// handleLDAPGetConfig returns current LDAP configuration (without sensitive data).
func handleLDAPGetConfig(c *gin.Context) {
	config := ldapService.GetConfig()
	if config == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "LDAP not configured",
		})
		return
	}

	// Return sanitized config (no passwords)
	sanitizedConfig := map[string]interface{}{
		"host":               config.Host,
		"port":               config.Port,
		"base_dn":            config.BaseDN,
		"bind_dn":            config.BindDN,
		"user_search_base":   config.UserSearchBase,
		"user_filter":        config.UserFilter,
		"group_search_base":  config.GroupSearchBase,
		"group_filter":       config.GroupFilter,
		"use_tls":            config.UseTLS,
		"start_tls":          config.StartTLS,
		"auto_create_users":  config.AutoCreateUsers,
		"auto_update_users":  config.AutoUpdateUsers,
		"auto_create_groups": config.AutoCreateGroups,
		"sync_interval":      config.SyncInterval,
		"default_role":       config.DefaultRole,
		"admin_groups":       config.AdminGroups,
		"user_groups":        config.UserGroups,
		"attribute_map":      config.AttributeMap,
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    sanitizedConfig,
	})
}

// handleLDAPGetSyncStatus returns synchronization status.
func handleLDAPGetSyncStatus(c *gin.Context) {
	status := ldapService.GetSyncStatus()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    status,
	})
}

// handleLDAPGetAuthLogs returns authentication logs.
func handleLDAPGetAuthLogs(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}

	logs := ldapService.GetAuthLogs(limit)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    logs,
		"count":   len(logs),
	})
}

// SetupLDAPRoutes configures LDAP API routes.
func SetupLDAPRoutes(r *gin.Engine) {
	// Initialize LDAP service
	InitializeLDAPService()

	// LDAP API routes
	ldapAPI := r.Group("/api/v1/ldap")
	{
		// Configuration
		ldapAPI.POST("/configure", handleLDAPConfigure)
		ldapAPI.GET("/config", handleLDAPGetConfig)
		ldapAPI.POST("/test", handleLDAPTestConnection)

		// Authentication
		ldapAPI.POST("/authenticate", handleLDAPAuthenticate)

		// User management
		ldapAPI.GET("/users/:username", handleLDAPGetUser)
		ldapAPI.GET("/groups", handleLDAPGetGroups)

		// Synchronization
		ldapAPI.POST("/sync/users", handleLDAPSyncUsers)
		ldapAPI.GET("/sync/status", handleLDAPGetSyncStatus)
		ldapAPI.POST("/import/users", handleLDAPImportUsers)

		// Monitoring
		ldapAPI.GET("/logs/auth", handleLDAPGetAuthLogs)
	}
}
