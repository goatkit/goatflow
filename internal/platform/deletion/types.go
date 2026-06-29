// Package deletion implements the GoatKit PaaS Entity Deletion system.
//
// Provides soft delete (recycle bin + PII anonymisation), hard delete (purge),
// restore, tombstone logging, plugin cascade, and auto-purge.
package deletion

import (
	"time"
)

// Deletion actions for tombstone log.
const (
	ActionSoftDelete = "soft_delete"
	ActionRestore    = "restore"
	ActionHardDelete = "hard_delete"
)

// Entity types (matching customfields and other GoatKit entity keys).
const (
	EntityTicket        = "ticket"
	EntityArticle       = "article"
	EntityContact       = "contact"
	EntityAgent         = "agent"
	EntityQueue         = "queue"
	EntityOrganisation  = "organisation"
	EntityCustomerGroup = "customer_group"
)

// RecycleBinEntry represents a soft-deleted entity in the recycle bin.
type RecycleBinEntry struct {
	ID         int64      `json:"id" db:"id"`
	EntityType string     `json:"entity_type" db:"entity_type"`
	EntityID   int64      `json:"entity_id" db:"entity_id"`
	EntityName *string    `json:"entity_name,omitempty" db:"entity_name"`
	DeletedBy  int        `json:"deleted_by" db:"deleted_by"`
	DeletedAt  time.Time  `json:"deleted_at" db:"deleted_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty" db:"expires_at"`
	OrgID      *int64     `json:"org_id,omitempty" db:"org_id"`
}

// DeletionLog represents a tombstone entry in gk_deletion_log.
type DeletionLog struct {
	ID         int64     `json:"id" db:"id"`
	EntityType string    `json:"entity_type" db:"entity_type"`
	EntityID   int64     `json:"entity_id" db:"entity_id"`
	Action     string    `json:"action" db:"action"`
	DeletedBy  int       `json:"deleted_by" db:"deleted_by"`
	DeletedAt  time.Time `json:"deleted_at" db:"deleted_at"`
	OrgID      *int64    `json:"org_id,omitempty" db:"org_id"`
	Reason     *string   `json:"reason,omitempty" db:"reason"`
}

// EntityDeleteHandler defines how a specific entity type is soft/hard deleted.
type EntityDeleteHandler struct {
	// SoftDelete marks the entity as deleted using its native mechanism.
	// Returns the entity's display name for the recycle bin.
	SoftDelete func(db interface{}, entityID int64, userID int) (entityName string, err error)

	// HardDelete physically removes the entity and all linked data.
	HardDelete func(db interface{}, entityID int64) error

	// Restore reverses a soft delete.
	Restore func(db interface{}, entityID int64, userID int) error

	// Anonymise replaces PII fields with [DELETED].
	Anonymise func(db interface{}, entityID int64) error
}

// AnonymiseField represents a field to anonymise on soft delete.
type AnonymiseField struct {
	Table  string // table name
	Column string // column name
	Value  string // replacement value (default: "[DELETED]")
}

// DefaultAnonymiseValue is the replacement for anonymised PII fields.
const DefaultAnonymiseValue = "[DELETED]"

// PII fields to anonymise per entity type.
var AnonymiseConfig = map[string][]AnonymiseField{
	EntityTicket: {
		{Table: "ticket", Column: "title", Value: DefaultAnonymiseValue},
		{Table: "ticket", Column: "customer_id", Value: DefaultAnonymiseValue},
		{Table: "ticket", Column: "customer_user_id", Value: DefaultAnonymiseValue},
	},
	EntityContact: {
		{Table: "customer_user", Column: "first_name", Value: DefaultAnonymiseValue},
		{Table: "customer_user", Column: "last_name", Value: DefaultAnonymiseValue},
		{Table: "customer_user", Column: "email", Value: DefaultAnonymiseValue},
		{Table: "customer_user", Column: "phone", Value: DefaultAnonymiseValue},
		{Table: "customer_user", Column: "mobile", Value: DefaultAnonymiseValue},
		{Table: "customer_user", Column: "street", Value: DefaultAnonymiseValue},
		{Table: "customer_user", Column: "city", Value: DefaultAnonymiseValue},
		{Table: "customer_user", Column: "zip", Value: DefaultAnonymiseValue},
		{Table: "customer_user", Column: "country", Value: DefaultAnonymiseValue},
	},
	EntityAgent: {
		{Table: "users", Column: "first_name", Value: DefaultAnonymiseValue},
		{Table: "users", Column: "last_name", Value: DefaultAnonymiseValue},
	},
	EntityCustomerGroup: {
		{Table: "customer_company", Column: "name", Value: DefaultAnonymiseValue},
		{Table: "customer_company", Column: "street", Value: DefaultAnonymiseValue},
		{Table: "customer_company", Column: "city", Value: DefaultAnonymiseValue},
		{Table: "customer_company", Column: "zip", Value: DefaultAnonymiseValue},
		{Table: "customer_company", Column: "country", Value: DefaultAnonymiseValue},
	},
}

// ValidActions returns all valid tombstone log actions.
func ValidActions() []string {
	return []string{ActionSoftDelete, ActionRestore, ActionHardDelete}
}

// IsValidAction checks if the given action is valid.
func IsValidAction(a string) bool {
	for _, v := range ValidActions() {
		if a == v {
			return true
		}
	}
	return false
}
