package api

import "github.com/gotrs-io/gotrs-ce/internal/routing"

func init() {
	// Register notification event handlers into the global routing registry
	routing.RegisterHandler("handleAdminNotificationEvents", HandleAdminNotificationEvents)
	routing.RegisterHandler("handleAdminNotificationEventNew", HandleAdminNotificationEventNew)
	routing.RegisterHandler("handleAdminNotificationEventEdit", HandleAdminNotificationEventEdit)
	routing.RegisterHandler("handleAdminNotificationEventGet", HandleAdminNotificationEventGet)
	routing.RegisterHandler("handleCreateNotificationEvent", HandleCreateNotificationEvent)
	routing.RegisterHandler("handleUpdateNotificationEvent", HandleUpdateNotificationEvent)
	routing.RegisterHandler("handleDeleteNotificationEvent", HandleDeleteNotificationEvent)
}
