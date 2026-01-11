// Package acl provides the ACL (Access Control List) execution engine.
// ACLs filter available options in ticket forms based on current context.
package acl

import (
	"context"
	"database/sql"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
)

// Service evaluates ACLs to filter available options.
type Service struct {
	db      *sql.DB
	aclRepo *repository.ACLRepository
	logger  *log.Logger
	// Cache of valid ACLs (can be refreshed)
	cachedACLs []*models.ACL
}

// Option configures the service.
type Option func(*Service)

// WithLogger sets a custom logger.
func WithLogger(l *log.Logger) Option {
	return func(s *Service) { s.logger = l }
}

// NewService creates a new ACL service.
func NewService(db *sql.DB, opts ...Option) *Service {
	s := &Service{
		db:      db,
		aclRepo: repository.NewACLRepository(db),
		logger:  log.Default(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// RefreshCache reloads valid ACLs from the database.
func (s *Service) RefreshCache(ctx context.Context) error {
	acls, err := s.aclRepo.GetValidACLs(ctx)
	if err != nil {
		return err
	}
	s.cachedACLs = acls
	return nil
}

// getACLs returns cached ACLs or loads them if not cached.
func (s *Service) getACLs(ctx context.Context) ([]*models.ACL, error) {
	if s.cachedACLs != nil {
		return s.cachedACLs, nil
	}
	return s.aclRepo.GetValidACLs(ctx)
}

// FilterOptions evaluates ACLs and filters the available options.
// Parameters:
//   - ctx: context for database operations
//   - aclCtx: current ticket/user context for matching
//   - returnType: what to filter (e.g., "Ticket", "Action")
//   - returnSubType: specific field to filter (e.g., "State", "Queue", "Priority")
//   - options: map of ID -> name for available options
//
// Returns filtered map of ID -> name.
func (s *Service) FilterOptions(
	ctx context.Context,
	aclCtx *models.ACLContext,
	returnType, returnSubType string,
	options map[int]string,
) (map[int]string, error) {
	if len(options) == 0 {
		return options, nil
	}

	acls, err := s.getACLs(ctx)
	if err != nil {
		return options, err
	}

	if len(acls) == 0 {
		return options, nil
	}

	result := models.NewACLResult()

	// Evaluate each ACL in order
	for _, acl := range acls {
		if !acl.IsValid() || !acl.HasChange() {
			continue
		}

		// Check if ACL matches current context
		if !s.matchesContext(acl, aclCtx) {
			continue
		}

		result.MatchedACLs = append(result.MatchedACLs, acl.Name)

		// Apply changes
		s.applyChanges(acl, returnType, returnSubType, options, result)

		// Stop if StopAfterMatch is set
		if acl.StopAfterMatch {
			break
		}
	}

	// Apply the accumulated result to filter options
	return s.applyResult(options, returnType, returnSubType, result), nil
}

// FilterActions evaluates ACLs and filters available actions.
// This is a convenience method for filtering action buttons.
func (s *Service) FilterActions(
	ctx context.Context,
	aclCtx *models.ACLContext,
	actions []string,
) ([]string, error) {
	if len(actions) == 0 {
		return actions, nil
	}

	acls, err := s.getACLs(ctx)
	if err != nil {
		return actions, err
	}

	if len(acls) == 0 {
		return actions, nil
	}

	// Build a set of allowed actions
	allowed := make(map[string]bool)
	for _, a := range actions {
		allowed[a] = true
	}

	for _, acl := range acls {
		if !acl.IsValid() || !acl.HasChange() {
			continue
		}

		if !s.matchesContext(acl, aclCtx) {
			continue
		}

		cc := acl.ConfigChange

		// Possible: only these actions allowed
		if cc.Possible != nil {
			if actionList, ok := cc.Possible["Action"]; ok {
				if vals, ok := actionList[""]; ok {
					newAllowed := make(map[string]bool)
					for _, v := range vals {
						if allowed[v] {
							newAllowed[v] = true
						}
					}
					allowed = newAllowed
				}
			}
		}

		// PossibleAdd: add these actions
		if cc.PossibleAdd != nil {
			if actionList, ok := cc.PossibleAdd["Action"]; ok {
				if vals, ok := actionList[""]; ok {
					for _, v := range vals {
						allowed[v] = true
					}
				}
			}
		}

		// PossibleNot: remove these actions
		if cc.PossibleNot != nil {
			if actionList, ok := cc.PossibleNot["Action"]; ok {
				if vals, ok := actionList[""]; ok {
					for _, v := range vals {
						delete(allowed, v)
					}
				}
			}
		}

		if acl.StopAfterMatch {
			break
		}
	}

	// Build filtered result
	var result []string
	for _, a := range actions {
		if allowed[a] {
			result = append(result, a)
		}
	}

	return result, nil
}

// matchesContext checks if an ACL matches the current context.
func (s *Service) matchesContext(acl *models.ACL, aclCtx *models.ACLContext) bool {
	if acl.ConfigMatch == nil {
		// No match conditions = always matches
		return true
	}

	// Check Properties (frontend/form values)
	if len(acl.ConfigMatch.Properties) > 0 {
		if !s.matchProperties(acl.ConfigMatch.Properties, aclCtx, true) {
			return false
		}
	}

	// Check PropertiesDatabase (database values)
	if len(acl.ConfigMatch.PropertiesDatabase) > 0 {
		if !s.matchProperties(acl.ConfigMatch.PropertiesDatabase, aclCtx, false) {
			return false
		}
	}

	return true
}

// matchProperties checks if properties match the context.
func (s *Service) matchProperties(props map[string]map[string][]string, aclCtx *models.ACLContext, useFrontend bool) bool {
	for category, fields := range props {
		for field, values := range fields {
			if !s.matchField(category, field, values, aclCtx, useFrontend) {
				return false
			}
		}
	}
	return true
}

// matchField checks if a specific field matches the expected values.
func (s *Service) matchField(category, field string, expected []string, aclCtx *models.ACLContext, useFrontend bool) bool {
	if len(expected) == 0 {
		return true
	}

	actual := s.getFieldValue(category, field, aclCtx, useFrontend)

	// Check if any expected value matches
	for _, exp := range expected {
		if s.valueMatches(exp, actual) {
			return true
		}
	}

	return false
}

// getFieldValue gets the current value of a field from context.
func (s *Service) getFieldValue(category, field string, aclCtx *models.ACLContext, useFrontend bool) string {
	switch category {
	case "Ticket":
		return s.getTicketFieldValue(field, aclCtx, useFrontend)
	case "User":
		return s.getUserFieldValue(field, aclCtx)
	case "CustomerUser":
		return s.getCustomerUserFieldValue(field, aclCtx)
	case "Queue":
		return s.getQueueFieldValue(field, aclCtx, useFrontend)
	case "DynamicField":
		return s.getDynamicFieldValue(field, aclCtx)
	case "Frontend":
		if field == "Action" {
			return aclCtx.Action
		}
	}
	return ""
}

// getTicketFieldValue gets ticket field values from context.
func (s *Service) getTicketFieldValue(field string, aclCtx *models.ACLContext, useFrontend bool) string {
	// Use form values if frontend matching and available
	if useFrontend {
		switch field {
		case "StateID", "State":
			if aclCtx.FormStateID != nil {
				return strconv.Itoa(*aclCtx.FormStateID)
			}
		case "QueueID", "Queue":
			if aclCtx.FormQueueID != nil {
				return strconv.Itoa(*aclCtx.FormQueueID)
			}
		case "PriorityID", "Priority":
			if aclCtx.FormPriorityID != nil {
				return strconv.Itoa(*aclCtx.FormPriorityID)
			}
		case "TypeID", "Type":
			if aclCtx.FormTypeID != nil {
				return strconv.Itoa(*aclCtx.FormTypeID)
			}
		case "ServiceID", "Service":
			if aclCtx.FormServiceID != nil {
				return strconv.Itoa(*aclCtx.FormServiceID)
			}
		case "SLAID", "SLA":
			if aclCtx.FormSLAID != nil {
				return strconv.Itoa(*aclCtx.FormSLAID)
			}
		case "OwnerID", "Owner":
			if aclCtx.FormOwnerID != nil {
				return strconv.Itoa(*aclCtx.FormOwnerID)
			}
		case "LockID", "Lock":
			if aclCtx.FormLockID != nil {
				return strconv.Itoa(*aclCtx.FormLockID)
			}
		}
	}

	// Fall back to database values
	switch field {
	case "StateID", "State":
		return strconv.Itoa(aclCtx.StateID)
	case "QueueID", "Queue":
		return strconv.Itoa(aclCtx.QueueID)
	case "PriorityID", "Priority":
		return strconv.Itoa(aclCtx.PriorityID)
	case "TypeID", "Type":
		return strconv.Itoa(aclCtx.TypeID)
	case "ServiceID", "Service":
		return strconv.Itoa(aclCtx.ServiceID)
	case "SLAID", "SLA":
		return strconv.Itoa(aclCtx.SLAID)
	case "OwnerID", "Owner":
		return strconv.Itoa(aclCtx.OwnerID)
	case "LockID", "Lock":
		return strconv.Itoa(aclCtx.LockID)
	case "CustomerID":
		return aclCtx.CustomerID
	case "TicketID":
		return strconv.Itoa(aclCtx.TicketID)
	}

	// Check ticket object if available
	if aclCtx.Ticket != nil {
		switch field {
		case "Title":
			return aclCtx.Ticket.Title
		}
	}

	return ""
}

// getUserFieldValue gets user field values from context.
func (s *Service) getUserFieldValue(field string, aclCtx *models.ACLContext) string {
	switch field {
	case "UserID":
		return strconv.Itoa(aclCtx.UserID)
	}
	return ""
}

// getCustomerUserFieldValue gets customer user field values from context.
func (s *Service) getCustomerUserFieldValue(field string, aclCtx *models.ACLContext) string {
	switch field {
	case "UserLogin":
		return aclCtx.CustomerUserID
	}
	return ""
}

// getQueueFieldValue gets queue field values from context.
func (s *Service) getQueueFieldValue(field string, aclCtx *models.ACLContext, useFrontend bool) string {
	switch field {
	case "QueueID":
		if useFrontend && aclCtx.FormQueueID != nil {
			return strconv.Itoa(*aclCtx.FormQueueID)
		}
		return strconv.Itoa(aclCtx.QueueID)
	}
	return ""
}

// getDynamicFieldValue gets dynamic field values from context.
func (s *Service) getDynamicFieldValue(field string, aclCtx *models.ACLContext) string {
	if aclCtx.DynamicFields == nil {
		return ""
	}
	if val, ok := aclCtx.DynamicFields[field]; ok {
		switch v := val.(type) {
		case string:
			return v
		case int:
			return strconv.Itoa(v)
		case int64:
			return strconv.FormatInt(v, 10)
		case []string:
			return strings.Join(v, ",")
		}
	}
	return ""
}

// valueMatches checks if expected value matches actual value.
// Supports wildcards (*), negation ([Not]), and regex ([RegExp]).
func (s *Service) valueMatches(expected, actual string) bool {
	// Handle negation prefix
	if strings.HasPrefix(expected, "[Not]") {
		pattern := strings.TrimPrefix(expected, "[Not]")
		return !s.valueMatchesPattern(pattern, actual)
	}

	// Handle regex prefix
	if strings.HasPrefix(expected, "[RegExp]") {
		pattern := strings.TrimPrefix(expected, "[RegExp]")
		matched, err := regexp.MatchString(pattern, actual)
		return err == nil && matched
	}

	return s.valueMatchesPattern(expected, actual)
}

// valueMatchesPattern checks if pattern matches value (supports wildcards).
func (s *Service) valueMatchesPattern(pattern, value string) bool {
	// Exact match
	if pattern == value {
		return true
	}

	// Wildcard matching
	if strings.Contains(pattern, "*") {
		// Convert wildcard to regex
		regexPattern := "^" + regexp.QuoteMeta(pattern) + "$"
		regexPattern = strings.ReplaceAll(regexPattern, `\*`, ".*")
		matched, err := regexp.MatchString(regexPattern, value)
		return err == nil && matched
	}

	return false
}

// applyChanges applies ACL changes to the result.
func (s *Service) applyChanges(acl *models.ACL, returnType, returnSubType string, options map[int]string, result *models.ACLResult) {
	cc := acl.ConfigChange
	if cc == nil {
		return
	}

	// Apply Possible (whitelist)
	if cc.Possible != nil {
		if typeRules, ok := cc.Possible[returnType]; ok {
			if values, ok := typeRules[returnSubType]; ok {
				ids := s.resolveValuesToIDs(values, options)
				if result.Allowed[returnSubType] == nil {
					result.Allowed[returnSubType] = ids
				} else {
					// Intersect with existing allowed
					result.Allowed[returnSubType] = intersectIDs(result.Allowed[returnSubType], ids)
				}
			}
		}
	}

	// Apply PossibleAdd
	if cc.PossibleAdd != nil {
		if typeRules, ok := cc.PossibleAdd[returnType]; ok {
			if values, ok := typeRules[returnSubType]; ok {
				ids := s.resolveValuesToIDs(values, options)
				result.Added[returnSubType] = append(result.Added[returnSubType], ids...)
			}
		}
	}

	// Apply PossibleNot (blacklist)
	if cc.PossibleNot != nil {
		if typeRules, ok := cc.PossibleNot[returnType]; ok {
			if values, ok := typeRules[returnSubType]; ok {
				ids := s.resolveValuesToIDs(values, options)
				result.Denied[returnSubType] = append(result.Denied[returnSubType], ids...)
			}
		}
	}
}

// resolveValuesToIDs converts value strings to IDs.
// Values can be IDs (numeric) or names (matched against options).
func (s *Service) resolveValuesToIDs(values []string, options map[int]string) []int {
	var ids []int

	for _, val := range values {
		// Try numeric ID first
		if id, err := strconv.Atoi(val); err == nil {
			ids = append(ids, id)
			continue
		}

		// Match by name (with wildcard support)
		for id, name := range options {
			if s.valueMatchesPattern(val, name) {
				ids = append(ids, id)
			}
		}
	}

	return ids
}

// applyResult applies the accumulated ACL result to filter options.
func (s *Service) applyResult(options map[int]string, returnType, returnSubType string, result *models.ACLResult) map[int]string {
	if len(result.MatchedACLs) == 0 {
		return options
	}

	filtered := make(map[int]string)

	// Start with all options if no Possible rule, or with Possible whitelist
	if allowed, ok := result.Allowed[returnSubType]; ok && len(allowed) > 0 {
		// Only include allowed IDs
		for _, id := range allowed {
			if name, exists := options[id]; exists {
				filtered[id] = name
			}
		}
	} else {
		// No whitelist, start with all options
		for id, name := range options {
			filtered[id] = name
		}
	}

	// Add any PossibleAdd items
	if added, ok := result.Added[returnSubType]; ok {
		for _, id := range added {
			if name, exists := options[id]; exists {
				filtered[id] = name
			}
		}
	}

	// Remove any PossibleNot items
	if denied, ok := result.Denied[returnSubType]; ok {
		for _, id := range denied {
			delete(filtered, id)
		}
	}

	return filtered
}

// intersectIDs returns the intersection of two ID slices.
func intersectIDs(a, b []int) []int {
	set := make(map[int]bool)
	for _, id := range a {
		set[id] = true
	}

	var result []int
	for _, id := range b {
		if set[id] {
			result = append(result, id)
		}
	}
	return result
}
