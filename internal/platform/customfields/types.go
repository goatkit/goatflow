// Package customfields implements GoatKit PaaS Core universal custom fields.
//
// Custom fields provide an EAV (Entity-Attribute-Value) store that works across
// all GoatKit entities. Plugins declare fields at registration; the platform
// handles storage, validation, UI rendering, and querying.
package customfields

import (
	"encoding/json"
	"time"
)

// Supported entity types.
const (
	EntityTicket        = "ticket"
	EntityArticle       = "article"
	EntityContact       = "contact"
	EntityAgent         = "agent"
	EntityGroup         = "group"
	EntityCustomerGroup = "customer_group"
	EntityQueue         = "queue"
	EntityOrganisation  = "organisation"
)

// Supported field types.
const (
	FieldText        = "text"
	FieldTextArea    = "textarea"
	FieldInteger     = "integer"
	FieldDecimal     = "decimal"
	FieldBoolean     = "boolean"
	FieldDate        = "date"
	FieldDateTime    = "datetime"
	FieldSelect      = "select"
	FieldMultiSelect = "multi_select"
	FieldURL         = "url"
	FieldEmail       = "email"
	FieldPhone       = "phone"
	FieldPoint       = "point"
	FieldPolygon     = "polygon"
	FieldAddress     = "address"
)

// Owner types.
const (
	OwnerPlugin = "plugin"
	OwnerAdmin  = "admin"
	OwnerLegacy = "legacy"
)

// ValidEntityTypes returns all supported entity type keys.
func ValidEntityTypes() []string {
	return []string{
		EntityTicket, EntityArticle, EntityContact, EntityAgent,
		EntityGroup, EntityCustomerGroup, EntityQueue, EntityOrganisation,
	}
}

// ValidFieldTypes returns all supported field type keys.
func ValidFieldTypes() []string {
	return []string{
		FieldText, FieldTextArea, FieldInteger, FieldDecimal, FieldBoolean,
		FieldDate, FieldDateTime, FieldSelect, FieldMultiSelect,
		FieldURL, FieldEmail, FieldPhone,
		FieldPoint, FieldPolygon, FieldAddress,
	}
}

// IsValidEntityType checks whether the given entity type is supported.
func IsValidEntityType(et string) bool {
	for _, v := range ValidEntityTypes() {
		if et == v {
			return true
		}
	}
	return false
}

// IsValidFieldType checks whether the given field type is supported.
func IsValidFieldType(ft string) bool {
	for _, v := range ValidFieldTypes() {
		if ft == v {
			return true
		}
	}
	return false
}

// FieldDef represents a custom field definition from gk_custom_field_def.
type FieldDef struct {
	ID          int64            `json:"id" db:"id"`
	Name        string           `json:"name" db:"name"`
	Label       string           `json:"label" db:"label"`
	EntityType  string           `json:"entity_type" db:"entity_type"`
	FieldType   string           `json:"field_type" db:"field_type"`
	OwnerType   string           `json:"owner_type" db:"owner_type"`
	OwnerName   *string          `json:"owner_name,omitempty" db:"owner_name"`
	MigratedFrom *int64          `json:"migrated_from,omitempty" db:"migrated_from"`
	Section     string           `json:"section" db:"section"`
	FieldOrder  int              `json:"field_order" db:"field_order"`
	Description *string          `json:"description,omitempty" db:"description"`
	Placeholder *string          `json:"placeholder,omitempty" db:"placeholder"`
	Required    bool             `json:"required" db:"required"`
	Config      *json.RawMessage `json:"config,omitempty" db:"config"`
	ValidID     int              `json:"valid_id" db:"valid_id"`
	CreateTime  time.Time        `json:"create_time" db:"create_time"`
	CreateBy    int              `json:"create_by" db:"create_by"`
	ChangeTime  time.Time        `json:"change_time" db:"change_time"`
	ChangeBy    int              `json:"change_by" db:"change_by"`
}

// IsActive returns true if this field definition is valid/enabled.
func (f *FieldDef) IsActive() bool {
	return f.ValidID == 1
}

