package customfields

import (
	"context"
	"encoding/json"
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"time"
)

// regexTimeout is the maximum time allowed for user-provided regex evaluation.
const regexTimeout = 100 * time.Millisecond

// ValidateFieldDef validates a custom field definition before create/update.
func ValidateFieldDef(f *FieldDef) error {
	if f.Name == "" {
		return fmt.Errorf("name is required")
	}
	if f.Label == "" {
		return fmt.Errorf("label is required")
	}
	if !IsValidEntityType(f.EntityType) {
		return fmt.Errorf("invalid entity type: %s", f.EntityType)
	}
	if !IsValidFieldType(f.FieldType) {
		return fmt.Errorf("invalid field type: %s", f.FieldType)
	}

	// Name must be lowercase alphanumeric + underscores.
	nameRe := regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	if !nameRe.MatchString(f.Name) {
		return fmt.Errorf("name must be lowercase alphanumeric with underscores, starting with a letter")
	}

	// Validate type-specific config if present.
	if f.Config != nil {
		cfg, err := f.ParsedConfig()
		if err != nil {
			return fmt.Errorf("invalid config JSON: %w", err)
		}
		if err := validateConfig(f.FieldType, cfg); err != nil {
			return fmt.Errorf("config validation: %w", err)
		}
	}

	return nil
}

// validateConfig checks type-specific config constraints.
func validateConfig(fieldType string, cfg *FieldConfig) error {
	switch fieldType {
	case FieldSelect:
		if len(cfg.Options) == 0 {
			return fmt.Errorf("select field requires at least one option")
		}
	case FieldMultiSelect:
		if len(cfg.Options) == 0 {
			return fmt.Errorf("multi_select field requires at least one option")
		}
	case FieldText:
		if cfg.Regex != "" {
			if _, err := regexp.Compile(cfg.Regex); err != nil {
				return fmt.Errorf("invalid regex pattern: %w", err)
			}
		}
	}
	return nil
}

// ValidateValue validates a single value against its field definition.
// Returns nil if valid.
func ValidateValue(def *FieldDef, val any) error {
	if val == nil {
		if def.Required {
			return fmt.Errorf("field %q is required", def.Label)
		}
		return nil
	}

	cfg, _ := def.ParsedConfig()
	if cfg == nil {
		cfg = &FieldConfig{}
	}

	switch def.FieldType {
	case FieldText, FieldTextArea:
		return validateText(def, cfg, val)
	case FieldInteger:
		return validateInteger(cfg, val)
	case FieldDecimal:
		return validateDecimal(cfg, val)
	case FieldBoolean:
		return validateBoolean(val)
	case FieldDate:
		return validateDate(cfg, val)
	case FieldDateTime:
		return validateDateTime(cfg, val)
	case FieldSelect:
		return validateSelect(cfg, val)
	case FieldMultiSelect:
		return validateMultiSelect(cfg, val)
	case FieldURL:
		return validateURL(cfg, val)
	case FieldEmail:
		return validateEmail(val)
	case FieldPhone:
		return validatePhone(val)
	case FieldPoint:
		return validatePoint(val)
	case FieldPolygon:
		return validatePolygon(val)
	case FieldAddress:
		return validateAddress(cfg, val)
	default:
		return fmt.Errorf("unsupported field type %q", def.FieldType)
	}
}

// ValidateValues validates all values in a map against their field definitions.
// Returns a map of field_name → error message for any invalid values.
func ValidateValues(defs map[string]*FieldDef, values map[string]any) map[string]string {
	errors := make(map[string]string)

	// Check required fields.
	for name, def := range defs {
		if def.Required {
			if _, ok := values[name]; !ok {
				errors[name] = fmt.Sprintf("field %q is required", def.Label)
			}
		}
	}

	// Validate provided values.
	for name, val := range values {
		def, ok := defs[name]
		if !ok {
			errors[name] = fmt.Sprintf("unknown field %q", name)
			continue
		}
		if err := ValidateValue(def, val); err != nil {
			errors[name] = err.Error()
		}
	}

	if len(errors) == 0 {
		return nil
	}
	return errors
}

