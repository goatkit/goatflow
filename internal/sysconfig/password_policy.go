package sysconfig

import (
	"database/sql"
	"regexp"
	"strings"
	"unicode"

	"github.com/goatkit/goatflow/internal/database"
)

// PasswordPolicy holds OTRS-compatible password policy settings.
// These settings are stored in sysconfig_default with names like:
// - CustomerPreferencesGroups###Password (for customers)
// - PreferencesGroups###Password (for agents)
type PasswordPolicy struct {
	// PasswordRegExp is a custom regex pattern that passwords must match.
	// Example: "[a-z]|[A-Z]|[0-9]" requires at least one letter or digit.
	PasswordRegExp string `json:"password_reg_exp"`

	// PasswordMinSize is the minimum password length. 0 = disabled.
	PasswordMinSize int `json:"password_min_size"`

	// PasswordMin2Lower2UpperCharacters requires at least 2 lowercase AND 2 uppercase letters.
	PasswordMin2Lower2UpperCharacters bool `json:"password_min_2_lower_2_upper_characters"`

	// PasswordMin2Characters requires at least 2 letter characters (alphabetic).
	PasswordMin2Characters bool `json:"password_min_2_characters"`

	// PasswordNeedDigit requires at least 1 digit (0-9).
	PasswordNeedDigit bool `json:"password_need_digit"`

	// PasswordMaxLoginFailed is the max failed login attempts before account locked (0 = disabled).
	// Note: This is only used for agents, not customers in OTRS.
	PasswordMaxLoginFailed int `json:"password_max_login_failed"`
}

// PasswordValidationError holds details about why password validation failed.
type PasswordValidationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// DefaultCustomerPasswordPolicy returns default policy (all disabled, matching OTRS defaults).
func DefaultCustomerPasswordPolicy() PasswordPolicy {
	return PasswordPolicy{
		PasswordRegExp:                    "",
		PasswordMinSize:                   0,
		PasswordMin2Lower2UpperCharacters: false,
		PasswordMin2Characters:            false,
		PasswordNeedDigit:                 false,
		PasswordMaxLoginFailed:            0,
	}
}

// DefaultAgentPasswordPolicy returns default policy for agents (all disabled, matching OTRS defaults).
func DefaultAgentPasswordPolicy() PasswordPolicy {
	return PasswordPolicy{
		PasswordRegExp:                    "",
		PasswordMinSize:                   0,
		PasswordMin2Lower2UpperCharacters: false,
		PasswordMin2Characters:            false,
		PasswordNeedDigit:                 false,
		PasswordMaxLoginFailed:            0,
	}
}

// LoadCustomerPasswordPolicy loads customer password policy from sysconfig.
func LoadCustomerPasswordPolicy(db *sql.DB) (PasswordPolicy, error) {
	policy := DefaultCustomerPasswordPolicy()

	// Try to load from sysconfig_default/sysconfig_modified
	// The OTRS setting name is CustomerPreferencesGroups###Password
	settings := map[string]interface{}{
		"CustomerPreferencesGroups###Password::PasswordRegExp":                    &policy.PasswordRegExp,
		"CustomerPreferencesGroups###Password::PasswordMinSize":                   &policy.PasswordMinSize,
		"CustomerPreferencesGroups###Password::PasswordMin2Lower2UpperCharacters": &policy.PasswordMin2Lower2UpperCharacters,
		"CustomerPreferencesGroups###Password::PasswordMin2Characters":            &policy.PasswordMin2Characters,
		"CustomerPreferencesGroups###Password::PasswordNeedDigit":                 &policy.PasswordNeedDigit,
	}

	for name, target := range settings {
		loadSysconfigValue(db, name, target)
	}

	return policy, nil
}

// LoadAgentPasswordPolicy loads agent password policy from sysconfig.
func LoadAgentPasswordPolicy(db *sql.DB) (PasswordPolicy, error) {
	policy := DefaultAgentPasswordPolicy()

	// The OTRS setting name is PreferencesGroups###Password
	settings := map[string]interface{}{
		"PreferencesGroups###Password::PasswordRegExp":                    &policy.PasswordRegExp,
		"PreferencesGroups###Password::PasswordMinSize":                   &policy.PasswordMinSize,
		"PreferencesGroups###Password::PasswordMin2Lower2UpperCharacters": &policy.PasswordMin2Lower2UpperCharacters,
		"PreferencesGroups###Password::PasswordMin2Characters":            &policy.PasswordMin2Characters,
		"PreferencesGroups###Password::PasswordNeedDigit":                 &policy.PasswordNeedDigit,
		"PreferencesGroups###Password::PasswordMaxLoginFailed":            &policy.PasswordMaxLoginFailed,
	}

	for name, target := range settings {
		loadSysconfigValue(db, name, target)
	}

	return policy, nil
}

