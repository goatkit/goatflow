package i18n

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

// languageCoverage holds coverage data for the summary table.
type languageCoverage struct {
	Code       string
	Name       string
	NativeName string
	Keys       int
	Coverage   float64
	HasJSON    bool
}

// TestTranslationCompleteness validates that all languages have complete translations.
// Languages are auto-detected based on JSON file existence in internal/i18n/translations/.
func TestTranslationCompleteness(t *testing.T) {
	i18n := GetInstance()

	// Get base keys from English
	baseKeys := i18n.GetAllKeys("en")
	if len(baseKeys) == 0 {
		t.Fatal("No translation keys found in base language (en)")
	}

	sort.Strings(baseKeys)
	t.Logf("Total base translation keys: %d", len(baseKeys))

	// Use rtl.go as source of truth - languages with JSON files are enabled
	enabledLanguages := GetEnabledLanguages()

	// Sort by language code for consistent output
	sort.Slice(enabledLanguages, func(i, j int) bool {
		return enabledLanguages[i].Code < enabledLanguages[j].Code
	})

	// Collect coverage data for summary table
	var coverageData []languageCoverage

	for _, langConfig := range enabledLanguages {
		lang := langConfig.Code
		t.Run(lang, func(t *testing.T) {
			langKeys := i18n.GetAllKeys(lang)
			langKeyMap := make(map[string]bool)
			for _, key := range langKeys {
				langKeyMap[key] = true
			}

			missing := []string{}
			untranslated := []string{}

			// Check each base key exists and is translated
			for _, key := range baseKeys {
				if !langKeyMap[key] {
					missing = append(missing, key)
				} else {
					// Check if properly translated (not returning the key itself)
					translation := i18n.T(lang, key)
					if translation == key {
						untranslated = append(untranslated, key)
					}
				}
			}

			// Report findings (only fail for English)
			if len(missing) > 0 && lang == "en" {
				t.Errorf("Base language %s is missing %d keys:", lang, len(missing))
				for i, key := range missing {
					if i < 10 {
						t.Errorf("  - %s", key)
					}
				}
				if len(missing) > 10 {
					t.Errorf("  ... and %d more", len(missing)-10)
				}
			}

			// Calculate coverage: percentage of base keys that are translated
			// Keys that exist in this language (matching base keys, excluding untranslated)
			translated := len(baseKeys) - len(missing) - len(untranslated)
			coverage := float64(translated) / float64(len(baseKeys)) * 100

			// Store for summary table
			coverageData = append(coverageData, languageCoverage{
				Code:       lang,
				Name:       langConfig.Name,
				NativeName: langConfig.NativeName,
				Keys:       translated,
				Coverage:   coverage,
				HasJSON:    true,
			})

			// Only enforce 100% for English (base language)
			if lang == "en" && coverage < 100.0 {
				t.Errorf("Base language %s must have 100%% coverage, got %.1f%%", lang, coverage)
			}
		})
	}

	// Add languages without JSON files to show what needs to be done
	enabledCodes := make(map[string]bool)
	for _, c := range coverageData {
		enabledCodes[c.Code] = true
	}
	for code, config := range SupportedLanguages {
		if !enabledCodes[code] {
			coverageData = append(coverageData, languageCoverage{
				Code:       code,
				Name:       config.Name,
				NativeName: config.NativeName,
				Keys:       0,
				Coverage:   0,
				HasJSON:    false,
			})
		}
	}

	// Sort all coverage data by language code
	sort.Slice(coverageData, func(i, j int) bool {
		return coverageData[i].Code < coverageData[j].Code
	})

	// Print summary table (use t.Log - shows with -v flag which toolbox-test uses)
	t.Log("")
	t.Log("┌──────────────────────────────────────────────────────────────┐")
	t.Log("│                  Translation Coverage Summary                │")
	t.Log("├──────┬──────────────┬──────────────────┬───────┬─────────────┤")
	t.Log("│ Code │ Name         │ Native           │ Keys  │ Coverage    │")
	t.Log("├──────┼──────────────┼──────────────────┼───────┼─────────────┤")

	withJSON := 0
	for _, c := range coverageData {
		if c.HasJSON {
			withJSON++
			status := "✓"
			if c.Coverage < 100.0 {
				status = " "
			}
			t.Logf("│ %-4s │ %-12s │ %-16s │ %5d │ %6.1f%% %s  │",
				c.Code, truncate(c.Name, 12), truncate(c.NativeName, 16), c.Keys, c.Coverage, status)
		} else {
			t.Logf("│ %-4s │ %-12s │ %-16s │     - │   No JSON   │",
				c.Code, truncate(c.Name, 12), truncate(c.NativeName, 16))
		}
	}

	t.Log("└──────┴──────────────┴──────────────────┴───────┴─────────────┘")
	t.Logf("Total: %d/%d languages with JSON files, %d base keys", withJSON, len(coverageData), len(baseKeys))
}

