package acl

import (
	"testing"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

func TestValueMatches(t *testing.T) {
	s := &Service{}

	tests := []struct {
		name     string
		expected string
		actual   string
		want     bool
	}{
		{"exact match", "open", "open", true},
		{"exact mismatch", "open", "closed", false},
		{"wildcard match", "open*", "open", true},
		{"wildcard match suffix", "open*", "open-ticket", true},
		{"wildcard match prefix", "*ticket", "open-ticket", true},
		{"wildcard match middle", "op*en", "open", true}, // op.*en matches "open" (op + "" + en)
		{"wildcard match all", "*", "anything", true},
		{"negation match", "[Not]open", "closed", true},
		{"negation mismatch", "[Not]open", "open", false},
		{"negation with wildcard", "[Not]open*", "closed", true},
		{"negation with wildcard mismatch", "[Not]open*", "open-ticket", false},
		{"regex match", "[RegExp]^open$", "open", true},
		{"regex mismatch", "[RegExp]^open$", "open-ticket", false},
		{"regex pattern", "[RegExp]open.*", "open-ticket", true},
		{"numeric match", "1", "1", true},
		{"numeric mismatch", "1", "2", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.valueMatches(tt.expected, tt.actual)
			if got != tt.want {
				t.Errorf("valueMatches(%q, %q) = %v, want %v", tt.expected, tt.actual, got, tt.want)
			}
		})
	}
}

func TestMatchesContext_NoConditions(t *testing.T) {
	s := &Service{}

	acl := &models.ACL{
		ID:          1,
		Name:        "Test ACL",
		ConfigMatch: nil, // No match conditions
	}

	ctx := &models.ACLContext{
		UserID:  1,
		QueueID: 1,
	}

	// ACL with no conditions should always match
	if !s.matchesContext(acl, ctx) {
		t.Error("ACL with no conditions should match all contexts")
	}
}

func TestMatchesContext_Properties(t *testing.T) {
	s := &Service{}

	tests := []struct {
		name    string
		acl     *models.ACL
		ctx     *models.ACLContext
		matches bool
	}{
		{
			name: "queue match",
			acl: &models.ACL{
				ID:   1,
				Name: "Queue ACL",
				ConfigMatch: &models.ACLConfigMatch{
					Properties: map[string]map[string][]string{
						"Ticket": {"QueueID": {"1", "2"}},
					},
				},
			},
			ctx: &models.ACLContext{
				QueueID: 1,
			},
			matches: true,
		},
		{
			name: "queue mismatch",
			acl: &models.ACL{
				ID:   1,
				Name: "Queue ACL",
				ConfigMatch: &models.ACLConfigMatch{
					Properties: map[string]map[string][]string{
						"Ticket": {"QueueID": {"1", "2"}},
					},
				},
			},
			ctx: &models.ACLContext{
				QueueID: 3,
			},
			matches: false,
		},
		{
			name: "state match",
			acl: &models.ACL{
				ID:   1,
				Name: "State ACL",
				ConfigMatch: &models.ACLConfigMatch{
					Properties: map[string]map[string][]string{
						"Ticket": {"StateID": {"1"}},
					},
				},
			},
			ctx: &models.ACLContext{
				StateID: 1,
			},
			matches: true,
		},
		{
			name: "form value match",
			acl: &models.ACL{
				ID:   1,
				Name: "Form ACL",
				ConfigMatch: &models.ACLConfigMatch{
					Properties: map[string]map[string][]string{
						"Ticket": {"StateID": {"2"}},
					},
				},
			},
			ctx: &models.ACLContext{
				StateID:     1,          // DB value
				FormStateID: intPtr(2),  // Form value
			},
			matches: true, // Properties uses form values
		},
		{
			name: "multiple conditions all match",
			acl: &models.ACL{
				ID:   1,
				Name: "Multi ACL",
				ConfigMatch: &models.ACLConfigMatch{
					Properties: map[string]map[string][]string{
						"Ticket": {
							"QueueID": {"1"},
							"StateID": {"1"},
						},
					},
				},
			},
			ctx: &models.ACLContext{
				QueueID: 1,
				StateID: 1,
			},
			matches: true,
		},
		{
			name: "multiple conditions partial match",
			acl: &models.ACL{
				ID:   1,
				Name: "Multi ACL",
				ConfigMatch: &models.ACLConfigMatch{
					Properties: map[string]map[string][]string{
						"Ticket": {
							"QueueID": {"1"},
							"StateID": {"2"},
						},
					},
				},
			},
			ctx: &models.ACLContext{
				QueueID: 1,
				StateID: 1, // StateID doesn't match
			},
			matches: false,
		},
		{
			name: "action match",
			acl: &models.ACL{
				ID:   1,
				Name: "Action ACL",
				ConfigMatch: &models.ACLConfigMatch{
					Properties: map[string]map[string][]string{
						"Frontend": {"Action": {"AgentTicketNote"}},
					},
				},
			},
			ctx: &models.ACLContext{
				Action: "AgentTicketNote",
			},
			matches: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.matchesContext(tt.acl, tt.ctx)
			if got != tt.matches {
				t.Errorf("matchesContext() = %v, want %v", got, tt.matches)
			}
		})
	}
}

