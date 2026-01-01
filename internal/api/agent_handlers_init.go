package api

import (
	"github.com/gin-gonic/gin"
)

// AgentHandlerRegistry holds references to agent handlers for external registration.
type AgentHandlerRegistry struct {
	NewTicket    gin.HandlerFunc
	CreateTicket gin.HandlerFunc
}

// GlobalAgentHandlers is the global registry for agent handlers.
var GlobalAgentHandlers = &AgentHandlerRegistry{}

// RegisterAgentHandlersForRouting registers agent handlers for YAML routing.
func RegisterAgentHandlersForRouting() {
	// Populate the global registry with our handlers
	GlobalAgentHandlers.NewTicket = AgentHandlerExports.HandleAgentNewTicket
	GlobalAgentHandlers.CreateTicket = AgentHandlerExports.HandleAgentCreateTicket
}