// truncate shortens a string to maxLen, adding ellipsis if needed.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s + strings.Repeat(" ", maxLen-len(s))
	}
	return s[:maxLen-1] + "…"
}

// formatCoverage returns a formatted coverage string with status indicator.
func formatCoverage(coverage float64) string {
	if coverage >= 100.0 {
		return fmt.Sprintf("%.1f%% ✓", coverage)
	}
	return fmt.Sprintf("%.1f%%", coverage)
}

// TestCriticalTranslations ensures critical UI keys are translated in all enabled languages.
func TestCriticalTranslations(t *testing.T) {
	i18n := GetInstance()

	// Define critical keys that must be translated
	criticalKeys := []string{
		"app.name",
		"app.title",
		"navigation.dashboard",
		"navigation.tickets",
		"navigation.queues",
		"auth.login",
		"auth.logout",
		"buttons.save",
		"buttons.cancel",
		"buttons.delete",
		"common.all",
		"status.active",
		"status.inactive",
		"tickets.new_ticket",
		"messages.loading",
		"messages.error_occurred",
	}

	// Use enabled languages (those with JSON files)
	enabledLanguages := GetEnabledLanguages()

	for _, langConfig := range enabledLanguages {
		lang := langConfig.Code
		t.Run(lang, func(t *testing.T) {
			for _, key := range criticalKeys {
				translation := i18n.T(lang, key)

				// Check translation exists and is not the key itself
				if translation == key {
					t.Errorf("Critical key %q not translated in %s", key, lang)
				}

				// Check translation is not empty
				if translation == "" {
					t.Errorf("Critical key %q has empty translation in %s", key, lang)
				}
			}
		})
	}
}

// TestTranslationConsistency checks for consistent formatting across languages.
func TestTranslationConsistency(t *testing.T) {
	i18n := GetInstance()

	// Keys that should have consistent format patterns
	formatKeys := map[string]string{
		"validation.min_length": "%d", // Should contain %d
		"validation.max_length": "%d",
		"pagination.showing":    "", // Check exists
	}

	// Use enabled languages (those with JSON files)
	enabledLanguages := GetEnabledLanguages()

	for key, expectedPattern := range formatKeys {
		t.Run(key, func(t *testing.T) {
			baseTranslation := i18n.T("en", key)

			if baseTranslation == key {
				t.Skipf("Key %s not found in base language", key)
				return
			}

			for _, langConfig := range enabledLanguages {
				lang := langConfig.Code
				translation := i18n.T(lang, key)

				if translation != key && expectedPattern != "" {
					// Check if pattern exists in translation
					if !contains(translation, expectedPattern) {
						t.Errorf("Key %s in %s missing expected pattern %q: %q",
							key, lang, expectedPattern, translation)
					}
				}
			}
		})
	}
}

// Helper function.
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsMiddle(s, substr))
}

func containsMiddle(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 1; i < len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
