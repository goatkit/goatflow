package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
)

//go:embed translations/*.json
var translationsFS embed.FS

// I18n handles internationalization.
type I18n struct {
	translations   map[string]map[string]interface{}
	defaultLang    string
	supportedLangs []string
	mu             sync.RWMutex
}

// Config represents i18n configuration.
type Config struct {
	DefaultLanguage    string
	SupportedLanguages []string
}

// Instance is the global i18n instance.
var Instance *I18n
var once sync.Once

// Initialize initializes the i18n system.
func Initialize(config *Config) error {
	var initErr error
	once.Do(func() {
		Instance = &I18n{
			translations:   make(map[string]map[string]interface{}),
			defaultLang:    config.DefaultLanguage,
			supportedLangs: config.SupportedLanguages,
		}

		// Load all translation files
		initErr = Instance.loadTranslations()
	})
	return initErr
}

// GetInstance returns the global i18n instance.
func GetInstance() *I18n {
	if Instance == nil {
		// Initialize with defaults if not already done
		Initialize(&Config{
			DefaultLanguage:    "en",
			SupportedLanguages: []string{"en", "es", "fr", "de", "pt", "ja", "zh", "ar", "ru", "it", "nl", "tlh"},
		})
	}
	return Instance
}

// loadTranslations loads all translation files from embedded filesystem.
func (i *I18n) loadTranslations() error {
	i.mu.Lock()
	defer i.mu.Unlock()

	// Clear and rebuild supported languages list from actual files
	i.supportedLangs = []string{}

	// Read translation files
	err := fs.WalkDir(translationsFS, "translations", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-JSON files
		if d.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}

		// Extract language code from filename (e.g., "en.json" -> "en")
		filename := filepath.Base(path)
		lang := strings.TrimSuffix(filename, ".json")

		// Read file content
		content, err := translationsFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read translation file %s: %w", path, err)
		}

		// Parse JSON
		var translations map[string]interface{}
		if err := json.Unmarshal(content, &translations); err != nil {
			return fmt.Errorf("failed to parse translation file %s: %w", path, err)
		}

		// Store translations
		i.translations[lang] = translations

		// Add to supported languages list
		i.supportedLangs = append(i.supportedLangs, lang)

		return nil
	})

	return err
}

// T translates a key to the specified language.
func (i *I18n) T(lang, key string, args ...interface{}) string {
	i.mu.RLock()
	defer i.mu.RUnlock()

	// Use default language if not supported
	if !i.isSupported(lang) {
		lang = i.defaultLang
	}

	// Get translation for language
	langTranslations, exists := i.translations[lang]
	if !exists {
		// Fall back to default language
		langTranslations = i.translations[i.defaultLang]
		if langTranslations == nil {
			return key // Return key if no translations available
		}
	}

	// Navigate through nested keys (e.g., "errors.validation.required")
	value := i.getNestedValue(langTranslations, key)
	if value == nil {
		// Try default language if key not found
		if lang != i.defaultLang {
			if defaultTranslations, ok := i.translations[i.defaultLang]; ok {
				value = i.getNestedValue(defaultTranslations, key)
			}
		}

		// Return key if translation not found
		if value == nil {
			return key
		}
	}

	// Convert to string and format with arguments if provided
	if str, ok := value.(string); ok {
		if len(args) > 0 {
			return fmt.Sprintf(str, args...)
		}
		return str
	}

	return key
}

// Translate is an alias for T.
func (i *I18n) Translate(lang, key string, args ...interface{}) string {
	return i.T(lang, key, args...)
}

// getNestedValue retrieves a nested value from a map using dot notation.
func (i *I18n) getNestedValue(m map[string]interface{}, key string) interface{} {
	keys := strings.Split(key, ".")
	var current interface{} = m

	for _, k := range keys {
		if currentMap, ok := current.(map[string]interface{}); ok {
			current = currentMap[k]
			if current == nil {
				return nil
			}
		} else {
			return nil
		}
	}

	return current
}

// isSupported checks if a language is supported.
func (i *I18n) isSupported(lang string) bool {
	for _, supported := range i.supportedLangs {
		if supported == lang {
			return true
		}
	}
	return false
}

