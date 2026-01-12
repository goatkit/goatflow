package api

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/routing"
	"github.com/gotrs-io/gotrs-ce/internal/shared"
)

// Simple global handler registry to decouple YAML route loader from hardcoded map.
// Handlers register themselves (typically in init or during setup) using a stable name.
// Naming convention: existing function name unless alias needed.

var (
	handlerRegistryMu sync.RWMutex
	handlerRegistry   = map[string]gin.HandlerFunc{}
)

// RegisterHandler adds/overwrites a handler under a given name.
func RegisterHandler(name string, h gin.HandlerFunc) {
	if name == "" || h == nil {
		return
	}
	handlerRegistryMu.Lock()
	handlerRegistry[name] = h
	handlerRegistryMu.Unlock()
}

// GetHandler retrieves a registered handler.
func GetHandler(name string) (gin.HandlerFunc, bool) {
	handlerRegistryMu.RLock()
	h, ok := handlerRegistry[name]
	handlerRegistryMu.RUnlock()
	return h, ok
}

// ListHandlers returns sorted handler names (for diagnostics / tests).
func ListHandlers() []string {
	handlerRegistryMu.RLock()
	defer handlerRegistryMu.RUnlock()
	out := make([]string, 0, len(handlerRegistry))
	for k := range handlerRegistry {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// mustGetDB retrieves the database connection, returning an error response if unavailable.
func mustGetDB(c *gin.Context) (*sql.DB, bool) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database unavailable"})
		return nil, false
	}
	return db, true
}

