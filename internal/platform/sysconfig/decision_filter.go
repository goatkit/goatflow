package sysconfig

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// DecisionFilterKey holds the inbound-mail decision filter settings as one
// JSON value.
const DecisionFilterKey = "PostMaster::DecisionFilter"

// Decision filter bounds. A System One server accepts up to 255 options per
// question; the logprob backends of goatkit-llm read single option letters,
// so 26 is the limit every backend can answer.
const (
	DecisionFilterDefaultThreshold = 0.5
	DecisionFilterDefaultTimeout   = 30
	DecisionFilterMaxTimeout       = 120
	DecisionFilterMaxOptions       = 26
	decisionFilterMaxName          = 200
	decisionFilterMaxDescription   = 500
	decisionFilterMaxInstructions  = 500
)

// DecisionFilterConfig configures the inbound-mail decision filter: before
// PostMaster creates a ticket, goatkit-llm's llm_decide answers one choice
// question per enabled field, and answers at or above Threshold confidence
// set the queue, priority or type of the new ticket.
type DecisionFilterConfig struct {
	Enabled bool `json:"enabled"`
	// ProviderID selects a goatkit-llm decide provider; 0 uses the org's
	// active decide provider.
	ProviderID     int64               `json:"provider_id,omitempty"`
	Threshold      float64             `json:"threshold"`
	TimeoutSeconds int                 `json:"timeout_seconds"`
	Queue          DecisionFilterField `json:"queue"`
	Priority       DecisionFilterField `json:"priority"`
	Type           DecisionFilterField `json:"type"`
}

// DecisionFilterField is one classified ticket attribute. Options are the
// candidate queue, priority or type names, in the order the question lists
// them.
type DecisionFilterField struct {
	Enabled      bool                   `json:"enabled"`
	Instructions string                 `json:"instructions,omitempty"`
	Options      []DecisionFilterOption `json:"options,omitempty"`
}

// DecisionFilterOption is a candidate answer: the stored name of a queue,
// priority or type, and an optional description telling the model when it
// applies.
type DecisionFilterOption struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// DefaultDecisionFilterConfig is the disabled filter with default bounds.
func DefaultDecisionFilterConfig() DecisionFilterConfig {
	return DecisionFilterConfig{
		Threshold:      DecisionFilterDefaultThreshold,
		TimeoutSeconds: DecisionFilterDefaultTimeout,
	}
}

// Normalize trims text and drops blank and duplicate options. Unset bounds
// are left for Validate to reject; LoadDecisionFilterConfig decodes over the
// defaults, so a stored value that omits them keeps the defaults.
func (c *DecisionFilterConfig) Normalize() {
	for _, f := range c.fields() {
		f.field.Instructions = strings.TrimSpace(f.field.Instructions)
		seen := make(map[string]bool, len(f.field.Options))
		opts := f.field.Options[:0]
		for _, o := range f.field.Options {
			o.Name = strings.TrimSpace(o.Name)
			o.Description = strings.TrimSpace(o.Description)
			if o.Name == "" || seen[o.Name] {
				continue
			}
			seen[o.Name] = true
			opts = append(opts, o)
		}
		f.field.Options = opts
	}
}

// Validate reports the first setting a decision request could not use. It
// expects a normalized config.
func (c DecisionFilterConfig) Validate() error {
	if c.ProviderID < 0 {
		return fmt.Errorf("provider_id must not be negative")
	}
	if c.Threshold < 0 || c.Threshold > 1 {
		return fmt.Errorf("threshold must be between 0 and 1")
	}
	if c.TimeoutSeconds < 1 || c.TimeoutSeconds > DecisionFilterMaxTimeout {
		return fmt.Errorf("timeout_seconds must be between 1 and %d", DecisionFilterMaxTimeout)
	}
	enabled := 0
	for _, f := range c.fields() {
		if len(f.field.Instructions) > decisionFilterMaxInstructions {
			return fmt.Errorf("%s instructions exceed %d characters", f.name, decisionFilterMaxInstructions)
		}
		for _, o := range f.field.Options {
			if len(o.Name) > decisionFilterMaxName {
				return fmt.Errorf("%s option name exceeds %d characters", f.name, decisionFilterMaxName)
			}
			if len(o.Description) > decisionFilterMaxDescription {
				return fmt.Errorf("%s option %q description exceeds %d characters", f.name, o.Name, decisionFilterMaxDescription)
			}
		}
		if !f.field.Enabled {
			continue
		}
		enabled++
		if n := len(f.field.Options); n < 2 || n > DecisionFilterMaxOptions {
			return fmt.Errorf("%s needs between 2 and %d options", f.name, DecisionFilterMaxOptions)
		}
	}
	if c.Enabled && enabled == 0 {
		return fmt.Errorf("enable at least one of queue, priority or type")
	}
	return nil
}

type namedDecisionField struct {
	name  string
	field *DecisionFilterField
}

func (c *DecisionFilterConfig) fields() []namedDecisionField {
	return []namedDecisionField{{"queue", &c.Queue}, {"priority", &c.Priority}, {"type", &c.Type}}
}

// LoadDecisionFilterConfig reads the decision filter settings. A missing
// value is the disabled default; a value that does not parse is an error so
// the filter stays off rather than run on a half-read config.
func LoadDecisionFilterConfig(db *sql.DB) (DecisionFilterConfig, error) {
	cfg := DefaultDecisionFilterConfig()
	raw, ok := sysconfigValue(db, DecisionFilterKey)
	if !ok || strings.TrimSpace(raw) == "" {
		return cfg, nil
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return DefaultDecisionFilterConfig(), fmt.Errorf("%s: %w", DecisionFilterKey, err)
	}
	cfg.Normalize()
	return cfg, nil
}

// SaveDecisionFilterConfig normalizes, validates and stores the settings.
// Validation failures come back as *DecisionFilterValidationError, before
// any storage check, so callers can report them as the caller's mistake.
func SaveDecisionFilterConfig(db *sql.DB, cfg DecisionFilterConfig, userID int) error {
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return &DecisionFilterValidationError{Err: err}
	}
	if db == nil {
		return fmt.Errorf("database connection unavailable")
	}
	value, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	defaultValue, _ := json.Marshal(DefaultDecisionFilterConfig())
	if err := ensureSysconfigDefault(db, sysconfigDefault{
		name:        DecisionFilterKey,
		description: "Classify new inbound mail with goatkit-llm decisions (queue, priority, type).",
		navigation:  "Core::Email::PostMaster",
		xmlFilename: "PostMaster.xml",
		xml:         `{"type":"json"}`,
		value:       string(defaultValue),
	}, userID); err != nil {
		return fmt.Errorf("sysconfig unavailable: %w", err)
	}
	if err := upsertSysconfigValue(db, DecisionFilterKey, string(value), userID); err != nil {
		return fmt.Errorf("sysconfig unavailable: %w", err)
	}
	return nil
}

// DecisionFilterValidationError marks a config the caller must correct (a
// 400, not a 500).
type DecisionFilterValidationError struct{ Err error }

func (e *DecisionFilterValidationError) Error() string { return e.Err.Error() }
func (e *DecisionFilterValidationError) Unwrap() error { return e.Err }
