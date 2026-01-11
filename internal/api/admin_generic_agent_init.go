package api

import "github.com/gotrs-io/gotrs-ce/internal/routing"

func init() {
	// Register Generic Agent handlers into the global routing registry
	routing.RegisterHandler("handleAdminGenericAgent", handleAdminGenericAgent)
	routing.RegisterHandler("handleAdminGenericAgentCreate", handleAdminGenericAgentCreate)
	routing.RegisterHandler("handleAdminGenericAgentUpdate", handleAdminGenericAgentUpdate)
	routing.RegisterHandler("handleAdminGenericAgentDelete", handleAdminGenericAgentDelete)
	routing.RegisterHandler("handleAdminGenericAgentGet", handleAdminGenericAgentGet)
}
