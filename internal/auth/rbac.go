package auth

import "github.com/goatkit/goatflow/internal/models"

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

	// Customer permissions.
	PermissionOwnTicketRead   Permission = "own:ticket:read"
	PermissionOwnTicketCreate Permission = "own:ticket:create"
)

type RBAC struct {
	rolePermissions map[models.UserRole][]Permission
}

func NewRBAC() *RBAC {
	rbac := &RBAC{
		rolePermissions: make(map[models.UserRole][]Permission),
	}
	rbac.initializePermissions()
	return rbac
}

func (r *RBAC) initializePermissions() {
	// Admin has all permissions
	r.rolePermissions[models.RoleAdmin] = []Permission{
		PermissionTicketCreate, PermissionTicketRead, PermissionTicketUpdate, PermissionTicketDelete,
		PermissionTicketAssign, PermissionTicketClose,
		PermissionUserCreate, PermissionUserRead, PermissionUserUpdate, PermissionUserDelete,
		PermissionAdminAccess, PermissionSystemConfig,
		PermissionReportView, PermissionReportCreate,
		PermissionOwnTicketRead, PermissionOwnTicketCreate,
	}

	// Agent has ticket and limited user permissions
	r.rolePermissions[models.RoleAgent] = []Permission{
		PermissionTicketCreate, PermissionTicketRead, PermissionTicketUpdate,
		PermissionTicketAssign, PermissionTicketClose,
		PermissionUserRead,
		PermissionReportView,
		PermissionOwnTicketRead, PermissionOwnTicketCreate,
	}

	// Customer can only manage their own tickets
	r.rolePermissions[models.RoleCustomer] = []Permission{
		PermissionOwnTicketRead, PermissionOwnTicketCreate,
	}
}

func (r *RBAC) HasPermission(role string, permission Permission) bool {
	userRole := models.UserRole(role)
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
	userRole := models.UserRole(role)
	return r.rolePermissions[userRole]
}

func (r *RBAC) CanAccessTicket(role string, ticketOwnerID, userID uint) bool {
	// Admins and Agents can access any ticket
	if role == string(models.RoleAdmin) || role == string(models.RoleAgent) {
		return true
	}

	// Customers can only access their own tickets
	if role == string(models.RoleCustomer) {
		return ticketOwnerID == userID
	}

	return false
}

func (r *RBAC) CanModifyUser(actorRole string, targetUserRole string) bool {
	// Only admins can modify other users
	if actorRole != string(models.RoleAdmin) {
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
