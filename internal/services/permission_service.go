package services

import (
	"database/sql"
	"fmt"

	"github.com/goatkit/goatflow/internal/database"
)

// PermissionService handles permission checking for tickets and queues.
type PermissionService struct {
	db *sql.DB
}

// NewPermissionService creates a new permission service.
func NewPermissionService(db *sql.DB) *PermissionService {
	return &PermissionService{db: db}
}

// CanWriteTicket checks if a user has write (rw) permission on a ticket's queue.
// Returns true if the user has 'rw' permission on the queue's group.
func (s *PermissionService) CanWriteTicket(userID int, ticketID int64) (bool, error) {
	// Get the group_id for the ticket's queue, then check if user has 'rw' on that group
	query := database.ConvertPlaceholders(`
		SELECT EXISTS(
			SELECT 1 FROM group_user gu
			JOIN queue q ON gu.group_id = q.group_id
			JOIN ticket t ON t.queue_id = q.id
			WHERE t.id = ? 
			  AND gu.user_id = ?
			  AND gu.permission_key = 'rw'
		)`)

	var hasAccess bool
	err := s.db.QueryRow(query, ticketID, userID).Scan(&hasAccess)
	if err != nil {
		return false, fmt.Errorf("failed to check write permission: %w", err)
	}

	return hasAccess, nil
}

// CanReadTicket checks if a user has at least read (ro) permission on a ticket's queue.
// Returns true if the user has 'ro' OR 'rw' permission on the queue's group.
func (s *PermissionService) CanReadTicket(userID int, ticketID int64) (bool, error) {
	query := database.ConvertPlaceholders(`
		SELECT EXISTS(
			SELECT 1 FROM group_user gu
			JOIN queue q ON gu.group_id = q.group_id
			JOIN ticket t ON t.queue_id = q.id
			WHERE t.id = ? 
			  AND gu.user_id = ?
			  AND gu.permission_key IN ('ro', 'rw')
		)`)

	var hasAccess bool
	err := s.db.QueryRow(query, ticketID, userID).Scan(&hasAccess)
	if err != nil {
		return false, fmt.Errorf("failed to check read permission: %w", err)
	}

	return hasAccess, nil
}

// CanWriteQueue checks if a user has write (rw) permission on a specific queue.
func (s *PermissionService) CanWriteQueue(userID int, queueID int) (bool, error) {
	query := database.ConvertPlaceholders(`
		SELECT EXISTS(
			SELECT 1 FROM group_user gu
			JOIN queue q ON gu.group_id = q.group_id
			WHERE q.id = ? 
			  AND gu.user_id = ?
			  AND gu.permission_key = 'rw'
		)`)

	var hasAccess bool
	err := s.db.QueryRow(query, queueID, userID).Scan(&hasAccess)
	if err != nil {
		return false, fmt.Errorf("failed to check queue write permission: %w", err)
	}

	return hasAccess, nil
}

// CanReadQueue checks if a user has at least read (ro) permission on a specific queue.
func (s *PermissionService) CanReadQueue(userID int, queueID int) (bool, error) {
	query := database.ConvertPlaceholders(`
		SELECT EXISTS(
			SELECT 1 FROM group_user gu
			JOIN queue q ON gu.group_id = q.group_id
			WHERE q.id = ? 
			  AND gu.user_id = ?
			  AND gu.permission_key IN ('ro', 'rw')
		)`)

	var hasAccess bool
	err := s.db.QueryRow(query, queueID, userID).Scan(&hasAccess)
	if err != nil {
		return false, fmt.Errorf("failed to check queue read permission: %w", err)
	}

	return hasAccess, nil
}