// --- Type-specific validators ---

func validateText(def *FieldDef, cfg *FieldConfig, val any) error {
	s, ok := val.(string)
	if !ok {
		return fmt.Errorf("expected string, got %T", val)
	}
	if cfg.MaxLength != nil && len(s) > *cfg.MaxLength {
		return fmt.Errorf("must be ≤ %d characters", *cfg.MaxLength)
	}
	if cfg.Regex != "" {
		re, err := regexp.Compile(cfg.Regex)
		if err != nil {
			return fmt.Errorf("invalid regex: %w", err)
		}
		// Run regex with timeout via context.
		ctx, cancel := context.WithTimeout(context.Background(), regexTimeout)
		defer cancel()
		done := make(chan bool, 1)
		var matched bool
		go func() {
			matched = re.MatchString(s)
			done <- true
		}()
		select {
		case <-done:
			if !matched {
				msg := cfg.RegexError
				if msg == "" {
					msg = "invalid format"
				}
				return fmt.Errorf("%s", msg)
			}
		case <-ctx.Done():
			return fmt.Errorf("regex validation timed out")
		}
	}
	return nil
}

func validateInteger(cfg *FieldConfig, val any) error {
	n, err := toInt64(val)
	if err != nil {
		return fmt.Errorf("expected integer, got %T", val)
	}
	if cfg.Min != nil && float64(n) < *cfg.Min {
		return fmt.Errorf("must be ≥ %.0f", *cfg.Min)
	}
	if cfg.Max != nil && float64(n) > *cfg.Max {
		return fmt.Errorf("must be ≤ %.0f", *cfg.Max)
	}
	return nil
}

func validateDecimal(cfg *FieldConfig, val any) error {
	d, err := toFloat64(val)
	if err != nil {
		return fmt.Errorf("expected decimal, got %T", val)
	}
	if cfg.Min != nil && d < *cfg.Min {
		return fmt.Errorf("must be ≥ %g", *cfg.Min)
	}
	if cfg.Max != nil && d > *cfg.Max {
		return fmt.Errorf("must be ≤ %g", *cfg.Max)
	}
	return nil
}

func validateBoolean(val any) error {
	_, err := toBool(val)
	return err
}

func validateDate(cfg *FieldConfig, val any) error {
	d, err := toDate(val)
	if err != nil {
		return fmt.Errorf("expected date (YYYY-MM-DD), got %T", val)
	}
	if cfg.MinDate != "" {
		minD, err := time.Parse("2006-01-02", cfg.MinDate)
		if err == nil && d.Before(minD) {
			return fmt.Errorf("date must be after %s", cfg.MinDate)
		}
	}
	if cfg.MaxDate != "" {
		maxD, err := time.Parse("2006-01-02", cfg.MaxDate)
		if err == nil && d.After(maxD) {
			return fmt.Errorf("date must be before %s", cfg.MaxDate)
		}
	}
	if cfg.FutureOnly && d.Before(time.Now().Truncate(24*time.Hour)) {
		return fmt.Errorf("date must be in the future")
	}
	return nil
}

func validateDateTime(cfg *FieldConfig, val any) error {
	_, err := toDateTime(val)
	if err != nil {
		return fmt.Errorf("expected datetime, got %T", val)
	}
	return nil
}

func validateSelect(cfg *FieldConfig, val any) error {
	s, ok := val.(string)
	if !ok {
		return fmt.Errorf("expected string, got %T", val)
	}
	if s == "" && cfg.AllowEmpty {
		return nil
	}
	for _, opt := range cfg.Options {
		if opt.Value == s {
			return nil
		}
	}
	return fmt.Errorf("invalid option: %s", s)
}

