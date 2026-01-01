package i18n

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestTemplateTranslationKeys verifies that all translation keys used in templates exist.
func TestTemplateTranslationKeys(t *testing.T) {
	// Initialize i18n
	if err := Initialize(&Config{
		DefaultLanguage:    "en",
		SupportedLanguages: []string{"en", "de"},
	}); err != nil {
		t.Fatalf("Failed to initialize i18n: %v", err)
	}

	i18n := GetInstance()

	// Pattern to match t("key") or {{ t("key") }} in templates
	// Make sure we only match actual translation function calls, not HTML content
	tFuncPattern := regexp.MustCompile(`\{\{[^}]*t\(["']([^"']+)["']\)[^}]*\}\}|[^a-zA-Z]t\(["']([^"']+)["']\)`)

	// Find all template files
	templatesDir := "../../templates"
	missingKeys := make(map[string][]string)

	err := filepath.Walk(templatesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Only check .pongo2 and .html files
		if !strings.HasSuffix(path, ".pongo2") && !strings.HasSuffix(path, ".html") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// Find all translation function calls
		matches := tFuncPattern.FindAllStringSubmatch(string(content), -1)

		for _, match := range matches {
			// The regex has two capture groups, check both
			key := ""
			if len(match) > 1 && match[1] != "" {
				key = match[1]
			} else if len(match) > 2 && match[2] != "" {
				key = match[2]
			}

			if key != "" {
				// Check if key exists in English translations
				value := i18n.T("en", key)
				if value == key {
					// Key doesn't exist (returns the key itself when not found)
					relPath, _ := filepath.Rel(templatesDir, path)
					missingKeys[relPath] = append(missingKeys[relPath], key)
				}
			}
		}

		return nil
	})

	if err != nil {
		t.Fatalf("Error walking templates directory: %v", err)
	}

	// Report missing keys
	if len(missingKeys) > 0 {
		t.Errorf("Found templates using non-existent translation keys:")
		for file, keys := range missingKeys {
			t.Errorf("  %s:", file)
			for _, key := range keys {
				// Get the value from the actual translation to suggest correct key
				suggestedKey := suggestCorrectKey(i18n, key)
				if suggestedKey != "" {
					t.Errorf("    - %s (did you mean: %s?)", key, suggestedKey)
				} else {
					t.Errorf("    - %s", key)
				}
			}
		}
	}
}

// suggestCorrectKey tries to find a similar key that exists.
func suggestCorrectKey(i18n *I18n, wrongKey string) string {
	// Common mistakes: using common.X when it should be buttons.X
	replacements := map[string]string{
		"common.submit":   "buttons.submit",
		"common.next":     "buttons.next",
		"common.previous": "buttons.previous",
		"common.back":     "buttons.back",
		"common.close":    "buttons.close",
		"common.add":      "buttons.add",
		"common.remove":   "buttons.remove",
	}

	if suggested, ok := replacements[wrongKey]; ok {
		// Verify the suggested key actually exists
		if i18n.T("en", suggested) != suggested {
			return suggested
		}
	}

	// Try to find if the key exists in a different section
	parts := strings.Split(wrongKey, ".")
	if len(parts) == 2 {
		sections := []string{"buttons", "common", "labels", "messages"}
		for _, section := range sections {
			tryKey := section + "." + parts[1]
			if i18n.T("en", tryKey) != tryKey {
				return tryKey
			}
		}
	}

	return ""
}

// TestTranslationKeyConsistency ensures consistent key usage across the codebase.
func TestTranslationKeyConsistency(t *testing.T) {
	// Initialize i18n
	if err := Initialize(&Config{
		DefaultLanguage:    "en",
		SupportedLanguages: []string{"en", "de"},
	}); err != nil {
		t.Fatalf("Failed to initialize i18n: %v", err)
	}

	i18n := GetInstance()
	enKeys := i18n.GetAllKeys("en")

	// Check for duplicate concepts in different sections
	// e.g., both buttons.submit and common.submit
	keysByName := make(map[string][]string)

	for _, key := range enKeys {
		parts := strings.Split(key, ".")
		if len(parts) >= 2 {
			name := parts[len(parts)-1]
			keysByName[name] = append(keysByName[name], key)
		}
	}

	// Report potential duplicates
	duplicates := []string{}
	for _, keys := range keysByName {
		if len(keys) > 1 {
			// Check if they have the same value
			values := make(map[string][]string)
			for _, key := range keys {
				value := i18n.T("en", key)
				values[value] = append(values[value], key)
			}

			// If all keys have the same value, they're duplicates
			for value, sameValueKeys := range values {
				if len(sameValueKeys) > 1 {
					duplicates = append(duplicates,
						fmt.Sprintf("Keys %v all have value '%s'", sameValueKeys, value))
				}
			}
		}
	}

	if len(duplicates) > 0 {
		t.Logf("Warning: Found duplicate translation keys with same values:")
		for _, dup := range duplicates {
			t.Logf("  - %s", dup)
		}
		t.Logf("Consider consolidating these to avoid confusion")
	}
}