// GetUserQueuePermissions returns all queues and permission levels for a user.
// Map key is queue_id, value is the highest permission level ('rw' > 'ro').
func (s *PermissionService) GetUserQueuePermissions(userID int) (map[int]string, error) {
	query := database.ConvertPlaceholders(`
		SELECT q.id, gu.permission_key
		FROM queue q
		JOIN group_user gu ON gu.group_id = q.group_id
		WHERE gu.user_id = ?
		  AND q.valid_id = 1
		ORDER BY q.id, gu.permission_key DESC`)

	rows, err := s.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user queue permissions: %w", err)
	}
	defer rows.Close()

	perms := make(map[int]string)
	for rows.Next() {
		var queueID int
		var permKey string
		if err := rows.Scan(&queueID, &permKey); err != nil {
			return nil, fmt.Errorf("failed to scan permission: %w", err)
		}
		// Keep highest permission (rw > ro)
		if existing, ok := perms[queueID]; !ok || permKey == "rw" && existing == "ro" {
			perms[queueID] = permKey
		}
	}

	return perms, rows.Err()
}

// Granular permission checks for OTRS-compatible permission model.
// Permission hierarchy: rw supersedes all granular permissions.
// Granular permissions: move_into, create, note, owner, priority

// HasPermission checks if a user has a specific permission on a queue.
// Returns true if user has the exact permission OR has 'rw' (which supersedes all).
func (s *PermissionService) HasPermission(userID int, queueID int, permKey string) (bool, error) {
	query := database.ConvertPlaceholders(`
		SELECT EXISTS(
			SELECT 1 FROM group_user gu
			JOIN queue q ON gu.group_id = q.group_id
			WHERE q.id = ? 
			  AND gu.user_id = ?
			  AND (gu.permission_key = ? OR gu.permission_key = 'rw')
		)`)

	var hasAccess bool
	err := s.db.QueryRow(query, queueID, userID, permKey).Scan(&hasAccess)
	if err != nil {
		return false, fmt.Errorf("failed to check %s permission: %w", permKey, err)
	}

	return hasAccess, nil
}

// HasTicketPermission checks if a user has a specific permission on a ticket's queue.
func (s *PermissionService) HasTicketPermission(userID int, ticketID int64, permKey string) (bool, error) {
	query := database.ConvertPlaceholders(`
		SELECT EXISTS(
			SELECT 1 FROM group_user gu
			JOIN queue q ON gu.group_id = q.group_id
			JOIN ticket t ON t.queue_id = q.id
			WHERE t.id = ? 
			  AND gu.user_id = ?
			  AND (gu.permission_key = ? OR gu.permission_key = 'rw')
		)`)

	var hasAccess bool
	err := s.db.QueryRow(query, ticketID, userID, permKey).Scan(&hasAccess)
	if err != nil {
		return false, fmt.Errorf("failed to check %s permission on ticket: %w", permKey, err)
	}

	return hasAccess, nil
}

// CanMoveInto checks if a user can move tickets into a queue.
// Requires 'move_into' or 'rw' permission.
func (s *PermissionService) CanMoveInto(userID int, queueID int) (bool, error) {
	return s.HasPermission(userID, queueID, "move_into")
}

// CanCreate checks if a user can create tickets in a queue.
// Requires 'create' or 'rw' permission.
func (s *PermissionService) CanCreate(userID int, queueID int) (bool, error) {
	return s.HasPermission(userID, queueID, "create")
}

// CanAddNote checks if a user can add notes to tickets in a queue.
// Requires 'note' or 'rw' permission.
func (s *PermissionService) CanAddNote(userID int, ticketID int64) (bool, error) {
	return s.HasTicketPermission(userID, ticketID, "note")
}

// CanBeOwner checks if a user can be assigned as owner in a queue.
// Requires 'owner' or 'rw' permission.
func (s *PermissionService) CanBeOwner(userID int, queueID int) (bool, error) {
	return s.HasPermission(userID, queueID, "owner")
}

// CanChangePriority checks if a user can change priority of tickets in a queue.
// Requires 'priority' or 'rw' permission.
func (s *PermissionService) CanChangePriority(userID int, ticketID int64) (bool, error) {
	return s.HasTicketPermission(userID, ticketID, "priority")
}

// =============================================================================
// CUSTOMER GROUP PERMISSIONS
// =============================================================================

