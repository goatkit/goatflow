// Package sysconfig provides system configuration management.
package sysconfig

import (
	"database/sql"
	_ "embed"
	"fmt"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/goatkit/goatflow/internal/database"
)

// CustomerPortalConfig holds global portal settings stored in sysconfig tables.
type CustomerPortalConfig struct {
	Enabled       bool
	LoginRequired bool
	Title         string
	FooterText    string
	LandingPage   string
}

type portalKeyDef struct {
	name        string
	description string
	xml         string
	defaultVal  string
}

func portalKeyDefs() []portalKeyDef {
	return []portalKeyDef{
		{"CustomerPortal::Enabled", "Allow customers to access the portal and ticket UI.", `{"type":"boolean","default":true}`, "true"},
		{"CustomerPortal::LoginRequired", "Require customer authentication before accessing the portal.", `{"type":"boolean","default":true}`, "true"},
		{"CustomerPortal::Title", "Portal title shown in header and HTML title.", `{"type":"string","default":"Customer Portal"}`, "Customer Portal"},
		{"CustomerPortal::FooterText", "Footer text displayed on customer portal pages.", `{"type":"string","default":"Powered by GoatFlow"}`, "Powered by GoatFlow"},
		{"CustomerPortal::LandingPage", "Relative path used after login (or on portal entry).", `{"type":"string","default":"/customer"}`, "/customer"},
	}
}

// DefaultCustomerPortalConfig returns the built-in defaults when no sysconfig rows exist.
func DefaultCustomerPortalConfig() CustomerPortalConfig {
	return CustomerPortalConfig{
		Enabled:       true,
		LoginRequired: true,
		Title:         "Customer Portal",
		FooterText:    "Powered by GoatFlow",
		LandingPage:   "/customer/tickets",
	}
}

func portalKeyName(base, customerID string) string {
	customerID = strings.TrimSpace(customerID)
	if customerID == "" {
		return base
	}
	return fmt.Sprintf("%s::%s", base, customerID)
}

func portalKeyNames(customerID string) map[string]string {
	return map[string]string{
		"enabled": portalKeyName("CustomerPortal::Enabled", customerID),
		"login":   portalKeyName("CustomerPortal::LoginRequired", customerID),
		"title":   portalKeyName("CustomerPortal::Title", customerID),
		"footer":  portalKeyName("CustomerPortal::FooterText", customerID),
		"landing": portalKeyName("CustomerPortal::LandingPage", customerID),
	}
}

// LoadCustomerPortalConfig reads portal settings from sysconfig tables.
func LoadCustomerPortalConfig(db *sql.DB) (CustomerPortalConfig, error) {
	cfg := DefaultCustomerPortalConfig()

	if db == nil {
		return cfg, fmt.Errorf("database connection unavailable")
	}

	if val, ok := sysconfigValue(db, "CustomerPortal::Enabled"); ok {
		cfg.Enabled = strings.EqualFold(val, "true")
	}
	if val, ok := sysconfigValue(db, "CustomerPortal::LoginRequired"); ok {
		cfg.LoginRequired = strings.EqualFold(val, "true")
	}
	if val, ok := sysconfigValue(db, "CustomerPortal::Title"); ok && val != "" {
		cfg.Title = val
	}
	if val, ok := sysconfigValue(db, "CustomerPortal::FooterText"); ok && val != "" {
		cfg.FooterText = val
	}
	if val, ok := sysconfigValue(db, "CustomerPortal::LandingPage"); ok && val != "" {
		cfg.LandingPage = val
	}

	return cfg, nil
}

// LoadCustomerPortalConfigForCompany returns overrides for a specific customer when present, falling back to global values.
func LoadCustomerPortalConfigForCompany(db *sql.DB, customerID string) (CustomerPortalConfig, error) {
	customerID = strings.TrimSpace(customerID)
	if customerID == "" {
		return LoadCustomerPortalConfig(db)
	}

	cfg, _ := LoadCustomerPortalConfig(db)
	names := portalKeyNames(customerID)

	if val, ok := sysconfigValue(db, names["enabled"]); ok {
		cfg.Enabled = strings.EqualFold(val, "true")
	}
	if val, ok := sysconfigValue(db, names["login"]); ok {
		cfg.LoginRequired = strings.EqualFold(val, "true")
	}
	if val, ok := sysconfigValue(db, names["title"]); ok && val != "" {
		cfg.Title = val
	}
	if val, ok := sysconfigValue(db, names["footer"]); ok && val != "" {
		cfg.FooterText = val
	}
	if val, ok := sysconfigValue(db, names["landing"]); ok && val != "" {
		cfg.LandingPage = val
	}

	return cfg, nil
}