func TestMatchesContext_PropertiesDatabase(t *testing.T) {
	s := &Service{}

	// PropertiesDatabase should use DB values, not form values
	acl := &models.ACL{
		ID:   1,
		Name: "DB ACL",
		ConfigMatch: &models.ACLConfigMatch{
			PropertiesDatabase: map[string]map[string][]string{
				"Ticket": {"StateID": {"1"}},
			},
		},
	}

	ctx := &models.ACLContext{
		StateID:     1,         // DB value
		FormStateID: intPtr(2), // Form value (different)
	}

	// Should match based on DB value (1), not form value (2)
	if !s.matchesContext(acl, ctx) {
		t.Error("PropertiesDatabase should match DB value, not form value")
	}

	// Now try with non-matching DB value
	ctx.StateID = 3
	if s.matchesContext(acl, ctx) {
		t.Error("PropertiesDatabase should not match when DB value differs")
	}
}

func TestApplyChanges(t *testing.T) {
	s := &Service{}

	options := map[int]string{
		1: "open",
		2: "closed",
		3: "pending",
		4: "new",
	}

	tests := []struct {
		name          string
		acl           *models.ACL
		returnType    string
		returnSubType string
		wantAllowed   []int
		wantDenied    []int
		wantAdded     []int
	}{
		{
			name: "possible whitelist",
			acl: &models.ACL{
				ID:   1,
				Name: "Whitelist ACL",
				ConfigChange: &models.ACLConfigChange{
					Possible: map[string]map[string][]string{
						"Ticket": {"State": {"1", "2"}},
					},
				},
			},
			returnType:    "Ticket",
			returnSubType: "State",
			wantAllowed:   []int{1, 2},
		},
		{
			name: "possible not blacklist",
			acl: &models.ACL{
				ID:   1,
				Name: "Blacklist ACL",
				ConfigChange: &models.ACLConfigChange{
					PossibleNot: map[string]map[string][]string{
						"Ticket": {"State": {"3", "4"}},
					},
				},
			},
			returnType:    "Ticket",
			returnSubType: "State",
			wantDenied:    []int{3, 4},
		},
		{
			name: "possible add",
			acl: &models.ACL{
				ID:   1,
				Name: "Add ACL",
				ConfigChange: &models.ACLConfigChange{
					PossibleAdd: map[string]map[string][]string{
						"Ticket": {"State": {"1"}},
					},
				},
			},
			returnType:    "Ticket",
			returnSubType: "State",
			wantAdded:     []int{1},
		},
		{
			name: "possible by name",
			acl: &models.ACL{
				ID:   1,
				Name: "Name ACL",
				ConfigChange: &models.ACLConfigChange{
					Possible: map[string]map[string][]string{
						"Ticket": {"State": {"open", "closed"}},
					},
				},
			},
			returnType:    "Ticket",
			returnSubType: "State",
			wantAllowed:   []int{1, 2}, // IDs of "open" and "closed"
		},
		{
			name: "possible with wildcard",
			acl: &models.ACL{
				ID:   1,
				Name: "Wildcard ACL",
				ConfigChange: &models.ACLConfigChange{
					Possible: map[string]map[string][]string{
						"Ticket": {"State": {"pend*"}},
					},
				},
			},
			returnType:    "Ticket",
			returnSubType: "State",
			wantAllowed:   []int{3}, // ID of "pending"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := models.NewACLResult()
			s.applyChanges(tt.acl, tt.returnType, tt.returnSubType, options, result)

			if len(tt.wantAllowed) > 0 {
				if !slicesEqual(result.Allowed[tt.returnSubType], tt.wantAllowed) {
					t.Errorf("Allowed = %v, want %v", result.Allowed[tt.returnSubType], tt.wantAllowed)
				}
			}

			if len(tt.wantDenied) > 0 {
				if !slicesEqual(result.Denied[tt.returnSubType], tt.wantDenied) {
					t.Errorf("Denied = %v, want %v", result.Denied[tt.returnSubType], tt.wantDenied)
				}
			}

			if len(tt.wantAdded) > 0 {
				if !slicesEqual(result.Added[tt.returnSubType], tt.wantAdded) {
					t.Errorf("Added = %v, want %v", result.Added[tt.returnSubType], tt.wantAdded)
				}
			}
		})
	}
}

