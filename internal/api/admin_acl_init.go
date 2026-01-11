package api

import "github.com/gotrs-io/gotrs-ce/internal/routing"

func init() {
	// Register ACL handlers into the global routing registry
	routing.RegisterHandler("handleAdminACL", handleAdminACL)
	routing.RegisterHandler("handleAdminACLCreate", handleAdminACLCreate)
	routing.RegisterHandler("handleAdminACLUpdate", handleAdminACLUpdate)
	routing.RegisterHandler("handleAdminACLDelete", handleAdminACLDelete)
	routing.RegisterHandler("handleAdminACLGet", handleAdminACLGet)
}
