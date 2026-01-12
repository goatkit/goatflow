package api

import "github.com/gotrs-io/gotrs-ce/internal/routing"

func init() {
	routing.RegisterHandler("handleAdminRoles", HandleAdminRoles)
	routing.RegisterHandler("handleAdminRoleCreate", HandleAdminRoleCreate)
	routing.RegisterHandler("handleAdminRoleGet", HandleAdminRoleGet)
	routing.RegisterHandler("handleAdminRoleUpdate", HandleAdminRoleUpdate)
	routing.RegisterHandler("handleAdminRoleDelete", HandleAdminRoleDelete)
	routing.RegisterHandler("handleAdminRoleUsers", HandleAdminRoleUsers)
	routing.RegisterHandler("handleAdminRoleUsersSearch", HandleAdminRoleUsersSearch)
	routing.RegisterHandler("handleAdminRoleUserAdd", HandleAdminRoleUserAdd)
	routing.RegisterHandler("handleAdminRoleUserRemove", HandleAdminRoleUserRemove)
	routing.RegisterHandler("handleAdminRolePermissions", HandleAdminRolePermissions)
	routing.RegisterHandler("handleAdminRolePermissionsUpdate", HandleAdminRolePermissionsUpdate)
}
