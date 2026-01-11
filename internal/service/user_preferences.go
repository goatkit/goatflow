package service

import (
	"database/sql"
	"fmt"
	"strconv"

	"github.com/gotrs-io/gotrs-ce/internal/constants"
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
	query := `
		SELECT preferences_value 
		FROM user_preferences 
		WHERE user_id = ? AND preferences_key = ?
	`

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
func (s *UserPreferencesService) SetPreference(userID int, key string, value string) error {
	// First, try to update existing preference
	updateQuery := `
		UPDATE user_preferences
		SET preferences_value = ?
		WHERE user_id = ? AND preferences_key = ?
	`

	// Parameters must match query order: value, userID, key
	result, err := s.db.Exec(updateQuery, []byte(value), userID, key)
	if err != nil {
		return fmt.Errorf("failed to update preference: %w", err)
	}

	// Check if any rows were updated
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	// If no rows were updated, insert new preference
	if rowsAffected == 0 {
		insertQuery := `
			INSERT INTO user_preferences (user_id, preferences_key, preferences_value)
			VALUES (?, ?, ?)
		`

		_, err = s.db.Exec(insertQuery, userID, key, []byte(value))
		if err != nil {
			return fmt.Errorf("failed to insert preference: %w", err)
		}
	}

	return nil
}

// DeletePreference removes a user preference.
func (s *UserPreferencesService) DeletePreference(userID int, key string) error {
	query := `DELETE FROM user_preferences WHERE user_id = ? AND preferences_key = ?`

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

// GetAllPreferences returns all preferences for a user.
func (s *UserPreferencesService) GetAllPreferences(userID int) (map[string]string, error) {
	query := `
		SELECT preferences_key, preferences_value 
		FROM user_preferences 
		WHERE user_id = ?
	`

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
