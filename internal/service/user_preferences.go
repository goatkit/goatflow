package service

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/goatkit/goatflow/internal/constants"
	"github.com/goatkit/goatflow/internal/database"
)

// UserPreferencesService handles user preference operations.
type UserPreferencesService struct {
	db *sql.DB
}

// NewUserPreferencesService creates a new user preferences service.
func NewUserPreferencesService(db *sql.DB) *UserPreferencesService {
	return &UserPreferencesService{db: db}
}

// GetPreference retrieves a user preference by key.
func (s *UserPreferencesService) GetPreference(userID int, key string) (string, error) {
	var value []byte
	query := database.ConvertPlaceholders(`
		SELECT preferences_value
		FROM user_preferences
		WHERE user_id = ? AND preferences_key = ?
	`)

	err := s.db.QueryRow(query, userID, key).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil // No preference set
		}
		return "", fmt.Errorf("failed to get preference: %w", err)
	}

	return string(value), nil
}

// SetPreference sets a user preference.
// Uses delete-then-insert within a transaction to avoid duplicates and ensure atomicity.
// (MySQL reports 0 rows affected when UPDATE sets the same value, which would cause
// incorrect INSERT with the old update-then-insert pattern.)
func (s *UserPreferencesService) SetPreference(userID int, key string, value string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Delete any existing rows for this user_id + key (handles duplicates too)
	deleteQuery := database.ConvertPlaceholders(`
		DELETE FROM user_preferences
		WHERE user_id = ? AND preferences_key = ?
	`)

	_, err = tx.Exec(deleteQuery, userID, key)
	if err != nil {
		return fmt.Errorf("failed to delete existing preference: %w", err)
	}

	// Insert the new value
	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO user_preferences (user_id, preferences_key, preferences_value)
		VALUES (?, ?, ?)
	`)

	_, err = tx.Exec(insertQuery, userID, key, []byte(value))
	if err != nil {
		return fmt.Errorf("failed to insert preference: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// DeletePreference removes a user preference.
func (s *UserPreferencesService) DeletePreference(userID int, key string) error {
	query := database.ConvertPlaceholders(`
		DELETE FROM user_preferences WHERE user_id = ? AND preferences_key = ?
	`)

	_, err := s.db.Exec(query, userID, key)
	if err != nil {
		return fmt.Errorf("failed to delete preference: %w", err)
	}

	return nil
}

// Returns 0 if no preference is set (use system default).
func (s *UserPreferencesService) GetSessionTimeout(userID int) int {
	value, err := s.GetPreference(userID, "SessionTimeout")
	if err != nil || value == "" {
		return 0
	}

	timeout, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	if timeout == 0 {
		return 0
	}

	// Enforce limits
	if timeout < constants.MinSessionTimeout {
		timeout = constants.MinSessionTimeout
	} else if timeout > constants.MaxSessionTimeout {
		timeout = constants.MaxSessionTimeout
	}

	return timeout
}

// SetSessionTimeout sets the user's preferred session timeout.
func (s *UserPreferencesService) SetSessionTimeout(userID int, timeout int) error {
	// Enforce limits
	if timeout != 0 { // 0 means use system default
		if timeout < constants.MinSessionTimeout {
			timeout = constants.MinSessionTimeout
		} else if timeout > constants.MaxSessionTimeout {
			timeout = constants.MaxSessionTimeout
		}
	}

	return s.SetPreference(userID, "SessionTimeout", strconv.Itoa(timeout))
}

// GetLanguage returns the user's preferred language.
// Returns empty string if no preference is set (use system detection).
func (s *UserPreferencesService) GetLanguage(userID int) string {
	value, err := s.GetPreference(userID, "Language")
	if err != nil || value == "" {
		return ""
	}
	return value
}

// SetLanguage sets the user's preferred language.
func (s *UserPreferencesService) SetLanguage(userID int, lang string) error {
	if lang == "" {
		// Empty language means "use system default" - delete the preference
		return s.DeletePreference(userID, "Language")
	}
	return s.SetPreference(userID, "Language", lang)
}

// GetTheme returns the user's preferred theme.
// Returns empty string if no preference is set.
func (s *UserPreferencesService) GetTheme(userID int) string {
	value, err := s.GetPreference(userID, "Theme")
	if err != nil || value == "" {
		return ""
	}
	return value
}

// SetTheme sets the user's preferred theme.
func (s *UserPreferencesService) SetTheme(userID int, theme string) error {
	if theme == "" {
		return s.DeletePreference(userID, "Theme")
	}
	return s.SetPreference(userID, "Theme", theme)
}

// GetThemeMode returns the user's preferred theme mode (light/dark).
// Returns empty string if no preference is set.
func (s *UserPreferencesService) GetThemeMode(userID int) string {
	value, err := s.GetPreference(userID, "ThemeMode")
	if err != nil || value == "" {
		return ""
	}
	return value
}

// SetThemeMode sets the user's preferred theme mode (light/dark).
func (s *UserPreferencesService) SetThemeMode(userID int, mode string) error {
	if mode == "" {
		return s.DeletePreference(userID, "ThemeMode")
	}
	return s.SetPreference(userID, "ThemeMode", mode)
}

// GetRemindersEnabled returns whether pending reminders are enabled for the user.
// Returns true if no preference is set (default: enabled).
func (s *UserPreferencesService) GetRemindersEnabled(userID int) bool {
	value, err := s.GetPreference(userID, "RemindersEnabled")
	if err != nil || value == "" {
		return true // Default: enabled
	}
	return value != "0"
}

// SetRemindersEnabled sets whether pending reminders are enabled for the user.
func (s *UserPreferencesService) SetRemindersEnabled(userID int, enabled bool) error {
	value := "1"
	if !enabled {
		value = "0"
	}
	return s.SetPreference(userID, "RemindersEnabled", value)
}

// GetAllPreferences returns all preferences for a user.
func (s *UserPreferencesService) GetAllPreferences(userID int) (map[string]string, error) {
	query := database.ConvertPlaceholders(`
		SELECT preferences_key, preferences_value
		FROM user_preferences
		WHERE user_id = ?
	`)

	rows, err := s.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get all preferences: %w", err)
	}
	defer rows.Close()

	prefs := make(map[string]string)
	for rows.Next() {
		var key string
		var value []byte

		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("failed to scan preference: %w", err)
		}

		prefs[key] = string(value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate preferences: %w", err)
	}

	return prefs, nil
}

// DashboardWidgetConfig represents the configuration for a dashboard widget.
type DashboardWidgetConfig struct {
	WidgetID   string `json:"widget_id"`          // Format: "plugin_name:widget_id"
	Enabled    bool   `json:"enabled"`
	Position   int    `json:"position"`            // Order on dashboard (lower = first)
	X          int    `json:"x"`                   // Grid column position
	Y          int    `json:"y"`                   // Grid row position
	W          int    `json:"w,omitempty"`          // Grid width (columns)
	H          int    `json:"h,omitempty"`          // Grid height (rows)
}

// GetDashboardWidgets returns the user's dashboard widget configuration.
// Returns nil if no configuration is set (show all widgets with defaults).
func (s *UserPreferencesService) GetDashboardWidgets(userID int) ([]DashboardWidgetConfig, error) {
	value, err := s.GetPreference(userID, "DashboardWidgets")
	if err != nil {
		return nil, err
	}
	if value == "" {
		return nil, nil // No config = show defaults
	}

	var config []DashboardWidgetConfig
	if err := json.Unmarshal([]byte(value), &config); err != nil {
		return nil, fmt.Errorf("failed to parse dashboard widget config: %w", err)
	}

	return config, nil
}

// SetDashboardWidgets saves the user's dashboard widget configuration.
func (s *UserPreferencesService) SetDashboardWidgets(userID int, config []DashboardWidgetConfig) error {
	if len(config) == 0 {
		return s.DeletePreference(userID, "DashboardWidgets")
	}

	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal dashboard widget config: %w", err)
	}

	return s.SetPreference(userID, "DashboardWidgets", string(data))
}

// IsWidgetEnabled checks if a specific widget is enabled for the user.
// If no config exists, returns true (all widgets enabled by default).
func (s *UserPreferencesService) IsWidgetEnabled(userID int, pluginName, widgetID string) bool {
	config, err := s.GetDashboardWidgets(userID)
	if err != nil || config == nil {
		return true // Default: all widgets enabled
	}

	fullID := pluginName + ":" + widgetID
	for _, w := range config {
		if w.WidgetID == fullID {
			return w.Enabled
		}
	}

	return true // Widget not in config = enabled by default
}
