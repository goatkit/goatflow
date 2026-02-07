package api

import "github.com/goatkit/goatflow/internal/routing"

func init() {
	// Register bulk ticket action handlers for YAML routing
	routing.GlobalHandlerMap["handleBulkTicketStatus"] = AgentHandlerExports.HandleBulkTicketStatus
	routing.GlobalHandlerMap["handleBulkTicketPriority"] = AgentHandlerExports.HandleBulkTicketPriority
	routing.GlobalHandlerMap["handleBulkTicketQueue"] = AgentHandlerExports.HandleBulkTicketQueue
	routing.GlobalHandlerMap["handleBulkTicketAssign"] = AgentHandlerExports.HandleBulkTicketAssign
	routing.GlobalHandlerMap["handleBulkTicketLock"] = AgentHandlerExports.HandleBulkTicketLock
	routing.GlobalHandlerMap["handleBulkTicketMerge"] = AgentHandlerExports.HandleBulkTicketMerge
	routing.GlobalHandlerMap["handleGetBulkActionOptions"] = AgentHandlerExports.HandleGetBulkActionOptions
	routing.GlobalHandlerMap["handleGetFilteredTicketIds"] = AgentHandlerExports.HandleGetFilteredTicketIds
}
