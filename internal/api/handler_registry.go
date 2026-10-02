package api

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"sort"
	"sync"

	"github.com/gin-gonic/gin"

	_ "github.com/goatkit/goatflow/internal/platform/api"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/httpcookie"
	"github.com/goatkit/goatflow/internal/platform/routing"
	"github.com/goatkit/goatflow/internal/platform/shared"
	_ "github.com/goatkit/goatflow/internal/selfservice" // registers the self-service route handlers
)

// Simple global handler registry to decouple YAML route loader from hardcoded map.
// Handlers register themselves (typically in init or during setup) using a stable name.
// Naming convention: existing function name unless alias needed.

var (
	handlerRegistryMu sync.RWMutex
	handlerRegistry   = map[string]gin.HandlerFunc{}
)

// RegisterHandler adds/overwrites a handler under a given name.
// Registers to both the local handlerRegistry AND routing.GlobalHandlerMap
// so that YAML route loader can find all handlers regardless of where they're registered.
func RegisterHandler(name string, h gin.HandlerFunc) {
	if name == "" || h == nil {
		return
	}
	handlerRegistryMu.Lock()
	handlerRegistry[name] = h
	handlerRegistryMu.Unlock()

	// Also register to GlobalHandlerMap for YAML route loading
	routing.GlobalHandlerMap[name] = h
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

// RoutingHandlerResolver exposes API-registered handlers through routing's resolver interface.
type RoutingHandlerResolver struct {
	registry *routing.HandlerRegistry
}

var _ routing.HandlerResolver = RoutingHandlerResolver{}

// NewRoutingHandlerResolver initializes core API handlers and returns a routing resolver.
func NewRoutingHandlerResolver() RoutingHandlerResolver {
	ensureCoreHandlers()

	registry := routing.NewHandlerRegistry()
	for name, handler := range registeredHandlers() {
		registry.Override(name, handler)
	}
	routing.RegisterExistingHandlers(registry)

	return RoutingHandlerResolver{registry: registry}
}

// Registry returns the populated routing registry for callers that need registry-specific APIs.
func (r RoutingHandlerResolver) Registry() *routing.HandlerRegistry {
	return r.registry
}

// Get resolves a registered API handler by route handler name.
func (r RoutingHandlerResolver) Get(name string) (gin.HandlerFunc, error) {
	if r.registry != nil {
		return r.registry.Get(name)
	}
	if h, ok := GetHandler(name); ok {
		return h, nil
	}
	if h, ok := routing.GlobalHandlerMap[name]; ok {
		return h, nil
	}
	return nil, fmt.Errorf("handler %s not found", name)
}

// GetMiddleware resolves registered routing middleware by name.
func (r RoutingHandlerResolver) GetMiddleware(name string) (gin.HandlerFunc, error) {
	if r.registry == nil {
		return nil, fmt.Errorf("middleware %s not found", name)
	}
	return r.registry.GetMiddleware(name)
}

// HandlerExists reports whether a route handler name is registered.
func (r RoutingHandlerResolver) HandlerExists(name string) bool {
	if r.registry != nil {
		return r.registry.HandlerExists(name)
	}
	if _, err := r.Get(name); err == nil {
		return true
	}
	return false
}

// IsFeatureEnabled reports whether a route feature flag is enabled.
func (r RoutingHandlerResolver) IsFeatureEnabled(name string) bool {
	if r.registry == nil {
		return false
	}
	return r.registry.IsFeatureEnabled(name)
}

// RegisteredHandlers returns a copy of all API handlers known to the resolver.
func (RoutingHandlerResolver) RegisteredHandlers() map[string]gin.HandlerFunc {
	return registeredHandlers()
}

func registeredHandlers() map[string]gin.HandlerFunc {
	handlerRegistryMu.RLock()
	out := make(map[string]gin.HandlerFunc, len(handlerRegistry)+len(routing.GlobalHandlerMap))
	for name, h := range handlerRegistry {
		out[name] = h
	}
	handlerRegistryMu.RUnlock()

	for name, h := range routing.GlobalHandlerMap {
		out[name] = h
	}
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
		"handle2FAPage":             handle2FAPage,
		"handleOIDCRedirect":        handleOIDCRedirect,
		"handleOIDCCallback":        handleOIDCCallback,
		"handleSAMLRedirect":        handleSAMLRedirect,
		"handleSAMLCallback":        handleSAMLCallback,
		"handleSAMLMetadata":        handleSAMLMetadata,
		"handleAuthLogin":           HandleAuthLogin,
		"handlePasskeyLoginBegin":   handlePasskeyLoginBegin,
		"handlePasskeyLoginFinish":  handlePasskeyLoginFinish,
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
		"handleApiTokensPage":         handleApiTokensPage,
		"handleProfile":               handleProfile,
		"HandleGetSessionTimeout":     HandleGetSessionTimeout,
		"HandleSetSessionTimeout":     HandleSetSessionTimeout,
		"HandleGetLanguage":           HandleGetLanguage,
		"HandleSetLanguage":           HandleSetLanguage,
		"HandleGetAvailableLanguages": HandleGetAvailableLanguages,
		"HandleSetPreLoginLanguage":   HandleSetPreLoginLanguage,
		"HandleGetAvailableThemes":    HandleGetAvailableThemes,
		"HandleSetPreLoginTheme":      HandleSetPreLoginTheme,
		"HandleGetTheme":              HandleGetTheme,
		"HandleSetTheme":              HandleSetTheme,
		"HandleGetRemindersEnabled":   HandleGetRemindersEnabled,
		"HandleSetRemindersEnabled":   HandleSetRemindersEnabled,
		"HandleDismissCoachmark":      HandleDismissCoachmark,
		"HandleSetWallpaper":          HandleSetWallpaper,
		"HandleGetProfile":            HandleGetProfile,
		"HandleUpdateProfile":         HandleUpdateProfile,
		"HandleAgentPasswordForm":     HandleAgentPasswordForm,
		"HandleAgentChangePassword":   HandleAgentChangePassword,
		"handleAdminReports":          handleAdminReports,
		"HandleMailAccountPollStatus": HandleMailAccountPollStatus,

		// Static and basic routes
		"handleStaticFiles":         HandleStaticFiles,
		"handleServiceWorkerConfig": HandleServiceWorkerConfig,
		"handleLogout":              handleLogout,
		"handleCustomerLogout": func(c *gin.Context) {
			// Delete session record from database (check customer-specific session cookie)
			if sessionID, err := c.Cookie("customer_session_id"); err == nil && sessionID != "" {
				if sessionSvc := shared.GetSessionService(); sessionSvc != nil {
					_ = sessionSvc.KillSession(sessionID) // Best effort, don't fail logout
				}
			}
			// Also check legacy session_id cookie for backwards compatibility
			if sessionID, err := c.Cookie("session_id"); err == nil && sessionID != "" {
				if sessionSvc := shared.GetSessionService(); sessionSvc != nil {
					_ = sessionSvc.KillSession(sessionID)
				}
			}
			// Clear customer-specific auth cookies
			httpcookie.SetAuth(c, "customer_auth_token", "", -1)
			httpcookie.SetAuth(c, "customer_access_token", "", -1)
			httpcookie.SetAuth(c, "customer_session_id", "", -1)
			httpcookie.SetAuthState(c, "goatflow_customer_logged_in", "", -1)
			// Also clear legacy cookies for backwards compatibility
			httpcookie.SetAuth(c, "auth_token", "", -1)
			httpcookie.SetAuth(c, "access_token", "", -1)
			httpcookie.SetAuth(c, "token", "", -1)
			httpcookie.SetAuth(c, "session_id", "", -1)
			httpcookie.SetAuthState(c, "goatflow_logged_in", "", -1)
			c.Header("HX-Redirect", "/customer/login")
			c.Redirect(http.StatusSeeOther, "/customer/login")
		},
		"handleCustomerLoginPage": handleCustomerLoginPage,
		"handleCustomerLogin": func(c *gin.Context) {
			handleCustomerLogin(shared.GetJWTManager())(c)
		},
		"handleCustomerPasskeyLoginBegin":  handleCustomerPasskeyLoginBegin,
		"handleCustomerPasskeyLoginFinish": handleCustomerPasskeyLoginFinish,
		"handle2FAVerify": func(c *gin.Context) {
			handle2FAVerify(shared.GetJWTManager())(c)
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
		"handleCustomerGetLanguage": func(c *gin.Context) {
			db, ok := mustGetDB(c)
			if !ok {
				return
			}
			handleCustomerGetLanguage(db)(c)
		},
		"handleCustomerSetLanguage": func(c *gin.Context) {
			db, ok := mustGetDB(c)
			if !ok {
				return
			}
			handleCustomerSetLanguage(db)(c)
		},
		"handleCustomerGetSessionTimeout": func(c *gin.Context) {
			db, ok := mustGetDB(c)
			if !ok {
				return
			}
			handleCustomerGetSessionTimeout(db)(c)
		},
		"handleCustomerSetSessionTimeout": func(c *gin.Context) {
			db, ok := mustGetDB(c)
			if !ok {
				return
			}
			handleCustomerSetSessionTimeout(db)(c)
		},
		// Customer ticket attachment handlers
		"handleCustomerGetAttachments": func(c *gin.Context) {
			db, ok := mustGetDB(c)
			if !ok {
				return
			}
			handleCustomerGetAttachments(db)(c)
		},
		"handleCustomerUploadAttachment": func(c *gin.Context) {
			db, ok := mustGetDB(c)
			if !ok {
				return
			}
			handleCustomerUploadAttachment(db)(c)
		},
		"handleCustomerDownloadAttachment": func(c *gin.Context) {
			db, ok := mustGetDB(c)
			if !ok {
				return
			}
			handleCustomerDownloadAttachment(db)(c)
		},
		"handleCustomerGetThumbnail": func(c *gin.Context) {
			db, ok := mustGetDB(c)
			if !ok {
				return
			}
			handleCustomerGetThumbnail(db)(c)
		},
		"handleCustomerViewAttachment": func(c *gin.Context) {
			db, ok := mustGetDB(c)
			if !ok {
				return
			}
			handleCustomerViewAttachment(db)(c)
		},
		"handleRoot": func(c *gin.Context) {
			c.Redirect(http.StatusFound, RootRedirectTarget())
		},

		// Health and metrics — real probes: /health pings the database with a
		// short timeout so a dead DB reports 503; /health/detailed adds the
		// cache check plus build/uptime info; /metrics serves Prometheus.
		"handleHealthCheck":         HandleHealthCheck,
		"handleDetailedHealthCheck": HandleDetailedHealthCheck,
		"handleMetrics":             HandleMetrics,

		// Redirect helpers
		"handleQueuesRedirect":   HandleRedirectQueues,
		"handleQueueMetaPartial": handleQueueMetaPartial,

		// Admin handlers
		"handleAdminDashboard":         handleAdminDashboard,
		"handleAdminUsers":             HandleAdminUsers,
		"HandleAdminUsersList":         HandleAdminUsersList,
		"handleAdminUserCreate":        HandleAdminUserCreate,
		"handleAdminUserGet":           HandleAdminUserGet,
		"handleAdminUserEdit":          HandleAdminUserEdit,
		"handleAdminUserUpdate":        HandleAdminUserUpdate,
		"handleAdminUserDelete":        HandleAdminUserDelete,
		"handleAdminUserGroups":        HandleAdminUserGroups,
		"handleAdminUsersStatus":       HandleAdminUsersStatus,
		"HandleAdminUserResetPassword": HandleAdminUserResetPassword,
		"handleAdminPasswordPolicy":    HandlePasswordPolicy,
		"handleAdminGroups":            handleAdminGroups,
		"handleCreateGroup":            handleCreateGroup,
		"handleGetGroup":               handleGetGroup,
		"handleUpdateGroup":            handleUpdateGroup,
		"handleDeleteGroup":            handleDeleteGroup,
		"handleGroupMembers":           handleGetGroupMembers,
		"handleAddUserToGroup":         handleAddUserToGroup,
		"handleRemoveUserFromGroup":    handleRemoveUserFromGroup,
		"handleGroupPermissions":       handleGroupPermissions,
		"handleSaveGroupPermissions":   handleSaveGroupPermissions,
		// Additional admin group APIs used in YAML
		"HandleAdminGroupsUsers":        HandleAdminGroupsUsers,
		"HandleAdminGroupsAddUser":      HandleAdminGroupsAddUser,
		"HandleAdminGroupsRemoveUser":   HandleAdminGroupsRemoveUser,
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
		"handleAdminDynamicModule":         handleAdminDynamicModule,
		// Dynamic Fields management handlers
		"handleAdminDynamicFields":                  handleAdminDynamicFields,
		"handleAdminDynamicFieldNew":                handleAdminDynamicFieldNew,
		"handleAdminDynamicFieldEdit":               handleAdminDynamicFieldEdit,
		"handleAdminDynamicFieldScreenConfig":       handleAdminDynamicFieldScreenConfig,
		"handleAdminDynamicFieldExportPage":         handleAdminDynamicFieldExportPage,
		"handleAdminDynamicFieldExportAction":       handleAdminDynamicFieldExportAction,
		"handleAdminDynamicFieldImportPage":         handleAdminDynamicFieldImportPage,
		"handleAdminDynamicFieldImportAction":       handleAdminDynamicFieldImportAction,
		"handleAdminDynamicFieldImportConfirm":      handleAdminDynamicFieldImportConfirm,
		"handleCreateDynamicField":                  handleCreateDynamicField,
		"handleUpdateDynamicField":                  handleUpdateDynamicField,
		"handleDeleteDynamicField":                  handleDeleteDynamicField,
		"handleAdminDynamicFieldScreenConfigSave":   handleAdminDynamicFieldScreenConfigSave,
		"handleAdminDynamicFieldScreenConfigSingle": handleAdminDynamicFieldScreenConfigSingle,
		// Custom Fields (GoatKit PaaS Core) handlers
		"handleAdminCustomFields":        handleAdminCustomFields,
		"handleAdminCustomFieldNew":      handleAdminCustomFieldNew,
		"handleAdminCustomFieldEdit":     handleAdminCustomFieldEdit,
		"handleCreateCustomField":        handleCreateCustomField,
		"handleUpdateCustomField":        handleUpdateCustomField,
		"handleDeleteCustomField":        handleDeleteCustomField,
		"handleAPIListOrgPluginAccess":   handleAPIListOrgPluginAccess,
		"handleAPISetOrgPluginAccess":    handleAPISetOrgPluginAccess,
		"handleAPIDeleteOrgPluginAccess": handleAPIDeleteOrgPluginAccess,
		"handleAPISetCaptivePlugin":      handleAPISetCaptivePlugin,
		// Dynamic Field Webservice AJAX handlers
		"handleDynamicFieldAutocomplete":   handleDynamicFieldAutocomplete,
		"handleDynamicFieldWebserviceTest": handleDynamicFieldWebserviceTest,
		// GenericInterface Webservice management handlers
		"handleAdminWebservices":         handleAdminWebservices,
		"handleAdminWebserviceNew":       handleAdminWebserviceNew,
		"handleAdminWebserviceEdit":      handleAdminWebserviceEdit,
		"handleAdminWebserviceGet":       handleAdminWebserviceGet,
		"handleCreateWebservice":         handleCreateWebservice,
		"handleUpdateWebservice":         handleUpdateWebservice,
		"handleDeleteWebservice":         handleDeleteWebservice,
		"handleTestWebservice":           handleTestWebservice,
		"handleAdminWebserviceHistory":   handleAdminWebserviceHistory,
		"handleRestoreWebserviceHistory": handleRestoreWebserviceHistory,
		"handleAdminStates":              handleAdminStates,
		"handleAdminStateCreate":         handleAdminStateCreate,
		"handleAdminStateUpdate":         handleAdminStateUpdate,
		"handleAdminStateDelete":         handleAdminStateDelete,
		"handleAdminTypes":               handleAdminTypes,
		"handleAdminTypeCreate":          handleAdminTypeCreate,
		"handleAdminTypeUpdate":          handleAdminTypeUpdate,
		"handleAdminTypeDelete":          handleAdminTypeDelete,
		"handleAdminServices":            handleAdminServices,
		"handleAdminServiceCreate":       handleAdminServiceCreate,
		"handleAdminServiceUpdate":       handleAdminServiceUpdate,
		"handleAdminServiceDelete":       handleAdminServiceDelete,
		"handleAdminSLA":                 handleAdminSLA,
		"handleAdminSLACreate":           handleAdminSLACreate,
		"handleAdminSLAUpdate":           handleAdminSLAUpdate,
		"handleAdminSLADelete":           handleAdminSLADelete,
		"handleAdminLookups":             handleAdminLookups,
		"dashboard_recent_tickets":       handleRecentTickets,
		"dashboard_activity_stream":      handleActivityStream,

		// Customer company handlers - full implementations
		"handleAdminCustomerCompanies": HandleAdminCustomerCompanies,
		"handleAdminNewCustomerCompany": func(c *gin.Context) {
			db, err := database.GetDB()
			if err != nil || db == nil {
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
		"handleAdminCustomerGroups":             handleAdminCustomerGroups,
		"handleAdminCustomerGroupEdit":          handleAdminCustomerGroupEdit,
		"handleAdminCustomerGroupUpdate":        handleAdminCustomerGroupUpdate,
		"handleAdminCustomerGroupByGroup":       handleAdminCustomerGroupByGroup,
		"handleAdminCustomerGroupByGroupUpdate": handleAdminCustomerGroupByGroupUpdate,
		"handleGetCustomerGroupPermissions":     handleGetCustomerGroupPermissions,

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

		// Agent handlers
		"handleAgentTickets":        AgentHandlerExports.HandleAgentTickets,
		"handleAgentTicketReply":    AgentHandlerExports.HandleAgentTicketReply,
		"handleAgentTicketNote":     AgentHandlerExports.HandleAgentTicketNote,
		"handleAgentTicketPhone":    AgentHandlerExports.HandleAgentTicketPhone,
		"handleAgentTicketStatus":   AgentHandlerExports.HandleAgentTicketStatus,
		"handleAgentTicketAssign":   AgentHandlerExports.HandleAgentTicketAssign,
		"handleAgentTicketPriority": AgentHandlerExports.HandleAgentTicketPriority,
		"handleAgentTicketQueue":    AgentHandlerExports.HandleAgentTicketQueue,
		"handleAgentTicketMerge":    AgentHandlerExports.HandleAgentTicketMerge,
		"handleAgentQueues":         AgentHandlerExports.HandleAgentQueues,
		// Ticket action APIs (YAML routes)
		"handleAddTicketTime":        handleAddTicketTime,
		"handleUpdateTicketStatus":   handleUpdateTicketStatus,
		"handleTicketReply":          handleTicketReply,
		"handleUpdateTicketPriority": handleUpdateTicketPriority,
		"handleUpdateTicketQueue":    handleUpdateTicketQueue,
		"HandleAPIQueueGet":          HandleAPIQueueGet,
		"HandleAPIQueueDetails":      HandleAPIQueueDetails,
		"HandleAPIQueueStatus":       HandleAPIQueueStatus,
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
		"HandleListGroupsAPI":        HandleListGroupsAPI,
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
		"HandleCreatePriorityAPI":    HandleCreatePriorityAPI,
		"HandleUpdatePriorityAPI":    HandleUpdatePriorityAPI,
		"HandleDeletePriorityAPI":    HandleDeletePriorityAPI,
		"HandleListTypesAPI":         HandleListTypesAPI,
		"HandleListStatesAPI":        HandleListStatesAPI,
		"HandleSearchAPI":            HandleSearchAPI,
		"HandleReindexAPI":           HandleReindexAPI,
		"HandleSearchHealthAPI":      HandleSearchHealthAPI,

		// Statistics API handlers
		"HandleDashboardStatisticsAPI":   HandleDashboardStatisticsAPI,
		"HandleTicketTrendsAPI":          HandleTicketTrendsAPI,
		"HandleAgentPerformanceAPI":      HandleAgentPerformanceAPI,
		"HandleQueueMetricsAPI":          HandleQueueMetricsAPI,
		"HandleTimeBasedAnalyticsAPI":    HandleTimeBasedAnalyticsAPI,
		"HandleCustomerStatisticsAPI":    HandleCustomerStatisticsAPI,
		"HandleExportStatisticsAPI":      HandleExportStatisticsAPI,
		"HandleTicketStateStatisticsAPI": HandleTicketStateStatisticsAPI,

		// Ticket API handlers (migrated from protectedAPI routes)
		"handleCreateTicket":       handleCreateTicket,
		"handleGetTicket":          handleGetTicket,
		"handleDeleteTicket":       handleDeleteTicket,
		"handleAddTicketNote":      handleAddTicketNote,
		"handleGetTicketHistory":   handleGetTicketHistory,
		"handleGetAvailableAgents": handleGetAvailableAgents,
		"handleAssignTicket":       handleAssignTicket,
		"handleCloseTicket":        handleCloseTicket,
		"handleReopenTicket":       handleReopenTicket,
		"handleSearchTickets":      handleSearchTickets,
		"handleFilterTickets":      handleFilterTickets,

		// Dashboard API handlers (migrated from protectedAPI routes)
		"handleRecentTickets":  handleRecentTickets,
		"handleActivityStream": handleActivityStream,

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

		// Customer handler (migrated from protectedAPI routes)
		"handleCustomerSearch": handleCustomerSearch,

		// Canned response handlers (canned_response table)
		"cannedResponses_GetResponses":           handleGetCannedResponses,
		"cannedResponses_CreateResponse":         handleCreateCannedResponse,
		"cannedResponses_GetPopularResponses":    handleGetPopularCannedResponses,
		"cannedResponses_GetCategories":          handleGetCannedResponseCategories,
		"cannedResponses_GetResponsesByCategory": handleGetCannedResponsesByCategory,
		"cannedResponses_SearchResponses":        handleSearchCannedResponses,
		"cannedResponses_GetResponsesForUser":    handleGetCannedResponses,
		"cannedResponses_GetStatistics":          handleGetCannedResponseStatistics,
		"cannedResponses_ExportResponses":        handleExportCannedResponses,
		"cannedResponses_ImportResponses":        handleImportCannedResponses,
		"cannedResponses_GetResponseByID":        handleGetCannedResponse,
		"cannedResponses_UpdateResponse":         handleUpdateCannedResponse,
		"cannedResponses_DeleteResponse":         handleDeleteCannedResponse,
		"cannedResponses_UseResponse":            handleUseCannedResponse,
		"cannedResponses_ShareResponse":          handleShareCannedResponse,
		"cannedResponses_CopyResponse":           handleCopyCannedResponse,
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