// CustomerCanAccessQueue checks if a customer (by login) has access to a queue via group_customer_user.
// This is in addition to company-based access (customer_id on ticket).
func (s *PermissionService) CustomerCanAccessQueue(customerLogin string, queueID int) (bool, error) {
	// Check if customer user has explicit group permission on the queue's group
	query := database.ConvertPlaceholders(`
		SELECT EXISTS(
			SELECT 1 FROM group_customer_user gcu
			JOIN queue q ON gcu.group_id = q.group_id
			WHERE gcu.user_id = ?
			  AND q.id = ?
			  AND gcu.permission_key IN ('ro', 'rw')
		)`)

	var hasAccess bool
	err := s.db.QueryRow(query, customerLogin, queueID).Scan(&hasAccess)
	if err != nil {
		return false, fmt.Errorf("failed to check customer queue access: %w", err)
	}

	return hasAccess, nil
}

// CustomerCompanyCanAccessQueue checks if a customer's company has access to a queue via group_customer.
func (s *PermissionService) CustomerCompanyCanAccessQueue(customerID string, queueID int) (bool, error) {
	query := database.ConvertPlaceholders(`
		SELECT EXISTS(
			SELECT 1 FROM group_customer gc
			JOIN queue q ON gc.group_id = q.group_id
			WHERE gc.customer_id = ?
			  AND q.id = ?
			  AND gc.permission_key IN ('ro', 'rw')
		)`)

	var hasAccess bool
	err := s.db.QueryRow(query, customerID, queueID).Scan(&hasAccess)
	if err != nil {
		return false, fmt.Errorf("failed to check customer company queue access: %w", err)
	}

	return hasAccess, nil
}

// CustomerCanAccessTicket checks if a customer can access a specific ticket.
// Access is granted if:
// 1. The ticket belongs to the customer's company (customer_id matches), OR
// 2. The customer user has explicit group access to the ticket's queue, OR
// 3. The customer's company has explicit group access to the ticket's queue
func (s *PermissionService) CustomerCanAccessTicket(customerLogin, customerCompanyID string, ticketID int64) (bool, error) {
	query := database.ConvertPlaceholders(`
		SELECT EXISTS(
			SELECT 1 FROM ticket t
			WHERE t.id = ?
			  AND (
			    -- Company owns the ticket
			    t.customer_id = ?
			    -- OR customer user has group access to the queue
			    OR EXISTS(
			      SELECT 1 FROM group_customer_user gcu
			      JOIN queue q ON gcu.group_id = q.group_id
			      WHERE gcu.user_id = ? AND q.id = t.queue_id
			        AND gcu.permission_key IN ('ro', 'rw')
			    )
			    -- OR customer company has group access to the queue
			    OR EXISTS(
			      SELECT 1 FROM group_customer gc
			      JOIN queue q ON gc.group_id = q.group_id
			      WHERE gc.customer_id = ? AND q.id = t.queue_id
			        AND gc.permission_key IN ('ro', 'rw')
			    )
			  )
		)`)

	var hasAccess bool
	err := s.db.QueryRow(query, ticketID, customerCompanyID, customerLogin, customerCompanyID).Scan(&hasAccess)
	if err != nil {
		return false, fmt.Errorf("failed to check customer ticket access: %w", err)
	}

	return hasAccess, nil
}

// =============================================================================
// GROUP MEMBERSHIP
// =============================================================================

// IsInGroup checks if a user is a member of a group by group name.
// Used for administrative access checks (e.g., "admin" group for MCP execute_sql).
func (s *PermissionService) IsInGroup(userID int, groupName string) (bool, error) {
	// Try both `groups` (MySQL/MariaDB) and `permission_groups` (Postgres/alternate schema)
	query := database.ConvertPlaceholders(`
		SELECT EXISTS(
			SELECT 1 FROM group_user gu
			JOIN ` + "`groups`" + ` g ON gu.group_id = g.id
			WHERE gu.user_id = ?
			  AND g.name = ?
		)`)

	var isMember bool
	err := s.db.QueryRow(query, userID, groupName).Scan(&isMember)
	if err != nil {
		return false, fmt.Errorf("failed to check group membership: %w", err)
	}

	return isMember, nil
}