func TestApplyResult(t *testing.T) {
	s := &Service{}

	options := map[int]string{
		1: "open",
		2: "closed",
		3: "pending",
		4: "new",
	}

	tests := []struct {
		name       string
		result     *models.ACLResult
		wantIDs    []int
		wantNotIDs []int
	}{
		{
			name: "no matches - return all",
			result: &models.ACLResult{
				MatchedACLs: []string{},
				Allowed:     make(map[string][]int),
				Denied:      make(map[string][]int),
				Added:       make(map[string][]int),
			},
			wantIDs: []int{1, 2, 3, 4},
		},
		{
			name: "whitelist only",
			result: &models.ACLResult{
				MatchedACLs: []string{"test"},
				Allowed:     map[string][]int{"State": {1, 2}},
				Denied:      make(map[string][]int),
				Added:       make(map[string][]int),
			},
			wantIDs:    []int{1, 2},
			wantNotIDs: []int{3, 4},
		},
		{
			name: "blacklist only",
			result: &models.ACLResult{
				MatchedACLs: []string{"test"},
				Allowed:     make(map[string][]int),
				Denied:      map[string][]int{"State": {3, 4}},
				Added:       make(map[string][]int),
			},
			wantIDs:    []int{1, 2},
			wantNotIDs: []int{3, 4},
		},
		{
			name: "whitelist plus add",
			result: &models.ACLResult{
				MatchedACLs: []string{"test"},
				Allowed:     map[string][]int{"State": {1}},
				Denied:      make(map[string][]int),
				Added:       map[string][]int{"State": {2}},
			},
			wantIDs:    []int{1, 2},
			wantNotIDs: []int{3, 4},
		},
		{
			name: "add then deny same ID",
			result: &models.ACLResult{
				MatchedACLs: []string{"test"},
				Allowed:     make(map[string][]int),
				Denied:      map[string][]int{"State": {2}},
				Added:       map[string][]int{"State": {2}}, // Add then deny
			},
			// Deny takes precedence
			wantIDs:    []int{1, 3, 4},
			wantNotIDs: []int{2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.applyResult(options, "Ticket", "State", tt.result)

			for _, id := range tt.wantIDs {
				if _, exists := got[id]; !exists {
					t.Errorf("Expected ID %d to be in result, but it's missing", id)
				}
			}

			for _, id := range tt.wantNotIDs {
				if _, exists := got[id]; exists {
					t.Errorf("Expected ID %d to NOT be in result, but it's present", id)
				}
			}
		})
	}
}

func TestFilterActions(t *testing.T) {
	s := &Service{}
	s.cachedACLs = []*models.ACL{
		{
			ID:      1,
			Name:    "Disable Close",
			ValidID: 1,
			ConfigMatch: &models.ACLConfigMatch{
				Properties: map[string]map[string][]string{
					"Ticket": {"StateID": {"1"}}, // Only for state 1
				},
			},
			ConfigChange: &models.ACLConfigChange{
				PossibleNot: map[string]map[string][]string{
					"Action": {"": {"AgentTicketClose"}},
				},
			},
		},
	}

	actions := []string{"AgentTicketNote", "AgentTicketClose", "AgentTicketPriority"}
	ctx := &models.ACLContext{
		StateID: 1, // Matches ACL
	}

	result, err := s.FilterActions(nil, ctx, actions)
	if err != nil {
		t.Fatalf("FilterActions error: %v", err)
	}

	// AgentTicketClose should be removed
	if containsString(result, "AgentTicketClose") {
		t.Error("AgentTicketClose should have been filtered out")
	}

	// Other actions should remain
	if !containsString(result, "AgentTicketNote") {
		t.Error("AgentTicketNote should still be present")
	}
	if !containsString(result, "AgentTicketPriority") {
		t.Error("AgentTicketPriority should still be present")
	}
}

func TestIntersectIDs(t *testing.T) {
	tests := []struct {
		name string
		a    []int
		b    []int
		want []int
	}{
		{"both empty", nil, nil, nil},
		{"a empty", nil, []int{1, 2}, nil},
		{"b empty", []int{1, 2}, nil, nil},
		{"no overlap", []int{1, 2}, []int{3, 4}, nil},
		{"full overlap", []int{1, 2}, []int{1, 2}, []int{1, 2}},
		{"partial overlap", []int{1, 2, 3}, []int{2, 3, 4}, []int{2, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := intersectIDs(tt.a, tt.b)
			if !slicesEqual(got, tt.want) {
				t.Errorf("intersectIDs(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// Helper functions

func intPtr(i int) *int {
	return &i
}

func slicesEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	// Create maps for comparison (order doesn't matter)
	am := make(map[int]bool)
	for _, v := range a {
		am[v] = true
	}
	for _, v := range b {
		if !am[v] {
			return false
		}
	}
	return true
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
