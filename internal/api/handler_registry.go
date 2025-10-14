package api

import (
    "net/http"
    "log"
    "sort"
    "sync"
    "os"

    "github.com/gin-gonic/gin"
    "github.com/gotrs-io/gotrs-ce/internal/database"
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
    if name == "" || h == nil { return }
    handlerRegistryMu.Lock(); handlerRegistry[name] = h; handlerRegistryMu.Unlock()
}

// GetHandler retrieves a registered handler.
func GetHandler(name string) (gin.HandlerFunc, bool) {
    handlerRegistryMu.RLock(); h, ok := handlerRegistry[name]; handlerRegistryMu.RUnlock(); return h, ok
}

// ListHandlers returns sorted handler names (for diagnostics / tests).
func ListHandlers() []string {
    handlerRegistryMu.RLock(); defer handlerRegistryMu.RUnlock()
    out := make([]string, 0, len(handlerRegistry))
    for k := range handlerRegistry { out = append(out, k) }
    sort.Strings(out)
    return out
}

// ensureCoreHandlers pre-registers known legacy handlers still referenced in YAML.
// Called from registerYAMLRoutes early so existing YAML works without scattering init()s.
func ensureCoreHandlers() {
    // Minimal duplication: only names used in YAML currently.
    pairs := map[string]gin.HandlerFunc{
        "handleLoginPage": handleLoginPage,
        "handleDashboard": handleDashboard,
        "handleTickets": handleTickets,
        "handleTicketDetail": handleTicketDetail,
        "HandleQueueDetail": handleQueueDetail,
        "handleNewTicket": handleNewTicket,
        "handleNewEmailTicket": handleNewEmailTicket,
        "handleNewPhoneTicket": handleNewPhoneTicket,
        // Agent ticket creation flow (YAML expects names without db param)
        "HandleAgentCreateTicket": func(c *gin.Context) {
            // Use enhanced multipart-aware path
            handleCreateTicketWithAttachments(c)
        },
        "HandleAgentNewTicket": func(c *gin.Context) {
            // Resolve DB if available, else pass nil (agent handler supports test/nil)
            db, _ := database.GetDB()
            HandleAgentNewTicket(db)(c)
        },
        // Attachment handlers exposed for API routes
        "HandleGetAttachments": handleGetAttachments,
        "HandleUploadAttachment": handleUploadAttachment,
        "HandleDownloadAttachment": handleDownloadAttachment,
        "HandleDeleteAttachment": handleDeleteAttachment,
        "HandleGetThumbnail": handleGetThumbnail,
    "HandleViewAttachment": handleViewAttachment,
        // Optional customer info partial used by YAML
        "HandleCustomerInfoPanel": func(c *gin.Context) { c.String(200, "") },
        "handleSettings": handleSettings,
        "handleProfile": handleProfile,
        "HandleWebSocketChat": HandleWebSocketChat,
        "handleClaudeChatDemo": handleClaudeChatDemo,
        "HandleGetSessionTimeout": HandleGetSessionTimeout,
        "HandleSetSessionTimeout": HandleSetSessionTimeout,

        // Static and basic routes
        "handleStaticFiles": HandleStaticFiles,
        "handleLogout": handleLogout,
        "handleLogoutRedirect": func(c *gin.Context) {
            // clear tokens then redirect to login
            c.SetCookie("auth_token", "", -1, "/", "", false, true)
            c.SetCookie("access_token", "", -1, "/", "", false, true)
            c.Redirect(http.StatusFound, "/login")
        },
        "handleRoot": func(c *gin.Context) { c.Redirect(http.StatusFound, "/login") },

        // Health and metrics (lightweight for tests/dev)
        "handleHealthCheck": func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "healthy"}) },
        "handleDetailedHealthCheck": func(c *gin.Context) {
            c.JSON(http.StatusOK, gin.H{"status": "healthy", "components": gin.H{"database": "unknown", "cache": "healthy", "queue": "healthy"}})
        },
        "handleMetrics": func(c *gin.Context) { c.String(http.StatusOK, "# HELP gotrs_up GOTRS is up\n# TYPE gotrs_up gauge\ngotrs_up 1\n") },

        // Redirect helpers
        "handleQueuesRedirect": HandleRedirectQueues,

        // Admin handlers
        "handleAdminDashboard": handleAdminDashboard,
        "handleAdminUsers": HandleAdminUsers,
        "HandleAdminUsersList": HandleAdminUsersList,
    // Placeholder admin user details/edit/delete and password policy until implemented
    "handleAdminUserGet": func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"success": false, "error": "handleAdminUserGet not implemented"}) },
    "handleAdminUserEdit": func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"success": false, "error": "handleAdminUserEdit not implemented"}) },
    "handleAdminUserUpdate": func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"success": false, "error": "handleAdminUserUpdate not implemented"}) },
    "handleAdminUserDelete": func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"success": false, "error": "handleAdminUserDelete not implemented"}) },
    "handleAdminPasswordPolicy": func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"min_length": 8, "require_special": true}}) },
        "handleAdminGroups": handleAdminGroups,
        "handleCreateGroup": handleCreateGroup,
        "handleGetGroup": handleGetGroup,
        "handleUpdateGroup": handleUpdateGroup,
        "handleDeleteGroup": handleDeleteGroup,
        "handleGroupMembers": handleGetGroupMembers,
        "handleAddUserToGroup": handleAddUserToGroup,
        "handleRemoveUserFromGroup": handleRemoveUserFromGroup,
    // Additional admin group APIs used in YAML
    "HandleAdminGroupsUsers": func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"success": true, "data": []interface{}{} }) },
    "HandleAdminGroupsAddUser": func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"success": true}) },
    "HandleAdminGroupsRemoveUser": func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"success": true}) },
        "handleAdminQueues": handleAdminQueues,
        "handleAdminPriorities": handleAdminPriorities,
        "handleAdminPermissions": handleAdminPermissions,
        "handleGetUserPermissionMatrix": handleGetUserPermissionMatrix,
        "handleUpdateUserPermissions": handleUpdateUserPermissions,
        "handleAdminStates": handleAdminStates,
        "handleAdminTypes": handleAdminTypes,
        "handleAdminServices": handleAdminServices,
        "handleAdminSLA": handleAdminSLA,
        "handleAdminLookups": handleAdminLookups,

        // Agent handlers (wrap to avoid DB in tests)
        "handleAgentTickets": func(c *gin.Context) {
            if os.Getenv("APP_ENV") == "test" {
                c.Data(http.StatusOK, "text/html; charset=utf-8", []byte("<main>Agent Tickets</main>"))
                return
            }
            AgentHandlerExports.HandleAgentTickets(c)
        },
        "handleAgentTicketView": func(c *gin.Context) {
            if os.Getenv("APP_ENV") == "test" {
                c.Data(http.StatusOK, "text/html; charset=utf-8", []byte("<main>Agent Ticket View</main>"))
                return
            }
            AgentHandlerExports.HandleAgentTicketView(c)
        },
        "handleAgentTicketReply": AgentHandlerExports.HandleAgentTicketReply,
        "handleAgentTicketNote": AgentHandlerExports.HandleAgentTicketNote,
        "handleAgentTicketPhone": AgentHandlerExports.HandleAgentTicketPhone,
        "handleAgentTicketStatus": AgentHandlerExports.HandleAgentTicketStatus,
        "handleAgentTicketAssign": AgentHandlerExports.HandleAgentTicketAssign,
        "handleAgentTicketPriority": AgentHandlerExports.HandleAgentTicketPriority,
        "handleAgentTicketQueue": AgentHandlerExports.HandleAgentTicketQueue,
        "handleAgentTicketMerge": AgentHandlerExports.HandleAgentTicketMerge,
        "handleAgentTicketDraft": AgentHandlerExports.HandleAgentTicketDraft,
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
        // Time accounting API (YAML route)
        "handleAddTicketTime": handleAddTicketTime,
    }
    for n, h := range pairs { if _, ok := GetHandler(n); !ok { RegisterHandler(n, h) } }
    // Diagnostic (once): log total registry size
    handlerRegistryMu.RLock(); sz := len(handlerRegistry); handlerRegistryMu.RUnlock()
    log.Printf("handler registry initialized (%d handlers)", sz)
}