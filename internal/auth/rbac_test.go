package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/goatkit/goatflow/internal/models"
)

func TestRBAC(t *testing.T) {
	rbac := NewRBAC()

	t.Run("Admin has all permissions", func(t *testing.T) {
		role := string(models.RoleAdmin)

		// Check various permissions
		assert.True(t, rbac.HasPermission(role, PermissionTicketCreate))
		assert.True(t, rbac.HasPermission(role, PermissionTicketDelete))
		assert.True(t, rbac.HasPermission(role, PermissionUserCreate))
		assert.True(t, rbac.HasPermission(role, PermissionUserDelete))
		assert.True(t, rbac.HasPermission(role, PermissionAdminAccess))
		assert.True(t, rbac.HasPermission(role, PermissionSystemConfig))
		assert.True(t, rbac.HasPermission(role, PermissionReportCreate))
	})

	t.Run("Agent has limited permissions", func(t *testing.T) {
		role := string(models.RoleAgent)

		// Agent can manage tickets
		assert.True(t, rbac.HasPermission(role, PermissionTicketCreate))
		assert.True(t, rbac.HasPermission(role, PermissionTicketRead))
		assert.True(t, rbac.HasPermission(role, PermissionTicketUpdate))
		assert.True(t, rbac.HasPermission(role, PermissionTicketAssign))
		assert.True(t, rbac.HasPermission(role, PermissionTicketClose))

		// Agent can read users but not create/delete
		assert.True(t, rbac.HasPermission(role, PermissionUserRead))
		assert.False(t, rbac.HasPermission(role, PermissionUserCreate))
		assert.False(t, rbac.HasPermission(role, PermissionUserDelete))

		// Agent cannot access admin functions
		assert.False(t, rbac.HasPermission(role, PermissionAdminAccess))
		assert.False(t, rbac.HasPermission(role, PermissionSystemConfig))

		// Agent can view reports
		assert.True(t, rbac.HasPermission(role, PermissionReportView))
		assert.False(t, rbac.HasPermission(role, PermissionReportCreate))
	})

	t.Run("Customer has minimal permissions", func(t *testing.T) {
		role := string(models.RoleCustomer)

		// Customer can only manage their own tickets
		assert.True(t, rbac.HasPermission(role, PermissionOwnTicketRead))
		assert.True(t, rbac.HasPermission(role, PermissionOwnTicketCreate))

		// Customer cannot manage other tickets
		assert.False(t, rbac.HasPermission(role, PermissionTicketCreate))
		assert.False(t, rbac.HasPermission(role, PermissionTicketRead))
		assert.False(t, rbac.HasPermission(role, PermissionTicketUpdate))
		assert.False(t, rbac.HasPermission(role, PermissionTicketDelete))

		// Customer cannot manage users
		assert.False(t, rbac.HasPermission(role, PermissionUserCreate))
		assert.False(t, rbac.HasPermission(role, PermissionUserRead))

		// Customer cannot access admin or reports
		assert.False(t, rbac.HasPermission(role, PermissionAdminAccess))
		assert.False(t, rbac.HasPermission(role, PermissionReportView))
	})

	t.Run("Invalid role has no permissions", func(t *testing.T) {
		assert.False(t, rbac.HasPermission("InvalidRole", PermissionTicketCreate))
		assert.False(t, rbac.HasPermission("", PermissionTicketRead))
	})

	t.Run("GetRolePermissions returns correct permissions", func(t *testing.T) {
		adminPerms := rbac.GetRolePermissions(string(models.RoleAdmin))
		assert.Greater(t, len(adminPerms), 10) // Admin should have many permissions

		agentPerms := rbac.GetRolePermissions(string(models.RoleAgent))
		assert.Greater(t, len(agentPerms), 5)            // Agent should have several permissions
		assert.Less(t, len(agentPerms), len(adminPerms)) // But less than admin

		customerPerms := rbac.GetRolePermissions(string(models.RoleCustomer))
		assert.Equal(t, 2, len(customerPerms)) // Customer should have exactly 2 permissions
	})

	t.Run("CanAccessTicket checks correctly", func(t *testing.T) {
		// Admin can access any ticket
		assert.True(t, rbac.CanAccessTicket(string(models.RoleAdmin), 100, 200))
		assert.True(t, rbac.CanAccessTicket(string(models.RoleAdmin), 1, 1))

		// Agent can access any ticket
		assert.True(t, rbac.CanAccessTicket(string(models.RoleAgent), 100, 200))
		assert.True(t, rbac.CanAccessTicket(string(models.RoleAgent), 1, 1))

		// Customer can only access their own tickets
		assert.True(t, rbac.CanAccessTicket(string(models.RoleCustomer), 100, 100))
		assert.False(t, rbac.CanAccessTicket(string(models.RoleCustomer), 100, 200))
	})

	t.Run("CanModifyUser checks correctly", func(t *testing.T) {
		// Only admin can modify users
		assert.True(t, rbac.CanModifyUser(string(models.RoleAdmin), string(models.RoleAgent)))
		assert.True(t, rbac.CanModifyUser(string(models.RoleAdmin), string(models.RoleCustomer)))

		// Agent cannot modify users
		assert.False(t, rbac.CanModifyUser(string(models.RoleAgent), string(models.RoleCustomer)))

		// Customer cannot modify users
		assert.False(t, rbac.CanModifyUser(string(models.RoleCustomer), string(models.RoleAgent)))
	})

	t.Run("CanAssignTicket checks correctly", func(t *testing.T) {
		assert.True(t, rbac.CanAssignTicket(string(models.RoleAdmin)))
		assert.True(t, rbac.CanAssignTicket(string(models.RoleAgent)))
		assert.False(t, rbac.CanAssignTicket(string(models.RoleCustomer)))
	})

	t.Run("CanCloseTicket checks correctly", func(t *testing.T) {
		assert.True(t, rbac.CanCloseTicket(string(models.RoleAdmin)))
		assert.True(t, rbac.CanCloseTicket(string(models.RoleAgent)))
		assert.False(t, rbac.CanCloseTicket(string(models.RoleCustomer)))
	})

	t.Run("CanAccessAdminPanel checks correctly", func(t *testing.T) {
		assert.True(t, rbac.CanAccessAdminPanel(string(models.RoleAdmin)))
		assert.False(t, rbac.CanAccessAdminPanel(string(models.RoleAgent)))
		assert.False(t, rbac.CanAccessAdminPanel(string(models.RoleCustomer)))
	})

	t.Run("CanViewReports checks correctly", func(t *testing.T) {
		assert.True(t, rbac.CanViewReports(string(models.RoleAdmin)))
		assert.True(t, rbac.CanViewReports(string(models.RoleAgent)))
		assert.False(t, rbac.CanViewReports(string(models.RoleCustomer)))
	})
}
