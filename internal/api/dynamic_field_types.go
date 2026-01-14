package api

import (
	"fmt"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

// Field type constants matching OTRS.
const (
	DFTypeText                 = "Text"
	DFTypeTextArea             = "TextArea"
	DFTypeCheckbox             = "Checkbox"
	DFTypeDropdown             = "Dropdown"
	DFTypeMultiselect          = "Multiselect"
	DFTypeDate                 = "Date"
	DFTypeDateTime             = "DateTime"
	DFTypeWebserviceDropdown   = "WebserviceDropdown"
	DFTypeWebserviceMultiselect = "WebserviceMultiselect"
)

// Object type constants matching OTRS.
const (
	DFObjectTicket          = "Ticket"
	DFObjectArticle         = "Article"
	DFObjectCustomerUser    = "CustomerUser"
	DFObjectCustomerCompany = "CustomerCompany"
)

// Screen config value constants.
const (
	DFScreenDisabled = 0
	DFScreenEnabled  = 1
	DFScreenRequired = 2
)

// ValidFieldTypes returns all supported field types.
func ValidFieldTypes() []string {
	return []string{
		DFTypeText,
		DFTypeTextArea,
		DFTypeCheckbox,
		DFTypeDropdown,
		DFTypeMultiselect,
		DFTypeDate,
		DFTypeDateTime,
		DFTypeWebserviceDropdown,
		DFTypeWebserviceMultiselect,
	}
}

// ValidObjectTypes returns all supported object types.
func ValidObjectTypes() []string {
	return []string{
		DFObjectTicket,
		DFObjectArticle,
		DFObjectCustomerUser,
		DFObjectCustomerCompany,
	}
}

// DynamicFieldConfig stores type-specific configuration (YAML-serialized for OTRS compatibility).
type DynamicFieldConfig struct {
	// Common to all types
	DefaultValue string `yaml:"DefaultValue,omitempty"`

	// Text/TextArea fields
	MaxLength   int     `yaml:"MaxLength,omitempty"`
	RegExList   []RegEx `yaml:"RegExList,omitempty"` // OTRS uses RegExList
	Link        string  `yaml:"Link,omitempty"`
	LinkPreview string  `yaml:"LinkPreview,omitempty"`
	Rows        int     `yaml:"Rows,omitempty"` // TextArea only
	Cols        int     `yaml:"Cols,omitempty"` // TextArea only

	// Dropdown/Multiselect
	PossibleValues     map[string]string `yaml:"PossibleValues,omitempty"`
	PossibleNone       int               `yaml:"PossibleNone,omitempty"` // OTRS uses 0/1 not bool
	TranslatableValues int               `yaml:"TranslatableValues,omitempty"`
	TreeView           int               `yaml:"TreeView,omitempty"`

	// Date/DateTime
	YearsInPast     int    `yaml:"YearsInPast,omitempty"`
	YearsInFuture   int    `yaml:"YearsInFuture,omitempty"`
	DateRestriction string `yaml:"DateRestriction,omitempty"` // none, DisablePastDates, DisableFutureDates
	YearsPeriod     int    `yaml:"YearsPeriod,omitempty"`

	// WebserviceDropdown/WebserviceMultiselect - OTRS compatible
	Webservice               string                      `yaml:"Webservice,omitempty"`               // Webservice name
	InvokerSearch            string                      `yaml:"InvokerSearch,omitempty"`            // Invoker for search/autocomplete
	InvokerGet               string                      `yaml:"InvokerGet,omitempty"`               // Invoker for get-by-id
	Backend                  string                      `yaml:"Backend,omitempty"`                  // Backend type (DirectRequest, etc.)
	StoredValue              string                      `yaml:"StoredValue,omitempty"`              // Field key to store (e.g., "ID")
	DisplayedValues          string                      `yaml:"DisplayedValues,omitempty"`          // Fields to display (comma-sep, e.g., "Name,Code")
	DisplayedValuesSeparator string                      `yaml:"DisplayedValuesSeparator,omitempty"` // Separator for display (e.g., " - ")
	SearchKeys               string                      `yaml:"SearchKeys,omitempty"`               // Fields to search (comma-sep)
	AutocompleteMinLength    int                         `yaml:"AutocompleteMinLength,omitempty"`    // Min chars before autocomplete (default: 3)
	QueryDelay               int                         `yaml:"QueryDelay,omitempty"`               // Delay in ms before query (default: 500)
	CacheTTL                 int                         `yaml:"CacheTTL,omitempty"`                 // Cache TTL in seconds (default: 60)
	Limit                    int                         `yaml:"Limit,omitempty"`                    // Result limit (default: 20)
	AdditionalDFStorage      []AdditionalDFStorageConfig `yaml:"AdditionalDFStorage,omitempty"`      // Auto-fill other fields from response
}

// AdditionalDFStorageConfig defines auto-fill behavior for webservice fields.
// When a webservice returns data, additional fields can be populated from the response.
type AdditionalDFStorageConfig struct {
	Type         string `yaml:"Type"`         // "Backend" (store in DB) or "Frontend" (display only)
	DynamicField string `yaml:"DynamicField"` // Target field name (without DynamicField_ prefix)
	Key          string `yaml:"Key"`          // Source key from webservice response
}

// RegEx represents a regex validation pattern (OTRS format).
type RegEx struct {
	Value        string `yaml:"Value"`
	ErrorMessage string `yaml:"ErrorMessage"`
}

// DynamicField represents a field definition from dynamic_field table.
type DynamicField struct {
	ID            int                 `json:"id" db:"id"`
	InternalField int                 `json:"internal_field" db:"internal_field"`
	Name          string              `json:"name" db:"name"`
	Label         string              `json:"label" db:"label"`
	FieldOrder    int                 `json:"field_order" db:"field_order"`
	FieldType     string              `json:"field_type" db:"field_type"`
	ObjectType    string              `json:"object_type" db:"object_type"`
	Config        *DynamicFieldConfig `json:"config"`
	ConfigRaw     []byte              `json:"-" db:"config"` // YAML blob from DB
	ValidID       int                 `json:"valid_id" db:"valid_id"`
	CreateTime    time.Time           `json:"create_time" db:"create_time"`
	CreateBy      int                 `json:"create_by" db:"create_by"`
	ChangeTime    time.Time           `json:"change_time" db:"change_time"`
	ChangeBy      int                 `json:"change_by" db:"change_by"`
}

// DynamicFieldValue represents a stored value from dynamic_field_value table.
type DynamicFieldValue struct {
	ID        int        `json:"id" db:"id"`
	FieldID   int        `json:"field_id" db:"field_id"`
	ObjectID  int64      `json:"object_id" db:"object_id"`
	ValueText *string    `json:"value_text,omitempty" db:"value_text"`
	ValueDate *time.Time `json:"value_date,omitempty" db:"value_date"`
	ValueInt  *int64     `json:"value_int,omitempty" db:"value_int"`
}

// DynamicFieldScreenConfig maps fields to screens.
type DynamicFieldScreenConfig struct {
	ID          int       `json:"id" db:"id"`
	FieldID     int       `json:"field_id" db:"field_id"`
	ScreenKey   string    `json:"screen_key" db:"screen_key"`
	ConfigValue int       `json:"config_value" db:"config_value"` // 0=disabled, 1=enabled, 2=required
	CreateTime  time.Time `json:"create_time" db:"create_time"`
	CreateBy    int       `json:"create_by" db:"create_by"`
	ChangeTime  time.Time `json:"change_time" db:"change_time"`
	ChangeBy    int       `json:"change_by" db:"change_by"`
}

// ScreenDefinition describes a screen that can display dynamic fields.
type ScreenDefinition struct {
	Key              string `json:"key"`
	Name             string `json:"name"`
	ObjectType       string `json:"object_type"`
	SupportsRequired bool   `json:"supports_required"`
	IsDisplayOnly    bool   `json:"is_display_only"`
}

// GetScreenDefinitions returns all screens that support dynamic fields.
func GetScreenDefinitions() []ScreenDefinition {
	return []ScreenDefinition{
		// Ticket screens
		{Key: "AgentTicketPhone", Name: "New Phone Ticket", ObjectType: DFObjectTicket, SupportsRequired: true},
		{Key: "AgentTicketEmail", Name: "New Email Ticket", ObjectType: DFObjectTicket, SupportsRequired: true},
		{Key: "AgentTicketZoom", Name: "Ticket Zoom", ObjectType: DFObjectTicket, IsDisplayOnly: true},
		{Key: "AgentTicketClose", Name: "Close Ticket", ObjectType: DFObjectTicket, SupportsRequired: true},
		{Key: "AgentTicketNote", Name: "Add Note", ObjectType: DFObjectTicket, SupportsRequired: true},
		{Key: "AgentTicketMove", Name: "Move Ticket", ObjectType: DFObjectTicket, SupportsRequired: true},
		{Key: "AgentTicketOwner", Name: "Change Owner", ObjectType: DFObjectTicket, SupportsRequired: true},
		{Key: "AgentTicketPriority", Name: "Change Priority", ObjectType: DFObjectTicket, SupportsRequired: true},
		{Key: "CustomerTicketMessage", Name: "Customer New Ticket", ObjectType: DFObjectTicket, SupportsRequired: true},
		{Key: "CustomerTicketZoom", Name: "Customer Ticket View", ObjectType: DFObjectTicket, IsDisplayOnly: true},
		// Article screens
		{Key: "AgentArticleZoom", Name: "Article View", ObjectType: DFObjectArticle, IsDisplayOnly: true},
		{Key: "AgentArticleNote", Name: "Agent Note Article", ObjectType: DFObjectArticle, SupportsRequired: true},
		{Key: "AgentArticleClose", Name: "Close Note Article", ObjectType: DFObjectArticle, SupportsRequired: true},
		{Key: "AgentArticleReply", Name: "Agent Reply Article", ObjectType: DFObjectArticle, SupportsRequired: true},
		{Key: "CustomerArticleReply", Name: "Customer Reply Article", ObjectType: DFObjectArticle, SupportsRequired: true},
	}
}

// ParseConfig deserializes YAML config blob into DynamicFieldConfig.
func (df *DynamicField) ParseConfig() error {
	if len(df.ConfigRaw) == 0 {
		df.Config = &DynamicFieldConfig{}
		return nil
	}

	var config DynamicFieldConfig
	if err := yaml.Unmarshal(df.ConfigRaw, &config); err != nil {
		return fmt.Errorf("failed to parse dynamic field config: %w", err)
	}
	df.Config = &config
	return nil
}

// SerializeConfig serializes DynamicFieldConfig to YAML for storage.
func (df *DynamicField) SerializeConfig() error {
	if df.Config == nil {
		df.ConfigRaw = nil
		return nil
	}

	data, err := yaml.Marshal(df.Config)
	if err != nil {
		return fmt.Errorf("failed to serialize dynamic field config: %w", err)
	}
	df.ConfigRaw = data
	return nil
}

// Validate checks the dynamic field for errors.
func (df *DynamicField) Validate() error {
	if df.Name == "" {
		return fmt.Errorf("name is required")
	}

	// Name must be alphanumeric only (OTRS requirement)
	nameRegex := regexp.MustCompile(`^[a-zA-Z0-9]+$`)
	if !nameRegex.MatchString(df.Name) {
		return fmt.Errorf("name must contain only alphanumeric characters")
	}

	if df.Label == "" {
		return fmt.Errorf("label is required")
	}

	if !isValidFieldType(df.FieldType) {
		return fmt.Errorf("invalid field type: %s", df.FieldType)
	}

	if !isValidObjectType(df.ObjectType) {
		return fmt.Errorf("invalid object type: %s", df.ObjectType)
	}

	// Type-specific validation
	if df.Config != nil {
		if err := df.validateConfigForType(); err != nil {
			return err
		}
	}

	return nil
}

func (df *DynamicField) validateConfigForType() error {
	switch df.FieldType {
	case DFTypeDropdown, DFTypeMultiselect:
		if len(df.Config.PossibleValues) == 0 {
			return fmt.Errorf("%s field requires at least one possible value", df.FieldType)
		}
	case DFTypeWebserviceDropdown, DFTypeWebserviceMultiselect:
		if df.Config.Webservice == "" {
			return fmt.Errorf("%s field requires a webservice to be configured", df.FieldType)
		}
		if df.Config.InvokerSearch == "" {
			return fmt.Errorf("%s field requires a search invoker to be configured", df.FieldType)
		}
		if df.Config.StoredValue == "" {
			return fmt.Errorf("%s field requires a stored value field to be configured", df.FieldType)
		}
		if df.Config.DisplayedValues == "" {
			return fmt.Errorf("%s field requires displayed values to be configured", df.FieldType)
		}
	}
	return nil
}

func isValidFieldType(ft string) bool {
	for _, valid := range ValidFieldTypes() {
		if ft == valid {
			return true
		}
	}
	return false
}

func isValidObjectType(ot string) bool {
	for _, valid := range ValidObjectTypes() {
		if ot == valid {
			return true
		}
	}
	return false
}

// IsActive returns true if the field is valid (active).
func (df *DynamicField) IsActive() bool {
	return df.ValidID == 1
}

// IsInternal returns true if this is an internal/system field.
func (df *DynamicField) IsInternal() bool {
	return df.InternalField == 1
}

// GetValueColumn returns the database column name used for storing values of this field type.
func (df *DynamicField) GetValueColumn() string {
	switch df.FieldType {
	case DFTypeCheckbox:
		return "value_int"
	case DFTypeDate, DFTypeDateTime:
		return "value_date"
	default:
		return "value_text"
	}
}

// SupportsAutoConfig returns true if the field type can use automatic configuration
// with sensible defaults, without requiring manual configuration input.
// Dropdown and Multiselect require PossibleValues and cannot use auto-config.
func SupportsAutoConfig(fieldType string) bool {
	switch fieldType {
	case DFTypeText, DFTypeTextArea, DFTypeCheckbox, DFTypeDate, DFTypeDateTime:
		return true
	default:
		return false
	}
}

// DefaultDynamicFieldConfig returns sensible default configuration for each field type.
// Used when auto-config mode is enabled to skip manual configuration.
func DefaultDynamicFieldConfig(fieldType string) *DynamicFieldConfig {
	switch fieldType {
	case DFTypeText:
		return &DynamicFieldConfig{
			MaxLength: 200,
		}
	case DFTypeTextArea:
		return &DynamicFieldConfig{
			Rows: 4,
			Cols: 60,
		}
	case DFTypeCheckbox:
		return &DynamicFieldConfig{
			DefaultValue: "0",
		}
	case DFTypeDate:
		return &DynamicFieldConfig{
			YearsInPast:   5,
			YearsInFuture: 5,
		}
	case DFTypeDateTime:
		return &DynamicFieldConfig{
			YearsInPast:   5,
			YearsInFuture: 5,
		}
	default:
		return &DynamicFieldConfig{}
	}
}
