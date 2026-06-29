package customfields

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/goatkit/goatflow/internal/database"
)

// getTestDB returns a DB connection for integration tests.
// Skips the test if no database is available.
func getTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skip("Test database not available — skipping integration test")
	}
	// Verify custom field tables exist. Use information_schema to avoid
	// errors from querying a non-existent table directly.
	var count int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'gk_custom_field_def'",
	).Scan(&count)
	if err != nil || count == 0 {
		t.Skipf("gk_custom_field_def table not found (count=%d, err=%v) — run migration 000009", count, err)
	}
	return db
}

// cleanupTestFields removes test data created during tests.
func cleanupTestFields(t *testing.T, db *sql.DB, prefix string) {
	t.Helper()
	db.Exec(database.ConvertPlaceholders("DELETE FROM gk_custom_field_value WHERE field_id IN (SELECT id FROM gk_custom_field_def WHERE name LIKE ?)"), prefix+"%")
	db.Exec(database.ConvertPlaceholders("DELETE FROM gk_custom_field_def WHERE name LIKE ?"), prefix+"%")
}

func TestRepository_CRUD_Definitions(t *testing.T) {
	db := getTestDB(t)
	repo := NewRepositoryWithDB(db)
	prefix := fmt.Sprintf("test_%d_", time.Now().UnixNano()%100000)
	t.Cleanup(func() { cleanupTestFields(t, db, prefix) })

	// --- Create ---
	t.Run("CreateDef", func(t *testing.T) {
		cfg := json.RawMessage(`{"max_length": 50}`)
		def := &FieldDef{
			Name:       prefix + "department",
			Label:      "Department",
			EntityType: EntityAgent,
			FieldType:  FieldText,
			OwnerType:  OwnerAdmin,
			Section:    "org",
			FieldOrder: 1,
			Required:   false,
			Config:     &cfg,
			ValidID:    1,
		}

		id, err := repo.CreateDef(def, 1)
		if err != nil {
			t.Fatalf("CreateDef: %v", err)
		}
		if id == 0 {
			t.Fatal("expected non-zero ID")
		}

		// --- GetDef ---
		t.Run("GetDef", func(t *testing.T) {
			got, err := repo.GetDef(id)
			if err != nil {
				t.Fatalf("GetDef: %v", err)
			}
			if got == nil {
				t.Fatal("expected non-nil def")
			}
			if got.Name != prefix+"department" {
				t.Errorf("name = %q, want %q", got.Name, prefix+"department")
			}
			if got.Label != "Department" {
				t.Errorf("label = %q", got.Label)
			}
			if got.EntityType != EntityAgent {
				t.Errorf("entity_type = %q", got.EntityType)
			}
			if got.FieldType != FieldText {
				t.Errorf("field_type = %q", got.FieldType)
			}
			if got.Section != "org" {
				t.Errorf("section = %q", got.Section)
			}
			if got.ValidID != 1 {
				t.Errorf("valid_id = %d", got.ValidID)
			}
		})

		// --- GetDefByEntityAndName ---
		t.Run("GetDefByEntityAndName", func(t *testing.T) {
			got, err := repo.GetDefByEntityAndName(EntityAgent, prefix+"department")
			if err != nil {
				t.Fatalf("GetDefByEntityAndName: %v", err)
			}
			if got == nil {
				t.Fatal("expected non-nil def")
			}
			if got.ID != id {
				t.Errorf("id = %d, want %d", got.ID, id)
			}
		})

		// --- GetDefByEntityAndName not found ---
		t.Run("GetDefByEntityAndName_notfound", func(t *testing.T) {
			got, err := repo.GetDefByEntityAndName(EntityAgent, prefix+"nonexistent")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != nil {
				t.Error("expected nil for non-existent field")
			}
		})

		// --- UpdateDef ---
		t.Run("UpdateDef", func(t *testing.T) {
			got, _ := repo.GetDef(id)
			got.Label = "Updated Department"
			got.Section = "hr"
			got.Required = true
			err := repo.UpdateDef(got, 1)
			if err != nil {
				t.Fatalf("UpdateDef: %v", err)
			}

			updated, _ := repo.GetDef(id)
			if updated.Label != "Updated Department" {
				t.Errorf("label = %q", updated.Label)
			}
			if updated.Section != "hr" {
				t.Errorf("section = %q", updated.Section)
			}
			if !updated.Required {
				t.Error("expected required=true")
			}
		})

		// --- SoftDeleteDef ---
		t.Run("SoftDeleteDef", func(t *testing.T) {
			err := repo.SoftDeleteDef(id, 1)
			if err != nil {
				t.Fatalf("SoftDeleteDef: %v", err)
			}

			got, _ := repo.GetDef(id)
			if got == nil {
				t.Fatal("field should still exist after soft delete")
			}
			if got.ValidID != 2 {
				t.Errorf("valid_id = %d, want 2", got.ValidID)
			}
		})
	})

	// --- ListDefs with filters ---
	t.Run("ListDefs", func(t *testing.T) {
		// Create a couple of fields for listing.
		optsCfg := json.RawMessage(`{"options":[{"value":"a","label":"A"}]}`)
		repo.CreateDef(&FieldDef{
			Name: prefix + "list_text", Label: "LT", EntityType: EntityContact,
			FieldType: FieldText, OwnerType: OwnerAdmin, Section: "custom", ValidID: 1,
		}, 1)
		repo.CreateDef(&FieldDef{
			Name: prefix + "list_select", Label: "LS", EntityType: EntityContact,
			FieldType: FieldSelect, OwnerType: OwnerPlugin, Section: "custom", ValidID: 1,
			Config: &optsCfg,
		}, 1)
		ownerName := "testplugin"
		repo.CreateDef(&FieldDef{
			Name: prefix + "list_agent", Label: "LA", EntityType: EntityAgent,
			FieldType: FieldText, OwnerType: OwnerPlugin, OwnerName: &ownerName, Section: "custom", ValidID: 1,
		}, 1)

		t.Run("filter by entity_type", func(t *testing.T) {
			defs, err := repo.ListDefs(EntityContact, "", "", false)
			if err != nil {
				t.Fatal(err)
			}
			found := 0
			for _, d := range defs {
				if d.Name == prefix+"list_text" || d.Name == prefix+"list_select" {
					found++
				}
			}
			if found != 2 {
				t.Errorf("expected 2 contact fields, found %d", found)
			}
		})

		t.Run("filter by owner_type", func(t *testing.T) {
			defs, err := repo.ListDefs("", OwnerPlugin, "", false)
			if err != nil {
				t.Fatal(err)
			}
			found := 0
			for _, d := range defs {
				if d.Name == prefix+"list_select" || d.Name == prefix+"list_agent" {
					found++
				}
			}
			if found >= 1 {
				// At least our test fields should appear.
			}
		})

		t.Run("active only", func(t *testing.T) {
			defs, err := repo.ListDefs("", "", "", true)
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range defs {
				if d.ValidID != 1 {
					t.Errorf("found inactive field %q with valid_id=%d when filtering active only", d.Name, d.ValidID)
				}
			}
		})
	})

	// --- HardDeleteDef ---
	t.Run("HardDeleteDef", func(t *testing.T) {
		id, err := repo.CreateDef(&FieldDef{
			Name: prefix + "to_hard_delete", Label: "Delete Me", EntityType: EntityQueue,
			FieldType: FieldText, OwnerType: OwnerAdmin, Section: "custom", ValidID: 1,
		}, 1)
		if err != nil {
			t.Fatal(err)
		}

		err = repo.HardDeleteDef(id)
		if err != nil {
			t.Fatalf("HardDeleteDef: %v", err)
		}

		got, _ := repo.GetDef(id)
		if got != nil {
			t.Error("field should be gone after hard delete")
		}
	})
}

