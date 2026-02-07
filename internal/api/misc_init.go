package api

import (
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/routing"
)

func init() {
	// Register misc handlers that are exported in exports.go
	// These handlers are referenced in YAML routes
	routing.RegisterHandler("HandleLegacyAgentTicketViewRedirect", HandleLegacyAgentTicketViewRedirect)
	routing.RegisterHandler("HandleLegacyTicketsViewRedirect", HandleLegacyTicketsViewRedirect)
	routing.RegisterHandler("handleUpdateTicketStatus", handleUpdateTicketStatus)
	routing.RegisterHandler("handleTicketHistoryFragment", HandleTicketHistoryFragment)
	routing.RegisterHandler("handleTicketLinksFragment", HandleTicketLinksFragment)

	// Handlers that need DB wrapper
	routing.RegisterHandler("handleTicketCustomerUsers", HandleTicketCustomerUsersWrapper)
}

// HandleTicketCustomerUsersWrapper wraps handleTicketCustomerUsers with DB lookup.
var HandleTicketCustomerUsersWrapper gin.HandlerFunc = func(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(500, gin.H{"error": "Database unavailable"})
		return
	}
	handleTicketCustomerUsers(db)(c)
}
