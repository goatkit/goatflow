package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/auth"
	"github.com/gotrs-io/gotrs-ce/internal/ldap"
	"github.com/gotrs-io/gotrs-ce/internal/middleware"
	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// APIRouter manages all v1 API routes
type APIRouter struct {
	rbac         *auth.RBAC
	jwtManager   *auth.JWTManager
	ldapHandlers *ldap.LDAPHandlers
}

// NewAPIRouter creates a new API router instance
func NewAPIRouter(rbac *auth.RBAC, jwtManager *auth.JWTManager, ldapHandlers *ldap.LDAPHandlers) *APIRouter {
	return &APIRouter{
		rbac:         rbac,
		jwtManager:   jwtManager,
		ldapHandlers: ldapHandlers,
	}
}

// SetupV1Routes configures all v1 API routes
func (router *APIRouter) SetupV1Routes(r *gin.Engine) {
	v1 := r.Group("/api/v1")
	
	// Add rate limiting middleware
	// v1.Use(middleware.RateLimit())
	
	// Add request ID middleware
	v1.Use(middleware.RequestID())
	
	// Add API version header
	v1.Use(func(c *gin.Context) {
		c.Header("X-API-Version", "1.0")
		c.Next()
	})
	
	// Public endpoints (no authentication required)
	router.setupPublicRoutes(v1)
	
	// Protected endpoints (authentication required)
	protected := v1.Group("")
	if router.jwtManager != nil {
		protected.Use(middleware.SessionMiddleware(router.jwtManager))
	}
	
	// User endpoints
	router.setupUserRoutes(protected)
	
	// Ticket endpoints
	router.setupTicketRoutes(protected)
	
	// Queue endpoints  
	router.setupQueueRoutes(protected)
	
	// Priority endpoints
	router.setupPriorityRoutes(protected)
	
	// Search endpoints
	router.setupSearchRoutes(protected)
	
	// File/attachment endpoints
	router.setupFileRoutes(protected)
	
	// Dashboard endpoints
	router.setupDashboardRoutes(protected)
	
	// Admin-only endpoints
	adminRoutes := protected.Group("")
	adminRoutes.Use(middleware.RequireAdminAccess(router.rbac))
	router.setupAdminRoutes(adminRoutes)
	
	// Agent-level endpoints
	agentRoutes := protected.Group("")
	agentRoutes.Use(middleware.RequireAgentAccess(router.rbac))
	router.setupAgentRoutes(agentRoutes)
}

// setupPublicRoutes configures public API endpoints
func (router *APIRouter) setupPublicRoutes(v1 *gin.RouterGroup) {
	// Health check
	v1.GET("/health", router.handleHealth)
	
	// API info
	v1.GET("/info", router.handleAPIInfo)
	
	// System status
	v1.GET("/status", router.handleSystemStatus)
	
	// Authentication endpoints
	auth := v1.Group("/auth")
	{
		auth.POST("/login", router.handleLogin)
		auth.POST("/refresh", router.handleRefreshToken)
		auth.POST("/logout", router.handleLogout)
		auth.POST("/register", router.handleRegister) // If registration is enabled
	}
}

// setupUserRoutes configures user-related endpoints
func (router *APIRouter) setupUserRoutes(protected *gin.RouterGroup) {
	users := protected.Group("/users")
	{
		users.GET("/me", router.handleGetCurrentUser)
		users.PUT("/me", router.handleUpdateCurrentUser)
		users.GET("/me/preferences", router.handleGetUserPreferences)
		users.PUT("/me/preferences", router.handleUpdateUserPreferences)
		users.POST("/me/password", router.handleChangePassword)
		users.GET("/me/sessions", router.handleGetUserSessions)
		users.DELETE("/me/sessions/:id", router.handleRevokeSession)
	}
}

