package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/services/adapter"
)

// Exported handlers for use by the routing system

// Core handlers
var (
	HandleLoginPage      = handleLoginPage
	HandleLogout         = handleLogout
	HandleDashboard      = handleDashboard
	HandleDashboardStats = handleDashboardStats
	HandleRecentTickets  = handleRecentTickets
	HandleActivityStream = handleActivityStream
)

// Auth API handlers are directly exported from auth_handlers.go

// Admin handlers
var (
	HandleAdminDashboard = handleAdminDashboard
	HandleAdminUsers     = handleAdminUsers
	HandleAdminUserEdit  = HandleAdminUserGet // Same handler for edit form
	HandleAdminPasswordPolicy = HandlePasswordPolicy
	HandleAdminGroups         = handleAdminGroups
	HandleGetGroup            = handleGetGroup
	HandleCreateGroup         = handleCreateGroup
	HandleUpdateGroup         = handleUpdateGroup
	HandleDeleteGroup         = handleDeleteGroup
	HandleGroupMembers        = handleGetGroupMembers
	HandleAddUserToGroup      = handleAddUserToGroup
	HandleRemoveUserFromGroup = handleRemoveUserFromGroup
	HandleAdminQueues         = handleAdminQueues
	HandleAdminPriorities     = handleAdminPriorities
	HandleAdminPermissions    = handleAdminPermissions // Renamed from roles
	HandleAdminStates         = handleAdminStates
	HandleAdminTypes          = handleAdminTypes
	HandleAdminServices       = handleAdminServices
	HandleAdminSLA            = handleAdminSLA
	HandleAdminLookups        = handleAdminLookups
	// Customer company handlers - wrapped to get database from adapter
	HandleAdminCustomerCompanies = func(c *gin.Context) {
		dbService, err := adapter.GetDatabase()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
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
)

// Ticket handlers
var (
	HandleTicketDetail = handleTicketDetail
)

// GetPongo2Renderer returns the pongo2 renderer for template rendering
func GetPongo2Renderer() *Pongo2Renderer {
	return pongo2Renderer
}

// InitPongo2Renderer initializes the global pongo2 renderer
func InitPongo2Renderer(templateDir string) {
	pongo2Renderer = NewPongo2Renderer(templateDir)
}
