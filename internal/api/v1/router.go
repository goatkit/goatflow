package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/api/v1/handlers"
	"github.com/gotrs-io/gotrs-ce/internal/middleware"
	"github.com/gotrs-io/gotrs-ce/internal/service"
)

// APIRouter handles all v1 API routes
type APIRouter struct {
	router          *gin.Engine
	authHandler     *handlers.AuthHandler
	userHandler     *handlers.UserHandler
	ticketHandler   *handlers.TicketHandler
	queueHandler    *handlers.QueueHandler
	workflowHandler *handlers.WorkflowHandler
	webhookHandler  *handlers.WebhookHandler
	searchHandler   *handlers.SearchHandler
	reportHandler   *handlers.ReportHandler
	authMiddleware  *middleware.AuthMiddleware
}

// NewAPIRouter creates a new API router
func NewAPIRouter(
	router *gin.Engine,
	authService *service.AuthService,
	userService *service.UserService,
	ticketService *service.TicketService,
	queueService *service.QueueService,
	workflowService *service.WorkflowService,
	webhookService *service.WebhookService,
	searchService *service.SearchService,
	reportService *service.ReportService,
) *APIRouter {
	authMiddleware := middleware.NewAuthMiddleware(authService)

	return &APIRouter{
		router:          router,
		authHandler:     handlers.NewAuthHandler(authService),
		userHandler:     handlers.NewUserHandler(userService),
		ticketHandler:   handlers.NewTicketHandler(ticketService),
		queueHandler:    handlers.NewQueueHandler(queueService),
		workflowHandler: handlers.NewWorkflowHandler(workflowService),
		webhookHandler:  handlers.NewWebhookHandler(webhookService),
		searchHandler:   handlers.NewSearchHandler(searchService),
		reportHandler:   handlers.NewReportHandler(reportService),
		authMiddleware:  authMiddleware,
	}
}