// ParsedConfig returns the parsed type-specific configuration.
func (f *FieldDef) ParsedConfig() (*FieldConfig, error) {
	if f.Config == nil {
		return &FieldConfig{}, nil
	}
	var cfg FieldConfig
	if err := json.Unmarshal(*f.Config, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// FieldValue represents a stored value from gk_custom_field_value.
type FieldValue struct {
	ID          int64            `json:"id" db:"id"`
	FieldID     int64            `json:"field_id" db:"field_id"`
	ObjectID    int64            `json:"object_id" db:"object_id"`
	ValText     *string          `json:"val_text,omitempty" db:"val_text"`
	ValInt      *int64           `json:"val_int,omitempty" db:"val_int"`
	ValDecimal  *float64         `json:"val_decimal,omitempty" db:"val_decimal"`
	ValDecimal2 *float64         `json:"val_decimal2,omitempty" db:"val_decimal2"`
	ValDate     *time.Time       `json:"val_date,omitempty" db:"val_date"`
	ValDatetime *time.Time       `json:"val_datetime,omitempty" db:"val_datetime"`
	ValJSON     *json.RawMessage `json:"val_json,omitempty" db:"val_json"`
}

// FieldConfig holds type-specific configuration stored in the config JSON column.
// Not all fields apply to all types — each field type uses a subset.
type FieldConfig struct {
	// text / textarea
	MaxLength  *int    `json:"max_length,omitempty"`
	Regex      string  `json:"regex,omitempty"`
	RegexError string  `json:"regex_error,omitempty"`

	// select / multi_select
	Options     []SelectOption `json:"options,omitempty"`
	AllowEmpty  bool           `json:"allow_empty,omitempty"`
	MinSelected *int           `json:"min_selected,omitempty"`
	MaxSelected *int           `json:"max_selected,omitempty"`

	// integer / decimal
	Min  *float64 `json:"min,omitempty"`
	Max  *float64 `json:"max,omitempty"`
	Step *float64 `json:"step,omitempty"`

	// date / datetime
	MinDate    string `json:"min_date,omitempty"`
	MaxDate    string `json:"max_date,omitempty"`
	FutureOnly bool   `json:"future_only,omitempty"`

	// point
	DefaultZoom   *int    `json:"default_zoom,omitempty"`
	DefaultCenter *LatLng `json:"default_center,omitempty"`

	// address
	Countries       []string `json:"countries,omitempty"`
	GeocodeProvider string   `json:"geocode_provider,omitempty"`

	// url
	AllowedSchemes []string `json:"allowed_schemes,omitempty"`

	// full-text search opt-in
	Searchable bool `json:"searchable,omitempty"`
}

// SelectOption represents a single option in a select or multi_select field.
type SelectOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// LatLng represents a geographic coordinate.
type LatLng struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// Address represents a structured address stored in val_json for address fields.
type Address struct {
	Line1    string  `json:"line1,omitempty"`
	Line2    string  `json:"line2,omitempty"`
	City     string  `json:"city,omitempty"`
	Region   string  `json:"region,omitempty"`
	Postcode string  `json:"postcode,omitempty"`
	Country  string  `json:"country,omitempty"`
	Lat      float64 `json:"lat,omitempty"`
	Lng      float64 `json:"lng,omitempty"`
}

// Atomic operation names for FieldOp.
const (
	OpIncrement = "increment"
	OpAppend    = "append"
	OpRemove    = "remove"
	OpCAS       = "cas"
	OpToggle    = "toggle"
)

// ValidFieldOps returns all supported atomic operation names.
func ValidFieldOps() []string {
	return []string{OpIncrement, OpAppend, OpRemove, OpCAS, OpToggle}
}

// FieldOp represents an atomic operation on a custom field value.
// This is the internal mirror of plugin.FieldOp.
type FieldOp struct {
	Op      string   `json:"op"`
	Value   any      `json:"value,omitempty"`
	Expect  any      `json:"expect,omitempty"`
	Floor   *float64 `json:"floor,omitempty"`
	Ceiling *float64 `json:"ceiling,omitempty"`
}

// AsFieldOp detects whether val is a FieldOp (either a direct struct or a
// JSON-decoded map with an "op" key, as arrives from gRPC plugins).
func AsFieldOp(val any) (*FieldOp, bool) {
	switch v := val.(type) {
	case FieldOp:
		return &v, true
	case *FieldOp:
		return v, true
	case map[string]any:
		opStr, ok := v["op"].(string)
		if !ok || opStr == "" {
			return nil, false
		}
		op := &FieldOp{Op: opStr, Value: v["value"], Expect: v["expect"]}
		if floor, ok := v["floor"]; ok {
			if f, err := toFloat64(floor); err == nil {
				op.Floor = &f
			}
		}
		if ceiling, ok := v["ceiling"]; ok {
			if f, err := toFloat64(ceiling); err == nil {
				op.Ceiling = &f
			}
		}
		return op, true
	default:
		return nil, false
	}
}

// FieldFilter is a query filter for CustomFieldsQuery.
type FieldFilter struct {
	Field    string `json:"field"`
	Operator string `json:"operator"` // eq, neq, gt, lt, gte, lte, like, in, between, near
	Value    any    `json:"value"`
	Value2   any    `json:"value2,omitempty"` // second value for between (upper bound) and near (radius km)
}

// ValidFilterOperators returns all supported filter operators.
func ValidFilterOperators() []string {
	return []string{"eq", "neq", "gt", "lt", "gte", "lte", "like", "in", "between", "near"}
}

// StorageColumn returns the primary storage column name for a given field type.
func StorageColumn(fieldType string) string {
	switch fieldType {
	case FieldText, FieldTextArea, FieldSelect, FieldURL, FieldEmail, FieldPhone:
		return "val_text"
	case FieldInteger, FieldBoolean:
		return "val_int"
	case FieldDecimal:
		return "val_decimal"
	case FieldDate:
		return "val_date"
	case FieldDateTime:
		return "val_datetime"
	case FieldMultiSelect, FieldPolygon, FieldAddress, FieldPoint:
		return "val_json"
	default:
		return "val_text"
	}
}
