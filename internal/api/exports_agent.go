package api

import (
	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// Agent handler exports that get database from connection pool
var (
	HandleAgentDashboard = func(c *gin.Context) {
		db, err := database.GetDB()
		if err != nil || db == nil {
			c.JSON(500, gin.H{"error": "Database connection failed"})
			return
		}
		handleAgentDashboard(db)(c)
	}

	HandleAgentTickets = func(c *gin.Context) {
		db, err := database.GetDB()
		if err != nil || db == nil {
			c.JSON(500, gin.H{"error": "Database connection failed"})
			return
		}
		handleAgentTickets(db)(c)
	}

	HandleAgentTicketView = func(c *gin.Context) {
        db, _ := database.GetDB()
		handleAgentTicketView(db)(c)
	}

	HandleAgentTicketReply = func(c *gin.Context) {
        db, _ := database.GetDB()
		handleAgentTicketReply(db)(c)
	}

	HandleAgentTicketNote = func(c *gin.Context) {
        db, _ := database.GetDB()
		handleAgentTicketNote(db)(c)
	}

	HandleAgentTicketPhone = func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentTicketPhone(db)(c)
	}

	HandleAgentTicketStatus = func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentTicketStatus(db)(c)
	}

	HandleAgentTicketAssign = func(c *gin.Context) {
		db, err := database.GetDB()
		if err != nil || db == nil {
			c.JSON(500, gin.H{"error": "Database connection failed"})
			return
		}
		handleAgentTicketAssign(db)(c)
	}

	HandleAgentTicketPriority = func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentTicketPriority(db)(c)
	}

	// NEWLY ADDED: Missing handlers that were causing 404 errors
	HandleAgentTicketQueue = func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentTicketQueue(db)(c)
	}

	HandleAgentTicketMerge = func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentTicketMerge(db)(c)
	}

	HandleTicketCustomerUsers = func(c *gin.Context) {
		db, _ := database.GetDB()
		handleTicketCustomerUsers(db)(c)
	}

	HandleAgentTicketDraft = func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentTicketDraft(db)(c)
	}

	HandleArticleAttachmentDownload = func(c *gin.Context) {
		db, _ := database.GetDB()
		handleArticleAttachmentDownload(db)(c)
	}

	// Additional agent handlers for full functionality
	// Note: Some handlers may not exist yet and will need to be implemented

	HandleAgentQueues = func(c *gin.Context) {
		db, err := database.GetDB()
		if err != nil || db == nil {
			c.JSON(500, gin.H{"error": "Database connection failed"})
			return
		}
		handleAgentQueues(db)(c)
	}

	HandleAgentQueueView = func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentQueueView(db)(c)
	}

	HandleAgentQueueLock = func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentQueueLock(db)(c)
	}

	HandleAgentQueueUnlock = func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentQueueUnlock(db)(c)
	}

	HandleAgentCustomers = func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentCustomers(db)(c)
	}

	HandleAgentCustomerView = func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentCustomerView(db)(c)
	}

	HandleAgentCustomerTickets = func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentCustomerTickets(db)(c)
	}

	HandleAgentSearch = func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentSearch(db)(c)
	}

	HandleAgentSearchResults = func(c *gin.Context) {
		db, _ := database.GetDB()
		handleAgentSearchResults(db)(c)
	}
)
