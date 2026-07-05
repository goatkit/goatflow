package customfields

import (
	"encoding/json"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// ValidateFieldDef
// ---------------------------------------------------------------------------

func TestValidateFieldDef(t *testing.T) {
	tests := []struct {
		name    string
		def     *FieldDef
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid text field",
			def:  &FieldDef{Name: "department", Label: "Department", EntityType: EntityAgent, FieldType: FieldText},
		},
		{
			name: "valid point field",
			def:  &FieldDef{Name: "hq_location", Label: "HQ", EntityType: EntityOrganisation, FieldType: FieldPoint},
		},
		{
			name: "valid select field with config",
			def: func() *FieldDef {
				cfg := json.RawMessage(`{"options":[{"value":"a","label":"A"}]}`)
				return &FieldDef{Name: "tier", Label: "Tier", EntityType: EntityContact, FieldType: FieldSelect, Config: &cfg}
			}(),
		},
		{
			name: "valid multi_select field with config",
			def: func() *FieldDef {
				cfg := json.RawMessage(`{"options":[{"value":"a","label":"A"},{"value":"b","label":"B"}]}`)
				return &FieldDef{Name: "tags", Label: "Tags", EntityType: EntityTicket, FieldType: FieldMultiSelect, Config: &cfg}
			}(),
		},
		{name: "missing name", def: &FieldDef{Label: "X", EntityType: EntityAgent, FieldType: FieldText}, wantErr: true, errMsg: "name is required"},
		{name: "missing label", def: &FieldDef{Name: "x", EntityType: EntityAgent, FieldType: FieldText}, wantErr: true, errMsg: "label is required"},
		{name: "invalid entity type", def: &FieldDef{Name: "x", Label: "X", EntityType: "bogus", FieldType: FieldText}, wantErr: true, errMsg: "invalid entity type"},
		{name: "invalid field type", def: &FieldDef{Name: "x", Label: "X", EntityType: EntityAgent, FieldType: "bogus"}, wantErr: true, errMsg: "invalid field type"},
		{name: "uppercase name", def: &FieldDef{Name: "Department", Label: "D", EntityType: EntityAgent, FieldType: FieldText}, wantErr: true, errMsg: "lowercase"},
		{name: "name with spaces", def: &FieldDef{Name: "my field", Label: "D", EntityType: EntityAgent, FieldType: FieldText}, wantErr: true, errMsg: "lowercase"},
		{name: "name starts with digit", def: &FieldDef{Name: "1abc", Label: "D", EntityType: EntityAgent, FieldType: FieldText}, wantErr: true, errMsg: "lowercase"},
		{name: "name with hyphens", def: &FieldDef{Name: "my-field", Label: "D", EntityType: EntityAgent, FieldType: FieldText}, wantErr: true, errMsg: "lowercase"},
		{
			name: "select without options fails",
			def: func() *FieldDef {
				cfg := json.RawMessage(`{}`)
				return &FieldDef{Name: "x", Label: "X", EntityType: EntityAgent, FieldType: FieldSelect, Config: &cfg}
			}(),
			wantErr: true, errMsg: "requires at least one option",
		},
		{
			name: "multi_select without options fails",
			def: func() *FieldDef {
				cfg := json.RawMessage(`{"options":[]}`)
				return &FieldDef{Name: "x", Label: "X", EntityType: EntityAgent, FieldType: FieldMultiSelect, Config: &cfg}
			}(),
			wantErr: true, errMsg: "requires at least one option",
		},
		{
			name: "text with invalid regex in config fails",
			def: func() *FieldDef {
				cfg := json.RawMessage(`{"regex":"[invalid"}`)
				return &FieldDef{Name: "x", Label: "X", EntityType: EntityAgent, FieldType: FieldText, Config: &cfg}
			}(),
			wantErr: true, errMsg: "invalid regex",
		},
		{
			name: "invalid config JSON fails",
			def: func() *FieldDef {
				cfg := json.RawMessage(`{not valid json}`)
				return &FieldDef{Name: "x", Label: "X", EntityType: EntityAgent, FieldType: FieldText, Config: &cfg}
			}(),
			wantErr: true, errMsg: "invalid config JSON",
		},
		{
			name: "nil config is fine",
			def:  &FieldDef{Name: "x", Label: "X", EntityType: EntityAgent, FieldType: FieldText, Config: nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFieldDef(tt.def)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ValidateValue — comprehensive per-type testing
// ---------------------------------------------------------------------------

func TestValidateValue(t *testing.T) {
	mkCfg := func(j string) *FieldDef {
		cfg := json.RawMessage(j)
		return &FieldDef{Config: &cfg}
	}

	tests := []struct {
		name    string
		def     *FieldDef
		val     any
		wantErr bool
		errMsg  string
	}{
		// --- required / nil ---
		{name: "nil on required", def: &FieldDef{FieldType: FieldText, Label: "X", Required: true}, val: nil, wantErr: true, errMsg: "required"},
		{name: "nil on optional", def: &FieldDef{FieldType: FieldText, Label: "X"}, val: nil},

		// --- text ---
		{name: "text valid", def: &FieldDef{FieldType: FieldText, Label: "X"}, val: "hello"},
		{name: "text wrong type", def: &FieldDef{FieldType: FieldText, Label: "X"}, val: 123, wantErr: true, errMsg: "expected string"},
		{
			name: "text max_length ok",
			def: func() *FieldDef {
				d := mkCfg(`{"max_length": 10}`)
				d.FieldType = FieldText
				d.Label = "X"
				return d
			}(),
			val: "short",
		},
		{
			name: "text max_length exceeded",
			def: func() *FieldDef {
				d := mkCfg(`{"max_length": 3}`)
				d.FieldType = FieldText
				d.Label = "X"
				return d
			}(),
			val: "toolong", wantErr: true, errMsg: "characters",
		},
		{
			name: "text regex match",
			def: func() *FieldDef {
				d := mkCfg(`{"regex":"^[0-9]{4}$"}`)
				d.FieldType = FieldText
				d.Label = "PIN"
				return d
			}(),
			val: "1234",
		},
		{
			name: "text regex no match uses custom error",
			def: func() *FieldDef {
				d := mkCfg(`{"regex":"^[A-Z]+$","regex_error":"uppercase only"}`)
				d.FieldType = FieldText
				d.Label = "Code"
				return d
			}(),
			val: "abc", wantErr: true, errMsg: "uppercase only",
		},
		{
			name: "text regex no match default error",
			def: func() *FieldDef {
				d := mkCfg(`{"regex":"^[0-9]+$"}`)
				d.FieldType = FieldText
				d.Label = "Num"
				return d
			}(),
			val: "abc", wantErr: true, errMsg: "invalid format",
		},

		// --- textarea ---
		{name: "textarea valid", def: &FieldDef{FieldType: FieldTextArea, Label: "X"}, val: "long text here"},
		{name: "textarea wrong type", def: &FieldDef{FieldType: FieldTextArea, Label: "X"}, val: 42, wantErr: true},

		// --- integer ---
		{name: "integer valid int64", def: &FieldDef{FieldType: FieldInteger, Label: "X"}, val: int64(42)},
		{name: "integer valid int", def: &FieldDef{FieldType: FieldInteger, Label: "X"}, val: 42},
		{name: "integer valid float64", def: &FieldDef{FieldType: FieldInteger, Label: "X"}, val: float64(42)},
		{name: "integer wrong type", def: &FieldDef{FieldType: FieldInteger, Label: "X"}, val: "notnum", wantErr: true},
		{
			name: "integer below min",
			def: func() *FieldDef {
				d := mkCfg(`{"min": 0, "max": 100}`)
				d.FieldType = FieldInteger
				d.Label = "Score"
				return d
			}(),
			val: int64(-1), wantErr: true, errMsg: "≥",
		},
		{
			name: "integer above max",
			def: func() *FieldDef {
				d := mkCfg(`{"min": 0, "max": 10}`)
				d.FieldType = FieldInteger
				d.Label = "Rating"
				return d
			}(),
			val: int64(11), wantErr: true, errMsg: "≤",
		},
		{
			name: "integer within range",
			def: func() *FieldDef {
				d := mkCfg(`{"min": 1, "max": 5}`)
				d.FieldType = FieldInteger
				d.Label = "Stars"
				return d
			}(),
			val: int64(3),
		},

		// --- decimal ---
		{name: "decimal valid", def: &FieldDef{FieldType: FieldDecimal, Label: "X"}, val: 3.14},
		{name: "decimal wrong type", def: &FieldDef{FieldType: FieldDecimal, Label: "X"}, val: "nope", wantErr: true},
		{
			name: "decimal below min",
			def: func() *FieldDef {
				d := mkCfg(`{"min": 0.0}`)
				d.FieldType = FieldDecimal
				d.Label = "Price"
				return d
			}(),
			val: -0.01, wantErr: true,
		},
		{
			name: "decimal above max",
			def: func() *FieldDef {
				d := mkCfg(`{"max": 999.99}`)
				d.FieldType = FieldDecimal
				d.Label = "Price"
				return d
			}(),
			val: 1000.00, wantErr: true,
		},

		// --- boolean ---
		{name: "boolean true", def: &FieldDef{FieldType: FieldBoolean, Label: "X"}, val: true},
		{name: "boolean false", def: &FieldDef{FieldType: FieldBoolean, Label: "X"}, val: false},
		{name: "boolean string 1", def: &FieldDef{FieldType: FieldBoolean, Label: "X"}, val: "1"},
		{name: "boolean string true", def: &FieldDef{FieldType: FieldBoolean, Label: "X"}, val: "true"},
		{name: "boolean string on", def: &FieldDef{FieldType: FieldBoolean, Label: "X"}, val: "on"},
		{name: "boolean int 0", def: &FieldDef{FieldType: FieldBoolean, Label: "X"}, val: 0},
		{name: "boolean wrong type", def: &FieldDef{FieldType: FieldBoolean, Label: "X"}, val: []string{}, wantErr: true},

		// --- date ---
		{name: "date valid", def: &FieldDef{FieldType: FieldDate, Label: "X"}, val: "2024-06-15"},
		{name: "date invalid format", def: &FieldDef{FieldType: FieldDate, Label: "X"}, val: "15/06/2024", wantErr: true},
		{name: "date wrong type", def: &FieldDef{FieldType: FieldDate, Label: "X"}, val: 123, wantErr: true},
		{
			name: "date before min_date",
			def: func() *FieldDef {
				d := mkCfg(`{"min_date":"2024-01-01"}`)
				d.FieldType = FieldDate
				d.Label = "Start"
				return d
			}(),
			val: "2023-12-31", wantErr: true, errMsg: "after",
		},
		{
			name: "date after max_date",
			def: func() *FieldDef {
				d := mkCfg(`{"max_date":"2025-12-31"}`)
				d.FieldType = FieldDate
				d.Label = "End"
				return d
			}(),
			val: "2026-01-01", wantErr: true, errMsg: "before",
		},

		// --- datetime ---
		{name: "datetime RFC3339", def: &FieldDef{FieldType: FieldDateTime, Label: "X"}, val: "2024-06-15T14:30:00Z"},
		{name: "datetime short form", def: &FieldDef{FieldType: FieldDateTime, Label: "X"}, val: "2024-06-15T14:30"},
		{name: "datetime long form", def: &FieldDef{FieldType: FieldDateTime, Label: "X"}, val: "2024-06-15 14:30:00"},
		{name: "datetime invalid", def: &FieldDef{FieldType: FieldDateTime, Label: "X"}, val: "not-a-datetime", wantErr: true},
		{name: "datetime wrong type", def: &FieldDef{FieldType: FieldDateTime, Label: "X"}, val: 123, wantErr: true},

		// --- select ---
		{
			name: "select valid option",
			def: func() *FieldDef {
				d := mkCfg(`{"options":[{"value":"a","label":"A"},{"value":"b","label":"B"}]}`)
				d.FieldType = FieldSelect
				d.Label = "Choice"
				return d
			}(),
			val: "a",
		},
		{
			name: "select invalid option",
			def: func() *FieldDef {
				d := mkCfg(`{"options":[{"value":"a","label":"A"}]}`)
				d.FieldType = FieldSelect
				d.Label = "Choice"
				return d
			}(),
			val: "z", wantErr: true, errMsg: "invalid option",
		},
		{
			name: "select empty allowed",
			def: func() *FieldDef {
				d := mkCfg(`{"options":[{"value":"a","label":"A"}],"allow_empty":true}`)
				d.FieldType = FieldSelect
				d.Label = "Choice"
				return d
			}(),
			val: "",
		},
		{
			name: "select wrong type",
			def: func() *FieldDef {
				d := mkCfg(`{"options":[{"value":"a","label":"A"}]}`)
				d.FieldType = FieldSelect
				d.Label = "X"
				return d
			}(),
			val: 42, wantErr: true, errMsg: "expected string",
		},

		// --- multi_select ---
		{
			name: "multi_select valid []string",
			def: func() *FieldDef {
				d := mkCfg(`{"options":[{"value":"a","label":"A"},{"value":"b","label":"B"},{"value":"c","label":"C"}]}`)
				d.FieldType = FieldMultiSelect
				d.Label = "Tags"
				return d
			}(),
			val: []string{"a", "b"},
		},
		{
			name: "multi_select valid []any",
			def: func() *FieldDef {
				d := mkCfg(`{"options":[{"value":"a","label":"A"},{"value":"b","label":"B"}]}`)
				d.FieldType = FieldMultiSelect
				d.Label = "Tags"
				return d
			}(),
			val: []any{"a", "b"},
		},
		{
			name: "multi_select invalid option",
			def: func() *FieldDef {
				d := mkCfg(`{"options":[{"value":"a","label":"A"}]}`)
				d.FieldType = FieldMultiSelect
				d.Label = "Tags"
				return d
			}(),
			val: []string{"a", "NOPE"}, wantErr: true, errMsg: "invalid option",
		},
		{
			name: "multi_select below min_selected",
			def: func() *FieldDef {
				d := mkCfg(`{"options":[{"value":"a","label":"A"},{"value":"b","label":"B"}],"min_selected":2}`)
				d.FieldType = FieldMultiSelect
				d.Label = "Tags"
				return d
			}(),
			val: []string{"a"}, wantErr: true, errMsg: "at least",
		},
		{
			name: "multi_select above max_selected",
			def: func() *FieldDef {
				d := mkCfg(`{"options":[{"value":"a","label":"A"},{"value":"b","label":"B"},{"value":"c","label":"C"}],"max_selected":1}`)
				d.FieldType = FieldMultiSelect
				d.Label = "Tags"
				return d
			}(),
			val: []string{"a", "b"}, wantErr: true, errMsg: "at most",
		},
		{
			name:    "multi_select wrong type",
			def:     &FieldDef{FieldType: FieldMultiSelect, Label: "X"},
			val:     "not-an-array",
			wantErr: true, errMsg: "expected array",
		},
		{
			name: "multi_select []any with non-string",
			def: func() *FieldDef {
				d := mkCfg(`{"options":[{"value":"a","label":"A"}]}`)
				d.FieldType = FieldMultiSelect
				d.Label = "Tags"
				return d
			}(),
			val: []any{123}, wantErr: true, errMsg: "strings",
		},

		// --- url ---
		{name: "url valid https", def: &FieldDef{FieldType: FieldURL, Label: "X"}, val: "https://example.com"},
		{name: "url valid http", def: &FieldDef{FieldType: FieldURL, Label: "X"}, val: "http://example.com/path"},
		{name: "url no host", def: &FieldDef{FieldType: FieldURL, Label: "X"}, val: "not-a-url", wantErr: true},
		{name: "url wrong type", def: &FieldDef{FieldType: FieldURL, Label: "X"}, val: 42, wantErr: true},
		{
			name: "url scheme restricted",
			def: func() *FieldDef {
				d := mkCfg(`{"allowed_schemes":["https"]}`)
				d.FieldType = FieldURL
				d.Label = "Web"
				return d
			}(),
			val: "http://insecure.com", wantErr: true, errMsg: "scheme",
		},
		{
			name: "url scheme allowed",
			def: func() *FieldDef {
				d := mkCfg(`{"allowed_schemes":["https"]}`)
				d.FieldType = FieldURL
				d.Label = "Web"
				return d
			}(),
			val: "https://secure.com",
		},

		// --- email ---
		{name: "email valid", def: &FieldDef{FieldType: FieldEmail, Label: "X"}, val: "test@example.com"},
		{name: "email invalid", def: &FieldDef{FieldType: FieldEmail, Label: "X"}, val: "not-an-email", wantErr: true},
		{name: "email wrong type", def: &FieldDef{FieldType: FieldEmail, Label: "X"}, val: 42, wantErr: true},

		// --- phone ---
		{name: "phone valid intl", def: &FieldDef{FieldType: FieldPhone, Label: "X"}, val: "+44 7700 900000"},
		{name: "phone valid parens", def: &FieldDef{FieldType: FieldPhone, Label: "X"}, val: "(020) 7946-0958"},
		{name: "phone too short", def: &FieldDef{FieldType: FieldPhone, Label: "X"}, val: "123", wantErr: true},
		{name: "phone letters", def: &FieldDef{FieldType: FieldPhone, Label: "X"}, val: "call me", wantErr: true},
		{name: "phone wrong type", def: &FieldDef{FieldType: FieldPhone, Label: "X"}, val: 12345, wantErr: true},

		// --- point ---
		{name: "point valid", def: &FieldDef{FieldType: FieldPoint, Label: "X"}, val: map[string]any{"lat": 51.5074, "lng": -0.1278}},
		{name: "point lat too high", def: &FieldDef{FieldType: FieldPoint, Label: "X"}, val: map[string]any{"lat": 91.0, "lng": 0.0}, wantErr: true, errMsg: "latitude"},
		{name: "point lat too low", def: &FieldDef{FieldType: FieldPoint, Label: "X"}, val: map[string]any{"lat": -91.0, "lng": 0.0}, wantErr: true, errMsg: "latitude"},
		{name: "point lng too high", def: &FieldDef{FieldType: FieldPoint, Label: "X"}, val: map[string]any{"lat": 0.0, "lng": 181.0}, wantErr: true, errMsg: "longitude"},
		{name: "point lng too low", def: &FieldDef{FieldType: FieldPoint, Label: "X"}, val: map[string]any{"lat": 0.0, "lng": -181.0}, wantErr: true, errMsg: "longitude"},
		{name: "point wrong type", def: &FieldDef{FieldType: FieldPoint, Label: "X"}, val: "not-a-point", wantErr: true},

		// --- polygon ---
		{
			name: "polygon valid",
			def:  &FieldDef{FieldType: FieldPolygon, Label: "X"},
			val:  map[string]any{"type": "Polygon", "coordinates": []any{[]any{[]any{0, 0}, []any{1, 0}, []any{1, 1}, []any{0, 0}}}},
		},
		{
			name: "polygon MultiPolygon valid",
			def:  &FieldDef{FieldType: FieldPolygon, Label: "X"},
			val:  map[string]any{"type": "MultiPolygon", "coordinates": []any{}},
		},
		{name: "polygon wrong type field", def: &FieldDef{FieldType: FieldPolygon, Label: "X"}, val: map[string]any{"type": "Point"}, wantErr: true, errMsg: "Polygon"},
		{name: "polygon no coordinates", def: &FieldDef{FieldType: FieldPolygon, Label: "X"}, val: map[string]any{"type": "Polygon"}, wantErr: true, errMsg: "coordinates"},
		{name: "polygon not an object", def: &FieldDef{FieldType: FieldPolygon, Label: "X"}, val: "not-geojson", wantErr: true},

		// --- address ---
		{
			name: "address valid",
			def:  &FieldDef{FieldType: FieldAddress, Label: "X"},
			val:  map[string]any{"line1": "123 High St", "city": "London", "country": "GB"},
		},
		{
			name: "address valid country restricted",
			def: func() *FieldDef {
				d := mkCfg(`{"countries":["GB","US"]}`)
				d.FieldType = FieldAddress
				d.Label = "Office"
				return d
			}(),
			val: map[string]any{"country": "GB"},
		},
		{
			name: "address invalid country",
			def: func() *FieldDef {
				d := mkCfg(`{"countries":["GB","US"]}`)
				d.FieldType = FieldAddress
				d.Label = "Office"
				return d
			}(),
			val: map[string]any{"country": "DE"}, wantErr: true, errMsg: "invalid country",
		},
		{
			name: "address empty country allowed when no restriction",
			def:  &FieldDef{FieldType: FieldAddress, Label: "X"},
			val:  map[string]any{"line1": "Somewhere"},
		},
		{name: "address wrong type", def: &FieldDef{FieldType: FieldAddress, Label: "X"}, val: "not-address", wantErr: true},

		// --- unsupported type ---
		{name: "unsupported field type", def: &FieldDef{FieldType: "mystery", Label: "X"}, val: "anything", wantErr: true, errMsg: "unsupported"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateValue(tt.def, tt.val)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ValidateValues — batch validation with required field checking
// ---------------------------------------------------------------------------

func TestValidateValues(t *testing.T) {
	optsCfg := json.RawMessage(`{"options":[{"value":"a","label":"A"},{"value":"b","label":"B"}]}`)

	defs := map[string]*FieldDef{
		"name": {Name: "name", Label: "Name", FieldType: FieldText, Required: true},
		"age":  {Name: "age", Label: "Age", FieldType: FieldInteger},
		"tier": {Name: "tier", Label: "Tier", FieldType: FieldSelect, Config: &optsCfg},
	}

	t.Run("all valid", func(t *testing.T) {
		errs := ValidateValues(defs, map[string]any{"name": "Alice", "age": int64(30), "tier": "a"})
		if errs != nil {
			t.Fatalf("expected no errors, got %v", errs)
		}
	})

	t.Run("missing required field", func(t *testing.T) {
		errs := ValidateValues(defs, map[string]any{"age": int64(30)})
		if errs == nil {
			t.Fatal("expected errors for missing required 'name'")
		}
		if _, ok := errs["name"]; !ok {
			t.Error("expected error on 'name' field")
		}
	})

	t.Run("invalid value type", func(t *testing.T) {
		errs := ValidateValues(defs, map[string]any{"name": "Bob", "age": "not-a-number"})
		if errs == nil {
			t.Fatal("expected errors")
		}
		if _, ok := errs["age"]; !ok {
			t.Error("expected error on 'age' field")
		}
	})

	t.Run("unknown field", func(t *testing.T) {
		errs := ValidateValues(defs, map[string]any{"name": "Bob", "bogus": "value"})
		if errs == nil {
			t.Fatal("expected errors for unknown field")
		}
		if _, ok := errs["bogus"]; !ok {
			t.Error("expected error on 'bogus' field")
		}
	})

	t.Run("empty values with no required", func(t *testing.T) {
		defsOpt := map[string]*FieldDef{
			"note": {Name: "note", Label: "Note", FieldType: FieldText},
		}
		errs := ValidateValues(defsOpt, map[string]any{})
		if errs != nil {
			t.Fatalf("expected no errors, got %v", errs)
		}
	})
}

// ---------------------------------------------------------------------------
// StorageColumn — exhaustive mapping
// ---------------------------------------------------------------------------

func TestStorageColumn(t *testing.T) {
	tests := []struct {
		fieldType string
		want      string
	}{
		{FieldText, "val_text"}, {FieldTextArea, "val_text"}, {FieldSelect, "val_text"},
		{FieldEmail, "val_text"}, {FieldPhone, "val_text"}, {FieldURL, "val_text"},
		{FieldInteger, "val_int"}, {FieldBoolean, "val_int"},
		{FieldDecimal, "val_decimal"},
		{FieldDate, "val_date"},
		{FieldDateTime, "val_datetime"},
		{FieldMultiSelect, "val_json"}, {FieldPoint, "val_json"},
		{FieldPolygon, "val_json"}, {FieldAddress, "val_json"},
		{"unknown", "val_text"}, // default fallback
	}
	for _, tt := range tests {
		t.Run(tt.fieldType, func(t *testing.T) {
			if got := StorageColumn(tt.fieldType); got != tt.want {
				t.Errorf("StorageColumn(%q) = %q, want %q", tt.fieldType, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Types — IsActive, ValidFilterOperators, ParsedConfig
// ---------------------------------------------------------------------------

func TestFieldDef_IsActive(t *testing.T) {
	active := FieldDef{ValidID: 1}
	if !active.IsActive() {
		t.Error("ValidID=1 should be active")
	}
	inactive := FieldDef{ValidID: 2}
	if inactive.IsActive() {
		t.Error("ValidID=2 should be inactive")
	}
}

func TestValidFilterOperators(t *testing.T) {
	ops := ValidFilterOperators()
	expected := []string{"eq", "neq", "gt", "lt", "gte", "lte", "like", "in", "between", "near"}
	if len(ops) != len(expected) {
		t.Fatalf("expected %d operators, got %d", len(expected), len(ops))
	}
	for i, op := range expected {
		if ops[i] != op {
			t.Errorf("operator[%d] = %q, want %q", i, ops[i], op)
		}
	}
}

func TestFieldDef_ParsedConfig(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		f := &FieldDef{}
		cfg, err := f.ParsedConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg == nil {
			t.Fatal("expected non-nil config")
		}
	})

	t.Run("valid config", func(t *testing.T) {
		raw := json.RawMessage(`{"max_length": 100, "searchable": true}`)
		f := &FieldDef{Config: &raw}
		cfg, err := f.ParsedConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.MaxLength == nil || *cfg.MaxLength != 100 {
			t.Error("expected max_length=100")
		}
		if !cfg.Searchable {
			t.Error("expected searchable=true")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		raw := json.RawMessage(`{bad}`)
		f := &FieldDef{Config: &raw}
		_, err := f.ParsedConfig()
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})
}

// ---------------------------------------------------------------------------
// Entity and field type validation
// ---------------------------------------------------------------------------

func TestIsValidEntityType(t *testing.T) {
	for _, et := range ValidEntityTypes() {
		if !IsValidEntityType(et) {
			t.Errorf("%q should be valid", et)
		}
	}
	if IsValidEntityType("bogus") {
		t.Error("bogus should be invalid")
	}
}

func TestIsValidFieldType(t *testing.T) {
	for _, ft := range ValidFieldTypes() {
		if !IsValidFieldType(ft) {
			t.Errorf("%q should be valid", ft)
		}
	}
	if IsValidFieldType("bogus") {
		t.Error("bogus should be invalid")
	}
}

// ---------------------------------------------------------------------------
// Type coercion helpers (exported via repository.go, tested here as internal)
// ---------------------------------------------------------------------------

func TestToInt64(t *testing.T) {
	tests := []struct {
		name    string
		val     any
		want    int64
		wantErr bool
	}{
		{name: "int", val: 42, want: 42},
		{name: "int64", val: int64(99), want: 99},
		{name: "float64", val: float64(7), want: 7},
		{name: "json.Number", val: json.Number("123"), want: 123},
		{name: "string fails", val: "nope", wantErr: true},
		{name: "nil fails", val: nil, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toInt64(tt.val)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name    string
		val     any
		want    float64
		wantErr bool
	}{
		{name: "float64", val: 3.14, want: 3.14},
		{name: "int", val: 42, want: 42.0},
		{name: "int64", val: int64(99), want: 99.0},
		{name: "json.Number", val: json.Number("2.718"), want: 2.718},
		{name: "string fails", val: "nope", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toFloat64(tt.val)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %f, want %f", got, tt.want)
			}
		})
	}
}

func TestToBool(t *testing.T) {
	tests := []struct {
		name    string
		val     any
		want    bool
		wantErr bool
	}{
		{name: "bool true", val: true, want: true},
		{name: "bool false", val: false, want: false},
		{name: "int 1", val: 1, want: true},
		{name: "int 0", val: 0, want: false},
		{name: "int64 1", val: int64(1), want: true},
		{name: "float64 1", val: float64(1), want: true},
		{name: "float64 0", val: float64(0), want: false},
		{name: "string true", val: "true", want: true},
		{name: "string 1", val: "1", want: true},
		{name: "string on", val: "on", want: true},
		{name: "string false", val: "false", want: false},
		{name: "string 0", val: "0", want: false},
		{name: "slice fails", val: []int{}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toBool(tt.val)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToDate(t *testing.T) {
	t.Run("valid string", func(t *testing.T) {
		d, err := toDate("2024-03-15")
		if err != nil {
			t.Fatal(err)
		}
		if d.Year() != 2024 || d.Month() != 3 || d.Day() != 15 {
			t.Errorf("unexpected date: %v", d)
		}
	})
	t.Run("invalid string", func(t *testing.T) {
		_, err := toDate("not-a-date")
		if err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("wrong type", func(t *testing.T) {
		_, err := toDate(42)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestToDateTime(t *testing.T) {
	t.Run("RFC3339", func(t *testing.T) {
		d, err := toDateTime("2024-03-15T10:30:00Z")
		if err != nil {
			t.Fatal(err)
		}
		if d.Hour() != 10 || d.Minute() != 30 {
			t.Errorf("unexpected time: %v", d)
		}
	})
	t.Run("short form", func(t *testing.T) {
		d, err := toDateTime("2024-03-15T10:30")
		if err != nil {
			t.Fatal(err)
		}
		if d.Hour() != 10 {
			t.Errorf("unexpected time: %v", d)
		}
	})
	t.Run("long form", func(t *testing.T) {
		d, err := toDateTime("2024-03-15 10:30:00")
		if err != nil {
			t.Fatal(err)
		}
		if d.Hour() != 10 {
			t.Errorf("unexpected time: %v", d)
		}
	})
	t.Run("invalid", func(t *testing.T) {
		_, err := toDateTime("nope")
		if err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("wrong type", func(t *testing.T) {
		_, err := toDateTime(42)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

// ---------------------------------------------------------------------------
// Legacy migration helpers
// ---------------------------------------------------------------------------

func TestToLowerSnake(t *testing.T) {
	tests := []struct{ input, want string }{
		{"DeviceIMEI", "device_i_m_e_i"},
		{"CustomerName", "customer_name"},
		{"simple", "simple"},
		{"ABC", "a_b_c"},
		{"firstName", "first_name"},
		{"HTMLParser", "h_t_m_l_parser"},
		{"a", "a"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := toLowerSnake(tt.input); got != tt.want {
				t.Errorf("toLowerSnake(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestConvertLegacyConfig(t *testing.T) {
	t.Run("empty bytes", func(t *testing.T) {
		result, err := convertLegacyConfig("Text", nil)
		if err != nil {
			t.Fatal(err)
		}
		if result != nil {
			t.Error("expected nil for empty config")
		}
	})

	t.Run("text with max_length", func(t *testing.T) {
		yaml := []byte("MaxLength: 200")
		result, err := convertLegacyConfig("Text", yaml)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		var cfg FieldConfig
		json.Unmarshal(*result, &cfg)
		if cfg.MaxLength == nil || *cfg.MaxLength != 200 {
			t.Error("expected max_length=200")
		}
	})

	t.Run("dropdown with possible values", func(t *testing.T) {
		yaml := []byte("PossibleValues:\n  low: Low\n  high: High")
		result, err := convertLegacyConfig("Dropdown", yaml)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		var cfg FieldConfig
		json.Unmarshal(*result, &cfg)
		if len(cfg.Options) != 2 {
			t.Errorf("expected 2 options, got %d", len(cfg.Options))
		}
	})

	t.Run("multiselect with possible values", func(t *testing.T) {
		yaml := []byte("PossibleValues:\n  a: Alpha\n  b: Beta")
		result, err := convertLegacyConfig("Multiselect", yaml)
		if err != nil {
			t.Fatal(err)
		}
		var cfg FieldConfig
		json.Unmarshal(*result, &cfg)
		if len(cfg.Options) != 2 {
			t.Errorf("expected 2 options, got %d", len(cfg.Options))
		}
	})

	t.Run("textarea returns nil config", func(t *testing.T) {
		yaml := []byte("Rows: 5")
		result, err := convertLegacyConfig("TextArea", yaml)
		if err != nil {
			t.Fatal(err)
		}
		// TextArea has no mapped config fields, so result should be nil (empty config)
		if result != nil {
			t.Errorf("expected nil for textarea with no mappable config, got %s", string(*result))
		}
	})

	t.Run("checkbox returns nil config", func(t *testing.T) {
		yaml := []byte("DefaultValue: 0")
		result, err := convertLegacyConfig("Checkbox", yaml)
		if err != nil {
			t.Fatal(err)
		}
		if result != nil {
			t.Errorf("expected nil, got %s", string(*result))
		}
	})

	t.Run("invalid yaml", func(t *testing.T) {
		_, err := convertLegacyConfig("Text", []byte(":::invalid"))
		if err == nil {
			t.Fatal("expected error for invalid yaml")
		}
	})

	t.Run("webservice dropdown flattens options", func(t *testing.T) {
		yaml := []byte("PossibleValues:\n  id1: Name1")
		result, err := convertLegacyConfig("WebserviceDropdown", yaml)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		var cfg FieldConfig
		json.Unmarshal(*result, &cfg)
		if len(cfg.Options) != 1 {
			t.Errorf("expected 1 option, got %d", len(cfg.Options))
		}
	})
}

// ---------------------------------------------------------------------------
// assignValue + extractValue round-trip (repository pure functions)
// ---------------------------------------------------------------------------

func TestAssignAndExtractValue(t *testing.T) {
	tests := []struct {
		name      string
		fieldType string
		input     any
		checkFn   func(t *testing.T, fv *FieldValue, extracted any)
	}{
		{
			name: "text round-trip", fieldType: FieldText, input: "hello",
			checkFn: func(t *testing.T, fv *FieldValue, extracted any) {
				if fv.ValText == nil || *fv.ValText != "hello" {
					t.Error("val_text should be 'hello'")
				}
				if extracted != "hello" {
					t.Errorf("extracted = %v, want 'hello'", extracted)
				}
			},
		},
		{
			name: "integer round-trip", fieldType: FieldInteger, input: int64(42),
			checkFn: func(t *testing.T, fv *FieldValue, extracted any) {
				if fv.ValInt == nil || *fv.ValInt != 42 {
					t.Error("val_int should be 42")
				}
				if extracted != int64(42) {
					t.Errorf("extracted = %v", extracted)
				}
			},
		},
		{
			name: "boolean true round-trip", fieldType: FieldBoolean, input: true,
			checkFn: func(t *testing.T, fv *FieldValue, extracted any) {
				if fv.ValInt == nil || *fv.ValInt != 1 {
					t.Error("val_int should be 1")
				}
				if extracted != true {
					t.Errorf("extracted = %v", extracted)
				}
			},
		},
		{
			name: "boolean false round-trip", fieldType: FieldBoolean, input: false,
			checkFn: func(t *testing.T, fv *FieldValue, extracted any) {
				if fv.ValInt == nil || *fv.ValInt != 0 {
					t.Error("val_int should be 0")
				}
				if extracted != false {
					t.Errorf("extracted = %v", extracted)
				}
			},
		},
		{
			name: "decimal round-trip", fieldType: FieldDecimal, input: 3.14,
			checkFn: func(t *testing.T, fv *FieldValue, extracted any) {
				if fv.ValDecimal == nil || *fv.ValDecimal != 3.14 {
					t.Error("val_decimal should be 3.14")
				}
			},
		},
		{
			name: "date round-trip", fieldType: FieldDate, input: "2024-06-15",
			checkFn: func(t *testing.T, fv *FieldValue, extracted any) {
				if fv.ValDate == nil {
					t.Fatal("val_date should not be nil")
				}
				if fv.ValDate.Year() != 2024 || fv.ValDate.Month() != 6 {
					t.Error("date mismatch")
				}
				if s, ok := extracted.(string); !ok || s != "2024-06-15" {
					t.Errorf("extracted = %v", extracted)
				}
			},
		},
		{
			name: "datetime round-trip", fieldType: FieldDateTime, input: "2024-06-15T10:30:00Z",
			checkFn: func(t *testing.T, fv *FieldValue, extracted any) {
				if fv.ValDatetime == nil {
					t.Fatal("val_datetime should not be nil")
				}
			},
		},
		{
			name: "point stores lat/lng in decimal columns", fieldType: FieldPoint,
			input: map[string]any{"lat": 51.5, "lng": -0.1},
			checkFn: func(t *testing.T, fv *FieldValue, extracted any) {
				if fv.ValJSON == nil {
					t.Fatal("val_json should not be nil")
				}
				if fv.ValDecimal == nil || *fv.ValDecimal != 51.5 {
					t.Error("lat not stored in val_decimal")
				}
				if fv.ValDecimal2 == nil || *fv.ValDecimal2 != -0.1 {
					t.Error("lng not stored in val_decimal2")
				}
				ll, ok := extracted.(LatLng)
				if !ok {
					t.Fatalf("expected LatLng, got %T", extracted)
				}
				if ll.Lat != 51.5 || ll.Lng != -0.1 {
					t.Error("lat/lng mismatch in extracted")
				}
			},
		},
		{
			name: "address extracts postcode and lat/lng", fieldType: FieldAddress,
			input: map[string]any{"line1": "123 High St", "postcode": "EC1A 1BB", "lat": 51.5, "lng": -0.1},
			checkFn: func(t *testing.T, fv *FieldValue, extracted any) {
				if fv.ValJSON == nil {
					t.Fatal("val_json should not be nil")
				}
				if fv.ValText == nil || *fv.ValText != "EC1A 1BB" {
					t.Error("postcode not stored in val_text")
				}
				if fv.ValDecimal == nil || *fv.ValDecimal != 51.5 {
					t.Error("lat not stored in val_decimal")
				}
				addr, ok := extracted.(Address)
				if !ok {
					t.Fatalf("expected Address, got %T", extracted)
				}
				if addr.Postcode != "EC1A 1BB" {
					t.Errorf("postcode = %q", addr.Postcode)
				}
			},
		},
		{
			name: "multi_select round-trip", fieldType: FieldMultiSelect,
			input: []string{"a", "b"},
			checkFn: func(t *testing.T, fv *FieldValue, extracted any) {
				if fv.ValJSON == nil {
					t.Fatal("val_json should not be nil")
				}
				arr, ok := extracted.([]string)
				if !ok {
					t.Fatalf("expected []string, got %T", extracted)
				}
				if len(arr) != 2 || arr[0] != "a" || arr[1] != "b" {
					t.Errorf("unexpected values: %v", arr)
				}
			},
		},
		{
			name: "email round-trip", fieldType: FieldEmail, input: "test@example.com",
			checkFn: func(t *testing.T, fv *FieldValue, extracted any) {
				if fv.ValText == nil || *fv.ValText != "test@example.com" {
					t.Error("val_text mismatch")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fv := &FieldValue{FieldID: 1, ObjectID: 100}
			err := assignValue(fv, tt.fieldType, tt.input)
			if err != nil {
				t.Fatalf("assignValue: %v", err)
			}

			extracted := extractValue(tt.fieldType, fv.ValText, fv.ValInt, fv.ValDecimal, fv.ValDecimal2, fv.ValDate, fv.ValDatetime, fv.ValJSON)
			tt.checkFn(t, fv, extracted)
		})
	}
}

func TestAssignValue_Errors(t *testing.T) {
	tests := []struct {
		name      string
		fieldType string
		input     any
		errMsg    string
	}{
		{name: "text wrong type", fieldType: FieldText, input: 42, errMsg: "expected string"},
		{name: "integer wrong type", fieldType: FieldInteger, input: "nope", errMsg: "expected integer"},
		{name: "boolean wrong type", fieldType: FieldBoolean, input: []int{}, errMsg: "expected boolean"},
		{name: "decimal wrong type", fieldType: FieldDecimal, input: "nope", errMsg: "expected decimal"},
		{name: "date wrong type", fieldType: FieldDate, input: 42, errMsg: "expected date"},
		{name: "datetime wrong type", fieldType: FieldDateTime, input: 42, errMsg: "expected datetime"},
		{name: "unsupported type", fieldType: "mystery", input: "x", errMsg: "unsupported"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fv := &FieldValue{}
			err := assignValue(fv, tt.fieldType, tt.input)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestExtractValue_NilColumns(t *testing.T) {
	// All nil columns should return nil for every type.
	for _, ft := range ValidFieldTypes() {
		t.Run(ft, func(t *testing.T) {
			result := extractValue(ft, nil, nil, nil, nil, nil, nil, nil)
			if result != nil {
				t.Errorf("expected nil for %s with all nil columns, got %v", ft, result)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// buildFilterClause
// ---------------------------------------------------------------------------

func TestBuildFilterClause(t *testing.T) {
	tests := []struct {
		name    string
		filter  FieldFilter
		col     string
		wantErr bool
		errMsg  string
	}{
		{name: "eq", filter: FieldFilter{Operator: "eq", Value: "test"}, col: "val_text"},
		{name: "neq", filter: FieldFilter{Operator: "neq", Value: "test"}, col: "val_text"},
		{name: "gt", filter: FieldFilter{Operator: "gt", Value: 10}, col: "val_int"},
		{name: "lt", filter: FieldFilter{Operator: "lt", Value: 10}, col: "val_int"},
		{name: "gte", filter: FieldFilter{Operator: "gte", Value: 10}, col: "val_int"},
		{name: "lte", filter: FieldFilter{Operator: "lte", Value: 10}, col: "val_int"},
		{name: "like", filter: FieldFilter{Operator: "like", Value: "%test%"}, col: "val_text"},
		{name: "between", filter: FieldFilter{Operator: "between", Value: 1, Value2: 10}, col: "val_int"},
		{name: "in", filter: FieldFilter{Operator: "in", Value: []any{"a", "b"}}, col: "val_text"},
		{
			name:   "near",
			filter: FieldFilter{Operator: "near", Value: map[string]any{"lat": 51.5, "lng": -0.1}, Value2: float64(25)},
			col:    "val_decimal",
		},
		{name: "near default radius", filter: FieldFilter{Operator: "near", Value: map[string]any{"lat": 51.5, "lng": -0.1}}, col: "val_decimal"},
		{name: "near wrong value type", filter: FieldFilter{Operator: "near", Value: "not-a-map"}, col: "val_decimal", wantErr: true, errMsg: "lat, lng"},
		{name: "in wrong value type", filter: FieldFilter{Operator: "in", Value: "not-array"}, col: "val_text", wantErr: true, errMsg: "array"},
		{name: "unknown operator", filter: FieldFilter{Operator: "regex"}, col: "val_text", wantErr: true, errMsg: "unsupported"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clause, args, err := buildFilterClause(1, tt.col, tt.filter)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if clause == "" {
				t.Error("expected non-empty clause")
			}
			if len(args) == 0 {
				t.Error("expected at least one arg")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ValidateFieldOp
// ---------------------------------------------------------------------------

func TestValidateFieldOp(t *testing.T) {
	decimalDef := &FieldDef{Name: "balance", FieldType: FieldDecimal}
	intDef := &FieldDef{Name: "counter", FieldType: FieldInteger}
	boolDef := &FieldDef{Name: "active", FieldType: FieldBoolean}
	textDef := &FieldDef{Name: "label", FieldType: FieldText}

	optsCfg := json.RawMessage(`{"options":[{"value":"a","label":"A"},{"value":"b","label":"B"}]}`)
	multiDef := &FieldDef{Name: "tags", FieldType: FieldMultiSelect, Config: &optsCfg}

	selectCfg := json.RawMessage(`{"options":[{"value":"queued","label":"Q"},{"value":"running","label":"R"}]}`)
	selectDef := &FieldDef{Name: "status", FieldType: FieldSelect, Config: &selectCfg}

	tests := []struct {
		name    string
		def     *FieldDef
		op      *FieldOp
		wantErr bool
	}{
		{name: "increment on decimal", def: decimalDef, op: &FieldOp{Op: OpIncrement, Value: 5.0}},
		{name: "increment on integer", def: intDef, op: &FieldOp{Op: OpIncrement, Value: int64(1)}},
		{name: "increment on text fails", def: textDef, op: &FieldOp{Op: OpIncrement, Value: 1.0}, wantErr: true},
		{name: "increment nil value fails", def: decimalDef, op: &FieldOp{Op: OpIncrement, Value: nil}, wantErr: true},
		{name: "toggle on boolean", def: boolDef, op: &FieldOp{Op: OpToggle}},
		{name: "toggle on text fails", def: textDef, op: &FieldOp{Op: OpToggle}, wantErr: true},
		{name: "append on multi_select", def: multiDef, op: &FieldOp{Op: OpAppend, Value: "a"}},
		{name: "append invalid option fails", def: multiDef, op: &FieldOp{Op: OpAppend, Value: "z"}, wantErr: true},
		{name: "append on text fails", def: textDef, op: &FieldOp{Op: OpAppend, Value: "x"}, wantErr: true},
		{name: "remove on multi_select", def: multiDef, op: &FieldOp{Op: OpRemove, Value: "b"}},
		{name: "cas on select", def: selectDef, op: &FieldOp{Op: OpCAS, Value: "running", Expect: "queued"}},
		{name: "cas invalid new value fails", def: selectDef, op: &FieldOp{Op: OpCAS, Value: "bogus", Expect: "queued"}, wantErr: true},
		{name: "unknown op fails", def: decimalDef, op: &FieldOp{Op: "unknown"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFieldOp(tt.def, tt.op)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateValue_FieldOp(t *testing.T) {
	// ValidateValue should detect FieldOp and delegate.
	def := &FieldDef{Name: "counter", FieldType: FieldInteger}

	// Via struct.
	err := ValidateValue(def, FieldOp{Op: OpIncrement, Value: int64(1)})
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}

	// Via map (as arrives from gRPC).
	err = ValidateValue(def, map[string]any{"op": "increment", "value": float64(1)})
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}

	// Invalid op via map.
	err = ValidateValue(def, map[string]any{"op": "toggle"})
	if err == nil {
		t.Error("expected error for toggle on integer")
	}
}

func TestAsFieldOp(t *testing.T) {
	t.Run("struct", func(t *testing.T) {
		op, ok := AsFieldOp(FieldOp{Op: OpIncrement, Value: 5.0})
		if !ok || op.Op != OpIncrement {
			t.Error("expected FieldOp struct detection")
		}
	})

	t.Run("pointer", func(t *testing.T) {
		op, ok := AsFieldOp(&FieldOp{Op: OpToggle})
		if !ok || op.Op != OpToggle {
			t.Error("expected *FieldOp detection")
		}
	})

	t.Run("map with op key", func(t *testing.T) {
		floor := 0.0
		op, ok := AsFieldOp(map[string]any{"op": "increment", "value": -5.0, "floor": floor})
		if !ok || op.Op != OpIncrement {
			t.Error("expected map detection")
		}
		if op.Floor == nil || *op.Floor != 0.0 {
			t.Errorf("floor = %v, want 0.0", op.Floor)
		}
	})

	t.Run("map without op key", func(t *testing.T) {
		_, ok := AsFieldOp(map[string]any{"value": 5})
		if ok {
			t.Error("should not detect map without op key")
		}
	})

	t.Run("plain value", func(t *testing.T) {
		_, ok := AsFieldOp("hello")
		if ok {
			t.Error("should not detect plain string")
		}
	})
}

func TestValidateValues_FieldOp_SkipsRequired(t *testing.T) {
	// A FieldOp on a required field should not trigger "required" error.
	defs := map[string]*FieldDef{
		"balance": {Name: "balance", Label: "Balance", FieldType: FieldDecimal, Required: true},
	}
	values := map[string]any{
		"balance": FieldOp{Op: OpIncrement, Value: 10.0},
	}
	errs := ValidateValues(defs, values)
	if errs != nil {
		t.Errorf("expected no errors, got %v", errs)
	}
}