func validateMultiSelect(cfg *FieldConfig, val any) error {
	// Accept []string or []any.
	var selected []string
	switch v := val.(type) {
	case []string:
		selected = v
	case []any:
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return fmt.Errorf("multi_select values must be strings")
			}
			selected = append(selected, s)
		}
	default:
		return fmt.Errorf("expected array, got %T", val)
	}

	// Validate option membership.
	optionSet := make(map[string]bool, len(cfg.Options))
	for _, opt := range cfg.Options {
		optionSet[opt.Value] = true
	}
	for _, s := range selected {
		if !optionSet[s] {
			return fmt.Errorf("invalid option: %s", s)
		}
	}

	if cfg.MinSelected != nil && len(selected) < *cfg.MinSelected {
		return fmt.Errorf("select at least %d options", *cfg.MinSelected)
	}
	if cfg.MaxSelected != nil && len(selected) > *cfg.MaxSelected {
		return fmt.Errorf("select at most %d options", *cfg.MaxSelected)
	}
	return nil
}

func validateURL(cfg *FieldConfig, val any) error {
	s, ok := val.(string)
	if !ok {
		return fmt.Errorf("expected string, got %T", val)
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return fmt.Errorf("invalid URL")
	}
	if len(cfg.AllowedSchemes) > 0 {
		allowed := false
		for _, scheme := range cfg.AllowedSchemes {
			if u.Scheme == scheme {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("URL scheme must be one of: %v", cfg.AllowedSchemes)
		}
	}
	return nil
}

func validateEmail(val any) error {
	s, ok := val.(string)
	if !ok {
		return fmt.Errorf("expected string, got %T", val)
	}
	if _, err := mail.ParseAddress(s); err != nil {
		return fmt.Errorf("invalid email address")
	}
	return nil
}

func validatePhone(val any) error {
	s, ok := val.(string)
	if !ok {
		return fmt.Errorf("expected string, got %T", val)
	}
	// Basic phone validation: allow digits, spaces, +, -, (, ).
	phoneRe := regexp.MustCompile(`^\+?[\d\s\-().]{5,20}$`)
	if !phoneRe.MatchString(s) {
		return fmt.Errorf("invalid phone number")
	}
	return nil
}

func validatePoint(val any) error {
	raw, err := json.Marshal(val)
	if err != nil {
		return fmt.Errorf("expected {lat, lng} object")
	}
	var ll LatLng
	if err := json.Unmarshal(raw, &ll); err != nil {
		return fmt.Errorf("expected {lat, lng} object")
	}
	if ll.Lat < -90 || ll.Lat > 90 {
		return fmt.Errorf("latitude must be between -90 and 90")
	}
	if ll.Lng < -180 || ll.Lng > 180 {
		return fmt.Errorf("longitude must be between -180 and 180")
	}
	return nil
}

func validatePolygon(val any) error {
	raw, err := json.Marshal(val)
	if err != nil {
		return fmt.Errorf("invalid GeoJSON polygon")
	}
	if len(raw) > 100*1024 {
		return fmt.Errorf("polygon GeoJSON must be ≤ 100KB")
	}
	// Basic structure check: must have "type" and "coordinates".
	var geojson map[string]any
	if err := json.Unmarshal(raw, &geojson); err != nil {
		return fmt.Errorf("invalid GeoJSON polygon")
	}
	if geojson["type"] != "Polygon" && geojson["type"] != "MultiPolygon" {
		return fmt.Errorf("GeoJSON type must be Polygon or MultiPolygon")
	}
	if _, ok := geojson["coordinates"]; !ok {
		return fmt.Errorf("GeoJSON must have coordinates")
	}
	return nil
}

func validateAddress(cfg *FieldConfig, val any) error {
	raw, err := json.Marshal(val)
	if err != nil {
		return fmt.Errorf("expected address object")
	}
	var addr Address
	if err := json.Unmarshal(raw, &addr); err != nil {
		return fmt.Errorf("expected address object")
	}
	if len(cfg.Countries) > 0 && addr.Country != "" {
		allowed := false
		for _, c := range cfg.Countries {
			if addr.Country == c {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("invalid country code: %s", addr.Country)
		}
	}
	return nil
}
