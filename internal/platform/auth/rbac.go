package auth

import platformmodels "github.com/goatkit/goatflow/internal/platform/models"

type Permission string

const (
	// Ticket permissions.
	PermissionTicketCreate Permission = "ticket:create"
	PermissionTicketRead   Permission = "ticket:read"
	PermissionTicketUpdate Permission = "ticket:update"
	PermissionTicketDelete Permission = "ticket:delete"
	PermissionTicketAssign Permission = "ticket:assign"
	PermissionTicketClose  Permission = "ticket:close"

	// User permissions.
	PermissionUserCreate Permission = "user:create"
	PermissionUserRead   Permission = "user:read"
	PermissionUserUpdate Permission = "user:update"
	PermissionUserDelete Permission = "user:delete"

	// Admin permissions.
	PermissionAdminAccess  Permission = "admin:access"
	PermissionSystemConfig Permission = "system:config"

	// Report permissions.
	PermissionReportView   Permission = "report:view"
	PermissionReportCreate Permission = "report:create"

	// Entity deletion permissions.
	PermissionEntityHardDelete Permission = "entity:hard_delete"

	// Customer permissions.
	PermissionOwnTicketRead   Permission = "own:ticket:read"
	PermissionOwnTicketCreate Permission = "own:ticket:create"
)

type RBAC struct {
	rolePermissions map[platformmodels.UserRole][]Permission
}

func NewRBAC() *RBAC {
	rbac := &RBAC{
		rolePermissions: make(map[platformmodels.UserRole][]Permission),
	}
	rbac.initializePermissions()
	return rbac
}

func (r *RBAC) initializePermissions() {
	// Admin has all permissions
	r.rolePermissions[platformmodels.RoleAdmin] = []Permission{
		PermissionTicketCreate, PermissionTicketRead, PermissionTicketUpdate, PermissionTicketDelete,
		PermissionTicketAssign, PermissionTicketClose,
		PermissionUserCreate, PermissionUserRead, PermissionUserUpdate, PermissionUserDelete,
		PermissionAdminAccess, PermissionSystemConfig,
		PermissionReportView, PermissionReportCreate,
		PermissionEntityHardDelete,
		PermissionOwnTicketRead, PermissionOwnTicketCreate,
	}

	// Agent has ticket and limited user permissions
	r.rolePermissions[platformmodels.RoleAgent] = []Permission{
		PermissionTicketCreate, PermissionTicketRead, PermissionTicketUpdate,
		PermissionTicketAssign, PermissionTicketClose,
		PermissionUserRead,
		PermissionReportView,
		PermissionOwnTicketRead, PermissionOwnTicketCreate,
	}

	// Customer can only manage their own tickets
	r.rolePermissions[platformmodels.RoleCustomer] = []Permission{
		PermissionOwnTicketRead, PermissionOwnTicketCreate,
	}
}

func (r *RBAC) HasPermission(role string, permission Permission) bool {
	userRole := platformmodels.UserRole(role)
	permissions, exists := r.rolePermissions[userRole]
	if !exists {
		return false
	}

	for _, p := range permissions {
		if p == permission {
			return true
		}
	}

	return false
}

func (r *RBAC) GetRolePermissions(role string) []Permission {
	userRole := platformmodels.UserRole(role)
	return r.rolePermissions[userRole]
}

func (r *RBAC) CanAccessTicket(role string, ticketOwnerID, userID uint) bool {
	// Admins and Agents can access any ticket
	if role == string(platformmodels.RoleAdmin) || role == string(platformmodels.RoleAgent) {
		return true
	}

	// Customers can only access their own tickets
	if role == string(platformmodels.RoleCustomer) {
		return ticketOwnerID == userID
	}

	return false
}

func (r *RBAC) CanModifyUser(actorRole string, targetUserRole string) bool {
	// Only admins can modify other users
	if actorRole != string(platformmodels.RoleAdmin) {
		return false
	}

	// Admins can modify anyone
	return true
}

func (r *RBAC) CanAssignTicket(role string) bool {
	return r.HasPermission(role, PermissionTicketAssign)
}

func (r *RBAC) CanCloseTicket(role string) bool {
	return r.HasPermission(role, PermissionTicketClose)
}

func (r *RBAC) CanAccessAdminPanel(role string) bool {
	return r.HasPermission(role, PermissionAdminAccess)
}

func (r *RBAC) CanViewReports(role string) bool {
	return r.HasPermission(role, PermissionReportView)
}
