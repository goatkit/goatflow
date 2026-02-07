package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/services/adapter"
)

// Exported handlers for use by the routing system

// Core handlers.
var (
	HandleLoginPage           = handleLoginPage
	HandleCustomerLoginPage   = handleCustomerLoginPage
	HandleLogout              = handleLogout
	HandleDashboard           = handleDashboard
	HandleDashboardStats      = handleDashboardStats
	HandleRecentTickets       = handleRecentTickets
	HandleActivityStream      = handleActivityStream
	HandlePendingReminderFeed = handlePendingReminderFeed
	HandleUpdateTicketStatus  = handleUpdateTicketStatus
)

// Auth API handlers are directly exported from auth_handlers.go

// Admin handlers.
var (
	HandleAdminDashboard = handleAdminDashboard
	// Users are handled by dynamic modules and admin_users_handlers.go.
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
	// Roles management.
	HandleAdminRoles                 = handleAdminRoles
	HandleAdminRoleCreate            = handleAdminRoleCreate
	HandleAdminRoleGet               = handleAdminRoleGet
	HandleAdminRoleUpdate            = handleAdminRoleUpdate
	HandleAdminRoleDelete            = handleAdminRoleDelete
	HandleAdminRoleUsers             = handleAdminRoleUsers
	HandleAdminRoleUsersSearch       = handleAdminRoleUsersSearch
	HandleAdminRoleUserAdd           = handleAdminRoleUserAdd
	HandleAdminRoleUserRemove        = handleAdminRoleUserRemove
	HandleAdminRolePermissions       = handleAdminRolePermissions
	HandleAdminRolePermissionsUpdate = handleAdminRolePermissionsUpdate
	// Customer company handlers - wrapped to get database from adapter.
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

	// Customer user ↔ services management.
	HandleAdminCustomerUserServices = func(c *gin.Context) {
		dbService, err := adapter.GetDatabase()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
			return
		}
		handleAdminCustomerUserServices(dbService.GetDB())(c)
	}
	HandleAdminCustomerUserServicesAllocate = func(c *gin.Context) {
		dbService, err := adapter.GetDatabase()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
			return
		}
		handleAdminCustomerUserServicesAllocate(dbService.GetDB())(c)
	}
	HandleAdminCustomerUserServicesUpdate = func(c *gin.Context) {
		dbService, err := adapter.GetDatabase()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
			return
		}
		handleAdminCustomerUserServicesUpdate(dbService.GetDB())(c)
	}
	HandleAdminServiceCustomerUsersAllocate = func(c *gin.Context) {
		dbService, err := adapter.GetDatabase()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
			return
		}
		handleAdminServiceCustomerUsersAllocate(dbService.GetDB())(c)
	}
	HandleAdminServiceCustomerUsersUpdate = func(c *gin.Context) {
		dbService, err := adapter.GetDatabase()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
			return
		}
		handleAdminServiceCustomerUsersUpdate(dbService.GetDB())(c)
	}
	HandleAdminDefaultServices = func(c *gin.Context) {
		dbService, err := adapter.GetDatabase()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
			return
		}
		handleAdminDefaultServices(dbService.GetDB())(c)
	}
	HandleAdminDefaultServicesUpdate = func(c *gin.Context) {
		dbService, err := adapter.GetDatabase()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
			return
		}
		handleAdminDefaultServicesUpdate(dbService.GetDB())(c)
	}

	// Dynamic Fields management.
	HandleAdminDynamicFields                  = handleAdminDynamicFields
	HandleAdminDynamicFieldNew                = handleAdminDynamicFieldNew
	HandleAdminDynamicFieldEdit               = handleAdminDynamicFieldEdit
	HandleAdminDynamicFieldScreenConfig       = handleAdminDynamicFieldScreenConfig
	HandleCreateDynamicField                  = handleCreateDynamicField
	HandleUpdateDynamicField                  = handleUpdateDynamicField
	HandleDeleteDynamicField                  = handleDeleteDynamicField
	HandleAdminDynamicFieldScreenConfigSave   = handleAdminDynamicFieldScreenConfigSave
	HandleAdminDynamicFieldScreenConfigSingle = handleAdminDynamicFieldScreenConfigSingle
)

// Ticket handlers.
var (
	HandleTicketDetail   = handleTicketDetail
	HandleQueueDetail    = handleQueueDetail
	HandleNewTicket      = handleNewTicket
	HandleNewEmailTicket = handleNewEmailTicket
	HandleNewPhoneTicket = handleNewPhoneTicket
)

// Attachment handlers (exported for routing).
var (
	HandleGetAttachments     = handleGetAttachments
	HandleUploadAttachment   = handleUploadAttachment
	HandleDownloadAttachment = handleDownloadAttachment
	HandleDeleteAttachment   = handleDeleteAttachment
	HandleGetThumbnail       = handleGetThumbnail
	HandleViewAttachment     = handleViewAttachment
)
