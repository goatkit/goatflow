package api

import (
	"github.com/gotrs-io/gotrs-ce/internal/routing"
)

// RegisterRoutingHandlers registers all handlers with the routing system
func RegisterRoutingHandlers() {
	// Register profile handlers
	routing.RegisterHandler("handleProfile", HandleProfile)
	routing.RegisterHandler("profile", HandleProfile) // YAML uses this name
	routing.RegisterHandler("HandleGetSessionTimeout", HandleGetSessionTimeout)
	routing.RegisterHandler("get_session_timeout", HandleGetSessionTimeout)
	routing.RegisterHandler("HandleSetSessionTimeout", HandleSetSessionTimeout)
	routing.RegisterHandler("set_session_timeout", HandleSetSessionTimeout)
	
	// Register redirect handlers
	routing.RegisterHandler("handleRedirectProfile", HandleRedirectProfile)
	routing.RegisterHandler("redirect_profile", HandleRedirectProfile)
	routing.RegisterHandler("handleRedirectTickets", HandleRedirectTickets)
	routing.RegisterHandler("redirect_tickets", HandleRedirectTickets)
	routing.RegisterHandler("handleRedirectTicketsNew", HandleRedirectTicketsNew)
	routing.RegisterHandler("redirect_tickets_new", HandleRedirectTicketsNew)
	routing.RegisterHandler("handleRedirectQueues", HandleRedirectQueues)
	routing.RegisterHandler("redirect_queues", HandleRedirectQueues)
	routing.RegisterHandler("handleRedirectSettings", HandleRedirectSettings)
	routing.RegisterHandler("redirect_settings", HandleRedirectSettings)
}

// init function runs automatically when the package is imported
func init() {
	RegisterRoutingHandlers()
}