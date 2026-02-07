// Package main provides translation testing utilities.
package main

import (
	"fmt"
	"strings"

	"github.com/goatkit/goatflow/internal/i18n"
)

func main() {
	// Initialize i18n system
	err := i18n.Initialize(&i18n.Config{
		DefaultLanguage:    "en",
		SupportedLanguages: []string{"en", "de", "es", "fr"},
	})
	if err != nil {
		fmt.Printf("Failed to initialize i18n: %v\n", err)
		return
	}

	i18nInstance := i18n.GetInstance()

	// Test direct translations
	fmt.Println("=== Direct Translation Tests ===")
	fmt.Printf("EN - Queue Singular: %s\n", i18nInstance.T("en", "admin.modules.queue.singular"))
	fmt.Printf("DE - Queue Singular: %s\n", i18nInstance.T("de", "admin.modules.queue.singular"))
	fmt.Printf("EN - Queue Name Label: %s\n", i18nInstance.T("en", "admin.modules.queue.fields.name.label"))
	fmt.Printf("DE - Queue Name Label: %s\n", i18nInstance.T("de", "admin.modules.queue.fields.name.label"))

	// Test translation key resolution
	fmt.Println("\n=== Translation Key Resolution ===")
	testKeys := []string{
		"@admin.modules.queue.singular",
		"@admin.modules.queue.plural",
		"@admin.modules.queue.fields.name.label",
		"@common.status",
		"@admin.statuses.valid",
	}

	for _, key := range testKeys {
		if strings.HasPrefix(key, "@") {
			cleanKey := strings.TrimPrefix(key, "@")
			fmt.Printf("Key: %s\n", key)
			fmt.Printf("  EN: %s\n", i18nInstance.T("en", cleanKey))
			fmt.Printf("  DE: %s\n", i18nInstance.T("de", cleanKey))
		}
	}

	// Test fallback for missing keys
	fmt.Println("\n=== Fallback Test ===")
	missingKey := "admin.modules.nonexistent.label"
	fmt.Printf("Missing key '%s': %s\n", missingKey, i18nInstance.T("en", missingKey))
}
