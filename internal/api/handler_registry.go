package api

import (
    "log"
    "sort"
    "sync"

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
    }
    for n, h := range pairs { if _, ok := GetHandler(n); !ok { RegisterHandler(n, h) } }
    // Diagnostic (once): log total registry size
    handlerRegistryMu.RLock(); sz := len(handlerRegistry); handlerRegistryMu.RUnlock()
    log.Printf("handler registry initialized (%d handlers)", sz)
}