package api

import "github.com/goatkit/goatflow/internal/platform/routing"

func init() {
	routing.RegisterHandler("HandleAdminExecuteSQL", HandleAdminExecuteSQL)
	routing.RegisterHandler("HandleDebugTicketNumber", HandleDebugTicketNumber)
	routing.RegisterHandler("HandleDebugConfigSources", HandleDebugConfigSources)
}
