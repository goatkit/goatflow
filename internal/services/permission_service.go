package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/service"
)

// PermissionService answers the agent ticket/queue permission checks of the
// ticket APIs. It uses the same effective-permission model as the queue
// access middleware (service.QueueAccessService): a permission on a queue's
// group comes from group_user or from a valid role (role_user -> group_role),
// 'rw' supersedes every granular key, and members of the admin group have
// full access.
type PermissionService struct {
	db     *sql.DB
	access *service.QueueAccessService
}

// NewPermissionService creates a new permission service.
func NewPermissionService(db *sql.DB) *PermissionService {
	return &PermissionService{db: db, access: service.NewQueueAccessService(db)}
}

// CanWriteTicket checks if a user has write (rw) permission on a ticket's queue.
func (s *PermissionService) CanWriteTicket(userID int, ticketID int64) (bool, error) {
	return s.HasTicketPermission(userID, ticketID, "rw")
}

// CanWriteQueue checks if a user has write (rw) permission on a specific queue.
func (s *PermissionService) CanWriteQueue(userID int, queueID int) (bool, error) {
	return s.HasPermission(userID, queueID, "rw")
}

// CanReadQueue checks if a user has at least read (ro) permission on a specific queue.
func (s *PermissionService) CanReadQueue(userID int, queueID int) (bool, error) {
	return s.HasPermission(userID, queueID, "ro")
}

// GetUserQueuePermissions returns every valid queue the user holds any
// effective permission on. Map key is queue_id, value is 'rw' when the user
// has rw on the queue's group, otherwise one of the granular keys held.
func (s *PermissionService) GetUserQueuePermissions(userID int) (map[int]string, error) {
	if userID <= 0 {
		return map[int]string{}, nil
	}
	ctx := context.Background()
	admin, err := s.access.IsAdmin(ctx, uint(userID))
	if err != nil {
		return nil, fmt.Errorf("failed to get user queue permissions: %w", err)
	}

	var rows *sql.Rows
	if admin {
		rows, err = s.db.QueryContext(ctx, database.ConvertPlaceholders(`
			SELECT id, 'rw' FROM queue WHERE valid_id = 1`))
	} else {
		rows, err = s.db.QueryContext(ctx, database.ConvertPlaceholders(`
			SELECT q.id, p.permission_key
			FROM queue q
			JOIN `+"`groups`"+` g ON g.id = q.group_id
			JOIN (
				SELECT gu.group_id, gu.permission_key
				FROM group_user gu
				WHERE gu.user_id = ?
				UNION
				SELECT gr.group_id, gr.permission_key
				FROM role_user ru
				JOIN roles r ON r.id = ru.role_id
				JOIN group_role gr ON gr.role_id = ru.role_id
				WHERE ru.user_id = ?
				  AND r.valid_id = 1
				  AND gr.permission_value = 1
			) p ON p.group_id = q.group_id
			WHERE q.valid_id = 1
			  AND g.valid_id = 1`), userID, userID)
	}
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
		if existing, ok := perms[queueID]; !ok || (permKey == "rw" && existing != "rw") {
			perms[queueID] = permKey
		}
	}
	return perms, rows.Err()
}

// Granular permission checks for the OTRS permission model: rw supersedes
// the granular keys move_into, create, note, owner, priority (and ro).

// HasPermission checks if a user has a specific permission on a queue.
func (s *PermissionService) HasPermission(userID int, queueID int, permKey string) (bool, error) {
	if userID <= 0 || queueID <= 0 {
		return false, nil
	}
	var groupID uint
	err := s.db.QueryRow(database.ConvertPlaceholders(
		`SELECT group_id FROM queue WHERE id = ?`), queueID).Scan(&groupID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check %s permission: %w", permKey, err)
	}
	return s.hasGroupPermission(uint(userID), groupID, permKey)
}

// HasTicketPermission checks if a user has a specific permission on a ticket's queue.
func (s *PermissionService) HasTicketPermission(userID int, ticketID int64, permKey string) (bool, error) {
	if userID <= 0 || ticketID <= 0 {
		return false, nil
	}
	var groupID uint
	err := s.db.QueryRow(database.ConvertPlaceholders(`
		SELECT q.group_id FROM ticket t JOIN queue q ON q.id = t.queue_id WHERE t.id = ?`), ticketID).Scan(&groupID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check %s permission on ticket: %w", permKey, err)
	}
	return s.hasGroupPermission(uint(userID), groupID, permKey)
}

func (s *PermissionService) hasGroupPermission(userID, groupID uint, permKey string) (bool, error) {
	ctx := context.Background()
	admin, err := s.access.IsAdmin(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("failed to check %s permission: %w", permKey, err)
	}
	if admin {
		return true, nil
	}
	ok, err := s.access.HasPermissionOnGroup(ctx, userID, groupID, permKey)
	if err != nil {
		return false, fmt.Errorf("failed to check %s permission: %w", permKey, err)
	}
	return ok, nil
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
