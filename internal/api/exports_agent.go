package api

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
)

// wrapDBHandler wraps a handler factory that requires a database connection.
func wrapDBHandler(handlerFactory func(*sql.DB) gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		db, err := database.GetDB()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database unavailable"})
			return
		}
		handlerFactory(db)(c)
	}
}

// AgentHandlerExports provides exported handler functions for agent routes.
var AgentHandlerExports = struct {
	HandleAgentTickets         gin.HandlerFunc
	HandleAgentTicketReply     gin.HandlerFunc
	HandleAgentTicketNote      gin.HandlerFunc
	HandleAgentTicketPhone     gin.HandlerFunc
	HandleAgentTicketStatus    gin.HandlerFunc
	HandleAgentTicketAssign    gin.HandlerFunc
	HandleAgentTicketPriority  gin.HandlerFunc
	HandleAgentTicketQueue     gin.HandlerFunc
	HandleAgentTicketMerge     gin.HandlerFunc
	HandleAgentTicketDraft     gin.HandlerFunc
	HandleAgentCustomerTickets gin.HandlerFunc
	HandleAgentCustomerView    gin.HandlerFunc
	HandleAgentSearch          gin.HandlerFunc
	HandleAgentSearchResults   gin.HandlerFunc
	HandleAgentNewTicket       gin.HandlerFunc
	HandleAgentCreateTicket    gin.HandlerFunc
	HandleAgentQueues          gin.HandlerFunc
	HandleAgentQueueView       gin.HandlerFunc
	HandleAgentQueueLock       gin.HandlerFunc
	HandleAgentCustomers       gin.HandlerFunc
	// Bulk ticket actions
	HandleBulkTicketStatus      gin.HandlerFunc
	HandleBulkTicketPriority    gin.HandlerFunc
	HandleBulkTicketQueue       gin.HandlerFunc
	HandleBulkTicketAssign      gin.HandlerFunc
	HandleBulkTicketLock        gin.HandlerFunc
	HandleBulkTicketMerge       gin.HandlerFunc
	HandleGetBulkActionOptions  gin.HandlerFunc
	HandleGetFilteredTicketIds  gin.HandlerFunc
}{
	HandleAgentTickets:         wrapDBHandler(handleAgentTickets),
	HandleAgentTicketReply:     wrapDBHandler(handleAgentTicketReply),
	HandleAgentTicketNote:      wrapDBHandler(handleAgentTicketNote),
	HandleAgentTicketPhone:     wrapDBHandler(handleAgentTicketPhone),
	HandleAgentTicketStatus:    wrapDBHandler(handleAgentTicketStatus),
	HandleAgentTicketAssign:    wrapDBHandler(handleAgentTicketAssign),
	HandleAgentTicketPriority:  wrapDBHandler(handleAgentTicketPriority),
	HandleAgentTicketQueue:     wrapDBHandler(handleAgentTicketQueue),
	HandleAgentTicketMerge:     wrapDBHandler(handleAgentTicketMerge),
	HandleAgentTicketDraft:     wrapDBHandler(handleAgentTicketDraft),
	HandleAgentCustomerTickets: wrapDBHandler(handleAgentCustomerTickets),
	HandleAgentCustomerView:    wrapDBHandler(handleAgentCustomerView),
	HandleAgentSearch:          wrapDBHandler(handleAgentSearch),
	HandleAgentSearchResults:   wrapDBHandler(handleAgentSearchResults),
	HandleAgentNewTicket:       wrapDBHandler(HandleAgentNewTicket),
	HandleAgentCreateTicket:    wrapDBHandler(HandleAgentCreateTicket),
	HandleAgentQueues:          wrapDBHandler(handleAgentQueues),
	HandleAgentQueueView:       wrapDBHandler(handleAgentQueueView),
	HandleAgentQueueLock:       wrapDBHandler(handleAgentQueueLock),
	HandleAgentCustomers:       wrapDBHandler(handleAgentCustomers),
	// Bulk ticket actions
	HandleBulkTicketStatus:      wrapDBHandler(handleBulkTicketStatus),
	HandleBulkTicketPriority:    wrapDBHandler(handleBulkTicketPriority),
	HandleBulkTicketQueue:       wrapDBHandler(handleBulkTicketQueue),
	HandleBulkTicketAssign:      wrapDBHandler(handleBulkTicketAssign),
	HandleBulkTicketLock:        wrapDBHandler(handleBulkTicketLock),
	HandleBulkTicketMerge:       wrapDBHandler(handleBulkTicketMerge),
	HandleGetBulkActionOptions:  wrapDBHandler(handleGetBulkActionOptions),
	HandleGetFilteredTicketIds:  wrapDBHandler(handleGetFilteredTicketIds),
}

// Provide package-level handler variables for tests and direct routing.
var (
	HandleAgentTickets        = AgentHandlerExports.HandleAgentTickets
	HandleAgentTicketReply    = AgentHandlerExports.HandleAgentTicketReply
	HandleAgentTicketNote     = AgentHandlerExports.HandleAgentTicketNote
	HandleAgentTicketPhone    = AgentHandlerExports.HandleAgentTicketPhone
	HandleAgentTicketStatus   = AgentHandlerExports.HandleAgentTicketStatus
	HandleAgentTicketAssign   = AgentHandlerExports.HandleAgentTicketAssign
	HandleAgentTicketPriority = AgentHandlerExports.HandleAgentTicketPriority
	HandleAgentTicketQueue    = AgentHandlerExports.HandleAgentTicketQueue
	HandleAgentTicketMerge    = AgentHandlerExports.HandleAgentTicketMerge
	HandleAgentTicketDraft    = AgentHandlerExports.HandleAgentTicketDraft
)

// RegisterAgentHandlers registers agent handlers for YAML routing.
func RegisterAgentHandlers() {
	// RegisterAgentHandlers registers agent handlers for YAML routing
	// TODO: Register handlers in GlobalHandlerMap for YAML routing
	// This is called from the routing package to avoid circular imports
	//
	//	if routing.GlobalHandlerMap != nil {
	//		routing.GlobalHandlerMap["handleAgentNewTicket"] = AgentHandlerExports.HandleAgentNewTicket
	//		routing.GlobalHandlerMap["handleAgentCreateTicket"] = AgentHandlerExports.HandleAgentCreateTicket
	//	}
}