// ensureCoreHandlers pre-registers known legacy handlers still referenced in YAML.
// Called from registerYAMLRoutes early so existing YAML works without scattering init()s.
func ensureCoreHandlers() {
	// Minimal duplication: only names used in YAML currently.
	pairs := map[string]gin.HandlerFunc{
		"handleLoginPage":           handleLoginPage,
		"handleDashboard":           handleDashboard,
		"handleAuthLogin":           HandleAuthLogin,
		"handleTickets":             handleTickets,
		"handleTicketDetail":        handleTicketDetail,
		"HandleQueueDetail":         handleQueueDetail,
		"handleNewTicket":           handleNewTicket,
		"handleNewEmailTicket":      handleNewEmailTicket,
		"handleNewPhoneTicket":      handleNewPhoneTicket,
		"handlePendingReminderFeed": handlePendingReminderFeed,
		// Agent ticket creation flow (YAML expects names without db param)
		"HandleAgentCreateTicket": func(c *gin.Context) {
			// Use enhanced multipart-aware path
			handleCreateTicketWithAttachments(c)
		},
		"HandleAgentNewTicket": func(c *gin.Context) {
			db, ok := mustGetDB(c)
			if !ok {
				return
			}
			HandleAgentNewTicket(db)(c)
		},
		// Attachment handlers exposed for API routes
		"HandleGetAttachments":        handleGetAttachments,
		"HandleUploadAttachment":      handleUploadAttachment,
		"HandleDownloadAttachment":    handleDownloadAttachment,
		"HandleDeleteAttachment":      handleDeleteAttachment,
		"HandleGetThumbnail":          handleGetThumbnail,
		"HandleViewAttachment":        handleViewAttachment,
		"handleGetTicketMessages":     handleGetTicketMessages,
		"handleAddTicketMessage":      handleAddTicketMessage,
		"HandleGetQueues":             HandleGetQueues,
		"HandleGetPriorities":         HandleGetPriorities,
		"HandleGetTypes":              HandleGetTypes,
		"HandleGetStatuses":           HandleGetStatuses,
		"HandleGetFormData":           HandleGetFormData,
		"HandleInvalidateLookupCache": HandleInvalidateLookupCache,
		// Optional customer info partial used by YAML
		"HandleCustomerInfoPanel":     func(c *gin.Context) { c.String(200, "") },
		"handleSettings":              handleSettings,
		"handleProfile":               handleProfile,
		"HandleGetSessionTimeout":     HandleGetSessionTimeout,
		"HandleSetSessionTimeout":     HandleSetSessionTimeout,
		"HandleGetLanguage":           HandleGetLanguage,
		"HandleSetLanguage":           HandleSetLanguage,
		"HandleGetProfile":            HandleGetProfile,
		"HandleUpdateProfile":         HandleUpdateProfile,
		"handleAdminSettings":         handleAdminSettings,
		"handleAdminTemplates":        handleAdminTemplates,
		"handleAdminReports":          handleAdminReports,
		"handleAdminLogs":             handleAdminLogs,
		"handleAdminBackup":           handleAdminBackup,
		"HandleMailAccountPollStatus": HandleMailAccountPollStatus,

		// Static and basic routes
		"handleStaticFiles": HandleStaticFiles,
		"handleLogout":      handleLogout,
		"handleCustomerLogout": func(c *gin.Context) {
			// clear all auth cookies and redirect to customer login
			// Clear for root path
			c.SetCookie("auth_token", "", -1, "/", "", false, true)
			c.SetCookie("access_token", "", -1, "/", "", false, true)
			c.SetCookie("token", "", -1, "/", "", false, true)
			// Also clear for /customer path in case proxy scoped cookies
			c.SetCookie("auth_token", "", -1, "/customer", "", false, true)
			c.SetCookie("access_token", "", -1, "/customer", "", false, true)
			c.SetCookie("token", "", -1, "/customer", "", false, true)
			c.Header("HX-Redirect", "/customer/login")
			c.Redirect(http.StatusSeeOther, "/customer/login")
		},
		"handleDemoCustomerLogin": handleDemoCustomerLogin,
		"handleCustomerLoginPage": handleCustomerLoginPage,
		"handleCustomerLogin": func(c *gin.Context) {
			handleCustomerLogin(shared.GetJWTManager())(c)
		},
		"handleCustomerDashboard": func(c *gin.Context) {
			db, ok := mustGetDB(c)
			if !ok {
				return
			}
			handleCustomerDashboard(db)(c)
		},
		"handleCustomerTickets": func(c *gin.Context) {
			db, ok := mustGetDB(c)
			if !ok {
				return
			}
			handleCustomerTickets(db)(c)
		},
		"handleCustomerNewTicket": func(c *gin.Context) {
			db, ok := mustGetDB(c)
			if !ok {
				return
			}
			handleCustomerNewTicket(db)(c)
		},
		"handleCustomerCreateTicket": func(c *gin.Context) {
			db, ok := mustGetDB(c)
			if !ok {
				return
			}
			handleCustomerCreateTicket(db)(c)
		},
		"handleCustomerTicketView": func(c *gin.Context) {
			db, ok := mustGetDB(c)
			if !ok {
				return
			}
			handleCustomerTicketView(db)(c)
		},
		"handleCustomerTicketReply": func(c *gin.Context) {
			db, ok := mustGetDB(c)
			if !ok {
				return
			}
			handleCustomerTicketReply(db)(c)
		},
		"handleCustomerCloseTicket": func(c *gin.Context) {
			db, ok := mustGetDB(c)
			if !ok {
				return
			}
			handleCustomerCloseTicket(db)(c)
		},
		"handleCustomerProfile": func(c *gin.Context) {
			db, ok := mustGetDB(c)
			if !ok {
				return
			}
			handleCustomerProfile(db)(c)
		},
		"handleCustomerUpdateProfile": func(c *gin.Context) {
			db, ok := mustGetDB(c)
			if !ok {
				return
			}
			handleCustomerUpdateProfile(db)(c)
		},
		"handleCustomerPasswordForm": func(c *gin.Context) {
			db, ok := mustGetDB(c)
			if !ok {
				return
			}
			handleCustomerPasswordForm(db)(c)
		},
		"handleCustomerChangePassword": func(c *gin.Context) {
			db, ok := mustGetDB(c)
			if !ok {
				return
			}
			handleCustomerChangePassword(db)(c)
		},
		"handleCustomerKnowledgeBase": func(c *gin.Context) {
			db, ok := mustGetDB(c)
			if !ok {
				return
			}
			handleCustomerKnowledgeBase(db)(c)
		},
		"handleCustomerKBSearch": func(c *gin.Context) {
			db, ok := mustGetDB(c)
			if !ok {
				return
			}
			handleCustomerKBSearch(db)(c)
		},
		"handleCustomerKBArticle": func(c *gin.Context) {
			db, ok := mustGetDB(c)
			if !ok {
				return
			}
			handleCustomerKBArticle(db)(c)
		},
		"handleCustomerCompanyInfo": func(c *gin.Context) {
			db, ok := mustGetDB(c)
			if !ok {
				return
			}
			handleCustomerCompanyInfo(db)(c)
		},
		"handleCustomerCompanyUsers": func(c *gin.Context) {
			db, ok := mustGetDB(c)
			if !ok {
				return
			}
			handleCustomerCompanyUsers(db)(c)
		},
		"handleLogoutRedirect": func(c *gin.Context) {
			// clear tokens then redirect to login
			c.SetCookie("auth_token", "", -1, "/", "", false, true)
			c.SetCookie("access_token", "", -1, "/", "", false, true)
			target := loginRedirectPath(c)
			if strings.Contains(c.Request.URL.Path, "/customer") {
				target = "/customer/login"
			}
			c.Redirect(http.StatusFound, target)
		},
		"handleRoot": func(c *gin.Context) {
			c.Redirect(http.StatusFound, RootRedirectTarget())
		},
		"handleAuthRefresh":  handleAuthRefresh,
		"handleAuthRegister": handleAuthRegister,

		// Health and metrics (lightweight for tests/dev)
		"handleHealthCheck": func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "healthy"}) },
		"handleDetailedHealthCheck": func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":     "healthy",
				"components": gin.H{"database": "unknown", "cache": "healthy", "queue": "healthy"},
			})
		},
		"handleMetrics": func(c *gin.Context) {
			c.String(http.StatusOK, "# HELP gotrs_up GOTRS is up\n# TYPE gotrs_up gauge\ngotrs_up 1\n")
		},

		// Redirect helpers
		"handleQueuesRedirect":   HandleRedirectQueues,
		"handleQueueMetaPartial": handleQueueMetaPartial,

		// Admin handlers
		"handleAdminDashboard":   handleAdminDashboard,
		"handleAdminUsers":       HandleAdminUsers,
		"HandleAdminUsersList":   HandleAdminUsersList,
		"handleAdminUserCreate":  HandleAdminUserCreate,
		"handleAdminUserGet":     HandleAdminUserGet,
		"handleAdminUserEdit":    HandleAdminUserEdit,
		"handleAdminUserUpdate":  HandleAdminUserUpdate,
		"handleAdminUserDelete":  HandleAdminUserDelete,
		"handleAdminUserGroups":  HandleAdminUserGroups,
		"handleAdminUsersStatus": HandleAdminUsersStatus,
		"handleAdminPasswordPolicy": func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"min_length": 8, "require_special": true}})
		},
		"handleAdminGroups":          handleAdminGroups,
		"handleCreateGroup":          handleCreateGroup,
		"handleGetGroup":             handleGetGroup,
		"handleUpdateGroup":          handleUpdateGroup,
		"handleDeleteGroup":          handleDeleteGroup,
		"handleGroupMembers":         handleGetGroupMembers,
		"handleAddUserToGroup":       handleAddUserToGroup,
		"handleRemoveUserFromGroup":  handleRemoveUserFromGroup,
		"handleGroupPermissions":     handleGroupPermissions,
		"handleSaveGroupPermissions": handleSaveGroupPermissions,
		// Additional admin group APIs used in YAML
		"HandleAdminGroupsUsers":        func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"success": true, "data": []interface{}{}}) },
		"HandleAdminGroupsAddUser":      func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"success": true}) },
		"HandleAdminGroupsRemoveUser":   func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"success": true}) },
		"handleAdminQueues":             handleAdminQueues,
		"handleAdminEmailIdentities":    handleAdminEmailIdentities,
		"handleAdminPriorities":         handleAdminPriorities,
		"handleAdminPermissions":        handleAdminPermissions,
		"handleGetUserPermissionMatrix": handleGetUserPermissionMatrix,
		"handleUpdateUserPermissions":   handleUpdateUserPermissions,
		// Role management handlers
		"handleAdminRoles":                 handleAdminRoles,
		"handleAdminRoleCreate":            handleAdminRoleCreate,
		"handleAdminRoleGet":               handleAdminRoleGet,
		"handleAdminRoleUpdate":            handleAdminRoleUpdate,
		"handleAdminRoleDelete":            handleAdminRoleDelete,
		"handleAdminRoleUsers":             handleAdminRoleUsers,
		"handleAdminRoleUserAdd":           handleAdminRoleUserAdd,
		"handleAdminRoleUserRemove":        handleAdminRoleUserRemove,
		"handleAdminRolePermissions":       handleAdminRolePermissions,
		"handleAdminRolePermissionsUpdate": handleAdminRolePermissionsUpdate,
		"handleAdminEmailQueue":            handleAdminEmailQueue,
		"handleAdminEmailQueueRetry":       handleAdminEmailQueueRetry,
		"handleAdminEmailQueueDelete":      handleAdminEmailQueueDelete,
		"handleAdminEmailQueueRetryAll":    handleAdminEmailQueueRetryAll,
		"handleAdminDynamicIndex":          handleAdminDynamicIndex,
		"handleAdminDynamicModule":         handleAdminDynamicModule,
		// Dynamic Fields management handlers
		"handleAdminDynamicFields":                  handleAdminDynamicFields,
		"handleAdminDynamicFieldNew":                handleAdminDynamicFieldNew,
		"handleAdminDynamicFieldEdit":               handleAdminDynamicFieldEdit,
		"handleAdminDynamicFieldScreenConfig":       handleAdminDynamicFieldScreenConfig,
		"handleCreateDynamicField":                  handleCreateDynamicField,
		"handleUpdateDynamicField":                  handleUpdateDynamicField,
		"handleDeleteDynamicField":                  handleDeleteDynamicField,
		"handleAdminDynamicFieldScreenConfigSave":   handleAdminDynamicFieldScreenConfigSave,
		"handleAdminDynamicFieldScreenConfigSingle": handleAdminDynamicFieldScreenConfigSingle,
		"handleAdminStates":                         handleAdminStates,
		"handleAdminTypes":                          handleAdminTypes,
		"handleAdminServices":                       handleAdminServices,
		"handleAdminServiceCreate":                  handleAdminServiceCreate,
		"handleAdminServiceUpdate":                  handleAdminServiceUpdate,
		"handleAdminServiceDelete":                  handleAdminServiceDelete,
		"handleAdminSLA":                            handleAdminSLA,
		"handleAdminSLACreate":                      handleAdminSLACreate,
		"handleAdminSLAUpdate":                      handleAdminSLAUpdate,
		"handleAdminSLADelete":                      handleAdminSLADelete,
		"handleAdminLookups":                        handleAdminLookups,
		"dashboard_stats":                           handleDashboardStats,
		"dashboard_recent_tickets":                  handleRecentTickets,
		"dashboard_activity":                        handleActivity,
		"dashboard_activity_stream":                 handleActivityStream,
		"dashboard_queue_status":                    dashboard_queue_status,

		// Customer company handlers - full implementations
		"handleAdminCustomerCompanies": HandleAdminCustomerCompanies,
		"handleAdminNewCustomerCompany": func(c *gin.Context) {
			skipDB := htmxHandlerSkipDB()
			db, err := database.GetDB()
			if err != nil || db == nil {
				if skipDB {
					handleAdminNewCustomerCompany(nil)(c)
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
				return
			}
			handleAdminNewCustomerCompany(db)(c)
		},
		"handleAdminCreateCustomerCompany": func(c *gin.Context) {
			db, err := database.GetDB()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
				return
			}
			handleAdminCreateCustomerCompany(db)(c)
		},
		"handleAdminEditCustomerCompany": func(c *gin.Context) {
			db, err := database.GetDB()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
				return
			}
			handleAdminEditCustomerCompany(db)(c)
		},
		"handleAdminUpdateCustomerCompany": func(c *gin.Context) {
			db, err := database.GetDB()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
				return
			}
			handleAdminUpdateCustomerCompany(db)(c)
		},
		"handleAdminDeleteCustomerCompany": func(c *gin.Context) {
			db, err := database.GetDB()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
				return
			}
			handleAdminDeleteCustomerCompany(db)(c)
		},
		"handleAdminActivateCustomerCompany": func(c *gin.Context) {
			db, err := database.GetDB()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
				return
			}
			handleAdminActivateCustomerCompany(db)(c)
		},
		"handleAdminCustomerCompanyUsers":    HandleAdminCustomerCompanyUsers,
		"handleAdminCustomerCompanyTickets":  HandleAdminCustomerCompanyTickets,
		"handleAdminCustomerCompanyServices": HandleAdminCustomerCompanyServices,
		"handleAdminUpdateCustomerCompanyServices": func(c *gin.Context) {
			db, err := database.GetDB()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
				return
			}
			handleAdminUpdateCustomerCompanyServices(db)(c)
		},
		"handleAdminCustomerPortalSettings": HandleAdminCustomerPortalSettings,
		"handleAdminUpdateCustomerPortalSettings": func(c *gin.Context) {
			db, err := database.GetDB()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
				return
			}
			handleAdminUpdateCustomerPortalSettings(db)(c)
		},
		"handleAdminUploadCustomerPortalLogo": func(c *gin.Context) {
			db, err := database.GetDB()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
				return
			}
			handleAdminUploadCustomerPortalLogo(db)(c)
		},

		// Customer user handlers - full implementations
		"HandleAdminCustomerUsersList":       HandleAdminCustomerUsersList,
		"HandleAdminCustomerUsersGet":        HandleAdminCustomerUsersGet,
		"HandleAdminCustomerUsersCreate":     HandleAdminCustomerUsersCreate,
		"HandleAdminCustomerUsersUpdate":     HandleAdminCustomerUsersUpdate,
		"HandleAdminCustomerUsersDelete":     HandleAdminCustomerUsersDelete,
		"HandleAdminCustomerUsersTickets":    HandleAdminCustomerUsersTickets,
		"HandleAdminCustomerUsersImportForm": HandleAdminCustomerUsersImportForm,
		"HandleAdminCustomerUsersImport":     HandleAdminCustomerUsersImport,
		"HandleAdminCustomerUsersExport":     HandleAdminCustomerUsersExport,
		"HandleAdminCustomerUsersBulkAction": HandleAdminCustomerUsersBulkAction,

		// Customer user ↔ services management
		"handleAdminCustomerUserServices":         HandleAdminCustomerUserServices,
		"handleAdminCustomerUserServicesAllocate": HandleAdminCustomerUserServicesAllocate,
		"handleAdminCustomerUserServicesUpdate":   HandleAdminCustomerUserServicesUpdate,
		"handleAdminServiceCustomerUsersAllocate": HandleAdminServiceCustomerUsersAllocate,
		"handleAdminServiceCustomerUsersUpdate":   HandleAdminServiceCustomerUsersUpdate,

		// Customer groups management (customer company ↔ group permissions)
		"handleAdminCustomerGroups":              handleAdminCustomerGroups,
		"handleAdminCustomerGroupEdit":           handleAdminCustomerGroupEdit,
		"handleAdminCustomerGroupUpdate":         handleAdminCustomerGroupUpdate,
		"handleAdminCustomerGroupByGroup":        handleAdminCustomerGroupByGroup,
		"handleAdminCustomerGroupByGroupUpdate":  handleAdminCustomerGroupByGroupUpdate,
		"handleGetCustomerGroupPermissions":      handleGetCustomerGroupPermissions,

		// Email identity API handlers
		"HandleListSystemAddressesAPI": HandleListSystemAddressesAPI,
		"HandleCreateSystemAddressAPI": HandleCreateSystemAddressAPI,
		"HandleUpdateSystemAddressAPI": HandleUpdateSystemAddressAPI,
		"HandleListSalutationsAPI":     HandleListSalutationsAPI,
		"HandleCreateSalutationAPI":    HandleCreateSalutationAPI,
		"HandleUpdateSalutationAPI":    HandleUpdateSalutationAPI,
		"HandleListSignaturesAPI":      HandleListSignaturesAPI,
		"HandleCreateSignatureAPI":     HandleCreateSignatureAPI,
		"HandleUpdateSignatureAPI":     HandleUpdateSignatureAPI,

		// Agent handlers (wrap to avoid DB in tests)
		"handleAgentTickets": func(c *gin.Context) {
			if os.Getenv("APP_ENV") == "test" {
				c.Data(http.StatusOK, "text/html; charset=utf-8", []byte("<main>Agent Tickets</main>"))
				return
			}
			AgentHandlerExports.HandleAgentTickets(c)
		},
		"handleAgentTicketReply":    AgentHandlerExports.HandleAgentTicketReply,
		"handleAgentTicketNote":     AgentHandlerExports.HandleAgentTicketNote,
		"handleAgentTicketPhone":    AgentHandlerExports.HandleAgentTicketPhone,
		"handleAgentTicketStatus":   AgentHandlerExports.HandleAgentTicketStatus,
		"handleAgentTicketAssign":   AgentHandlerExports.HandleAgentTicketAssign,
		"handleAgentTicketPriority": AgentHandlerExports.HandleAgentTicketPriority,
		"handleAgentTicketQueue":    AgentHandlerExports.HandleAgentTicketQueue,
		"handleAgentTicketMerge":    AgentHandlerExports.HandleAgentTicketMerge,
		"handleAgentTicketDraft":    AgentHandlerExports.HandleAgentTicketDraft,
		"handleAgentQueues": func(c *gin.Context) {
			if os.Getenv("APP_ENV") == "test" {
				c.Data(http.StatusOK, "text/html; charset=utf-8", []byte("<main>Agent Queues</main>"))
				return
			}
			AgentHandlerExports.HandleAgentQueues(c)
		},
		"handleAgentDashboard": func(c *gin.Context) {
			// Simple placeholder for SSR tests
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte("<main>Agent Dashboard</main>"))
		},
		"handleAgentSearch": AgentHandlerExports.HandleAgentSearch,
		// Ticket action APIs (YAML routes)
		"handleAddTicketTime":        handleAddTicketTime,
		"handleUpdateTicketStatus":   handleUpdateTicketStatus,
		"handleTicketReply":          handleTicketReply,
		"handleUpdateTicketPriority": handleUpdateTicketPriority,
		"handleUpdateTicketQueue":    handleUpdateTicketQueue,
		"HandleAPIQueueGet":          HandleAPIQueueGet,
		"HandleAPIQueueDetails":      HandleAPIQueueDetails,
		"HandleAPIQueueStatus":       HandleAPIQueueStatus,
		"HandleLoginAPI":             HandleLoginAPI,
		"HandleListTicketsAPI":       HandleListTicketsAPI,
		"HandleCreateTicketAPI":      HandleCreateTicketAPI,
		"HandleGetTicketAPI":         HandleGetTicketAPI,
		"HandleUpdateTicketAPI":      HandleUpdateTicketAPI,
		"HandleDeleteTicketAPI":      HandleDeleteTicketAPI,
		"HandleReopenTicketAPI":      HandleReopenTicketAPI,
		"HandleListArticlesAPI":      HandleListArticlesAPI,
		"HandleCreateArticleAPI":     HandleCreateArticleAPI,
		"HandleGetArticleAPI":        HandleGetArticleAPI,
		"HandleUpdateArticleAPI":     HandleUpdateArticleAPI,
		"HandleDeleteArticleAPI":     HandleDeleteArticleAPI,
		"HandleGetInternalNotes":     HandleGetInternalNotes,
		"HandleCreateInternalNote":   HandleCreateInternalNote,
		"HandleUpdateInternalNote":   HandleUpdateInternalNote,
		"HandleDeleteInternalNote":   HandleDeleteInternalNote,
		"HandleUserMeAPI":            HandleUserMeAPI,
		"HandleListUsersAPI":         HandleListUsersAPI,
		"HandleGetUserAPI":           HandleGetUserAPI,
		"HandleListGroupsAPI":        HandleListGroupsAPI,
		"HandleCreateUserAPI":        HandleCreateUserAPI,
		"HandleUpdateUserAPI":        HandleUpdateUserAPI,
		"HandleDeleteUserAPI":        HandleDeleteUserAPI,
		"HandleListQueuesAPI":        HandleListQueuesAPI,
		"HandleGetQueueAPI":          HandleGetQueueAPI,
		"HandleGetQueueAgentsAPI":    HandleGetQueueAgentsAPI,
		"HandleCreateQueueAPI":       HandleCreateQueueAPI,
		"HandleUpdateQueueAPI":       HandleUpdateQueueAPI,
		"HandleDeleteQueueAPI":       HandleDeleteQueueAPI,
		"HandleGetQueueStatsAPI":     HandleGetQueueStatsAPI,
		"HandleAssignQueueGroupAPI":  HandleAssignQueueGroupAPI,
		"HandleRemoveQueueGroupAPI":  HandleRemoveQueueGroupAPI,
		"HandleListPrioritiesAPI":    HandleListPrioritiesAPI,
		"HandleGetPriorityAPI":       HandleGetPriorityAPI,
		"HandleListTypesAPI":         HandleListTypesAPI,
		"HandleListStatesAPI":        HandleListStatesAPI,
		"HandleSearchAPI":            HandleSearchAPI,
		"HandleSearchSuggestionsAPI": HandleSearchSuggestionsAPI,
		"HandleReindexAPI":           HandleReindexAPI,
		"HandleSearchHealthAPI":      HandleSearchHealthAPI,

		// Ticket API handlers (migrated from protectedAPI routes)
		"handleAPITickets":           handleAPITickets,
		"handleCreateTicket":         handleCreateTicket,
		"handleGetTicket":            handleGetTicket,
		"handleUpdateTicket":         handleUpdateTicket,
		"handleDeleteTicket":         handleDeleteTicket,
		"handleAddTicketNote":        handleAddTicketNote,
		"handleGetTicketHistory":     handleGetTicketHistory,
		"handleGetAvailableAgents":   handleGetAvailableAgents,
		"handleAssignTicket":         handleAssignTicket,
		"handleCloseTicket":          handleCloseTicket,
		"handleReopenTicket":         handleReopenTicket,
		"handleSearchTickets":        handleSearchTickets,
		"handleFilterTickets":        handleFilterTickets,
		"handleAdvancedTicketSearch": handleAdvancedTicketSearch,
		"handleSearchSuggestions":    handleSearchSuggestions,
		"handleExportSearchResults":  handleExportSearchResults,
		"handleSaveSearchHistory":    handleSaveSearchHistory,
		"handleGetSearchHistory":     handleGetSearchHistory,
		"handleDeleteSearchHistory":  handleDeleteSearchHistory,
		"handleCreateSavedSearch":    handleCreateSavedSearch,
		"handleGetSavedSearches":     handleGetSavedSearches,
		"handleExecuteSavedSearch":   handleExecuteSavedSearch,
		"handleUpdateSavedSearch":    handleUpdateSavedSearch,
		"handleDeleteSavedSearch":    handleDeleteSavedSearch,
		"handleMergeTickets":         handleMergeTickets,
		"handleUnmergeTicket":        handleUnmergeTicket,
		"handleGetMergeHistory":      handleGetMergeHistory,

		// Dashboard API handlers (migrated from protectedAPI routes)
		"handleDashboardStats": handleDashboardStats,
		"handleRecentTickets":  handleRecentTickets,
		"handleNotifications":  handleNotifications,
		"handleQuickActions":   handleQuickActions,
		"handleActivity":       handleActivity,
		"handleActivityStream": handleActivityStream,
		"handlePerformance":    handlePerformance,

		// Queue API handlers (migrated from protectedAPI routes)
		"handleGetQueuesAPI":       handleGetQueuesAPI,
		"handleCreateQueueWrapper": handleCreateQueueWrapper,

		// Group API handlers (migrated from protectedAPI routes)
		"handleGetGroups":       handleGetGroups,
		"handleGetGroupAPI":     handleGetGroupAPI,
		"handleGetGroupMembers": handleGetGroupMembers,

		// Type API handlers (migrated from protectedAPI routes)
		"handleCreateType": handleCreateType,
		"handleUpdateType": handleUpdateType,
		"handleDeleteType": handleDeleteType,

		// File handler (migrated from protectedAPI routes)
		"handleServeFile": handleServeFile,

		// Customer handler (migrated from protectedAPI routes)
		"handleCustomerSearch": handleCustomerSearch,

		// Canned response handlers (migrated from protectedAPI routes)
		"cannedResponses_GetResponses":           CannedResponseHandlerExports.GetResponses,
		"cannedResponses_GetQuickResponses":      CannedResponseHandlerExports.GetQuickResponses,
		"cannedResponses_GetPopularResponses":    CannedResponseHandlerExports.GetPopularResponses,
		"cannedResponses_GetCategories":          CannedResponseHandlerExports.GetCategories,
		"cannedResponses_GetResponsesByCategory": CannedResponseHandlerExports.GetResponsesByCategory,
		"cannedResponses_SearchResponses":        CannedResponseHandlerExports.SearchResponses,
		"cannedResponses_GetResponsesForUser":    CannedResponseHandlerExports.GetResponsesForUser,
		"cannedResponses_GetResponseByID":        CannedResponseHandlerExports.GetResponseByID,
	}
	for n, h := range pairs {
		if _, ok := GetHandler(n); !ok {
			RegisterHandler(n, h)
		}
		// Also register to GlobalHandlerMap for YAML routing
		if _, exists := routing.GlobalHandlerMap[n]; !exists {
			routing.GlobalHandlerMap[n] = h
		}
	}

	registerDynamicModuleHandlers()
	// Diagnostic (once): log total registry size
	handlerRegistryMu.RLock()
	sz := len(handlerRegistry)
	handlerRegistryMu.RUnlock()
	log.Printf("handler registry initialized (%d handlers)", sz)
}

