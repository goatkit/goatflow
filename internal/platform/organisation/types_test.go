package organisation

import (
	"context"
	"testing"
)

func TestValidStatuses(t *testing.T) {
	statuses := ValidStatuses()
	if len(statuses) != 3 {
		t.Errorf("expected 3 statuses, got %d", len(statuses))
	}
	for _, s := range statuses {
		if !IsValidStatus(s) {
			t.Errorf("%q should be valid", s)
		}
	}
	if IsValidStatus("bogus") {
		t.Error("bogus should be invalid")
	}
}

func TestValidRoles(t *testing.T) {
	roles := ValidRoles()
	if len(roles) != 3 {
		t.Errorf("expected 3 roles, got %d", len(roles))
	}
	for _, r := range roles {
		if !IsValidRole(r) {
			t.Errorf("%q should be valid", r)
		}
	}
	if IsValidRole("bogus") {
		t.Error("bogus should be invalid")
	}
}

func TestOrganisation_IsActive(t *testing.T) {
	tests := []struct {
		name string
		org  Organisation
		want bool
	}{
		{"active and valid", Organisation{ValidID: 1, Status: StatusActive}, true},
		{"suspended", Organisation{ValidID: 1, Status: StatusSuspended}, false},
		{"archived", Organisation{ValidID: 1, Status: StatusArchived}, false},
		{"invalid valid_id", Organisation{ValidID: 2, Status: StatusActive}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.org.IsActive(); got != tt.want {
				t.Errorf("IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOrgContext(t *testing.T) {
	t.Run("no org context returns 0", func(t *testing.T) {
		ctx := context.Background()
		if id := OrgIDFromContext(ctx); id != 0 {
			t.Errorf("expected 0, got %d", id)
		}
		if HasOrgContext(ctx) {
			t.Error("should not have org context")
		}
	})

	t.Run("with org context", func(t *testing.T) {
		ctx := WithOrgID(context.Background(), 42)
		if id := OrgIDFromContext(ctx); id != 42 {
			t.Errorf("expected 42, got %d", id)
		}
		if !HasOrgContext(ctx) {
			t.Error("should have org context")
		}
	})

	t.Run("nested context preserves org", func(t *testing.T) {
		ctx := WithOrgID(context.Background(), 99)
		ctx = context.WithValue(ctx, "other_key", "other_value")
		if id := OrgIDFromContext(ctx); id != 99 {
			t.Errorf("expected 99, got %d", id)
		}
	})
}
