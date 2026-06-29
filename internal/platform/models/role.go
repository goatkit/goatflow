package models

import "time"

// Role represents a user role in the system.
type Role struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Permissions []string          `json:"permissions"`
	IsSystem    bool              `json:"is_system"`
	IsActive    bool              `json:"is_active"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// Permission represents a system permission.
type Permission struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	IsSystem    bool   `json:"is_system"`
}

// Additional role names (RoleAdmin and RoleAgent are defined in user.go).
const (
	RoleUser  = "user"
	RoleGuest = "guest"
)

// Common permissions.
const (
	PermissionViewTickets     = "view_tickets"
	PermissionCreateTickets   = "create_tickets"
	PermissionEditTickets     = "edit_tickets"
	PermissionDeleteTickets   = "delete_tickets"
	PermissionAssignTickets   = "assign_tickets"
	PermissionViewAllTickets  = "view_all_tickets"
	PermissionManageUsers     = "manage_users"
	PermissionManageQueues    = "manage_queues"
	PermissionManageSettings  = "manage_settings"
	PermissionViewReports     = "view_reports"
	PermissionManageTemplates = "manage_templates"
	PermissionManageWorkflows = "manage_workflows"
)