// setupTicketRoutes configures ticket-related endpoints
func (router *APIRouter) setupTicketRoutes(protected *gin.RouterGroup) {
	tickets := protected.Group("/tickets")
	
	// Require ticket read permission
	tickets.Use(middleware.RequireAnyPermission(router.rbac, auth.PermissionTicketRead, auth.PermissionOwnTicketRead))
	{
		// Basic CRUD
		tickets.GET("", router.handleListTickets)
		tickets.POST("", middleware.RequireAnyPermission(router.rbac, auth.PermissionTicketCreate, auth.PermissionOwnTicketCreate), router.handleCreateTicket)
		tickets.GET("/:id", middleware.RequireTicketAccess(router.rbac), router.handleGetTicket)
		tickets.PUT("/:id", middleware.RequirePermission(router.rbac, auth.PermissionTicketUpdate), router.handleUpdateTicket)
		tickets.DELETE("/:id", middleware.RequirePermission(router.rbac, auth.PermissionTicketDelete), router.handleDeleteTicket)
		
		// Ticket actions
		tickets.POST("/:id/assign", middleware.RequirePermission(router.rbac, auth.PermissionTicketAssign), router.handleAssignTicket)
		tickets.POST("/:id/close", middleware.RequirePermission(router.rbac, auth.PermissionTicketClose), router.handleCloseTicket)
		tickets.POST("/:id/reopen", middleware.RequirePermission(router.rbac, auth.PermissionTicketUpdate), router.handleReopenTicket)
		tickets.POST("/:id/priority", middleware.RequirePermission(router.rbac, auth.PermissionTicketUpdate), router.handleUpdateTicketPriority)
		tickets.POST("/:id/queue", middleware.RequirePermission(router.rbac, auth.PermissionTicketUpdate), router.handleMoveTicketQueue)
		
		// Articles/messages
		tickets.GET("/:id/articles", router.handleGetTicketArticles)
		tickets.POST("/:id/articles", middleware.RequirePermission(router.rbac, auth.PermissionTicketUpdate), router.handleAddTicketArticle)
		tickets.GET("/:id/articles/:article_id", router.handleGetTicketArticle)
		
		// Attachments
		tickets.GET("/:id/attachments", router.handleGetTicketAttachments)
		tickets.POST("/:id/attachments", middleware.RequirePermission(router.rbac, auth.PermissionTicketUpdate), router.handleUploadTicketAttachment)
		tickets.GET("/:id/attachments/:attachment_id", router.handleDownloadTicketAttachment)
		tickets.DELETE("/:id/attachments/:attachment_id", middleware.RequirePermission(router.rbac, auth.PermissionTicketUpdate), router.handleDeleteTicketAttachment)
		
		// History/timeline
		tickets.GET("/:id/history", router.handleGetTicketHistory)
		
		// SLA and escalation
		tickets.GET("/:id/sla", router.handleGetTicketSLA)
		tickets.POST("/:id/escalate", middleware.RequirePermission(router.rbac, auth.PermissionTicketUpdate), router.handleEscalateTicket)
		
		// Merge/split operations
		tickets.POST("/:id/merge", middleware.RequirePermission(router.rbac, auth.PermissionTicketUpdate), router.handleMergeTickets)
		tickets.POST("/:id/split", middleware.RequirePermission(router.rbac, auth.PermissionTicketUpdate), router.handleSplitTicket)
		
		// Bulk operations
		tickets.POST("/bulk/assign", middleware.RequirePermission(router.rbac, auth.PermissionTicketAssign), router.handleBulkAssignTickets)
		tickets.POST("/bulk/close", middleware.RequirePermission(router.rbac, auth.PermissionTicketClose), router.handleBulkCloseTickets)
		tickets.POST("/bulk/priority", middleware.RequirePermission(router.rbac, auth.PermissionTicketUpdate), router.handleBulkUpdatePriority)
		tickets.POST("/bulk/queue", middleware.RequirePermission(router.rbac, auth.PermissionTicketUpdate), router.handleBulkMoveQueue)
	}
}

