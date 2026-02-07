package api

import "github.com/goatkit/goatflow/internal/routing"

func init() {
	// Register SLA handlers into the global routing registry
	routing.RegisterHandler("handleAdminSLA", HandleAdminSLA)
	routing.RegisterHandler("handleAdminSLACreate", HandleAdminSLACreate)
	routing.RegisterHandler("handleAdminSLAUpdate", HandleAdminSLAUpdate)
	routing.RegisterHandler("handleAdminSLADelete", HandleAdminSLADelete)
}
