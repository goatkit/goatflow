package api

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/audit"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/sysconfig"
)

// auditReadAccessEnabled reports the cached Audit::Settings record_read_access flag.
var auditReadAccessEnabled = func() bool {
	db, err := database.GetDB()
	if err != nil || db == nil {
		return false
	}
	return sysconfig.AuditReadAccessEnabled(db)
}

// recordAuditRequest stores an event for the request; a variable for tests.
var recordAuditRequest = audit.RecordRequest

// recordReadAccess records an access-category event when read-access
// recording is switched on.
func recordReadAccess(c *gin.Context, action, targetType, targetID, label string, details map[string]any) {
	if !auditReadAccessEnabled() {
		return
	}
	e := audit.FromRequest(c, audit.CategoryAccess, action, audit.OutcomeSuccess)
	e.TargetType, e.TargetID, e.TargetLabel = targetType, targetID, label
	for k, v := range details {
		if e.Details == nil {
			e.Details = make(map[string]any, len(details))
		}
		e.Details[k] = v
	}
	recordAuditRequest(c, e)
}

// recordTicketView records access.ticket.view for a ticket.
func recordTicketView(c *gin.Context, ticketID int64, ticketNumber string) {
	recordReadAccess(c, "access.ticket.view", "ticket", audit.ID(ticketID), ticketNumber, nil)
}

// recordAttachmentDownload records access.attachment.download.
func recordAttachmentDownload(c *gin.Context, ticketID int64, articleID int64, fileID int64, filename string) {
	recordReadAccess(c, "access.attachment.download", "attachment", strconv.FormatInt(fileID, 10), filename,
		map[string]any{"ticket_id": ticketID, "article_id": articleID, "filename": filename})
}