func ensurePortalDefault(db *sql.DB, targetName string, def portalKeyDef, userID int) error {
	if db == nil {
		return fmt.Errorf("database connection unavailable")
	}
	if userID == 0 {
		userID = 1
	}

	var id int
	err := db.QueryRow(database.ConvertPlaceholders(`
		SELECT id FROM sysconfig_default WHERE name = ? LIMIT 1
	`), targetName).Scan(&id)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}

	if database.IsMySQL() {
		_, insertErr := db.Exec(database.ConvertPlaceholders(`
			INSERT INTO sysconfig_default (
				name, description, navigation, is_invisible, is_readonly, is_required, is_valid,
				has_configlevel, user_modification_possible, user_modification_active, user_preferences_group,
				xml_content_raw, xml_content_parsed, xml_filename, effective_value, is_dirty,
				exclusive_lock_guid, exclusive_lock_user_id, exclusive_lock_expiry_time,
				create_time, create_by, change_time, change_by
			) VALUES (
				?, ?, 'Frontend::Customer::Portal', 0, 0, 0, 1,
				0, 1, 1, NULL,
				?, ?, 'CustomerPortal.xml', ?, 0,
				'', NULL, NULL,
				CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP, ?
			)
			ON DUPLICATE KEY UPDATE
				description = VALUES(description),
				xml_content_raw = VALUES(xml_content_raw),
				xml_content_parsed = VALUES(xml_content_parsed),
				effective_value = VALUES(effective_value),
				change_time = CURRENT_TIMESTAMP,
				change_by = VALUES(change_by)
		`), targetName, def.description, def.xml, def.xml, def.defaultVal, userID, userID)
		return insertErr
	}

	insertErr := db.QueryRow(database.ConvertPlaceholders(`
		INSERT INTO sysconfig_default (
			name, description, navigation, is_invisible, is_readonly, is_required, is_valid,
			has_configlevel, user_modification_possible, user_modification_active, user_preferences_group,
			xml_content_raw, xml_content_parsed, xml_filename, effective_value, is_dirty,
			exclusive_lock_guid, exclusive_lock_user_id, exclusive_lock_expiry_time,
			create_time, create_by, change_time, change_by
		) VALUES (
			?, ?, 'Frontend::Customer::Portal', 0, 0, 0, 1,
			0, 1, 1, NULL,
			?, ?, 'CustomerPortal.xml', ?, 0,
			'', NULL, NULL,
			CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP, ?
		)
		ON CONFLICT (name) DO UPDATE SET
			description = EXCLUDED.description,
			xml_content_raw = EXCLUDED.xml_content_raw,
			xml_content_parsed = EXCLUDED.xml_content_parsed,
			effective_value = EXCLUDED.effective_value,
			change_time = EXCLUDED.change_time,
			change_by = EXCLUDED.change_by
		RETURNING id
	`), targetName, def.description, def.xml, def.xml, def.defaultVal, userID).Scan(&id)
	return insertErr
}

// SaveCustomerPortalConfig persists portal settings as sysconfig overrides.
func SaveCustomerPortalConfig(db *sql.DB, cfg CustomerPortalConfig, userID int) error {
	if db == nil {
		return fmt.Errorf("database connection unavailable")
	}

	for _, def := range portalKeyDefs() {
		targetName := portalKeyName(def.name, "")
		if err := ensurePortalDefault(db, targetName, def, userID); err != nil {
			return fmt.Errorf("sysconfig unavailable: %w", err)
		}
	}

	if err := upsertSysconfigValue(db, "CustomerPortal::Enabled", boolToString(cfg.Enabled), userID); err != nil {
		return fmt.Errorf("sysconfig unavailable: %w", err)
	}
	if err := upsertSysconfigValue(db, "CustomerPortal::LoginRequired", boolToString(cfg.LoginRequired), userID); err != nil {
		return fmt.Errorf("sysconfig unavailable: %w", err)
	}
	if err := upsertSysconfigValue(db, "CustomerPortal::Title", cfg.Title, userID); err != nil {
		return fmt.Errorf("sysconfig unavailable: %w", err)
	}
	if err := upsertSysconfigValue(db, "CustomerPortal::FooterText", cfg.FooterText, userID); err != nil {
		return fmt.Errorf("sysconfig unavailable: %w", err)
	}
	if err := upsertSysconfigValue(db, "CustomerPortal::LandingPage", cfg.LandingPage, userID); err != nil {
		return fmt.Errorf("sysconfig unavailable: %w", err)
	}

	return nil
}

