package i18n

import (
	"testing"
)

func TestUITranslations(t *testing.T) {
	i18n := GetInstance()

	// Test cases for common UI text that should be translated
	tests := []struct {
		name        string
		lang        string
		key         string
		shouldExist bool
		notEqual    string // Should not equal this value (to ensure translation happened)
	}{
		// Welcome messages
		{"English welcome back", "en", "dashboard.welcome_back", true, "dashboard.welcome_back"},
		{"German welcome back", "de", "dashboard.welcome_back", true, "dashboard.welcome_back"},
		{"English welcome message", "en", "dashboard.welcome_message", true, "dashboard.welcome_message"},

		// User menu items
		{"English profile", "en", "user.profile", true, "user.profile"},
		{"English settings", "en", "user.settings", true, "user.settings"},
		{"English logout", "en", "user.logout", true, "user.logout"},
		{"German profile", "de", "user.profile", true, "user.profile"},
		{"German settings", "de", "user.settings", true, "user.settings"},
		{"German logout", "de", "user.logout", true, "user.logout"},

		// Dashboard sections
		{"English recent activity", "en", "dashboard.recent_activity", true, "dashboard.recent_activity"},
		{"English quick_stats", "en", "dashboard.quick_stats", true, "dashboard.quick_stats"},
		{"English my_tickets", "en", "dashboard.my_tickets", true, "dashboard.my_tickets"},
		{"English open_tickets", "en", "dashboard.open_tickets", true, "dashboard.open_tickets"},
		{"German recent activity", "de", "dashboard.recent_activity", true, "dashboard.recent_activity"},

		// Common actions
		{"English view_all", "en", "common.view_all", true, "common.view_all"},
		{"English see_more", "en", "common.see_more", true, "common.see_more"},
		{"English refresh", "en", "common.refresh", true, "common.refresh"},
		{"German view_all", "de", "common.view_all", true, "common.view_all"},

		// Time-related
		{"English today", "en", "time.today", true, "time.today"},
		{"English yesterday", "en", "time.yesterday", true, "time.yesterday"},
		{"English this_week", "en", "time.this_week", true, "time.this_week"},
		{"English last_week", "en", "time.last_week", true, "time.last_week"},
		{"German today", "de", "time.today", true, "time.today"},
	}

	failedTests := []string{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := i18n.T(tt.lang, tt.key)

			// Check if translation exists (not returning the key itself)
			if tt.shouldExist && result == tt.key {
				failedTests = append(failedTests, tt.key)
				t.Errorf("Translation missing for key %q in language %q", tt.key, tt.lang)
			}

			// Check that it's actually translated (not the same as the key)
			if tt.notEqual != "" && result == tt.notEqual {
				t.Errorf("Expected translation for %q in %q, but got the key itself", tt.key, tt.lang)
			}
		})
	}

	// Summary of missing translations
	if len(failedTests) > 0 {
		t.Errorf("\nMissing translations for keys: %v", failedTests)
	}
}

func TestDashboardPageTranslations(t *testing.T) {
	i18n := GetInstance()

	// Specific dashboard page text
	requiredKeys := []struct {
		key         string
		languages   []string
		description string
	}{
		{"dashboard.welcome_back", []string{"en", "de"}, "Welcome back message"},
		{"dashboard.overview", []string{"en", "de"}, "Dashboard overview title"},
		{"dashboard.recent_tickets", []string{"en", "de"}, "Recent tickets section"},
		{"dashboard.quick_actions", []string{"en", "de"}, "Quick actions section"},
		{"dashboard.statistics", []string{"en", "de"}, "Statistics section"},
		{"dashboard.performance", []string{"en", "de"}, "Performance metrics"},
	}

	missingTranslations := map[string][]string{}

	for _, req := range requiredKeys {
		for _, lang := range req.languages {
			result := i18n.T(lang, req.key)
			if result == req.key {
				if _, exists := missingTranslations[lang]; !exists {
					missingTranslations[lang] = []string{}
				}
				missingTranslations[lang] = append(missingTranslations[lang], req.key)
			}
		}
	}

	// Report missing translations by language
	for lang, keys := range missingTranslations {
		t.Errorf("Language %q missing translations for: %v", lang, keys)
	}
}

func TestTemplateTextTranslations(t *testing.T) {
	i18n := GetInstance()

	// Test that common template text is translated
	templateText := []struct {
		key  string
		desc string
	}{
		{"common.loading", "Loading indicator"},
		{"common.error", "Error message"},
		{"common.success", "Success message"},
		{"common.warning", "Warning message"},
		{"common.info", "Info message"},
		{"common.confirm_action", "Confirm action prompt"},
		{"common.are_you_sure", "Are you sure prompt"},
	}

	for _, tt := range templateText {
		// Test English
		enResult := i18n.T("en", tt.key)
		if enResult == tt.key {
			t.Errorf("English translation missing for %s (%s)", tt.key, tt.desc)
		}

		// Test German
		deResult := i18n.T("de", tt.key)
		if deResult == tt.key {
			t.Errorf("German translation missing for %s (%s)", tt.key, tt.desc)
		}
	}
}