// setupQueueRoutes configures queue-related endpoints
func (router *APIRouter) setupQueueRoutes(protected *gin.RouterGroup) {
	queues := protected.Group("/queues")
	queues.Use(middleware.RequireAgentAccess(router.rbac))
	{
		queues.GET("", router.handleListQueues)
		queues.POST("", middleware.RequireAdminAccess(router.rbac), router.handleCreateQueue)
		queues.GET("/:id", router.handleGetQueue)
		queues.PUT("/:id", middleware.RequireAdminAccess(router.rbac), router.handleUpdateQueue)
		queues.DELETE("/:id", middleware.RequireAdminAccess(router.rbac), router.handleDeleteQueue)
		
		// Queue tickets
		queues.GET("/:id/tickets", router.handleGetQueueTickets)
		queues.GET("/:id/stats", router.handleGetQueueStats)
	}
}

// setupPriorityRoutes configures priority-related endpoints
func (router *APIRouter) setupPriorityRoutes(protected *gin.RouterGroup) {
	priorities := protected.Group("/priorities")
	{
		priorities.GET("", router.handleListPriorities)
		priorities.POST("", middleware.RequireAdminAccess(router.rbac), router.handleCreatePriority)
		priorities.GET("/:id", router.handleGetPriority)
		priorities.PUT("/:id", middleware.RequireAdminAccess(router.rbac), router.handleUpdatePriority)
		priorities.DELETE("/:id", middleware.RequireAdminAccess(router.rbac), router.handleDeletePriority)
	}
}

// setupSearchRoutes configures search endpoints
func (router *APIRouter) setupSearchRoutes(protected *gin.RouterGroup) {
	search := protected.Group("/search")
	{
		search.GET("", router.handleGlobalSearch)
		search.GET("/tickets", middleware.RequireAnyPermission(router.rbac, auth.PermissionTicketRead, auth.PermissionOwnTicketRead), router.handleSearchTickets)
		search.GET("/users", middleware.RequirePermission(router.rbac, auth.PermissionUserRead), router.handleSearchUsers)
		search.GET("/suggestions", router.handleSearchSuggestions)
		
		// Saved searches
		search.GET("/saved", router.handleGetSavedSearches)
		search.POST("/saved", router.handleCreateSavedSearch)
		search.GET("/saved/:id", router.handleGetSavedSearch)
		search.PUT("/saved/:id", router.handleUpdateSavedSearch)
		search.DELETE("/saved/:id", router.handleDeleteSavedSearch)
		search.POST("/saved/:id/execute", router.handleExecuteSavedSearch)
	}
}

// setupFileRoutes configures file/attachment endpoints
func (router *APIRouter) setupFileRoutes(protected *gin.RouterGroup) {
	files := protected.Group("/files")
	{
		files.POST("/upload", router.handleUploadFile)
		files.GET("/:id", router.handleDownloadFile)
		files.DELETE("/:id", router.handleDeleteFile)
		files.GET("/:id/info", router.handleGetFileInfo)
	}
}

// setupDashboardRoutes configures dashboard endpoints
func (router *APIRouter) setupDashboardRoutes(protected *gin.RouterGroup) {
	dashboard := protected.Group("/dashboard")
	{
		dashboard.GET("/stats", router.handleGetDashboardStats)
		dashboard.GET("/charts/tickets-by-status", router.handleGetTicketsByStatusChart)
		dashboard.GET("/charts/tickets-by-priority", router.handleGetTicketsByPriorityChart)
		dashboard.GET("/charts/tickets-over-time", router.handleGetTicketsOverTimeChart)
		dashboard.GET("/activity", router.handleGetRecentActivity)
		dashboard.GET("/my-tickets", router.handleGetMyTickets)
		dashboard.GET("/notifications", router.handleGetNotifications)
		dashboard.POST("/notifications/:id/read", router.handleMarkNotificationRead)
	}
}

