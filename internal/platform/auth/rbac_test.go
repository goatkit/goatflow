package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"

	platformmodels "github.com/goatkit/goatflow/internal/platform/models"
)

func TestRBAC(t *testing.T) {
	t.Parallel()
	rbac := NewRBAC()

	t.Run("Admin has all permissions", func(t *testing.T) {
		role := string(platformmodels.RoleAdmin)

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
		role := string(platformmodels.RoleAgent)

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
		role := string(platformmodels.RoleCustomer)

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
		adminPerms := rbac.GetRolePermissions(string(platformmodels.RoleAdmin))
		assert.Greater(t, len(adminPerms), 10) // Admin should have many permissions

		agentPerms := rbac.GetRolePermissions(string(platformmodels.RoleAgent))
		assert.Greater(t, len(agentPerms), 5)            // Agent should have several permissions
		assert.Less(t, len(agentPerms), len(adminPerms)) // But less than admin

		customerPerms := rbac.GetRolePermissions(string(platformmodels.RoleCustomer))
		assert.Equal(t, 2, len(customerPerms)) // Customer should have exactly 2 permissions
	})

	t.Run("CanAccessTicket checks correctly", func(t *testing.T) {
		// Admin can access any ticket
		assert.True(t, rbac.CanAccessTicket(string(platformmodels.RoleAdmin), 100, 200))
		assert.True(t, rbac.CanAccessTicket(string(platformmodels.RoleAdmin), 1, 1))

		// Agent can access any ticket
		assert.True(t, rbac.CanAccessTicket(string(platformmodels.RoleAgent), 100, 200))
		assert.True(t, rbac.CanAccessTicket(string(platformmodels.RoleAgent), 1, 1))

		// Customer can only access their own tickets
		assert.True(t, rbac.CanAccessTicket(string(platformmodels.RoleCustomer), 100, 100))
		assert.False(t, rbac.CanAccessTicket(string(platformmodels.RoleCustomer), 100, 200))
	})

	t.Run("CanModifyUser checks correctly", func(t *testing.T) {
		// Only admin can modify users
		assert.True(t, rbac.CanModifyUser(string(platformmodels.RoleAdmin), string(platformmodels.RoleAgent)))
		assert.True(t, rbac.CanModifyUser(string(platformmodels.RoleAdmin), string(platformmodels.RoleCustomer)))

		// Agent cannot modify users
		assert.False(t, rbac.CanModifyUser(string(platformmodels.RoleAgent), string(platformmodels.RoleCustomer)))

		// Customer cannot modify users
		assert.False(t, rbac.CanModifyUser(string(platformmodels.RoleCustomer), string(platformmodels.RoleAgent)))
	})

	t.Run("CanAssignTicket checks correctly", func(t *testing.T) {
		assert.True(t, rbac.CanAssignTicket(string(platformmodels.RoleAdmin)))
		assert.True(t, rbac.CanAssignTicket(string(platformmodels.RoleAgent)))
		assert.False(t, rbac.CanAssignTicket(string(platformmodels.RoleCustomer)))
	})

	t.Run("CanCloseTicket checks correctly", func(t *testing.T) {
		assert.True(t, rbac.CanCloseTicket(string(platformmodels.RoleAdmin)))
		assert.True(t, rbac.CanCloseTicket(string(platformmodels.RoleAgent)))
		assert.False(t, rbac.CanCloseTicket(string(platformmodels.RoleCustomer)))
	})

	t.Run("CanAccessAdminPanel checks correctly", func(t *testing.T) {
		assert.True(t, rbac.CanAccessAdminPanel(string(platformmodels.RoleAdmin)))
		assert.False(t, rbac.CanAccessAdminPanel(string(platformmodels.RoleAgent)))
		assert.False(t, rbac.CanAccessAdminPanel(string(platformmodels.RoleCustomer)))
	})

	t.Run("CanViewReports checks correctly", func(t *testing.T) {
		assert.True(t, rbac.CanViewReports(string(platformmodels.RoleAdmin)))
		assert.True(t, rbac.CanViewReports(string(platformmodels.RoleAgent)))
		assert.False(t, rbac.CanViewReports(string(platformmodels.RoleCustomer)))
	})
 }