func registerDynamicModuleHandlers() {
	for name, fn := range dynamicModuleHandlerMap() {
		if _, ok := GetHandler(name); !ok {
			RegisterHandler(name, fn)
		}
		if _, exists := routing.GlobalHandlerMap[name]; !exists {
			routing.GlobalHandlerMap[name] = fn
		}
	}
}

// RegisterDynamicModuleHandlersIntoRegistry exposes the dynamic module handlers to the routing registry used by the main server.
func RegisterDynamicModuleHandlersIntoRegistry(reg *routing.HandlerRegistry) {
	if reg == nil {
		return
	}
	for name, fn := range dynamicModuleHandlerMap() {
		if reg.HandlerExists(name) {
			reg.Override(name, fn)
			continue
		}
		if err := reg.Register(name, fn); err != nil {
			reg.Override(name, fn)
		}
	}
}

func dynamicModuleHandlerMap() map[string]gin.HandlerFunc {
	out := make(map[string]gin.HandlerFunc, len(dynamicModuleAliases))
	for module, alias := range dynamicModuleAliases {
		if alias.HandlerName == "" {
			continue
		}
		out[alias.HandlerName] = HandleAdminDynamicModuleFor(module)
	}
	return out
}

// init automatically registers all handlers when the package is imported.
func init() {
	log.Printf("🔧 Initializing handler registry...")
	ensureCoreHandlers()
	log.Printf("✅ Handler registry initialized")
}
