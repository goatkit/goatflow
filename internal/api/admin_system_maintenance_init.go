package api

import "github.com/goatkit/goatflow/internal/routing"

func init() {
	// Register System Maintenance handlers into the global routing registry
	routing.RegisterHandler("handleAdminSystemMaintenance", handleAdminSystemMaintenance)
	routing.RegisterHandler("handleAdminSystemMaintenanceNew", handleAdminSystemMaintenanceNew)
	routing.RegisterHandler("handleAdminSystemMaintenanceEdit", handleAdminSystemMaintenanceEdit)
	routing.RegisterHandler("handleCreateSystemMaintenance", handleCreateSystemMaintenance)
	routing.RegisterHandler("handleUpdateSystemMaintenance", handleUpdateSystemMaintenance)
	routing.RegisterHandler("handleDeleteSystemMaintenance", handleDeleteSystemMaintenance)
	routing.RegisterHandler("handleGetSystemMaintenance", handleGetSystemMaintenance)
}