// GetSupportedLanguages returns the list of supported languages.
func (i *I18n) GetSupportedLanguages() []string {
	return i.supportedLangs
}

// GetDefaultLanguage returns the default language.
func (i *I18n) GetDefaultLanguage() string {
	return i.defaultLang
}

// SetDefaultLanguage sets the default language.
func (i *I18n) SetDefaultLanguage(lang string) error {
	if !i.isSupported(lang) {
		return fmt.Errorf("language %s is not supported", lang)
	}
	i.mu.Lock()
	i.defaultLang = lang
	i.mu.Unlock()
	return nil
}

// AddTranslation adds or updates a translation.
func (i *I18n) AddTranslation(lang, key, value string) {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.translations[lang] == nil {
		i.translations[lang] = make(map[string]interface{})
	}

	// Handle nested keys
	keys := strings.Split(key, ".")
	current := i.translations[lang]

	for i, k := range keys {
		if i == len(keys)-1 {
			// Last key, set the value
			current[k] = value
		} else {
			// Navigate or create nested map
			if next, ok := current[k].(map[string]interface{}); ok {
				current = next
			} else {
				next := make(map[string]interface{})
				current[k] = next
				current = next
			}
		}
	}
}

// LoadCustomTranslations loads custom translations from a JSON string.
func (i *I18n) LoadCustomTranslations(lang string, jsonData string) error {
	var translations map[string]interface{}
	if err := json.Unmarshal([]byte(jsonData), &translations); err != nil {
		return fmt.Errorf("failed to parse custom translations: %w", err)
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	// Merge with existing translations
	if i.translations[lang] == nil {
		i.translations[lang] = translations
	} else {
		// Merge translations
		i.mergeTranslations(i.translations[lang], translations)
	}

	return nil
}

// mergeTranslations recursively merges source into target.
func (i *I18n) mergeTranslations(target, source map[string]interface{}) {
	for key, value := range source {
		if targetValue, exists := target[key]; exists {
			// If both are maps, merge recursively
			if targetMap, ok := targetValue.(map[string]interface{}); ok {
				if sourceMap, ok := value.(map[string]interface{}); ok {
					i.mergeTranslations(targetMap, sourceMap)
					continue
				}
			}
		}
		// Otherwise, replace the value
		target[key] = value
	}
}

// GetTranslations returns all translations for a language.
func (i *I18n) GetTranslations(lang string) map[string]interface{} {
	i.mu.RLock()
	defer i.mu.RUnlock()

	return i.translations[lang]
}

// GetAllKeys returns all translation keys for a language in dot notation.
func (i *I18n) GetAllKeys(lang string) []string {
	i.mu.RLock()
	defer i.mu.RUnlock()

	translations := i.translations[lang]
	if translations == nil {
		return []string{}
	}

	keys := []string{}
	i.extractKeys(translations, "", &keys)
	return keys
}

// extractKeys recursively extracts all keys from nested maps.
func (i *I18n) extractKeys(m map[string]interface{}, prefix string, keys *[]string) {
	for key, value := range m {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		if nestedMap, ok := value.(map[string]interface{}); ok {
			// Recursively extract keys from nested map
			i.extractKeys(nestedMap, fullKey, keys)
		} else {
			// Leaf node - add to keys
			*keys = append(*keys, fullKey)
		}
	}
}

// Helper functions for common translations

// Error returns a translated error message.
func Error(lang, key string, args ...interface{}) string {
	return GetInstance().T(lang, "errors."+key, args...)
}

// Success returns a translated success message.
func Success(lang, key string, args ...interface{}) string {
	return GetInstance().T(lang, "success."+key, args...)
}

// Label returns a translated label.
func Label(lang, key string, args ...interface{}) string {
	return GetInstance().T(lang, "labels."+key, args...)
}

// Button returns a translated button text.
func Button(lang, key string, args ...interface{}) string {
	return GetInstance().T(lang, "buttons."+key, args...)
}

// Message returns a translated message.
func Message(lang, key string, args ...interface{}) string {
	return GetInstance().T(lang, "messages."+key, args...)
}

// Validation returns a translated validation message.
func Validation(lang, key string, args ...interface{}) string {
	return GetInstance().T(lang, "validation."+key, args...)
}
