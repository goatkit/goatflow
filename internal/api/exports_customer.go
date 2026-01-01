package api

import (
	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/services/adapter"
)

// Customer handler exports that get database from connection pool.
var (
	HandleCustomerDashboard = func(c *gin.Context) {
		db, _ := adapter.GetDB()
		handleCustomerDashboard(db)(c)
	}

	HandleCustomerTickets = func(c *gin.Context) {
		db, _ := adapter.GetDB()
		handleCustomerTickets(db)(c)
	}

	HandleCustomerNewTicket = func(c *gin.Context) {
		db, _ := adapter.GetDB()
		handleCustomerNewTicket(db)(c)
	}

	HandleCustomerCreateTicket = func(c *gin.Context) {
		db, _ := adapter.GetDB()
		handleCustomerCreateTicket(db)(c)
	}

	HandleCustomerTicketView = func(c *gin.Context) {
		db, _ := adapter.GetDB()
		handleCustomerTicketView(db)(c)
	}

	HandleCustomerTicketReply = func(c *gin.Context) {
		db, _ := adapter.GetDB()
		handleCustomerTicketReply(db)(c)
	}

	HandleCustomerCloseTicket = func(c *gin.Context) {
		db, _ := adapter.GetDB()
		handleCustomerCloseTicket(db)(c)
	}

	HandleCustomerProfile = func(c *gin.Context) {
		db, _ := adapter.GetDB()
		handleCustomerProfile(db)(c)
	}

	HandleCustomerUpdateProfile = func(c *gin.Context) {
		db, _ := adapter.GetDB()
		handleCustomerUpdateProfile(db)(c)
	}

	HandleCustomerPasswordForm = func(c *gin.Context) {
		db, _ := adapter.GetDB()
		handleCustomerPasswordForm(db)(c)
	}

	HandleCustomerChangePassword = func(c *gin.Context) {
		db, _ := adapter.GetDB()
		handleCustomerChangePassword(db)(c)
	}

	HandleCustomerKnowledgeBase = func(c *gin.Context) {
		db, _ := adapter.GetDB()
		handleCustomerKnowledgeBase(db)(c)
	}

	HandleCustomerKBSearch = func(c *gin.Context) {
		db, _ := adapter.GetDB()
		handleCustomerKBSearch(db)(c)
	}

	HandleCustomerKBArticle = func(c *gin.Context) {
		db, _ := adapter.GetDB()
		handleCustomerKBArticle(db)(c)
	}

	HandleCustomerCompanyInfo = func(c *gin.Context) {
		db, _ := adapter.GetDB()
		handleCustomerCompanyInfo(db)(c)
	}

	HandleCustomerCompanyUsers = func(c *gin.Context) {
		db, _ := adapter.GetDB()
		handleCustomerCompanyUsers(db)(c)
	}
)
