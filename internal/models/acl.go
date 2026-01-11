package models

import (
	"time"
)

// ACL represents an Access Control List entry.
type ACL struct {
	ID             int              `json:"id"`
	Name           string           `json:"name"`
	Comments       *string          `json:"comments,omitempty"`
	Description    *string          `json:"description,omitempty"`
	ValidID        int              `json:"valid_id"`
	StopAfterMatch bool             `json:"stop_after_match"`
	ConfigMatch    *ACLConfigMatch  `json:"config_match,omitempty"`
	ConfigChange   *ACLConfigChange `json:"config_change,omitempty"`
	CreateTime     time.Time        `json:"create_time"`
	CreateBy       int              `json:"create_by"`
	ChangeTime     time.Time        `json:"change_time"`
	ChangeBy       int              `json:"change_by"`
}

// ACLConfigMatch defines the conditions for when an ACL applies.
// Matches against current ticket/form values (Properties) or database values (PropertiesDatabase).
type ACLConfigMatch struct {
	// Properties matches against current form/frontend values
	Properties map[string]map[string][]string `json:"Properties,omitempty"`
	// PropertiesDatabase matches against values from the database
	PropertiesDatabase map[string]map[string][]string `json:"PropertiesDatabase,omitempty"`
}

// ACLConfigChange defines what changes the ACL makes to available options.
type ACLConfigChange struct {
	// Possible is a whitelist - ONLY these values are allowed
	Possible map[string]map[string][]string `json:"Possible,omitempty"`
	// PossibleAdd adds these values to the current options
	PossibleAdd map[string]map[string][]string `json:"PossibleAdd,omitempty"`
	// PossibleNot is a blacklist - these values are removed
	PossibleNot map[string]map[string][]string `json:"PossibleNot,omitempty"`
}

// IsValid returns true if the ACL is enabled (valid_id = 1).
func (a *ACL) IsValid() bool {
	return a.ValidID == 1
}

// HasMatch returns true if the ACL has any match conditions defined.
func (a *ACL) HasMatch() bool {
	if a.ConfigMatch == nil {
		return false
	}
	return len(a.ConfigMatch.Properties) > 0 || len(a.ConfigMatch.PropertiesDatabase) > 0
}

// HasChange returns true if the ACL has any change rules defined.
func (a *ACL) HasChange() bool {
	if a.ConfigChange == nil {
		return false
	}
	return len(a.ConfigChange.Possible) > 0 ||
		len(a.ConfigChange.PossibleAdd) > 0 ||
		len(a.ConfigChange.PossibleNot) > 0
}

// ACLContext provides the context for ACL evaluation.
type ACLContext struct {
	// User context
	UserID         int
	CustomerUserID string

	// Ticket context (from database for PropertiesDatabase matching)
	TicketID     int
	Ticket       *Ticket
	QueueID      int
	StateID      int
	PriorityID   int
	TypeID       int
	ServiceID    int
	SLAID        int
	OwnerID      int
	LockID       int
	CustomerID   string

	// Frontend/Form context (for Properties matching)
	// These represent current form values that may differ from DB
	FormQueueID    *int
	FormStateID    *int
	FormPriorityID *int
	FormTypeID     *int
	FormServiceID  *int
	FormSLAID      *int
	FormOwnerID    *int
	FormLockID     *int

	// Dynamic fields
	DynamicFields map[string]interface{}

	// Action context (which action/screen is being used)
	Action string
	// Frontend indicates whether to match frontend/form values
	Frontend bool
}

// ACLResult contains the result of ACL evaluation.
type ACLResult struct {
	// MatchedACLs contains the names of ACLs that matched
	MatchedACLs []string
	// Allowed contains IDs that are allowed (from Possible rules)
	Allowed map[string][]int
	// Denied contains IDs that are denied (from PossibleNot rules)
	Denied map[string][]int
	// Added contains IDs that were added (from PossibleAdd rules)
	Added map[string][]int
}

// NewACLResult creates a new empty ACL result.
func NewACLResult() *ACLResult {
	return &ACLResult{
		MatchedACLs: []string{},
		Allowed:     make(map[string][]int),
		Denied:      make(map[string][]int),
		Added:       make(map[string][]int),
	}
}
