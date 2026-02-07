package service

import (
	"database/sql"
	"fmt"
	"strconv"

	"github.com/goatkit/goatflow/internal/constants"
	"github.com/goatkit/goatflow/internal/database"
)

// CustomerPreferencesService handles customer preference operations.
// Note: customer_preferences uses string user_id (login) unlike user_preferences which uses numeric ID.
type CustomerPreferencesService struct {
	db *sql.DB
}

// NewCustomerPreferencesService creates a new customer preferences service.
func NewCustomerPreferencesService(db *sql.DB) *CustomerPreferencesService {
	return &CustomerPreferencesService{db: db}
}

// GetPreference retrieves a customer preference by key.
func (s *CustomerPreferencesService) GetPreference(userLogin string, key string) (string, error) {
	var value sql.NullString
	query := database.ConvertPlaceholders(`
		SELECT preferences_value
		FROM customer_preferences
		WHERE user_id = ? AND preferences_key = ?
	`)

	err := s.db.QueryRow(query, userLogin, key).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil // No preference set
		}
		return "", fmt.Errorf("failed to get preference: %w", err)
	}

	if !value.Valid {
		return "", nil
	}
	return value.String, nil
}

// SetPreference sets a customer preference.
// Uses delete-then-insert within a transaction to avoid duplicates and ensure atomicity.
func (s *CustomerPreferencesService) SetPreference(userLogin string, key string, value string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Delete any existing rows for this user_id + key
	deleteQuery := database.ConvertPlaceholders(`
		DELETE FROM customer_preferences
		WHERE user_id = ? AND preferences_key = ?
	`)

	_, err = tx.Exec(deleteQuery, userLogin, key)
	if err != nil {
		return fmt.Errorf("failed to delete existing preference: %w", err)
	}

	// Insert the new value
	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO customer_preferences (user_id, preferences_key, preferences_value)
		VALUES (?, ?, ?)
	`)

	_, err = tx.Exec(insertQuery, userLogin, key, value)
	if err != nil {
		return fmt.Errorf("failed to insert preference: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// DeletePreference removes a customer preference.
func (s *CustomerPreferencesService) DeletePreference(userLogin string, key string) error {
	query := database.ConvertPlaceholders(`
		DELETE FROM customer_preferences WHERE user_id = ? AND preferences_key = ?
	`)

	_, err := s.db.Exec(query, userLogin, key)
	if err != nil {
		return fmt.Errorf("failed to delete preference: %w", err)
	}

	return nil
}

// GetSessionTimeout returns the customer's preferred session timeout.
// Returns 0 if no preference is set (use system default).
func (s *CustomerPreferencesService) GetSessionTimeout(userLogin string) int {
	value, err := s.GetPreference(userLogin, "SessionTimeout")
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

// SetSessionTimeout sets the customer's preferred session timeout.
func (s *CustomerPreferencesService) SetSessionTimeout(userLogin string, timeout int) error {
	// Enforce limits
	if timeout != 0 { // 0 means use system default
		if timeout < constants.MinSessionTimeout {
			timeout = constants.MinSessionTimeout
		} else if timeout > constants.MaxSessionTimeout {
			timeout = constants.MaxSessionTimeout
		}
	}

	return s.SetPreference(userLogin, "SessionTimeout", strconv.Itoa(timeout))
}

// GetLanguage returns the customer's preferred language.
// Returns empty string if no preference is set (use system detection).
func (s *CustomerPreferencesService) GetLanguage(userLogin string) string {
	value, err := s.GetPreference(userLogin, "Language")
	if err != nil || value == "" {
		return ""
	}
	return value
}

// SetLanguage sets the customer's preferred language.
func (s *CustomerPreferencesService) SetLanguage(userLogin string, lang string) error {
	if lang == "" {
		// Empty language means "use system default" - delete the preference
		return s.DeletePreference(userLogin, "Language")
	}
	return s.SetPreference(userLogin, "Language", lang)
}

// GetTheme returns the customer's preferred theme.
func (s *CustomerPreferencesService) GetTheme(userLogin string) string {
	value, err := s.GetPreference(userLogin, "Theme")
	if err != nil || value == "" {
		return ""
	}
	return value
}

// SetTheme sets the customer's preferred theme.
func (s *CustomerPreferencesService) SetTheme(userLogin string, theme string) error {
	if theme == "" {
		return s.DeletePreference(userLogin, "Theme")
	}
	return s.SetPreference(userLogin, "Theme", theme)
}

// GetThemeMode returns the customer's preferred theme mode (light/dark).
func (s *CustomerPreferencesService) GetThemeMode(userLogin string) string {
	value, err := s.GetPreference(userLogin, "ThemeMode")
	if err != nil || value == "" {
		return ""
	}
	return value
}

// SetThemeMode sets the customer's preferred theme mode (light/dark).
func (s *CustomerPreferencesService) SetThemeMode(userLogin string, mode string) error {
	if mode == "" {
		return s.DeletePreference(userLogin, "ThemeMode")
	}
	return s.SetPreference(userLogin, "ThemeMode", mode)
}

// GetAllPreferences returns all preferences for a customer.
func (s *CustomerPreferencesService) GetAllPreferences(userLogin string) (map[string]string, error) {
	query := database.ConvertPlaceholders(`
		SELECT preferences_key, preferences_value
		FROM customer_preferences
		WHERE user_id = ?
	`)

	rows, err := s.db.Query(query, userLogin)
	if err != nil {
		return nil, fmt.Errorf("failed to get all preferences: %w", err)
	}
	defer rows.Close()

	prefs := make(map[string]string)
	for rows.Next() {
		var key string
		var value sql.NullString

		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("failed to scan preference: %w", err)
		}

		if value.Valid {
			prefs[key] = value.String
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate preferences: %w", err)
	}

	return prefs, nil
}