// setupAdminRoutes configures admin-only endpoints
func (router *APIRouter) setupAdminRoutes(adminRoutes *gin.RouterGroup) {
	admin := adminRoutes.Group("/admin")
	{
		// User management
		users := admin.Group("/users")
		{
			users.GET("", router.handleListAllUsers)
			users.POST("", router.handleCreateUser)
			users.GET("/:id", router.handleGetUser)
			users.PUT("/:id", router.handleUpdateUser)
			users.DELETE("/:id", router.handleDeleteUser)
			users.POST("/:id/activate", router.handleActivateUser)
			users.POST("/:id/deactivate", router.handleDeactivateUser)
			users.POST("/:id/reset-password", router.handleResetUserPassword)
		}
		
		// System configuration
		system := admin.Group("/system")
		{
			system.GET("/config", router.handleGetSystemConfig)
			system.PUT("/config", router.handleUpdateSystemConfig)
			system.GET("/stats", router.handleGetSystemStats)
			system.POST("/maintenance", router.handleToggleMaintenanceMode)
			system.GET("/logs", router.handleGetSystemLogs)
			system.POST("/backup", router.handleCreateBackup)
			system.GET("/backups", router.handleListBackups)
			system.POST("/restore/:backup_id", router.handleRestoreBackup)
		}
		
		// Audit logs
		audit := admin.Group("/audit")
		{
			audit.GET("/logs", router.handleGetAuditLogs)
			audit.GET("/logs/:id", router.handleGetAuditLog)
			audit.GET("/stats", router.handleGetAuditStats)
		}
		
		// Reports
		reports := admin.Group("/reports")
		{
			reports.GET("/tickets", router.handleGetTicketReports)
			reports.GET("/users", router.handleGetUserReports)
			reports.GET("/sla", router.handleGetSLAReports)
			reports.GET("/performance", router.handleGetPerformanceReports)
			reports.POST("/export", router.handleExportReport)
		}
		
		// LDAP configuration and management
		if router.ldapHandlers != nil {
			router.ldapHandlers.SetupLDAPRoutes(admin)
		}
	}
}

// setupAgentRoutes configures agent-level endpoints
func (router *APIRouter) setupAgentRoutes(agentRoutes *gin.RouterGroup) {
	agent := agentRoutes.Group("/agent")
	{
		// Canned responses
		canned := agent.Group("/canned-responses")
		{
			canned.GET("", router.handleListCannedResponses)
			canned.POST("", router.handleCreateCannedResponse)
			canned.GET("/:id", router.handleGetCannedResponse)
			canned.PUT("/:id", router.handleUpdateCannedResponse)
			canned.DELETE("/:id", router.handleDeleteCannedResponse)
			canned.GET("/categories", router.handleGetCannedResponseCategories)
		}
		
		// Templates
		templates := agent.Group("/templates")
		{
			templates.GET("", router.handleListTicketTemplates)
			templates.POST("", router.handleCreateTicketTemplate)
			templates.GET("/:id", router.handleGetTicketTemplate)
			templates.PUT("/:id", router.handleUpdateTicketTemplate)
			templates.DELETE("/:id", router.handleDeleteTicketTemplate)
		}
		
		// Agent statistics
		stats := agent.Group("/stats")
		{
			stats.GET("/my-performance", router.handleGetMyPerformance)
			stats.GET("/workload", router.handleGetMyWorkload)
			stats.GET("/response-times", router.handleGetMyResponseTimes)
		}
	}
}

// Response helper structures
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
}

type PaginatedResponse struct {
	Success    bool        `json:"success"`
	Data       interface{} `json:"data"`
	Pagination Pagination  `json:"pagination"`
	Error      string      `json:"error,omitempty"`
}

type Pagination struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

// Helper functions
func sendSuccess(c *gin.Context, data interface{}) {
	c.JSON(200, APIResponse{
		Success: true,
		Data:    data,
	})
}

func sendError(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, APIResponse{
		Success: false,
		Error:   message,
	})
}

func sendPaginatedResponse(c *gin.Context, data interface{}, pagination Pagination) {
	c.JSON(200, PaginatedResponse{
		Success:    true,
		Data:       data,
		Pagination: pagination,
	})
}