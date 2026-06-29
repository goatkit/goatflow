package models

import "time"

// Group represents a user group in the system.
type Group struct {
	// OTRS-compatible fields (required for database operations)
	ID         interface{} `json:"id"` // Can be int or string depending on context
	Name       string      `json:"name"`
	Comments   string      `json:"comments,omitempty"`
	ValidID    int         `json:"valid_id,omitempty"`
	CreateTime time.Time   `json:"create_time,omitempty"`
	CreateBy   int         `json:"create_by,omitempty"`
	ChangeTime time.Time   `json:"change_time,omitempty"`
	ChangeBy   int         `json:"change_by,omitempty"`

	// Additional fields for LDAP/extended functionality
	Description string            `json:"description,omitempty"`
	Type        string            `json:"type,omitempty"`    // ldap, local, external
	DN          string            `json:"dn,omitempty"`      // LDAP Distinguished Name
	Members     []string          `json:"members,omitempty"` // User IDs
	Permissions []string          `json:"permissions,omitempty"`
	IsActive    bool              `json:"is_active,omitempty"`
	IsSystem    bool              `json:"is_system,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at,omitempty"`
	UpdatedAt   time.Time         `json:"updated_at,omitempty"`
}

// GroupMembership represents a user's membership in a group.
type GroupMembership struct {
	GroupID string    `json:"group_id"`
	UserID  string    `json:"user_id"`
	Role    string    `json:"role,omitempty"` // member, admin, etc.
	AddedBy string    `json:"added_by,omitempty"`
	AddedAt time.Time `json:"added_at"`
}

// Common group types.
const (
	GroupTypeLocal    = "local"
	GroupTypeLDAP     = "ldap"
	GroupTypeExternal = "external"
)

// Common group roles.
const (
	GroupRoleMember = "member"
	GroupRoleAdmin  = "admin"
	GroupRoleOwner  = "owner"
)
