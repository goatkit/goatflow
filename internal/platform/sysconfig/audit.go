package sysconfig

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// AuditSettingsKey holds the audit log settings as one JSON value.
const AuditSettingsKey = "Audit::Settings"

// Audit retention bounds; 0 keeps events forever.
const (
	AuditDefaultRetentionDays = 365
	AuditMaxRetentionDays     = 3650
	auditReadAccessCacheTTL   = time.Minute
)

// AuditSettings configures the audit log: how long events are kept and
// whether read access (ticket views, downloads, exports) is recorded.
type AuditSettings struct {
	RetentionDays    int  `json:"retention_days"`
	RecordReadAccess bool `json:"record_read_access"`
}

// DefaultAuditSettings keeps events for a year and does not record reads.
func DefaultAuditSettings() AuditSettings {
	return AuditSettings{RetentionDays: AuditDefaultRetentionDays}
}

// Validate reports a setting outside its bounds.
func (s AuditSettings) Validate() error {
	if s.RetentionDays < 0 || s.RetentionDays > AuditMaxRetentionDays {
		return fmt.Errorf("retention_days must be between 0 and %d", AuditMaxRetentionDays)
	}
	return nil
}

// LoadAuditSettings reads the audit settings. A missing value is the
// default; a value that does not parse is an error with the default returned.
func LoadAuditSettings(db *sql.DB) (AuditSettings, error) {
	cfg := DefaultAuditSettings()
	raw, ok := sysconfigValue(db, AuditSettingsKey)
	if !ok || strings.TrimSpace(raw) == "" {
		return cfg, nil
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return DefaultAuditSettings(), fmt.Errorf("%s: %w", AuditSettingsKey, err)
	}
	if err := cfg.Validate(); err != nil {
		return DefaultAuditSettings(), fmt.Errorf("%s: %w", AuditSettingsKey, err)
	}
	return cfg, nil
}

// SaveAuditSettings validates and stores the settings. Validation failures
// come back as *AuditSettingsValidationError.
func SaveAuditSettings(db *sql.DB, cfg AuditSettings, userID int) error {
	if err := cfg.Validate(); err != nil {
		return &AuditSettingsValidationError{Err: err}
	}
	if db == nil {
		return fmt.Errorf("database connection unavailable")
	}
	value, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	defaultValue, _ := json.Marshal(DefaultAuditSettings())
	if err := ensureSysconfigDefault(db, sysconfigDefault{
		name:        AuditSettingsKey,
		description: "Audit log retention (days, 0 keeps forever) and read-access recording.",
		navigation:  "Core::Security",
		xmlFilename: "Framework.xml",
		xml:         `{"type":"json"}`,
		value:       string(defaultValue),
	}, userID); err != nil {
		return fmt.Errorf("sysconfig unavailable: %w", err)
	}
	if err := upsertSysconfigValue(db, AuditSettingsKey, string(value), userID); err != nil {
		return fmt.Errorf("sysconfig unavailable: %w", err)
	}
	InvalidateAuditReadAccess()
	return nil
}

// AuditSettingsValidationError marks settings the caller must correct.
type AuditSettingsValidationError struct{ Err error }

func (e *AuditSettingsValidationError) Error() string { return e.Err.Error() }
func (e *AuditSettingsValidationError) Unwrap() error { return e.Err }

var auditReadAccess struct {
	mu      sync.Mutex
	value   bool
	expires time.Time
}

// AuditReadAccessEnabled reports whether read access is recorded. The value
// is cached for a minute so ticket views do not read sysconfig each time.
func AuditReadAccessEnabled(db *sql.DB) bool {
	auditReadAccess.mu.Lock()
	defer auditReadAccess.mu.Unlock()
	now := time.Now()
	if now.Before(auditReadAccess.expires) {
		return auditReadAccess.value
	}
	enabled := false
	if db != nil {
		cfg, _ := LoadAuditSettings(db)
		enabled = cfg.RecordReadAccess
	}
	auditReadAccess.value = enabled
	auditReadAccess.expires = now.Add(auditReadAccessCacheTTL)
	return enabled
}

// InvalidateAuditReadAccess drops the cached read-access flag.
func InvalidateAuditReadAccess() {
	auditReadAccess.mu.Lock()
	auditReadAccess.expires = time.Time{}
	auditReadAccess.mu.Unlock()
}
