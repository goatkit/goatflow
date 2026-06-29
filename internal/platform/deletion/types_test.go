package deletion

import (
	"testing"
)

func TestValidActions(t *testing.T) {
	actions := ValidActions()
	if len(actions) != 3 {
		t.Errorf("expected 3 actions, got %d", len(actions))
	}
	for _, a := range actions {
		if !IsValidAction(a) {
			t.Errorf("%q should be valid", a)
		}
	}
	if IsValidAction("bogus") {
		t.Error("bogus should be invalid")
	}
}

func TestAnonymiseConfig(t *testing.T) {
	// Verify ticket has expected PII fields.
	ticketFields := AnonymiseConfig[EntityTicket]
	if len(ticketFields) != 3 {
		t.Errorf("expected 3 ticket PII fields, got %d", len(ticketFields))
	}

	// Verify contact has the most PII fields.
	contactFields := AnonymiseConfig[EntityContact]
	if len(contactFields) < 5 {
		t.Errorf("expected at least 5 contact PII fields, got %d", len(contactFields))
	}

	// Verify all fields have non-empty table and column.
	for entityType, fields := range AnonymiseConfig {
		for _, f := range fields {
			if f.Table == "" || f.Column == "" {
				t.Errorf("%s: field has empty table or column", entityType)
			}
			if f.Value == "" {
				t.Errorf("%s.%s: field has empty value", f.Table, f.Column)
			}
		}
	}
}

func TestRecycleBinEntry_ToJSON(t *testing.T) {
	name := "Test Ticket"
	e := &RecycleBinEntry{
		EntityType: EntityTicket,
		EntityID:   42,
		EntityName: &name,
		DeletedBy:  1,
	}
	m := e.ToJSON()
	if m["entity_type"] != EntityTicket {
		t.Errorf("entity_type = %v", m["entity_type"])
	}
	if m["entity_id"] != int64(42) {
		t.Errorf("entity_id = %v", m["entity_id"])
	}
	if m["entity_name"] != "Test Ticket" {
		t.Errorf("entity_name = %v", m["entity_name"])
	}
}

func TestRecycleBinToJSON(t *testing.T) {
	entries := []RecycleBinEntry{
		{EntityType: EntityTicket, EntityID: 1, DeletedBy: 1},
		{EntityType: EntityContact, EntityID: 2, DeletedBy: 1},
	}
	data, err := RecycleBinToJSON(entries)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty JSON")
	}
}
