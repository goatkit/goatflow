package i18n

import (
	"testing"
)

func TestI18nInitialization(t *testing.T) {
	// Test that i18n instance can be created
	i18n := GetInstance()
	if i18n == nil {
		t.Fatal("Failed to get i18n instance")
	}
}

func TestSupportedLanguages(t *testing.T) {
	i18n := GetInstance()
	langs := i18n.GetSupportedLanguages()

	// Should have at least English and German
	if len(langs) < 2 {
		t.Errorf("Expected at least 2 languages, got %d", len(langs))
	}

	// Check that English and German are supported
	hasEn := false
	hasDe := false
	for _, lang := range langs {
		if lang == "en" {
			hasEn = true
		}
		if lang == "de" {
			hasDe = true
		}
	}

	if !hasEn {
		t.Error("English (en) should be supported")
	}
	if !hasDe {
		t.Error("German (de) should be supported")
	}
}

func TestTranslationKeys(t *testing.T) {
	i18n := GetInstance()

	tests := []struct {
		name        string
		lang        string
		key         string
		expected    string
		shouldExist bool
	}{
		// English translations
		{"English app name", "en", "app.name", "GoatFlow", true},
		{"English dashboard", "en", "navigation.dashboard", "Dashboard", true},
		{"English queues", "en", "navigation.queues", "Queues", true},
		{"English admin dashboard", "en", "admin.dashboard", "Admin Dashboard", true},
		{"English queue new", "en", "queues.new_queue", "New Queue", true},

		// German translations
		{"German app name", "de", "app.name", "GoatFlow", true},
		{"German dashboard", "de", "navigation.dashboard", "Übersicht", true},
		{"German queues", "de", "navigation.queues", "Warteschlangen", true},
		{"German admin dashboard", "de", "admin.dashboard", "Admin-Dashboard", true},
		{"German queue new", "de", "queues.new_queue", "Neue Queue", true},

		// Common translations
		{"English all", "en", "common.all", "All", true},
		{"German all", "de", "common.all", "Alle", true},
		{"English unassigned", "en", "common.unassigned", "Unassigned", true},
		{"German unassigned", "de", "common.unassigned", "Nicht zugewiesen", true},

		// Status translations
		{"English status active", "en", "status.active", "Active", true},
		{"German status active", "de", "status.active", "Aktiv", true},

		// Non-existent key should return the key itself
		{"Non-existent key", "en", "non.existent.key", "non.existent.key", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := i18n.T(tt.lang, tt.key)

			if tt.shouldExist {
				if result != tt.expected {
					t.Errorf("T(%q, %q) = %q; want %q", tt.lang, tt.key, result, tt.expected)
				}
			} else {
				// For non-existent keys, should return the key itself
				if result != tt.key {
					t.Errorf("T(%q, %q) should return key itself for missing translation, got %q", tt.lang, tt.key, result)
				}
			}
		})
	}
}

func TestNestedTranslations(t *testing.T) {
	i18n := GetInstance()

	// Test that nested keys work correctly
	tests := []struct {
		lang string
		key  string
	}{
		{"en", "admin.dashboard"},
		{"en", "admin.total_users"},
		{"en", "queues.new_queue"},
		{"en", "tickets.ticket"},
		{"de", "admin.dashboard"},
		{"de", "queues.new_queue"},
	}

	for _, tt := range tests {
		result := i18n.T(tt.lang, tt.key)
		// Should not return the key itself (meaning translation was found)
		if result == tt.key {
			t.Errorf("Translation missing for %s in language %s", tt.key, tt.lang)
		}
	}
}

func TestFallbackToEnglish(t *testing.T) {
	i18n := GetInstance()

	// Test fallback for unsupported language
	result := i18n.T("fr", "app.name")
	if result != "GoatFlow" {
		t.Errorf("Should fallback to English for unsupported language, got %q", result)
	}
}

func TestTranslationWithArgs(t *testing.T) {
	i18n := GetInstance()

	// Note: This test assumes the translation system supports placeholders
	// We may need to adjust based on actual implementation
	tests := []struct {
		lang     string
		key      string
		args     []interface{}
		expected string
	}{
		// If we have translations with placeholders like "Showing %d results"
		// {"en", "common.showing_count", []interface{}{10}, "Showing 10 results"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := i18n.T(tt.lang, tt.key, tt.args...)
			if result != tt.expected {
				t.Errorf("T(%q, %q, %v) = %q; want %q", tt.lang, tt.key, tt.args, result, tt.expected)
			}
		})
	}
}
