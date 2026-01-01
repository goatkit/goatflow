// Package main provides translation extraction.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// TranslationKey represents a translation key found in templates.
type TranslationKey struct {
	Key     string
	File    string
	Line    int
	Context string
	Default string
}

// TranslationMap represents the structure of a translation JSON file.
type TranslationMap map[string]interface{}

func main() {
	var (
		templateDir = flag.String("templates", "./templates", "Template directory to scan")
		outputDir   = flag.String("output", "./internal/i18n/translations", "Output directory for translation files")
		languages   = flag.String("languages", "en,de", "Comma-separated list of languages to generate")
		moduleMode  = flag.Bool("module", false, "Extract only module-specific translations")
		dryRun      = flag.Bool("dry-run", false, "Only show what would be extracted without writing files")
		verbose     = flag.Bool("verbose", false, "Show verbose output")
	)
	flag.Parse()

	langs := strings.Split(*languages, ",")

	// Find all template files
	templateFiles, err := findTemplateFiles(*templateDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding template files: %v\n", err)
		os.Exit(1)
	}

	if *verbose {
		fmt.Printf("Found %d template files\n", len(templateFiles))
	}

	// Extract translation keys
	keys := make(map[string]*TranslationKey)
	for _, file := range templateFiles {
		extractedKeys, err := extractKeysFromFile(file, *moduleMode)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error extracting from %s: %v\n", file, err)
			continue
		}

		for _, key := range extractedKeys {
			// Use the first occurrence of each key
			if _, exists := keys[key.Key]; !exists {
				keys[key.Key] = key
			}
		}
	}

	if *verbose {
		fmt.Printf("Extracted %d unique translation keys\n", len(keys))
	}

	// Group keys by prefix for better organization
	groupedKeys := groupKeysByPrefix(keys)

	if *dryRun {
		// Just print what we found
		printExtractedKeys(groupedKeys)
		return
	}

	// Generate translation files for each language
	for _, lang := range langs {
		if err := generateTranslationFile(*outputDir, lang, groupedKeys, *verbose); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating %s translations: %v\n", lang, err)
		}
	}

	fmt.Printf("Successfully extracted %d translation keys for %d languages\n", len(keys), len(langs))
}

// findTemplateFiles recursively finds all .pongo2 and .html template files.
func findTemplateFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && (strings.HasSuffix(path, ".pongo2") || strings.HasSuffix(path, ".html")) {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

// extractKeysFromFile extracts translation keys from a single template file.
func extractKeysFromFile(filename string, moduleMode bool) ([]*TranslationKey, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var keys []*TranslationKey
	scanner := bufio.NewScanner(file)
	lineNum := 0

	// Regex patterns for different translation function formats
	patterns := []*regexp.Regexp{
		// {{ t("key") }}
		regexp.MustCompile(`{{\s*t\s*\(\s*"([^"]+)"\s*(?:,\s*([^)]+))?\s*\)\s*}}`),
		// {{ t('key') }}
		regexp.MustCompile(`{{\s*t\s*\(\s*'([^']+)'\s*(?:,\s*([^)]+))?\s*\)\s*}}`),
		// {{ t("key" ~ variable) }} - dynamic keys
		regexp.MustCompile(`{{\s*t\s*\(\s*"([^"]+)"\s*~[^)]+\)\s*}}`),
		// @key format for module definitions
		regexp.MustCompile(`@([a-zA-Z0-9._]+)`),
	}

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		for _, pattern := range patterns {
			matches := pattern.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				key := match[1]

				// Skip non-translation @ symbols (like @click in Alpine.js)
				if strings.HasPrefix(match[0], "@") && strings.Contains(key, "click") {
					continue
				}

				// Filter based on mode
				if moduleMode && !strings.Contains(key, "module") {
					continue
				}

				// Handle dynamic keys (with ~) specially
				if strings.Contains(match[0], "~") {
					// Extract the base part before ~
					key = extractDynamicKeyBase(key)
				}

				keys = append(keys, &TranslationKey{
					Key:     key,
					File:    filename,
					Line:    lineNum,
					Context: strings.TrimSpace(line),
					Default: extractDefault(match),
				})
			}
		}
	}

	return keys, scanner.Err()
}

// extractDynamicKeyBase extracts the base part of a dynamic key.
func extractDynamicKeyBase(key string) string {
	// For keys like "admin.modules." we keep it as is
	// This will be expanded with actual module names later
	if strings.HasSuffix(key, ".") {
		return key + "[module]"
	}
	return key
}

// extractDefault attempts to extract a default value from the match.
func extractDefault(match []string) string {
	if len(match) > 2 && match[2] != "" {
		// Clean up the default value
		def := strings.TrimSpace(match[2])
		def = strings.Trim(def, `"'`)
		return def
	}
	return ""
}

