package v1

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/service"
)

// LDAPHandler handles LDAP-related API endpoints.
type LDAPHandler struct {
	ldapService *service.LDAPService
}

// NewLDAPHandler creates a new LDAP handler.
func NewLDAPHandler(ldapService *service.LDAPService) *LDAPHandler {
	return &LDAPHandler{
		ldapService: ldapService,
	}
}

// POST /api/v1/ldap/configure.
func (h *LDAPHandler) ConfigureLDAP(c *gin.Context) {
	var config service.LDAPConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body: " + err.Error(),
		})
		return
	}

	if err := h.ldapService.ConfigureLDAP(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Failed to configure LDAP: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "LDAP configuration saved successfully",
	})
}

// POST /api/v1/ldap/test.
func (h *LDAPHandler) TestConnection(c *gin.Context) {
	var config service.LDAPConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body: " + err.Error(),
		})
		return
	}

	startTime := time.Now()
	err := h.ldapService.TestConnection(&config)
	responseTime := time.Since(startTime).Milliseconds()

	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success":       false,
			"error":         err.Error(),
			"response_time": responseTime,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "Connection successful",
		"response_time": responseTime,
	})
}

// POST /api/v1/ldap/authenticate.
func (h *LDAPHandler) AuthenticateUser(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body: " + err.Error(),
		})
		return
	}

	user, err := h.ldapService.AuthenticateUser(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Authentication failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"user":    user,
	})
}

// GET /api/v1/ldap/users/:username.
func (h *LDAPHandler) GetUser(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Username is required",
		})
		return
	}

	user, err := h.ldapService.GetUser(username)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"user":    user,
	})
}

// GET /api/v1/ldap/groups.
func (h *LDAPHandler) GetGroups(c *gin.Context) {
	groups, err := h.ldapService.GetGroups()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to retrieve groups: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"groups":  groups,
		"count":   len(groups),
	})
}

// POST /api/v1/ldap/sync/users.
func (h *LDAPHandler) SyncUsers(c *gin.Context) {
	result, err := h.ldapService.SyncUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Sync failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"result":  result,
	})
}

// GET /api/v1/ldap/sync/status.
func (h *LDAPHandler) GetSyncStatus(c *gin.Context) {
	status := h.ldapService.GetSyncStatus()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"status":  status,
	})
}

// GET /api/v1/ldap/users/search.
func (h *LDAPHandler) SearchUsers(c *gin.Context) {
	query := c.Query("q")
	limitStr := c.DefaultQuery("limit", "50")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 50
	}

	// For demo purposes, return a simulated search result
	// In real implementation, this would use LDAP search
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"users":   []interface{}{},
		"query":   query,
		"limit":   limit,
		"message": "LDAP user search functionality - implementation depends on specific LDAP schema",
	})
}

// GET /api/v1/ldap/config.
func (h *LDAPHandler) GetConfiguration(c *gin.Context) {
	status := h.ldapService.GetSyncStatus()

	// Return masked configuration for security
	config := gin.H{
		"configured":    status["configured"],
		"auto_sync":     status["auto_sync"],
		"sync_interval": status["sync_interval"],
		"last_sync":     status["last_sync"],
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"config":  config,
	})
}

// POST /api/v1/ldap/import/users.
func (h *LDAPHandler) ImportUsers(c *gin.Context) {
	var req struct {
		Usernames []string `json:"usernames" binding:"required"`
		DryRun    bool     `json:"dry_run"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body: " + err.Error(),
		})
		return
	}

	if len(req.Usernames) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "At least one username is required",
		})
		return
	}

	results := make([]gin.H, 0, len(req.Usernames))

	for _, username := range req.Usernames {
		result := gin.H{
			"username": username,
			"success":  false,
		}

		user, err := h.ldapService.GetUser(username)
		if err != nil {
			result["error"] = err.Error()
			results = append(results, result)
			continue
		}

		if !req.DryRun {
			// In real implementation, create/update user in database
			result["success"] = true
			result["user"] = gin.H{
				"email":        user.Email,
				"display_name": user.DisplayName,
				"department":   user.Department,
				"title":        user.Title,
			}
		} else {
			result["success"] = true
			result["preview"] = gin.H{
				"email":        user.Email,
				"display_name": user.DisplayName,
				"department":   user.Department,
				"title":        user.Title,
				"groups":       user.Groups,
			}
		}

		results = append(results, result)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"results": results,
		"dry_run": req.DryRun,
	})
}

// GET /api/v1/ldap/mappings/users.
func (h *LDAPHandler) GetUserMappings(c *gin.Context) {
	// This would query the mapping repository in real implementation
	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"mappings": []interface{}{},
		"message":  "User mappings endpoint - requires database implementation",
	})
}

// GET /api/v1/ldap/mappings/groups.
func (h *LDAPHandler) GetGroupMappings(c *gin.Context) {
	// This would query the mapping repository in real implementation
	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"mappings": []interface{}{},
		"message":  "Group mappings endpoint - requires database implementation",
	})
}

// GET /api/v1/ldap/logs/auth.
func (h *LDAPHandler) GetAuthenticationLogs(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	username := c.Query("username")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 100
	}

	// This would query the auth log repository in real implementation
	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"logs":     []interface{}{},
		"username": username,
		"limit":    limit,
		"message":  "Authentication logs endpoint - requires database implementation",
	})
}

// POST /api/v1/ldap/disable.
func (h *LDAPHandler) DisableLDAP(c *gin.Context) {
	// Stop the LDAP service
	h.ldapService.Stop()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "LDAP integration has been disabled",
	})
}

// RegisterRoutes registers LDAP routes.
func (h *LDAPHandler) RegisterRoutes(r *gin.RouterGroup) {
	ldap := r.Group("/ldap")
	{
		// Configuration
		ldap.POST("/configure", h.ConfigureLDAP)
		ldap.POST("/test", h.TestConnection)
		ldap.GET("/config", h.GetConfiguration)
		ldap.POST("/disable", h.DisableLDAP)

		// Authentication
		ldap.POST("/authenticate", h.AuthenticateUser)

		// User management
		ldap.GET("/users/:username", h.GetUser)
		ldap.GET("/users/search", h.SearchUsers)
		ldap.POST("/import/users", h.ImportUsers)

		// Group management
		ldap.GET("/groups", h.GetGroups)

		// Synchronization
		ldap.POST("/sync/users", h.SyncUsers)
		ldap.GET("/sync/status", h.GetSyncStatus)

		// Mappings
		ldap.GET("/mappings/users", h.GetUserMappings)
		ldap.GET("/mappings/groups", h.GetGroupMappings)

		// Logs
		ldap.GET("/logs/auth", h.GetAuthenticationLogs)
	}
}
