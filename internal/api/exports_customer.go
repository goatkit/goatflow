package api

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/services/adapter"
)

// wrapAdapterDBHandler wraps a handler factory that requires a database connection via adapter.
func wrapAdapterDBHandler(handlerFactory func(*sql.DB) gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		db, err := adapter.GetDB()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database unavailable"})
			return
		}
		handlerFactory(db)(c)
	}
}

// Customer handler exports that get database from connection pool.
var (
	HandleCustomerDashboard      = wrapAdapterDBHandler(handleCustomerDashboard)
	HandleCustomerTickets        = wrapAdapterDBHandler(handleCustomerTickets)
	HandleCustomerNewTicket      = wrapAdapterDBHandler(handleCustomerNewTicket)
	HandleCustomerCreateTicket   = wrapAdapterDBHandler(handleCustomerCreateTicket)
	HandleCustomerTicketView     = wrapAdapterDBHandler(handleCustomerTicketView)
	HandleCustomerTicketReply    = wrapAdapterDBHandler(handleCustomerTicketReply)
	HandleCustomerCloseTicket    = wrapAdapterDBHandler(handleCustomerCloseTicket)
	HandleCustomerProfile        = wrapAdapterDBHandler(handleCustomerProfile)
	HandleCustomerUpdateProfile  = wrapAdapterDBHandler(handleCustomerUpdateProfile)
	HandleCustomerPasswordForm   = wrapAdapterDBHandler(handleCustomerPasswordForm)
	HandleCustomerChangePassword = wrapAdapterDBHandler(handleCustomerChangePassword)
	HandleCustomerKnowledgeBase  = wrapAdapterDBHandler(handleCustomerKnowledgeBase)
	HandleCustomerKBSearch       = wrapAdapterDBHandler(handleCustomerKBSearch)
	HandleCustomerKBArticle      = wrapAdapterDBHandler(handleCustomerKBArticle)
	HandleCustomerCompanyInfo        = wrapAdapterDBHandler(handleCustomerCompanyInfo)
	HandleCustomerCompanyUsers       = wrapAdapterDBHandler(handleCustomerCompanyUsers)
	HandleCustomerGetLanguage        = wrapAdapterDBHandler(handleCustomerGetLanguage)
	HandleCustomerSetLanguage        = wrapAdapterDBHandler(handleCustomerSetLanguage)
	HandleCustomerGetSessionTimeout  = wrapAdapterDBHandler(handleCustomerGetSessionTimeout)
	HandleCustomerSetSessionTimeout  = wrapAdapterDBHandler(handleCustomerSetSessionTimeout)

	// Customer attachment handlers
	HandleCustomerGetAttachments    = wrapAdapterDBHandler(handleCustomerGetAttachments)
	HandleCustomerUploadAttachment  = wrapAdapterDBHandler(handleCustomerUploadAttachment)
	HandleCustomerDownloadAttachment = wrapAdapterDBHandler(handleCustomerDownloadAttachment)
	HandleCustomerGetThumbnail      = wrapAdapterDBHandler(handleCustomerGetThumbnail)
	HandleCustomerViewAttachment    = wrapAdapterDBHandler(handleCustomerViewAttachment)
)