// groupKeysByPrefix groups translation keys by their prefix.
func groupKeysByPrefix(keys map[string]*TranslationKey) map[string]map[string]*TranslationKey {
	grouped := make(map[string]map[string]*TranslationKey)

	for key, tk := range keys {
		parts := strings.Split(key, ".")
		prefix := "root"
		if len(parts) > 1 {
			prefix = parts[0]
		}

		if grouped[prefix] == nil {
			grouped[prefix] = make(map[string]*TranslationKey)
		}
		grouped[prefix][key] = tk
	}

	return grouped
}

// printExtractedKeys prints the extracted keys in a readable format.
func printExtractedKeys(grouped map[string]map[string]*TranslationKey) {
	var prefixes []string
	for prefix := range grouped {
		prefixes = append(prefixes, prefix)
	}
	sort.Strings(prefixes)

	for _, prefix := range prefixes {
		fmt.Printf("\n=== %s ===\n", prefix)

		var keys []string
		for key := range grouped[prefix] {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for _, key := range keys {
			tk := grouped[prefix][key]
			fmt.Printf("  %s", key)
			if tk.Default != "" {
				fmt.Printf(" (default: %s)", tk.Default)
			}
			fmt.Printf(" [%s:%d]\n", filepath.Base(tk.File), tk.Line)
		}
	}
}

// generateTranslationFile generates or updates a translation file for a language.
func generateTranslationFile(outputDir, lang string, grouped map[string]map[string]*TranslationKey, verbose bool) error {
	filename := filepath.Join(outputDir, fmt.Sprintf("%s.json", lang))

	// Load existing translations if file exists
	existing := make(TranslationMap)
	if data, err := os.ReadFile(filename); err == nil {
		if err := json.Unmarshal(data, &existing); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not parse existing %s: %v\n", filename, err)
		}
	}

	// Build the new translation map
	translations := buildTranslationMap(grouped, existing, lang)

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	// Write the translation file
	data, err := json.MarshalIndent(translations, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return err
	}

	if verbose {
		fmt.Printf("Wrote %s with %d keys\n", filename, countKeys(translations))
	}

	return nil
}

// buildTranslationMap builds a nested translation map from grouped keys.
func buildTranslationMap(grouped map[string]map[string]*TranslationKey, existing TranslationMap, lang string) TranslationMap {
	result := make(TranslationMap)

	// Copy existing translations first
	for k, v := range existing {
		result[k] = v
	}

	// Add new keys
	for _, keys := range grouped {
		for _, tk := range keys {
			setNestedValue(result, tk.Key, getTranslationValue(tk, lang))
		}
	}

	return result
}

// setNestedValue sets a value in a nested map structure.
func setNestedValue(m TranslationMap, key string, value string) {
	parts := strings.Split(key, ".")
	current := m

	for i, part := range parts {
		if i == len(parts)-1 {
			// Last part - set the value if it doesn't exist
			if _, exists := current[part]; !exists {
				current[part] = value
			}
		} else {
			// Create nested map if it doesn't exist
			if current[part] == nil {
				current[part] = make(TranslationMap)
			}

			// Move to the next level
			if next, ok := current[part].(TranslationMap); ok {
				current = next
			} else if next, ok := current[part].(map[string]interface{}); ok {
				current = TranslationMap(next)
			} else {
				// Can't go deeper, key conflict
				return
			}
		}
	}
}

// getTranslationValue generates an appropriate translation value.
func getTranslationValue(tk *TranslationKey, lang string) string {
	// If we have a default, use it for English
	if tk.Default != "" && lang == "en" {
		return tk.Default
	}

	// Generate a readable default based on the key
	parts := strings.Split(tk.Key, ".")
	if len(parts) > 0 {
		lastPart := parts[len(parts)-1]
		// Convert snake_case or camelCase to Title Case
		words := regexp.MustCompile(`[_\-]|([a-z])([A-Z])`).ReplaceAllString(lastPart, "$1 $2")
		title := cases.Title(language.English)
		words = title.String(strings.ToLower(words))

		// Add [DE] prefix for German to indicate it needs translation
		if lang == "de" {
			return "[DE] " + words
		}
		return words
	}

	return tk.Key
}

// countKeys counts the total number of leaf keys in a translation map.
func countKeys(m TranslationMap) int {
	count := 0
	for _, v := range m {
		switch v := v.(type) {
		case string:
			count++
		case TranslationMap:
			count += countKeys(v)
		case map[string]interface{}:
			count += countKeys(TranslationMap(v))
		}
	}
	return count
}
