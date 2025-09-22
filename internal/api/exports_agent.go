package api

import (
	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// AgentHandlerExports provides exported handler functions for agent routes
var AgentHandlerExports = struct {
	HandleAgentTickets         gin.HandlerFunc
	HandleAgentTicketView      gin.HandlerFunc
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
}{
	HandleAgentTickets: func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentTickets(db)(c)
	},
	HandleAgentTicketView: func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentTicketView(db)(c)
	},
	HandleAgentTicketReply: func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentTicketReply(db)(c)
	},
	HandleAgentTicketNote: func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentTicketNote(db)(c)
	},
	HandleAgentTicketPhone: func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentTicketPhone(db)(c)
	},
	HandleAgentTicketStatus: func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentTicketStatus(db)(c)
	},
	HandleAgentTicketAssign: func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentTicketAssign(db)(c)
	},
	HandleAgentTicketPriority: func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentTicketPriority(db)(c)
	},
	HandleAgentTicketQueue: func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentTicketQueue(db)(c)
	},
	HandleAgentTicketMerge: func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentTicketMerge(db)(c)
	},
	HandleAgentTicketDraft: func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentTicketDraft(db)(c)
	},
	HandleAgentCustomerTickets: func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentCustomerTickets(db)(c)
	},
	HandleAgentCustomerView: func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentCustomerView(db)(c)
	},
	HandleAgentSearch: func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentSearch(db)(c)
	},
	HandleAgentSearchResults: func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentSearchResults(db)(c)
	},
	HandleAgentNewTicket: func(c *gin.Context) {
		db, _ := database.GetDB()
		HandleAgentNewTicket(db)(c)
	},
	HandleAgentCreateTicket: func(c *gin.Context) {
		db, _ := database.GetDB()
		HandleAgentCreateTicket(db)(c)
	},
	HandleAgentQueues: func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentQueues(db)(c)
	},
	HandleAgentQueueView: func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentQueueView(db)(c)
	},
	HandleAgentQueueLock: func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentQueueLock(db)(c)
	},
	HandleAgentCustomers: func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentCustomers(db)(c)
	},
}

// Provide package-level handler variables for tests and direct routing
var (
	HandleAgentTickets        = AgentHandlerExports.HandleAgentTickets
	HandleAgentTicketView     = AgentHandlerExports.HandleAgentTicketView
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

// RegisterAgentHandlers registers agent handlers for YAML routing
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