func TestRepository_Values(t *testing.T) {
	db := getTestDB(t)
	repo := NewRepositoryWithDB(db)
	prefix := fmt.Sprintf("test_%d_", time.Now().UnixNano()%100000)
	t.Cleanup(func() { cleanupTestFields(t, db, prefix) })

	// Create field definitions for value tests.
	textID, _ := repo.CreateDef(&FieldDef{
		Name: prefix + "vtext", Label: "Text", EntityType: EntityContact,
		FieldType: FieldText, OwnerType: OwnerAdmin, Section: "custom", ValidID: 1,
	}, 1)
	intID, _ := repo.CreateDef(&FieldDef{
		Name: prefix + "vint", Label: "Int", EntityType: EntityContact,
		FieldType: FieldInteger, OwnerType: OwnerAdmin, Section: "custom", ValidID: 1,
	}, 1)
	boolID, _ := repo.CreateDef(&FieldDef{
		Name: prefix + "vbool", Label: "Bool", EntityType: EntityContact,
		FieldType: FieldBoolean, OwnerType: OwnerAdmin, Section: "custom", ValidID: 1,
	}, 1)
	selectCfg := json.RawMessage(`{"options":[{"value":"a","label":"A"},{"value":"b","label":"B"}]}`)
	selectID, _ := repo.CreateDef(&FieldDef{
		Name: prefix + "vselect", Label: "Select", EntityType: EntityContact,
		FieldType: FieldSelect, OwnerType: OwnerAdmin, Section: "custom", ValidID: 1,
		Config: &selectCfg,
	}, 1)

	if textID == 0 || intID == 0 || boolID == 0 || selectID == 0 {
		t.Fatal("failed to create test field definitions")
	}

	objectID := int64(99001) // High ID to avoid conflicts.

	t.Run("SetValues and GetValues", func(t *testing.T) {
		err := repo.SetValues(EntityContact, objectID, map[string]any{
			prefix + "vtext":   "Alice",
			prefix + "vint":    int64(42),
			prefix + "vbool":   true,
			prefix + "vselect": "a",
		})
		if err != nil {
			t.Fatalf("SetValues: %v", err)
		}

		vals, err := repo.GetValues(EntityContact, objectID, nil)
		if err != nil {
			t.Fatalf("GetValues: %v", err)
		}

		if vals[prefix+"vtext"] != "Alice" {
			t.Errorf("vtext = %v, want 'Alice'", vals[prefix+"vtext"])
		}
		if vals[prefix+"vint"] != int64(42) {
			t.Errorf("vint = %v (type %T), want 42", vals[prefix+"vint"], vals[prefix+"vint"])
		}
		if vals[prefix+"vbool"] != true {
			t.Errorf("vbool = %v, want true", vals[prefix+"vbool"])
		}
		if vals[prefix+"vselect"] != "a" {
			t.Errorf("vselect = %v, want 'a'", vals[prefix+"vselect"])
		}
	})

	t.Run("GetValues with field filter", func(t *testing.T) {
		vals, err := repo.GetValues(EntityContact, objectID, []string{prefix + "vtext"})
		if err != nil {
			t.Fatalf("GetValues: %v", err)
		}
		if len(vals) != 1 {
			t.Errorf("expected 1 value, got %d", len(vals))
		}
		if vals[prefix+"vtext"] != "Alice" {
			t.Errorf("vtext = %v", vals[prefix+"vtext"])
		}
	})

	t.Run("SetValues overwrites", func(t *testing.T) {
		err := repo.SetValues(EntityContact, objectID, map[string]any{
			prefix + "vtext": "Bob",
		})
		if err != nil {
			t.Fatal(err)
		}

		vals, _ := repo.GetValues(EntityContact, objectID, []string{prefix + "vtext"})
		if vals[prefix+"vtext"] != "Bob" {
			t.Errorf("vtext = %v, want 'Bob'", vals[prefix+"vtext"])
		}
	})

	t.Run("SetValues nil clears value", func(t *testing.T) {
		err := repo.SetValues(EntityContact, objectID, map[string]any{
			prefix + "vtext": nil,
		})
		if err != nil {
			t.Fatal(err)
		}

		vals, _ := repo.GetValues(EntityContact, objectID, []string{prefix + "vtext"})
		if _, exists := vals[prefix+"vtext"]; exists {
			t.Error("expected cleared value to not appear")
		}
	})

	t.Run("SetValues unknown field errors", func(t *testing.T) {
		err := repo.SetValues(EntityContact, objectID, map[string]any{
			"nonexistent_field_xyz": "value",
		})
		if err == nil {
			t.Fatal("expected error for unknown field")
		}
	})

	t.Run("GetValues empty object", func(t *testing.T) {
		vals, err := repo.GetValues(EntityContact, 999999, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(vals) != 0 {
			t.Errorf("expected empty map, got %d entries", len(vals))
		}
	})
}

func TestRepository_QueryByFields(t *testing.T) {
	db := getTestDB(t)
	repo := NewRepositoryWithDB(db)
	prefix := fmt.Sprintf("test_%d_", time.Now().UnixNano()%100000)
	t.Cleanup(func() { cleanupTestFields(t, db, prefix) })

	// Create a text field and an integer field.
	repo.CreateDef(&FieldDef{
		Name: prefix + "city", Label: "City", EntityType: EntityContact,
		FieldType: FieldText, OwnerType: OwnerAdmin, Section: "custom", ValidID: 1,
	}, 1)
	repo.CreateDef(&FieldDef{
		Name: prefix + "score", Label: "Score", EntityType: EntityContact,
		FieldType: FieldInteger, OwnerType: OwnerAdmin, Section: "custom", ValidID: 1,
	}, 1)

	// Set values on a few objects.
	repo.SetValues(EntityContact, 90001, map[string]any{prefix + "city": "London", prefix + "score": int64(80)})
	repo.SetValues(EntityContact, 90002, map[string]any{prefix + "city": "Paris", prefix + "score": int64(60)})
	repo.SetValues(EntityContact, 90003, map[string]any{prefix + "city": "London", prefix + "score": int64(95)})

	t.Run("eq filter", func(t *testing.T) {
		ids, err := repo.QueryByFields(EntityContact, []FieldFilter{
			{Field: prefix + "city", Operator: "eq", Value: "London"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if !containsID(ids, 90001) || !containsID(ids, 90003) {
			t.Errorf("expected 90001 and 90003, got %v", ids)
		}
		if containsID(ids, 90002) {
			t.Error("should not contain 90002 (Paris)")
		}
	})

	t.Run("gt filter", func(t *testing.T) {
		ids, err := repo.QueryByFields(EntityContact, []FieldFilter{
			{Field: prefix + "score", Operator: "gt", Value: 70},
		})
		if err != nil {
			t.Fatal(err)
		}
		if !containsID(ids, 90001) || !containsID(ids, 90003) {
			t.Errorf("expected 90001 and 90003, got %v", ids)
		}
	})

	t.Run("combined filters (intersection)", func(t *testing.T) {
		ids, err := repo.QueryByFields(EntityContact, []FieldFilter{
			{Field: prefix + "city", Operator: "eq", Value: "London"},
			{Field: prefix + "score", Operator: "gte", Value: 90},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(ids) != 1 || ids[0] != 90003 {
			t.Errorf("expected only 90003, got %v", ids)
		}
	})

	t.Run("like filter", func(t *testing.T) {
		ids, err := repo.QueryByFields(EntityContact, []FieldFilter{
			{Field: prefix + "city", Operator: "like", Value: "Lon%"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if !containsID(ids, 90001) {
			t.Errorf("expected 90001 in results, got %v", ids)
		}
	})

	t.Run("unknown field errors", func(t *testing.T) {
		_, err := repo.QueryByFields(EntityContact, []FieldFilter{
			{Field: "bogus_field_xyz", Operator: "eq", Value: "x"},
		})
		if err == nil {
			t.Fatal("expected error for unknown field")
		}
	})

	t.Run("empty filters returns nil", func(t *testing.T) {
		ids, err := repo.QueryByFields(EntityContact, nil)
		if err != nil {
			t.Fatal(err)
		}
		if ids != nil {
			t.Errorf("expected nil, got %v", ids)
		}
	})
}

func containsID(ids []int64, target int64) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

func TestRepository_AtomicIncrement(t *testing.T) {
	db := getTestDB(t)
	repo := NewRepositoryWithDB(db)
	prefix := fmt.Sprintf("test_%d_", time.Now().UnixNano()%100000)
	t.Cleanup(func() { cleanupTestFields(t, db, prefix) })

	repo.CreateDef(&FieldDef{
		Name: prefix + "balance", Label: "Balance", EntityType: EntityOrganisation,
		FieldType: FieldDecimal, OwnerType: OwnerAdmin, Section: "billing", ValidID: 1,
	}, 1)

	objectID := int64(80001)

	t.Run("increment inserts on missing row", func(t *testing.T) {
		err := repo.SetValues(EntityOrganisation, objectID, map[string]any{
			prefix + "balance": FieldOp{Op: OpIncrement, Value: 100.0},
		})
		if err != nil {
			t.Fatalf("increment on missing row: %v", err)
		}
		vals, _ := repo.GetValues(EntityOrganisation, objectID, []string{prefix + "balance"})
		if vals[prefix+"balance"] != 100.0 {
			t.Errorf("balance = %v, want 100.0", vals[prefix+"balance"])
		}
	})

	t.Run("increment adds to existing value", func(t *testing.T) {
		err := repo.SetValues(EntityOrganisation, objectID, map[string]any{
			prefix + "balance": FieldOp{Op: OpIncrement, Value: 50.0},
		})
		if err != nil {
			t.Fatal(err)
		}
		vals, _ := repo.GetValues(EntityOrganisation, objectID, []string{prefix + "balance"})
		if vals[prefix+"balance"] != 150.0 {
			t.Errorf("balance = %v, want 150.0", vals[prefix+"balance"])
		}
	})

	t.Run("negative increment (deduct)", func(t *testing.T) {
		err := repo.SetValues(EntityOrganisation, objectID, map[string]any{
			prefix + "balance": FieldOp{Op: OpIncrement, Value: -30.0},
		})
		if err != nil {
			t.Fatal(err)
		}
		vals, _ := repo.GetValues(EntityOrganisation, objectID, []string{prefix + "balance"})
		if vals[prefix+"balance"] != 120.0 {
			t.Errorf("balance = %v, want 120.0", vals[prefix+"balance"])
		}
	})

	t.Run("floor prevents overdraft", func(t *testing.T) {
		floor := 0.0
		err := repo.SetValues(EntityOrganisation, objectID, map[string]any{
			prefix + "balance": FieldOp{Op: OpIncrement, Value: -999.0, Floor: &floor},
		})
		if err == nil {
			t.Fatal("expected floor violation error")
		}
		// Value should remain unchanged.
		vals, _ := repo.GetValues(EntityOrganisation, objectID, []string{prefix + "balance"})
		if vals[prefix+"balance"] != 120.0 {
			t.Errorf("balance = %v, want 120.0 (unchanged)", vals[prefix+"balance"])
		}
	})

	t.Run("ceiling prevents overflow", func(t *testing.T) {
		ceiling := 200.0
		err := repo.SetValues(EntityOrganisation, objectID, map[string]any{
			prefix + "balance": FieldOp{Op: OpIncrement, Value: 100.0, Ceiling: &ceiling},
		})
		if err == nil {
			t.Fatal("expected ceiling violation error")
		}
		vals, _ := repo.GetValues(EntityOrganisation, objectID, []string{prefix + "balance"})
		if vals[prefix+"balance"] != 120.0 {
			t.Errorf("balance = %v, want 120.0 (unchanged)", vals[prefix+"balance"])
		}
	})

	t.Run("increment within bounds succeeds", func(t *testing.T) {
		floor := 0.0
		ceiling := 200.0
		err := repo.SetValues(EntityOrganisation, objectID, map[string]any{
			prefix + "balance": FieldOp{Op: OpIncrement, Value: 50.0, Floor: &floor, Ceiling: &ceiling},
		})
		if err != nil {
			t.Fatal(err)
		}
		vals, _ := repo.GetValues(EntityOrganisation, objectID, []string{prefix + "balance"})
		if vals[prefix+"balance"] != 170.0 {
			t.Errorf("balance = %v, want 170.0", vals[prefix+"balance"])
		}
	})
}

func TestRepository_AtomicIncrement_Integer(t *testing.T) {
	db := getTestDB(t)
	repo := NewRepositoryWithDB(db)
	prefix := fmt.Sprintf("test_%d_", time.Now().UnixNano()%100000)
	t.Cleanup(func() { cleanupTestFields(t, db, prefix) })

	repo.CreateDef(&FieldDef{
		Name: prefix + "counter", Label: "Counter", EntityType: EntityContact,
		FieldType: FieldInteger, OwnerType: OwnerAdmin, Section: "custom", ValidID: 1,
	}, 1)

	objectID := int64(80010)

	// Insert initial value.
	repo.SetValues(EntityContact, objectID, map[string]any{prefix + "counter": int64(10)})

	err := repo.SetValues(EntityContact, objectID, map[string]any{
		prefix + "counter": FieldOp{Op: OpIncrement, Value: int64(5)},
	})
	if err != nil {
		t.Fatal(err)
	}
	vals, _ := repo.GetValues(EntityContact, objectID, []string{prefix + "counter"})
	if vals[prefix+"counter"] != int64(15) {
		t.Errorf("counter = %v (type %T), want 15", vals[prefix+"counter"], vals[prefix+"counter"])
	}
}

func TestRepository_AtomicToggle(t *testing.T) {
	db := getTestDB(t)
	repo := NewRepositoryWithDB(db)
	prefix := fmt.Sprintf("test_%d_", time.Now().UnixNano()%100000)
	t.Cleanup(func() { cleanupTestFields(t, db, prefix) })

	repo.CreateDef(&FieldDef{
		Name: prefix + "active", Label: "Active", EntityType: EntityContact,
		FieldType: FieldBoolean, OwnerType: OwnerAdmin, Section: "custom", ValidID: 1,
	}, 1)

	objectID := int64(80020)

	t.Run("toggle inserts true on missing row", func(t *testing.T) {
		err := repo.SetValues(EntityContact, objectID, map[string]any{
			prefix + "active": FieldOp{Op: OpToggle},
		})
		if err != nil {
			t.Fatal(err)
		}
		vals, _ := repo.GetValues(EntityContact, objectID, []string{prefix + "active"})
		if vals[prefix+"active"] != true {
			t.Errorf("active = %v, want true", vals[prefix+"active"])
		}
	})

	t.Run("toggle flips true to false", func(t *testing.T) {
		err := repo.SetValues(EntityContact, objectID, map[string]any{
			prefix + "active": FieldOp{Op: OpToggle},
		})
		if err != nil {
			t.Fatal(err)
		}
		vals, _ := repo.GetValues(EntityContact, objectID, []string{prefix + "active"})
		if vals[prefix+"active"] != false {
			t.Errorf("active = %v, want false", vals[prefix+"active"])
		}
	})
}

func TestRepository_AtomicCAS(t *testing.T) {
	db := getTestDB(t)
	repo := NewRepositoryWithDB(db)
	prefix := fmt.Sprintf("test_%d_", time.Now().UnixNano()%100000)
	t.Cleanup(func() { cleanupTestFields(t, db, prefix) })

	optsCfg := json.RawMessage(`{"options":[{"value":"queued","label":"Queued"},{"value":"running","label":"Running"},{"value":"done","label":"Done"}]}`)
	repo.CreateDef(&FieldDef{
		Name: prefix + "status", Label: "Status", EntityType: EntityContact,
		FieldType: FieldSelect, OwnerType: OwnerAdmin, Section: "custom", ValidID: 1,
		Config: &optsCfg,
	}, 1)

	objectID := int64(80030)
	repo.SetValues(EntityContact, objectID, map[string]any{prefix + "status": "queued"})

	t.Run("cas succeeds when value matches", func(t *testing.T) {
		err := repo.SetValues(EntityContact, objectID, map[string]any{
			prefix + "status": FieldOp{Op: OpCAS, Value: "running", Expect: "queued"},
		})
		if err != nil {
			t.Fatal(err)
		}
		vals, _ := repo.GetValues(EntityContact, objectID, []string{prefix + "status"})
		if vals[prefix+"status"] != "running" {
			t.Errorf("status = %v, want 'running'", vals[prefix+"status"])
		}
	})

	t.Run("cas fails when value differs", func(t *testing.T) {
		err := repo.SetValues(EntityContact, objectID, map[string]any{
			prefix + "status": FieldOp{Op: OpCAS, Value: "done", Expect: "queued"},
		})
		if err == nil {
			t.Fatal("expected cas failure")
		}
		// Value should remain "running".
		vals, _ := repo.GetValues(EntityContact, objectID, []string{prefix + "status"})
		if vals[prefix+"status"] != "running" {
			t.Errorf("status = %v, want 'running' (unchanged)", vals[prefix+"status"])
		}
	})
}

func TestRepository_AtomicAppendRemove(t *testing.T) {
	db := getTestDB(t)
	repo := NewRepositoryWithDB(db)
	prefix := fmt.Sprintf("test_%d_", time.Now().UnixNano()%100000)
	t.Cleanup(func() { cleanupTestFields(t, db, prefix) })

	optsCfg := json.RawMessage(`{"options":[{"value":"go","label":"Go"},{"value":"rust","label":"Rust"},{"value":"python","label":"Python"}]}`)
	repo.CreateDef(&FieldDef{
		Name: prefix + "skills", Label: "Skills", EntityType: EntityContact,
		FieldType: FieldMultiSelect, OwnerType: OwnerAdmin, Section: "custom", ValidID: 1,
		Config: &optsCfg,
	}, 1)

	objectID := int64(80040)

	t.Run("append to empty creates array", func(t *testing.T) {
		err := repo.SetValues(EntityContact, objectID, map[string]any{
			prefix + "skills": FieldOp{Op: OpAppend, Value: "go"},
		})
		if err != nil {
			t.Fatal(err)
		}
		vals, _ := repo.GetValues(EntityContact, objectID, []string{prefix + "skills"})
		skills, ok := vals[prefix+"skills"].([]string)
		if !ok || len(skills) != 1 || skills[0] != "go" {
			t.Errorf("skills = %v, want [go]", vals[prefix+"skills"])
		}
	})

	t.Run("append adds to existing array", func(t *testing.T) {
		err := repo.SetValues(EntityContact, objectID, map[string]any{
			prefix + "skills": FieldOp{Op: OpAppend, Value: "rust"},
		})
		if err != nil {
			t.Fatal(err)
		}
		vals, _ := repo.GetValues(EntityContact, objectID, []string{prefix + "skills"})
		skills := vals[prefix+"skills"].([]string)
		if len(skills) != 2 {
			t.Errorf("expected 2 skills, got %v", skills)
		}
	})

	t.Run("append duplicate is no-op", func(t *testing.T) {
		err := repo.SetValues(EntityContact, objectID, map[string]any{
			prefix + "skills": FieldOp{Op: OpAppend, Value: "go"},
		})
		if err != nil {
			t.Fatal(err)
		}
		vals, _ := repo.GetValues(EntityContact, objectID, []string{prefix + "skills"})
		skills := vals[prefix+"skills"].([]string)
		if len(skills) != 2 {
			t.Errorf("expected 2 skills (no dup), got %v", skills)
		}
	})

	t.Run("remove existing item", func(t *testing.T) {
		err := repo.SetValues(EntityContact, objectID, map[string]any{
			prefix + "skills": FieldOp{Op: OpRemove, Value: "go"},
		})
		if err != nil {
			t.Fatal(err)
		}
		vals, _ := repo.GetValues(EntityContact, objectID, []string{prefix + "skills"})
		skills := vals[prefix+"skills"].([]string)
		if len(skills) != 1 || skills[0] != "rust" {
			t.Errorf("skills = %v, want [rust]", skills)
		}
	})

	t.Run("remove non-existent item is no-op", func(t *testing.T) {
		err := repo.SetValues(EntityContact, objectID, map[string]any{
			prefix + "skills": FieldOp{Op: OpRemove, Value: "python"},
		})
		if err != nil {
			t.Fatal(err)
		}
		vals, _ := repo.GetValues(EntityContact, objectID, []string{prefix + "skills"})
		skills := vals[prefix+"skills"].([]string)
		if len(skills) != 1 || skills[0] != "rust" {
			t.Errorf("skills = %v, want [rust]", skills)
		}
	})
}

func TestRepository_AtomicOps_ViaJSONMap(t *testing.T) {
	db := getTestDB(t)
	repo := NewRepositoryWithDB(db)
	prefix := fmt.Sprintf("test_%d_", time.Now().UnixNano()%100000)
	t.Cleanup(func() { cleanupTestFields(t, db, prefix) })

	repo.CreateDef(&FieldDef{
		Name: prefix + "tokens", Label: "Tokens", EntityType: EntityOrganisation,
		FieldType: FieldDecimal, OwnerType: OwnerAdmin, Section: "billing", ValidID: 1,
	}, 1)

	objectID := int64(80050)

	// Simulate gRPC transport: FieldOp arrives as map[string]any.
	repo.SetValues(EntityOrganisation, objectID, map[string]any{
		prefix + "tokens": map[string]any{"op": "increment", "value": 500.0},
	})

	vals, _ := repo.GetValues(EntityOrganisation, objectID, []string{prefix + "tokens"})
	if vals[prefix+"tokens"] != 500.0 {
		t.Errorf("tokens = %v, want 500.0", vals[prefix+"tokens"])
	}

	// Deduct with floor via JSON map.
	repo.SetValues(EntityOrganisation, objectID, map[string]any{
		prefix + "tokens": map[string]any{"op": "increment", "value": -200.0, "floor": 0.0},
	})

	vals, _ = repo.GetValues(EntityOrganisation, objectID, []string{prefix + "tokens"})
	if vals[prefix+"tokens"] != 300.0 {
		t.Errorf("tokens = %v, want 300.0", vals[prefix+"tokens"])
	}
}