// SetupRoutes configures all API v1 routes
func (r *APIRouter) SetupRoutes() {
	v1 := r.router.Group("/api/v1")
	
	// Health check
	v1.GET("/health", r.healthCheck)
	v1.GET("/version", r.versionInfo)
	
	// Public routes
	public := v1.Group("")
	{
		// Authentication
		public.POST("/auth/login", r.authHandler.Login)
		public.POST("/auth/register", r.authHandler.Register)
		public.POST("/auth/refresh", r.authHandler.RefreshToken)
		public.POST("/auth/forgot-password", r.authHandler.ForgotPassword)
		public.POST("/auth/reset-password", r.authHandler.ResetPassword)
		
		// OAuth2
		public.GET("/oauth/authorize", r.authHandler.OAuthAuthorize)
		public.POST("/oauth/token", r.authHandler.OAuthToken)
		public.GET("/oauth/.well-known/openid-configuration", r.authHandler.OpenIDConfiguration)
		
		// Webhooks (incoming)
		public.POST("/webhooks/incoming/:id", r.webhookHandler.HandleIncoming)
	}
	
	// Protected routes
	protected := v1.Group("")
	protected.Use(r.authMiddleware.RequireAuth())
	{
		// Current user
		protected.GET("/me", r.userHandler.GetCurrentUser)
		protected.PUT("/me", r.userHandler.UpdateCurrentUser)
		protected.PUT("/me/password", r.userHandler.ChangePassword)
		protected.GET("/me/permissions", r.userHandler.GetMyPermissions)
		protected.GET("/me/notifications", r.userHandler.GetMyNotifications)
		protected.PUT("/me/notifications/:id/read", r.userHandler.MarkNotificationRead)
		
		// Users (admin only)
		protected.GET("/users", r.authMiddleware.RequirePermission("users.view"), r.userHandler.ListUsers)
		protected.POST("/users", r.authMiddleware.RequirePermission("users.create"), r.userHandler.CreateUser)
		protected.GET("/users/:id", r.authMiddleware.RequirePermission("users.view"), r.userHandler.GetUser)
		protected.PUT("/users/:id", r.authMiddleware.RequirePermission("users.edit"), r.userHandler.UpdateUser)
		protected.DELETE("/users/:id", r.authMiddleware.RequirePermission("users.delete"), r.userHandler.DeleteUser)
		protected.POST("/users/:id/activate", r.authMiddleware.RequirePermission("users.edit"), r.userHandler.ActivateUser)
		protected.POST("/users/:id/deactivate", r.authMiddleware.RequirePermission("users.edit"), r.userHandler.DeactivateUser)
		protected.GET("/users/:id/sessions", r.authMiddleware.RequirePermission("users.view"), r.userHandler.GetUserSessions)
		protected.DELETE("/users/:id/sessions", r.authMiddleware.RequirePermission("users.edit"), r.userHandler.RevokeUserSessions)
		
		// Roles
		protected.GET("/roles", r.authMiddleware.RequirePermission("roles.view"), r.userHandler.ListRoles)
		protected.POST("/roles", r.authMiddleware.RequirePermission("roles.create"), r.userHandler.CreateRole)
		protected.GET("/roles/:id", r.authMiddleware.RequirePermission("roles.view"), r.userHandler.GetRole)
		protected.PUT("/roles/:id", r.authMiddleware.RequirePermission("roles.edit"), r.userHandler.UpdateRole)
		protected.DELETE("/roles/:id", r.authMiddleware.RequirePermission("roles.delete"), r.userHandler.DeleteRole)
		protected.GET("/roles/:id/permissions", r.authMiddleware.RequirePermission("roles.view"), r.userHandler.GetRolePermissions)
		protected.PUT("/roles/:id/permissions", r.authMiddleware.RequirePermission("roles.edit"), r.userHandler.UpdateRolePermissions)
		
		// Groups
		protected.GET("/groups", r.authMiddleware.RequirePermission("groups.view"), r.userHandler.ListGroups)
		protected.POST("/groups", r.authMiddleware.RequirePermission("groups.create"), r.userHandler.CreateGroup)
		protected.GET("/groups/:id", r.authMiddleware.RequirePermission("groups.view"), r.userHandler.GetGroup)
		protected.PUT("/groups/:id", r.authMiddleware.RequirePermission("groups.edit"), r.userHandler.UpdateGroup)
		protected.DELETE("/groups/:id", r.authMiddleware.RequirePermission("groups.delete"), r.userHandler.DeleteGroup)
		protected.GET("/groups/:id/members", r.authMiddleware.RequirePermission("groups.view"), r.userHandler.GetGroupMembers)
		protected.POST("/groups/:id/members", r.authMiddleware.RequirePermission("groups.edit"), r.userHandler.AddGroupMember)
		protected.DELETE("/groups/:id/members/:userId", r.authMiddleware.RequirePermission("groups.edit"), r.userHandler.RemoveGroupMember)
		
		// Organizations
		protected.GET("/organizations", r.authMiddleware.RequirePermission("organizations.view"), r.userHandler.ListOrganizations)
		protected.POST("/organizations", r.authMiddleware.RequirePermission("organizations.create"), r.userHandler.CreateOrganization)
		protected.GET("/organizations/:id", r.authMiddleware.RequirePermission("organizations.view"), r.userHandler.GetOrganization)
		protected.PUT("/organizations/:id", r.authMiddleware.RequirePermission("organizations.edit"), r.userHandler.UpdateOrganization)
		protected.DELETE("/organizations/:id", r.authMiddleware.RequirePermission("organizations.delete"), r.userHandler.DeleteOrganization)
		protected.GET("/organizations/:id/users", r.authMiddleware.RequirePermission("organizations.view"), r.userHandler.GetOrganizationUsers)
		
		// Tickets
		protected.GET("/tickets", r.ticketHandler.ListTickets)
		protected.POST("/tickets", r.ticketHandler.CreateTicket)
		protected.GET("/tickets/:id", r.ticketHandler.GetTicket)
		protected.PUT("/tickets/:id", r.ticketHandler.UpdateTicket)
		protected.DELETE("/tickets/:id", r.authMiddleware.RequirePermission("tickets.delete"), r.ticketHandler.DeleteTicket)
		protected.POST("/tickets/:id/assign", r.ticketHandler.AssignTicket)
		protected.POST("/tickets/:id/close", r.ticketHandler.CloseTicket)
		protected.POST("/tickets/:id/reopen", r.ticketHandler.ReopenTicket)
		protected.POST("/tickets/:id/merge", r.ticketHandler.MergeTickets)
		protected.POST("/tickets/:id/split", r.ticketHandler.SplitTicket)
		protected.GET("/tickets/:id/history", r.ticketHandler.GetTicketHistory)
		protected.GET("/tickets/:id/watchers", r.ticketHandler.GetTicketWatchers)
		protected.POST("/tickets/:id/watchers", r.ticketHandler.AddTicketWatcher)
		protected.DELETE("/tickets/:id/watchers/:userId", r.ticketHandler.RemoveTicketWatcher)
		
		// Ticket Messages
		protected.GET("/tickets/:id/messages", r.ticketHandler.ListTicketMessages)
		protected.POST("/tickets/:id/messages", r.ticketHandler.CreateTicketMessage)
		protected.PUT("/tickets/:id/messages/:messageId", r.ticketHandler.UpdateTicketMessage)
		protected.DELETE("/tickets/:id/messages/:messageId", r.ticketHandler.DeleteTicketMessage)
		
		// Ticket Attachments
		protected.GET("/tickets/:id/attachments", r.ticketHandler.ListTicketAttachments)
		protected.POST("/tickets/:id/attachments", r.ticketHandler.UploadAttachment)
		protected.GET("/tickets/:id/attachments/:attachmentId", r.ticketHandler.DownloadAttachment)
		protected.DELETE("/tickets/:id/attachments/:attachmentId", r.ticketHandler.DeleteAttachment)
		
		// Ticket Notes
		protected.GET("/tickets/:id/notes", r.ticketHandler.ListTicketNotes)
		protected.POST("/tickets/:id/notes", r.ticketHandler.CreateTicketNote)
		protected.PUT("/tickets/:id/notes/:noteId", r.ticketHandler.UpdateTicketNote)
		protected.DELETE("/tickets/:id/notes/:noteId", r.ticketHandler.DeleteTicketNote)
		
		// Ticket Tags
		protected.GET("/tickets/:id/tags", r.ticketHandler.GetTicketTags)
		protected.POST("/tickets/:id/tags", r.ticketHandler.AddTicketTag)
		protected.DELETE("/tickets/:id/tags/:tag", r.ticketHandler.RemoveTicketTag)
		
		// Ticket Templates
		protected.GET("/ticket-templates", r.ticketHandler.ListTicketTemplates)
		protected.POST("/ticket-templates", r.authMiddleware.RequirePermission("templates.create"), r.ticketHandler.CreateTicketTemplate)
		protected.GET("/ticket-templates/:id", r.ticketHandler.GetTicketTemplate)
		protected.PUT("/ticket-templates/:id", r.authMiddleware.RequirePermission("templates.edit"), r.ticketHandler.UpdateTicketTemplate)
		protected.DELETE("/ticket-templates/:id", r.authMiddleware.RequirePermission("templates.delete"), r.ticketHandler.DeleteTicketTemplate)
		
		// Canned Responses
		protected.GET("/canned-responses", r.ticketHandler.ListCannedResponses)
		protected.POST("/canned-responses", r.ticketHandler.CreateCannedResponse)
		protected.GET("/canned-responses/:id", r.ticketHandler.GetCannedResponse)
		protected.PUT("/canned-responses/:id", r.ticketHandler.UpdateCannedResponse)
		protected.DELETE("/canned-responses/:id", r.ticketHandler.DeleteCannedResponse)
		
		// Queues
		protected.GET("/queues", r.queueHandler.ListQueues)
		protected.POST("/queues", r.authMiddleware.RequirePermission("queues.create"), r.queueHandler.CreateQueue)
		protected.GET("/queues/:id", r.queueHandler.GetQueue)
		protected.PUT("/queues/:id", r.authMiddleware.RequirePermission("queues.edit"), r.queueHandler.UpdateQueue)
		protected.DELETE("/queues/:id", r.authMiddleware.RequirePermission("queues.delete"), r.queueHandler.DeleteQueue)
		protected.GET("/queues/:id/agents", r.queueHandler.GetQueueAgents)
		protected.POST("/queues/:id/agents", r.authMiddleware.RequirePermission("queues.edit"), r.queueHandler.AddQueueAgent)
		protected.DELETE("/queues/:id/agents/:agentId", r.authMiddleware.RequirePermission("queues.edit"), r.queueHandler.RemoveQueueAgent)
		protected.GET("/queues/:id/stats", r.queueHandler.GetQueueStats)
		
		// SLA
		protected.GET("/sla", r.ticketHandler.ListSLAPolicies)
		protected.POST("/sla", r.authMiddleware.RequirePermission("sla.create"), r.ticketHandler.CreateSLAPolicy)
		protected.GET("/sla/:id", r.ticketHandler.GetSLAPolicy)
		protected.PUT("/sla/:id", r.authMiddleware.RequirePermission("sla.edit"), r.ticketHandler.UpdateSLAPolicy)
		protected.DELETE("/sla/:id", r.authMiddleware.RequirePermission("sla.delete"), r.ticketHandler.DeleteSLAPolicy)
		protected.GET("/sla/:id/metrics", r.ticketHandler.GetSLAMetrics)
		
		// Workflows
		protected.GET("/workflows", r.workflowHandler.ListWorkflows)
		protected.POST("/workflows", r.authMiddleware.RequirePermission("workflows.create"), r.workflowHandler.CreateWorkflow)
		protected.GET("/workflows/:id", r.workflowHandler.GetWorkflow)
		protected.PUT("/workflows/:id", r.authMiddleware.RequirePermission("workflows.edit"), r.workflowHandler.UpdateWorkflow)
		protected.DELETE("/workflows/:id", r.authMiddleware.RequirePermission("workflows.delete"), r.workflowHandler.DeleteWorkflow)
		protected.POST("/workflows/:id/activate", r.authMiddleware.RequirePermission("workflows.edit"), r.workflowHandler.ActivateWorkflow)
		protected.POST("/workflows/:id/deactivate", r.authMiddleware.RequirePermission("workflows.edit"), r.workflowHandler.DeactivateWorkflow)
		protected.POST("/workflows/:id/test", r.workflowHandler.TestWorkflow)
		protected.GET("/workflows/:id/executions", r.workflowHandler.GetWorkflowExecutions)
		protected.POST("/workflows/:id/deploy", r.authMiddleware.RequirePermission("workflows.deploy"), r.workflowHandler.DeployWorkflow)
		
		// Workflow Templates
		protected.GET("/workflow-templates", r.workflowHandler.ListWorkflowTemplates)
		protected.GET("/workflow-templates/:id", r.workflowHandler.GetWorkflowTemplate)
		protected.POST("/workflow-templates/:id/use", r.workflowHandler.UseWorkflowTemplate)
		
		// Workflow Analytics
		protected.GET("/workflows/analytics", r.workflowHandler.GetWorkflowAnalytics)
		protected.GET("/workflows/:id/analytics", r.workflowHandler.GetWorkflowAnalyticsByID)
		protected.GET("/workflows/analytics/performance", r.workflowHandler.GetPerformanceAnalysis)
		protected.GET("/workflows/analytics/triggers", r.workflowHandler.GetTriggerAnalysis)
		protected.GET("/workflows/analytics/actions", r.workflowHandler.GetActionAnalysis)
		protected.GET("/workflows/analytics/errors", r.workflowHandler.GetErrorAnalysis)
		
		// Escalation
		protected.GET("/escalation/policies", r.workflowHandler.ListEscalationPolicies)
		protected.POST("/escalation/policies", r.authMiddleware.RequirePermission("escalation.create"), r.workflowHandler.CreateEscalationPolicy)
		protected.GET("/escalation/policies/:id", r.workflowHandler.GetEscalationPolicy)
		protected.PUT("/escalation/policies/:id", r.authMiddleware.RequirePermission("escalation.edit"), r.workflowHandler.UpdateEscalationPolicy)
		protected.DELETE("/escalation/policies/:id", r.authMiddleware.RequirePermission("escalation.delete"), r.workflowHandler.DeleteEscalationPolicy)
		protected.POST("/escalation/tickets/:id", r.workflowHandler.EscalateTicket)
		protected.GET("/escalation/history/:ticketId", r.workflowHandler.GetEscalationHistory)
		
		// Business Hours
		protected.GET("/business-hours", r.workflowHandler.ListBusinessHours)
		protected.POST("/business-hours", r.authMiddleware.RequirePermission("business_hours.create"), r.workflowHandler.CreateBusinessHours)
		protected.GET("/business-hours/:id", r.workflowHandler.GetBusinessHours)
		protected.PUT("/business-hours/:id", r.authMiddleware.RequirePermission("business_hours.edit"), r.workflowHandler.UpdateBusinessHours)
		protected.DELETE("/business-hours/:id", r.authMiddleware.RequirePermission("business_hours.delete"), r.workflowHandler.DeleteBusinessHours)
		
		// Webhooks
		protected.GET("/webhooks", r.authMiddleware.RequirePermission("webhooks.view"), r.webhookHandler.ListWebhooks)
		protected.POST("/webhooks", r.authMiddleware.RequirePermission("webhooks.create"), r.webhookHandler.CreateWebhook)
		protected.GET("/webhooks/:id", r.authMiddleware.RequirePermission("webhooks.view"), r.webhookHandler.GetWebhook)
		protected.PUT("/webhooks/:id", r.authMiddleware.RequirePermission("webhooks.edit"), r.webhookHandler.UpdateWebhook)
		protected.DELETE("/webhooks/:id", r.authMiddleware.RequirePermission("webhooks.delete"), r.webhookHandler.DeleteWebhook)
		protected.POST("/webhooks/:id/test", r.authMiddleware.RequirePermission("webhooks.test"), r.webhookHandler.TestWebhook)
		protected.GET("/webhooks/:id/logs", r.authMiddleware.RequirePermission("webhooks.view"), r.webhookHandler.GetWebhookLogs)
		protected.POST("/webhooks/:id/enable", r.authMiddleware.RequirePermission("webhooks.edit"), r.webhookHandler.EnableWebhook)
		protected.POST("/webhooks/:id/disable", r.authMiddleware.RequirePermission("webhooks.edit"), r.webhookHandler.DisableWebhook)
		
		// Search
		protected.GET("/search", r.searchHandler.Search)
		protected.GET("/search/tickets", r.searchHandler.SearchTickets)
		protected.GET("/search/users", r.searchHandler.SearchUsers)
		protected.GET("/search/organizations", r.searchHandler.SearchOrganizations)
		protected.GET("/search/suggest", r.searchHandler.GetSuggestions)
		protected.POST("/search/saved", r.searchHandler.SaveSearch)
		protected.GET("/search/saved", r.searchHandler.ListSavedSearches)
		protected.DELETE("/search/saved/:id", r.searchHandler.DeleteSavedSearch)
		protected.GET("/search/history", r.searchHandler.GetSearchHistory)
		
		// Reports
		protected.GET("/reports", r.reportHandler.ListReports)
		protected.GET("/reports/dashboard", r.reportHandler.GetDashboardData)
		protected.GET("/reports/tickets", r.reportHandler.GetTicketReport)
		protected.GET("/reports/agents", r.reportHandler.GetAgentReport)
		protected.GET("/reports/customers", r.reportHandler.GetCustomerReport)
		protected.GET("/reports/sla", r.reportHandler.GetSLAReport)
		protected.GET("/reports/trends", r.reportHandler.GetTrendsReport)
		protected.POST("/reports/export", r.reportHandler.ExportReport)
		protected.GET("/reports/scheduled", r.authMiddleware.RequirePermission("reports.schedule"), r.reportHandler.ListScheduledReports)
		protected.POST("/reports/scheduled", r.authMiddleware.RequirePermission("reports.schedule"), r.reportHandler.CreateScheduledReport)
		protected.DELETE("/reports/scheduled/:id", r.authMiddleware.RequirePermission("reports.schedule"), r.reportHandler.DeleteScheduledReport)
		
		// Audit Logs
		protected.GET("/audit", r.authMiddleware.RequirePermission("audit.view"), r.userHandler.GetAuditLogs)
		protected.GET("/audit/export", r.authMiddleware.RequirePermission("audit.export"), r.userHandler.ExportAuditLogs)
		
		// System Settings
		protected.GET("/settings", r.authMiddleware.RequirePermission("settings.view"), r.userHandler.GetSettings)
		protected.PUT("/settings", r.authMiddleware.RequirePermission("settings.edit"), r.userHandler.UpdateSettings)
		protected.GET("/settings/email", r.authMiddleware.RequirePermission("settings.view"), r.userHandler.GetEmailSettings)
		protected.PUT("/settings/email", r.authMiddleware.RequirePermission("settings.edit"), r.userHandler.UpdateEmailSettings)
		protected.POST("/settings/email/test", r.authMiddleware.RequirePermission("settings.test"), r.userHandler.TestEmailSettings)
		
		// Integrations
		protected.GET("/integrations", r.authMiddleware.RequirePermission("integrations.view"), r.workflowHandler.ListIntegrations)
		protected.POST("/integrations/slack", r.authMiddleware.RequirePermission("integrations.create"), r.workflowHandler.ConfigureSlack)
		protected.POST("/integrations/teams", r.authMiddleware.RequirePermission("integrations.create"), r.workflowHandler.ConfigureTeams)
		protected.POST("/integrations/ldap", r.authMiddleware.RequirePermission("integrations.create"), r.workflowHandler.ConfigureLDAP)
		protected.POST("/integrations/oauth/:provider", r.authMiddleware.RequirePermission("integrations.create"), r.workflowHandler.ConfigureOAuth)
		protected.DELETE("/integrations/:id", r.authMiddleware.RequirePermission("integrations.delete"), r.workflowHandler.DeleteIntegration)
		protected.POST("/integrations/:id/test", r.authMiddleware.RequirePermission("integrations.test"), r.workflowHandler.TestIntegration)
		
		// API Keys
		protected.GET("/api-keys", r.authMiddleware.RequirePermission("api_keys.view"), r.userHandler.ListAPIKeys)
		protected.POST("/api-keys", r.authMiddleware.RequirePermission("api_keys.create"), r.userHandler.CreateAPIKey)
		protected.GET("/api-keys/:id", r.authMiddleware.RequirePermission("api_keys.view"), r.userHandler.GetAPIKey)
		protected.DELETE("/api-keys/:id", r.authMiddleware.RequirePermission("api_keys.delete"), r.userHandler.DeleteAPIKey)
		protected.POST("/api-keys/:id/regenerate", r.authMiddleware.RequirePermission("api_keys.edit"), r.userHandler.RegenerateAPIKey)
	}
	
	// WebSocket endpoints
	protected := v1.Group("")
	protected.Use(r.authMiddleware.RequireAuth())
	{
		protected.GET("/ws", r.handleWebSocket)
		protected.GET("/ws/tickets/:id", r.handleTicketWebSocket)
	}
}

// healthCheck returns API health status
func (r *APIRouter) healthCheck(c *gin.Context) {
	c.JSON(200, gin.H{
		"status": "healthy",
		"timestamp": time.Now().Unix(),
	})
}

// versionInfo returns API version information
func (r *APIRouter) versionInfo(c *gin.Context) {
	c.JSON(200, gin.H{
		"version": "1.0.0",
		"api_version": "v1",
		"build": "2025.08.17",
		"features": []string{
			"tickets",
			"workflows",
			"webhooks",
			"search",
			"reports",
			"integrations",
			"oauth2",
			"websocket",
		},
	})
}

// handleWebSocket handles WebSocket connections
func (r *APIRouter) handleWebSocket(c *gin.Context) {
	// WebSocket implementation would go here
	// This would handle real-time updates
}

// handleTicketWebSocket handles WebSocket for specific ticket updates
func (r *APIRouter) handleTicketWebSocket(c *gin.Context) {
	// WebSocket implementation for ticket-specific updates
}