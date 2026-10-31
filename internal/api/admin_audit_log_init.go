package api

import "github.com/goatkit/goatflow/internal/platform/routing"

func init() {
	routing.RegisterHandler("handleAdminAuditLog", HandleAdminAuditLog)
	routing.RegisterHandler("handleAdminAuditLogList", HandleAdminAuditLogList)
	routing.RegisterHandler("handleAdminAuditLogExport", HandleAdminAuditLogExport)
	routing.RegisterHandler("handleSaveAdminAuditSettings", HandleSaveAdminAuditSettings)
}
