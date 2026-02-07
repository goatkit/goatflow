package api

import (
	"testing"

	"github.com/goatkit/goatflow/internal/models"
)

func TestMakeAdminGroupEntryIncludesComments(t *testing.T) {
	group := &models.Group{
		ID:       uint(42),
		Name:     "alpha",
		Comments: "Primary support team",
		ValidID:  1,
	}

	entry := makeAdminGroupEntry(group, 7)

	if got := entry["Comments"]; got != group.Comments {
		t.Fatalf("expected Comments %q, got %v", group.Comments, got)
	}

	if got := entry["Description"]; got != group.Comments {
		t.Fatalf("expected Description %q, got %v", group.Comments, got)
	}
}
