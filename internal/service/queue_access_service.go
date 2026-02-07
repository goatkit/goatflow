// Package service provides business logic services.
package service

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/goatkit/goatflow/internal/database"
)

// QueueAccessService handles queue permission checks for users.
// It resolves both direct permissions (via group_user) and role-based
// permissions (via role_user -> group_role) following OTRS compatibility.
type QueueAccessService struct {
	db *sql.DB
}

// NewQueueAccessService creates a new queue access service.
func NewQueueAccessService(db *sql.DB) *QueueAccessService {
	return &QueueAccessService{db: db}
}

// AccessibleQueue represents a queue the user can access.
type AccessibleQueue struct {
	QueueID   uint   `json:"queue_id"`
	QueueName string `json:"queue_name"`
	GroupID   uint   `json:"group_id"`
	GroupName string `json:"group_name"`
}

// GetUserEffectiveGroupIDs returns all group IDs the user has access to
// with the specified permission type. This includes both direct permissions
// (from group_user) and role-based permissions (from role_user -> group_role).
// The 'rw' permission supersedes all others, so if checking for 'ro', users
// with 'rw' are also included.
func (s *QueueAccessService) GetUserEffectiveGroupIDs(ctx context.Context, userID uint, permType string) ([]uint, error) {
	// Query combines direct permissions and role-based permissions
	// OTRS rule: 'rw' supersedes all other permissions
	query := database.ConvertPlaceholders(`
		SELECT DISTINCT gu.group_id
		FROM group_user gu
		JOIN ` + "`groups`" + ` g ON gu.group_id = g.id
		WHERE gu.user_id = ?
		  AND g.valid_id = 1
		  AND (gu.permission_key = ? OR gu.permission_key = 'rw')
		UNION
		SELECT DISTINCT gr.group_id
		FROM role_user ru
		JOIN roles r ON ru.role_id = r.id
		JOIN group_role gr ON ru.role_id = gr.role_id
		JOIN ` + "`groups`" + ` g ON gr.group_id = g.id
		WHERE ru.user_id = ?
		  AND r.valid_id = 1
		  AND g.valid_id = 1
		  AND (gr.permission_key = ? OR gr.permission_key = 'rw')
		  AND gr.permission_value = 1
	`)

	rows, err := s.db.QueryContext(ctx, query, userID, permType, userID, permType)
	if err != nil {
		return nil, fmt.Errorf("failed to get effective groups: %w", err)
	}
	defer rows.Close()

	var groupIDs []uint
	for rows.Next() {
		var groupID uint
		if err := rows.Scan(&groupID); err != nil {
			return nil, fmt.Errorf("failed to scan group ID: %w", err)
		}
		groupIDs = append(groupIDs, groupID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return groupIDs, nil
}

// GetAccessibleQueueIDs returns all queue IDs the user can access with
// the specified permission type.
func (s *QueueAccessService) GetAccessibleQueueIDs(ctx context.Context, userID uint, permType string) ([]uint, error) {
	// First get all group IDs the user has access to
	groupIDs, err := s.GetUserEffectiveGroupIDs(ctx, userID, permType)
	if err != nil {
		return nil, err
	}

	if len(groupIDs) == 0 {
		return []uint{}, nil
	}

	// Build query with placeholders for group IDs
	placeholders := "?"
	args := make([]interface{}, len(groupIDs))
	args[0] = groupIDs[0]
	for i := 1; i < len(groupIDs); i++ {
		placeholders += ", ?"
		args[i] = groupIDs[i]
	}

	query := database.ConvertPlaceholders(fmt.Sprintf(`
		SELECT q.id
		FROM queue q
		JOIN `+"`groups`"+` g ON q.group_id = g.id
		WHERE q.valid_id = 1
		  AND g.valid_id = 1
		  AND q.group_id IN (%s)
		ORDER BY q.name
	`, placeholders))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get accessible queues: %w", err)
	}
	defer rows.Close()

	var queueIDs []uint
	for rows.Next() {
		var queueID uint
		if err := rows.Scan(&queueID); err != nil {
			return nil, fmt.Errorf("failed to scan queue ID: %w", err)
		}
		queueIDs = append(queueIDs, queueID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return queueIDs, nil
}

// GetAccessibleQueues returns full queue information for all queues the user
// can access with the specified permission type.
func (s *QueueAccessService) GetAccessibleQueues(ctx context.Context, userID uint, permType string) ([]AccessibleQueue, error) {
	// First get all group IDs the user has access to
	groupIDs, err := s.GetUserEffectiveGroupIDs(ctx, userID, permType)
	if err != nil {
		return nil, err
	}

	if len(groupIDs) == 0 {
		return []AccessibleQueue{}, nil
	}

	// Build query with placeholders for group IDs
	placeholders := "?"
	args := make([]interface{}, len(groupIDs))
	args[0] = groupIDs[0]
	for i := 1; i < len(groupIDs); i++ {
		placeholders += ", ?"
		args[i] = groupIDs[i]
	}

	query := database.ConvertPlaceholders(fmt.Sprintf(`
		SELECT q.id, q.name, q.group_id, g.name as group_name
		FROM queue q
		JOIN `+"`groups`"+` g ON q.group_id = g.id
		WHERE q.valid_id = 1
		  AND g.valid_id = 1
		  AND q.group_id IN (%s)
		ORDER BY q.name
	`, placeholders))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get accessible queues: %w", err)
	}
	defer rows.Close()

	var queues []AccessibleQueue
	for rows.Next() {
		var q AccessibleQueue
		if err := rows.Scan(&q.QueueID, &q.QueueName, &q.GroupID, &q.GroupName); err != nil {
			return nil, fmt.Errorf("failed to scan queue: %w", err)
		}
		queues = append(queues, q)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return queues, nil
}

// HasQueueAccess checks if the user has the specified permission type for
// a specific queue.
func (s *QueueAccessService) HasQueueAccess(ctx context.Context, userID, queueID uint, permType string) (bool, error) {
	// Get queue's group ID
	var queueGroupID uint
	query := database.ConvertPlaceholders(`
		SELECT group_id FROM queue WHERE id = ? AND valid_id = 1
	`)
	err := s.db.QueryRowContext(ctx, query, queueID).Scan(&queueGroupID)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil // Queue doesn't exist or is invalid
		}
		return false, fmt.Errorf("failed to get queue group: %w", err)
	}

	// Check if user has permission on that group
	groupIDs, err := s.GetUserEffectiveGroupIDs(ctx, userID, permType)
	if err != nil {
		return false, err
	}

	for _, gid := range groupIDs {
		if gid == queueGroupID {
			return true, nil
		}
	}

	return false, nil
}

// IsAdmin checks if the user is in the admin group.
// Admin users bypass all queue permission checks.
func (s *QueueAccessService) IsAdmin(ctx context.Context, userID uint) (bool, error) {
	query := database.ConvertPlaceholders(`
		SELECT COUNT(*)
		FROM group_user gu
		JOIN ` + "`groups`" + ` g ON gu.group_id = g.id
		WHERE gu.user_id = ?
		  AND g.name = 'admin'
		  AND g.valid_id = 1
	`)

	var count int
	err := s.db.QueryRowContext(ctx, query, userID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check admin status: %w", err)
	}

	return count > 0, nil
}

// GetQueueGroupID returns the group ID for a specific queue.
func (s *QueueAccessService) GetQueueGroupID(ctx context.Context, queueID uint) (uint, error) {
	query := database.ConvertPlaceholders(`
		SELECT group_id FROM queue WHERE id = ? AND valid_id = 1
	`)

	var groupID uint
	err := s.db.QueryRowContext(ctx, query, queueID).Scan(&groupID)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("queue not found or invalid")
		}
		return 0, fmt.Errorf("failed to get queue group: %w", err)
	}

	return groupID, nil
}

// HasPermissionOnGroup checks if user has specific permission on a group.
func (s *QueueAccessService) HasPermissionOnGroup(ctx context.Context, userID, groupID uint, permType string) (bool, error) {
	groupIDs, err := s.GetUserEffectiveGroupIDs(ctx, userID, permType)
	if err != nil {
		return false, err
	}

	for _, gid := range groupIDs {
		if gid == groupID {
			return true, nil
		}
	}

	return false, nil
}