// SaveCustomerPortalConfigForCompany writes overrides scoped to a specific customer, creating defaults on demand.
func SaveCustomerPortalConfigForCompany(db *sql.DB, customerID string, cfg CustomerPortalConfig, userID int) error {
	customerID = strings.TrimSpace(customerID)
	if customerID == "" {
		return SaveCustomerPortalConfig(db, cfg, userID)
	}
	if db == nil {
		return fmt.Errorf("database connection unavailable")
	}

	names := portalKeyNames(customerID)
	for _, def := range portalKeyDefs() {
		targetName := portalKeyName(def.name, customerID)
		if err := ensurePortalDefault(db, targetName, def, userID); err != nil {
			return fmt.Errorf("sysconfig unavailable: %w", err)
		}
	}

	if err := upsertSysconfigValue(db, names["enabled"], boolToString(cfg.Enabled), userID); err != nil {
		return fmt.Errorf("sysconfig unavailable: %w", err)
	}
	if err := upsertSysconfigValue(db, names["login"], boolToString(cfg.LoginRequired), userID); err != nil {
		return fmt.Errorf("sysconfig unavailable: %w", err)
	}
	if err := upsertSysconfigValue(db, names["title"], cfg.Title, userID); err != nil {
		return fmt.Errorf("sysconfig unavailable: %w", err)
	}
	if err := upsertSysconfigValue(db, names["footer"], cfg.FooterText, userID); err != nil {
		return fmt.Errorf("sysconfig unavailable: %w", err)
	}
	if err := upsertSysconfigValue(db, names["landing"], cfg.LandingPage, userID); err != nil {
		return fmt.Errorf("sysconfig unavailable: %w", err)
	}

	return nil
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

//go:embed defaults.yaml
var portalDefaultsYAML []byte

var (
	portalDefaults     map[string]string
	portalDefaultsOnce sync.Once
)

func loadPortalDefaults() map[string]string {
	portalDefaultsOnce.Do(func() {
		portalDefaults = map[string]string{}
		var parsed map[string]string
		if err := yaml.Unmarshal(portalDefaultsYAML, &parsed); err == nil {
			portalDefaults = parsed
		}
	})
	return portalDefaults
}

func sysconfigValue(db *sql.DB, name string) (string, bool) {
	if db == nil {
		if val, ok := loadPortalDefaults()[name]; ok {
			return val, true
		}
		return "", false
	}

	var value sql.NullString
	fallback := func() (string, bool) {
		if val, ok := loadPortalDefaults()[name]; ok {
			return val, true
		}
		return "", false
	}

	// Prefer modified overrides first
	err := db.QueryRow(database.ConvertPlaceholders(`
		SELECT effective_value
		  FROM sysconfig_modified
		 WHERE name = ? AND is_valid = 1
	  ORDER BY change_time DESC
		 LIMIT 1
	`), name).Scan(&value)
	if err == nil && value.Valid {
		return value.String, true
	}
	if err != nil && err != sql.ErrNoRows {
		if isSysconfigMissing(err) {
			return fallback()
		}
		return "", false
	}

	// Fallback to default
	err = db.QueryRow(database.ConvertPlaceholders(`
		SELECT effective_value
		  FROM sysconfig_default
		 WHERE name = ? AND is_valid = 1
		 LIMIT 1
	`), name).Scan(&value)
	if err == nil && value.Valid {
		return value.String, true
	}
	if err != nil {
		if isSysconfigMissing(err) || err == sql.ErrNoRows {
			return fallback()
		}
		return "", false
	}

	return fallback()
}

func upsertSysconfigValue(db *sql.DB, name, value string, userID int) error {
	if userID == 0 {
		userID = 1
	}

	var defaultID int
	err := db.QueryRow(database.ConvertPlaceholders(`
		SELECT id FROM sysconfig_default WHERE name = ? AND is_valid = 1 LIMIT 1
	`), name).Scan(&defaultID)
	if err != nil {
		return fmt.Errorf("sysconfig default missing for %s: %w", name, err)
	}

	// Try update existing override
	result, err := db.Exec(database.ConvertPlaceholders(`
		UPDATE sysconfig_modified
		   SET effective_value = ?,
		       is_valid = 1,
		       user_modification_active = 1,
		       is_dirty = 0,
		       reset_to_default = 0,
		       change_by = ?,
		       change_time = CURRENT_TIMESTAMP
		 WHERE name = ?
	`), value, userID, name)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows > 0 {
		return nil
	}

	_, err = db.Exec(database.ConvertPlaceholders(`
		INSERT INTO sysconfig_modified (
			sysconfig_default_id, name, user_id, is_valid, user_modification_active,
			effective_value, is_dirty, reset_to_default, create_time, create_by, change_time, change_by
		)
		VALUES (?, ?, NULL, 1, 1, ?, 0, 0, CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP, ?)
	`), defaultID, name, value, userID, userID)
	return err
}

func isSysconfigMissing(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "does not exist") || strings.Contains(msg, "undefined table") || strings.Contains(msg, "relation does not exist") || strings.Contains(msg, "no such table")
}
