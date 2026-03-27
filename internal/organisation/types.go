// Package organisation implements GoatKit PaaS Core multi-tenancy.
//
// Provides organisation entities, user membership, per-org sysconfig overrides,
// and org context resolution for request scoping.
package organisation

import (
	"time"
)

// Organisation statuses.
const (
	StatusActive    = "active"
	StatusSuspended = "suspended"
	StatusArchived  = "archived"
)

// Membership roles.
const (
	RoleMember = "member"
	RoleAdmin  = "admin"
	RoleOwner  = "owner"
)

// ValidStatuses returns all supported organisation statuses.
func ValidStatuses() []string {
	return []string{StatusActive, StatusSuspended, StatusArchived}
}

// ValidRoles returns all supported membership roles.
func ValidRoles() []string {
	return []string{RoleMember, RoleAdmin, RoleOwner}
}

// IsValidStatus checks whether the given status is supported.
func IsValidStatus(s string) bool {
	for _, v := range ValidStatuses() {
		if s == v {
			return true
		}
	}
	return false
}

// IsValidRole checks whether the given role is supported.
func IsValidRole(r string) bool {
	for _, v := range ValidRoles() {
		if r == v {
			return true
		}
	}
	return false
}

// Organisation represents a row in gk_organisation.
type Organisation struct {
	ID                int64     `json:"id" db:"id"`
	Name              string    `json:"name" db:"name"`
	Slug              string    `json:"slug" db:"slug"`
	ParentID          *int64    `json:"parent_id,omitempty" db:"parent_id"`
	Status            string    `json:"status" db:"status"`
	CustomerCompanyID *string   `json:"customer_company_id,omitempty" db:"customer_company_id"`
	ValidID           int       `json:"valid_id" db:"valid_id"`
	CreateTime        time.Time `json:"create_time" db:"create_time"`
	CreateBy          int       `json:"create_by" db:"create_by"`
	ChangeTime        time.Time `json:"change_time" db:"change_time"`
	ChangeBy          int       `json:"change_by" db:"change_by"`
}

// IsActive returns true if the organisation is active and valid.
func (o *Organisation) IsActive() bool {
	return o.ValidID == 1 && o.Status == StatusActive
}

// UserOrganisation represents a row in gk_user_organisation.
type UserOrganisation struct {
	ID             int64     `json:"id" db:"id"`
	OrgID          int64     `json:"org_id" db:"org_id"`
	UserID         *int      `json:"user_id,omitempty" db:"user_id"`
	CustomerLogin  *string   `json:"customer_login,omitempty" db:"customer_login"`
	Role           string    `json:"role" db:"role"`
	IsDefault      bool      `json:"is_default" db:"is_default"`
	CreateTime     time.Time `json:"create_time" db:"create_time"`
	CreateBy       int       `json:"create_by" db:"create_by"`
}

// SysconfigOrg represents a per-org sysconfig override in sysconfig_org.
type SysconfigOrg struct {
	ID             int64     `json:"id" db:"id"`
	OrgID          int64     `json:"org_id" db:"org_id"`
	Name           string    `json:"name" db:"name"`
	EffectiveValue []byte    `json:"effective_value" db:"effective_value"`
	IsValid        int       `json:"is_valid" db:"is_valid"`
	CreateTime     time.Time `json:"create_time" db:"create_time"`
	CreateBy       int       `json:"create_by" db:"create_by"`
	ChangeTime     time.Time `json:"change_time" db:"change_time"`
	ChangeBy       int       `json:"change_by" db:"change_by"`
}

// OrgWithMemberCount extends Organisation with a member count for admin list views.
type OrgWithMemberCount struct {
	Organisation
	MemberCount int `json:"member_count"`
}