// loadSysconfigValue loads a single sysconfig value into the target pointer.
func loadSysconfigValue(db *sql.DB, name string, target interface{}) {
	// First try sysconfig_modified, then fall back to sysconfig_default
	query := database.ConvertPlaceholders(`
		SELECT COALESCE(
			(SELECT effective_value FROM sysconfig_modified WHERE name = ? AND is_valid = 1 LIMIT 1),
			(SELECT effective_value FROM sysconfig_default WHERE name = ? AND is_valid = 1 LIMIT 1)
		)
	`)

	var value sql.NullString
	err := db.QueryRow(query, name, name).Scan(&value)
	if err != nil || !value.Valid {
		return
	}

	switch t := target.(type) {
	case *string:
		*t = value.String
	case *int:
		var i int
		if _, err := parseIntFromString(value.String, &i); err == nil {
			*t = i
		}
	case *bool:
		*t = value.String == "1" || strings.ToLower(value.String) == "true"
	}
}

// parseIntFromString parses an integer from string, handling various formats.
func parseIntFromString(s string, target *int) (int, error) {
	s = strings.TrimSpace(s)
	var i int
	_, err := regexp.MatchString(`^\d+$`, s)
	if err == nil {
		for _, r := range s {
			if r >= '0' && r <= '9' {
				i = i*10 + int(r-'0')
			}
		}
		*target = i
	}
	return i, err
}

// ValidatePassword validates a password against the policy.
// Returns nil if valid, or a PasswordValidationError if invalid.
// Validation order matches OTRS: RegExp -> MinSize -> 2Lower2Upper -> NeedDigit -> 2Characters
func (p *PasswordPolicy) ValidatePassword(password string) *PasswordValidationError {
	// 1. Custom regex pattern
	if p.PasswordRegExp != "" {
		matched, err := regexp.MatchString(p.PasswordRegExp, password)
		if err != nil || !matched {
			return &PasswordValidationError{
				Code:    "regexp_mismatch",
				Message: "password_policy.regexp_mismatch",
			}
		}
	}

	// 2. Minimum size
	if p.PasswordMinSize > 0 && len(password) < p.PasswordMinSize {
		return &PasswordValidationError{
			Code:    "min_size",
			Message: "password_policy.min_size",
		}
	}

	// 3. Require 2 lowercase AND 2 uppercase
	if p.PasswordMin2Lower2UpperCharacters {
		lowerCount := 0
		upperCount := 0
		for _, r := range password {
			if unicode.IsLower(r) {
				lowerCount++
			} else if unicode.IsUpper(r) {
				upperCount++
			}
		}
		if lowerCount < 2 || upperCount < 2 {
			return &PasswordValidationError{
				Code:    "min_2_lower_2_upper",
				Message: "password_policy.min_2_lower_2_upper",
			}
		}
	}

	// 4. Require at least 1 digit
	if p.PasswordNeedDigit {
		hasDigit := false
		for _, r := range password {
			if unicode.IsDigit(r) {
				hasDigit = true
				break
			}
		}
		if !hasDigit {
			return &PasswordValidationError{
				Code:    "need_digit",
				Message: "password_policy.need_digit",
			}
		}
	}

	// 5. Require at least 2 letter characters
	if p.PasswordMin2Characters {
		letterCount := 0
		for _, r := range password {
			if unicode.IsLetter(r) {
				letterCount++
			}
		}
		if letterCount < 2 {
			return &PasswordValidationError{
				Code:    "min_2_characters",
				Message: "password_policy.min_2_characters",
			}
		}
	}

	return nil
}

// GetRequirements returns a list of active password requirements for display.
func (p *PasswordPolicy) GetRequirements() []string {
	var requirements []string

	if p.PasswordMinSize > 0 {
		requirements = append(requirements, "min_size")
	}
	if p.PasswordMin2Lower2UpperCharacters {
		requirements = append(requirements, "min_2_lower_2_upper")
	}
	if p.PasswordNeedDigit {
		requirements = append(requirements, "need_digit")
	}
	if p.PasswordMin2Characters {
		requirements = append(requirements, "min_2_characters")
	}
	if p.PasswordRegExp != "" {
		requirements = append(requirements, "regexp")
	}

	return requirements
}

// HasRequirements returns true if any password policy rules are enabled.
func (p *PasswordPolicy) HasRequirements() bool {
	return p.PasswordMinSize > 0 ||
		p.PasswordMin2Lower2UpperCharacters ||
		p.PasswordNeedDigit ||
		p.PasswordMin2Characters ||
		p.PasswordRegExp != ""
}
