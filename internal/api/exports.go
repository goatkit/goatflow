package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/services/adapter"
)

// Exported handlers for use by the routing system

// Core handlers
var (
	HandleLoginPage           = handleLoginPage
	HandleCustomerLoginPage   = handleCustomerLoginPage
	HandleLogout              = handleLogout
	HandleDashboard           = handleDashboard
	HandleDashboardStats      = handleDashboardStats
	HandleRecentTickets       = handleRecentTickets
	DashboardQueueStatus      = dashboard_queue_status
	HandleActivityStream      = handleActivityStream
	HandlePendingReminderFeed = handlePendingReminderFeed
	HandleUpdateTicketStatus  = handleUpdateTicketStatus
)

// Auth API handlers are directly exported from auth_handlers.go

// Admin handlers
var (
	HandleAdminDashboard = handleAdminDashboard
	// Users are handled by dynamic modules and admin_users_handlers.go
	HandleAdminUserEdit           = HandleAdminUserGet // Same handler for edit form
	HandleAdminPasswordPolicy     = HandlePasswordPolicy
	HandleAdminGroups             = handleAdminGroups
	HandleGetGroup                = handleGetGroup
	HandleCreateGroup             = handleCreateGroup
	HandleUpdateGroup             = handleUpdateGroup
	HandleDeleteGroup             = handleDeleteGroup
	HandleGroupMembers            = handleGetGroupMembers
	HandleAddUserToGroup          = handleAddUserToGroup
	HandleRemoveUserFromGroup     = handleRemoveUserFromGroup
	HandleGroupPermissions        = handleGroupPermissions
	HandleSaveGroupPermissions    = handleSaveGroupPermissions
	HandleAdminQueues             = handleAdminQueues
	HandleAdminEmailIdentities    = handleAdminEmailIdentities
	HandleAdminPriorities         = handleAdminPriorities
	HandleAdminPermissions        = handleAdminPermissions // Renamed from roles
	HandleGetUserPermissionMatrix = handleGetUserPermissionMatrix
	HandleUpdateUserPermissions   = handleUpdateUserPermissions
	HandleAdminEmailQueue         = handleAdminEmailQueue
	HandleAdminEmailQueueRetry    = handleAdminEmailQueueRetry
	HandleAdminEmailQueueDelete   = handleAdminEmailQueueDelete
	HandleAdminEmailQueueRetryAll = handleAdminEmailQueueRetryAll
	HandleAdminStates             = handleAdminStates
	HandleAdminTypes              = handleAdminTypes
	HandleAdminServices           = handleAdminServices
	HandleAdminServiceCreate      = handleAdminServiceCreate
	HandleAdminServiceUpdate      = handleAdminServiceUpdate
	HandleAdminServiceDelete      = handleAdminServiceDelete
	HandleAdminSLA                = handleAdminSLA
	HandleAdminSLACreate          = handleAdminSLACreate
	HandleAdminSLAUpdate          = handleAdminSLAUpdate
	HandleAdminSLADelete          = handleAdminSLADelete
	HandleAdminLookups            = handleAdminLookups
	// Customer company handlers - wrapped to get database from adapter
	HandleAdminCustomerCompanies = func(c *gin.Context) {
		dbService, err := adapter.GetDatabase()
		if err != nil {
			handleAdminCustomerCompanies(nil)(c)
			return
		}
		handleAdminCustomerCompanies(dbService.GetDB())(c)
	}
	HandleAdminCustomerCompanyUsers = func(c *gin.Context) {
		dbService, err := adapter.GetDatabase()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
			return
		}
		handleAdminCustomerCompanyUsers(dbService.GetDB())(c)
	}
	HandleAdminCustomerCompanyTickets = func(c *gin.Context) {
		dbService, err := adapter.GetDatabase()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
			return
		}
		handleAdminCustomerCompanyTickets(dbService.GetDB())(c)
	}
	HandleAdminCustomerCompanyServices = func(c *gin.Context) {
		dbService, err := adapter.GetDatabase()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
			return
		}
		handleAdminCustomerCompanyServices(dbService.GetDB())(c)
	}
	HandleAdminCustomerPortalSettings = func(c *gin.Context) {
		dbService, err := adapter.GetDatabase()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
			return
		}
		handleAdminCustomerPortalSettings(dbService.GetDB())(c)
	}
	HandleAdminNewCustomerCompany = func(c *gin.Context) {
		dbService, err := adapter.GetDatabase()
		if err != nil {
			handleAdminNewCustomerCompany(nil)(c)
			return
		}
		handleAdminNewCustomerCompany(dbService.GetDB())(c)
	}
	HandleAdminCreateCustomerCompany = func(c *gin.Context) {
		dbService, err := adapter.GetDatabase()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
			return
		}
		handleAdminCreateCustomerCompany(dbService.GetDB())(c)
	}
	HandleAdminEditCustomerCompany = func(c *gin.Context) {
		dbService, err := adapter.GetDatabase()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
			return
		}
		handleAdminEditCustomerCompany(dbService.GetDB())(c)
	}
	HandleAdminUpdateCustomerCompany = func(c *gin.Context) {
		dbService, err := adapter.GetDatabase()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
			return
		}
		handleAdminUpdateCustomerCompany(dbService.GetDB())(c)
	}
	HandleAdminDeleteCustomerCompany = func(c *gin.Context) {
		dbService, err := adapter.GetDatabase()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
			return
		}
		handleAdminDeleteCustomerCompany(dbService.GetDB())(c)
	}
	HandleAdminUpdateCustomerCompanyServices = func(c *gin.Context) {
		dbService, err := adapter.GetDatabase()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
			return
		}
		handleAdminUpdateCustomerCompanyServices(dbService.GetDB())(c)
	}
	HandleAdminUpdateCustomerPortalSettings = func(c *gin.Context) {
		dbService, err := adapter.GetDatabase()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
			return
		}
		handleAdminUpdateCustomerPortalSettings(dbService.GetDB())(c)
	}
	HandleAdminActivateCustomerCompany = func(c *gin.Context) {
		dbService, err := adapter.GetDatabase()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
			return
		}
		handleAdminActivateCustomerCompany(dbService.GetDB())(c)
	}
	HandleAdminUploadCustomerPortalLogo = func(c *gin.Context) {
		dbService, err := adapter.GetDatabase()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
			return
		}
		handleAdminUploadCustomerPortalLogo(dbService.GetDB())(c)
	}
)

// Ticket handlers
var (
	HandleTicketDetail = handleTicketDetail
	HandleQueueDetail  = handleQueueDetail
)

// Attachment handlers (exported for routing)
var (
	HandleGetAttachments     = handleGetAttachments
	HandleUploadAttachment   = handleUploadAttachment
	HandleDownloadAttachment = handleDownloadAttachment
	HandleDeleteAttachment   = handleDeleteAttachment
	HandleGetThumbnail       = handleGetThumbnail
	HandleViewAttachment     = handleViewAttachment
)

// Dev handlers
var (
	HandleDevDashboard  = handleDevDashboard
	HandleClaudeTickets = handleClaudeTickets
	HandleDevAction     = handleDevAction
	HandleDevLogs       = handleDevLogs
	HandleDevDatabase   = handleDevDatabase
)

// GetPongo2Renderer returns the pongo2 renderer for template rendering
func GetPongo2Renderer() *Pongo2Renderer {
	return pongo2Renderer
}

// InitPongo2Renderer initializes the global pongo2 renderer
func InitPongo2Renderer(templateDir string) {
	pongo2Renderer = NewPongo2Renderer(templateDir)
}
