package models

import "time"

// DBRole represents a role in the database (Znuny-compatible)
// Maps to the `roles` table
type DBRole struct {
	ID         int       `json:"id"`
	Name       string    `json:"name"`
	Comments   string    `json:"comments"`
	ValidID    int       `json:"valid_id"`
	CreateTime time.Time `json:"create_time"`
	CreateBy   int       `json:"create_by"`
	ChangeTime time.Time `json:"change_time"`
	ChangeBy   int       `json:"change_by"`
}

// IsValid returns true if the role is active (valid_id = 1)
func (r *DBRole) IsValid() bool {
	return r.ValidID == 1
}

// DBRoleUser represents a user-role assignment
// Maps to the `role_user` table
type DBRoleUser struct {
	UserID     int       `json:"user_id"`
	RoleID     int       `json:"role_id"`
	CreateTime time.Time `json:"create_time"`
	CreateBy   int       `json:"create_by"`
	ChangeTime time.Time `json:"change_time"`
	ChangeBy   int       `json:"change_by"`
}

// DBGroupRole represents a role-group permission assignment
// Maps to the `group_role` table
type DBGroupRole struct {
	RoleID          int       `json:"role_id"`
	GroupID         int       `json:"group_id"`
	PermissionKey   string    `json:"permission_key"`
	PermissionValue int       `json:"permission_value"`
	CreateTime      time.Time `json:"create_time"`
	CreateBy        int       `json:"create_by"`
	ChangeTime      time.Time `json:"change_time"`
	ChangeBy        int       `json:"change_by"`
}

// Permission types (Znuny-compatible)
var PermissionTypes = []string{
	"ro",        // Read-only access
	"move_into", // Move tickets into queue
	"create",    // Create tickets in queue
	"note",      // Add notes to tickets
	"owner",     // Take ownership of tickets
	"priority",  // Change ticket priority
	"rw",        // Read/Write - full access (implies all others)
}
