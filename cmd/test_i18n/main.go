// Package main provides i18n testing utilities.
package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gotrs-io/gotrs-ce/internal/i18n"
)

func main() {
	i18nInstance := i18n.GetInstance()

	// Get all supported languages
	languages := i18nInstance.GetSupportedLanguages()
	fmt.Printf("Supported languages: %v\n\n", languages)

	// Get all keys from English (base language)
	baseKeys := i18nInstance.GetAllKeys("en")
	sort.Strings(baseKeys)

	fmt.Printf("Total translation keys: %d\n", len(baseKeys))
	fmt.Println(strings.Repeat("=", 80))

	// Check each language
	for _, lang := range languages {
		fmt.Printf("\nChecking language: %s\n", lang)
		fmt.Println(strings.Repeat("-", 40))

		langKeys := i18nInstance.GetAllKeys(lang)
		langKeyMap := make(map[string]bool)
		for _, key := range langKeys {
			langKeyMap[key] = true
		}

		// Track statistics
		missing := []string{}
		untranslated := []string{}

		// Check each base key
		for _, key := range baseKeys {
			// Check if key exists in this language
			if !langKeyMap[key] {
				missing = append(missing, key)
			} else {
				// Check if translation returns the key itself (not translated)
				translation := i18nInstance.T(lang, key)
				if translation == key {
					untranslated = append(untranslated, key)
				}
			}
		}

		// Check for extra keys (in language but not in base)
		extra := []string{}
		for _, key := range langKeys {
			found := false
			for _, baseKey := range baseKeys {
				if key == baseKey {
					found = true
					break
				}
			}
			if !found {
				extra = append(extra, key)
			}
		}

		// Print statistics
		coverage := float64(len(langKeys)-len(untranslated)) / float64(len(baseKeys)) * 100
		fmt.Printf("  Coverage: %.1f%% (%d/%d keys translated)\n",
			coverage, len(langKeys)-len(untranslated), len(baseKeys))

		if len(missing) > 0 {
			fmt.Printf("  ❌ Missing keys (%d):\n", len(missing))
			for i, key := range missing {
				if i < 5 {
					fmt.Printf("     - %s\n", key)
				}
			}
			if len(missing) > 5 {
				fmt.Printf("     ... and %d more\n", len(missing)-5)
			}
		}

		if len(untranslated) > 0 {
			fmt.Printf("  ⚠️  Untranslated keys (%d):\n", len(untranslated))
			for i, key := range untranslated {
				if i < 5 {
					fmt.Printf("     - %s\n", key)
				}
			}
			if len(untranslated) > 5 {
				fmt.Printf("     ... and %d more\n", len(untranslated)-5)
			}
		}

		if len(extra) > 0 {
			fmt.Printf("  ℹ️  Extra keys not in base (%d):\n", len(extra))
			for i, key := range extra {
				if i < 5 {
					fmt.Printf("     - %s\n", key)
				}
			}
			if len(extra) > 5 {
				fmt.Printf("     ... and %d more\n", len(extra)-5)
			}
		}

		if len(missing) == 0 && len(untranslated) == 0 && len(extra) == 0 {
			fmt.Println("  ✅ All keys properly translated!")
		}
	}

	// Sample some translations to verify they work
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("Sample translations:")

	sampleKeys := []string{
		"app.name",
		"navigation.dashboard",
		"tickets.new_ticket",
		"common.all",
		"status.active",
	}

	for _, key := range sampleKeys {
		fmt.Printf("\n%s:\n", key)
		for _, lang := range []string{"en", "de", "es", "fr"} {
			translation := i18nInstance.T(lang, key)
			status := "✓"
			if translation == key {
				status = "✗"
			}
			fmt.Printf("  %s %s: %q\n", status, lang, translation)
		}
	}
}
