package middleware

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/i18n"
)

const (
	// LanguageContextKey is the key for storing language in context.
	LanguageContextKey = "language"
	// DefaultLanguage is the default language.
	DefaultLanguage = "en"
)

// I18nMiddleware handles language detection and sets it in context.
type I18nMiddleware struct {
	i18n *i18n.I18n
}

// NewI18nMiddleware creates a new i18n middleware.
func NewI18nMiddleware() *I18nMiddleware {
	return &I18nMiddleware{
		i18n: i18n.GetInstance(),
	}
}

// Handle returns the middleware handler function.
func (m *I18nMiddleware) Handle() gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := m.detectLanguage(c)

		// Debug output
		if queryLang := c.Query("lang"); queryLang != "" {
			fmt.Printf("DEBUG Middleware: Query lang=%s, Detected lang=%s, Supported=%v\n", queryLang, lang, m.i18n.GetSupportedLanguages())
		}

		// Store language in context
		c.Set(LanguageContextKey, lang)

		// Set language in response header
		c.Header("Content-Language", lang)

		c.Next()
	}
}

// detectLanguage detects the user's preferred language.
func (m *I18nMiddleware) detectLanguage(c *gin.Context) string {
	// Priority order for language detection:
	// 1. Query parameter (?lang=es)
	// 2. Cookie (lang=es)
	// 3. User preference (from authenticated user)
	// 4. Accept-Language header
	// 5. Default language

	// 1. Check query parameter
	if lang := c.Query("lang"); lang != "" {
		if m.isSupported(lang) {
			// Set cookie for future requests
			c.SetCookie("lang", lang, 86400*30, "/", "", false, true)
			return lang
		}
	}

	// 2. Check cookie
	if lang, err := c.Cookie("lang"); err == nil && lang != "" {
		if m.isSupported(lang) {
			return lang
		}
	}

	// 3. Check user preference (if authenticated)
	if userLang := m.getUserLanguage(c); userLang != "" {
		if m.isSupported(userLang) {
			return userLang
		}
	}

	// 4. Check Accept-Language header
	if lang := m.parseAcceptLanguage(c.GetHeader("Accept-Language")); lang != "" {
		return lang
	}

	// 5. Return default language
	return DefaultLanguage
}

// parseAcceptLanguage parses the Accept-Language header.
func (m *I18nMiddleware) parseAcceptLanguage(header string) string {
	if header == "" {
		return ""
	}

	// Parse Accept-Language header
	// Example: "en-US,en;q=0.9,es;q=0.8"
	languages := strings.Split(header, ",")

	for _, lang := range languages {
		// Remove quality value if present
		parts := strings.Split(strings.TrimSpace(lang), ";")
		langCode := strings.TrimSpace(parts[0])

		// Extract primary language code (e.g., "en" from "en-US")
		if idx := strings.Index(langCode, "-"); idx > 0 {
			langCode = langCode[:idx]
		}

		// Check if language is supported
		if m.isSupported(langCode) {
			return langCode
		}
	}

	return ""
}

// getUserLanguage gets the language preference from authenticated user.
func (m *I18nMiddleware) getUserLanguage(c *gin.Context) string {
	// Check if user is authenticated
	if userID, exists := c.Get("user_id"); exists {
		// In a real implementation, this would fetch user preferences from database
		// For now, return empty to use other detection methods
		_ = userID
		return ""
	}
	return ""
}

// isSupported checks if a language is supported.
func (m *I18nMiddleware) isSupported(lang string) bool {
	supportedLangs := m.i18n.GetSupportedLanguages()
	for _, supported := range supportedLangs {
		if supported == lang {
			return true
		}
	}
	return false
}

// GetLanguage gets the current language from context.
func GetLanguage(c *gin.Context) string {
	if lang, exists := c.Get(LanguageContextKey); exists {
		if langStr, ok := lang.(string); ok {
			return langStr
		}
	}
	return DefaultLanguage
}

// T translates a key in the current language.
func T(c *gin.Context, key string, args ...interface{}) string {
	lang := GetLanguage(c)
	return i18n.GetInstance().T(lang, key, args...)
}

// TranslateError translates an error message.
func TranslateError(c *gin.Context, key string, args ...interface{}) string {
	lang := GetLanguage(c)
	return i18n.Error(lang, key, args...)
}

// TranslateSuccess translates a success message.
func TranslateSuccess(c *gin.Context, key string, args ...interface{}) string {
	lang := GetLanguage(c)
	return i18n.Success(lang, key, args...)
}

// TranslateValidation translates a validation message.
func TranslateValidation(c *gin.Context, key string, args ...interface{}) string {
	lang := GetLanguage(c)
	return i18n.Validation(lang, key, args...)
}

// SetLanguageCookie sets the language preference cookie.
func SetLanguageCookie(c *gin.Context, lang string) {
	c.SetCookie("lang", lang, 86400*30, "/", "", false, true)
}

// ClearLanguageCookie clears the language preference cookie.
func ClearLanguageCookie(c *gin.Context) {
	c.SetCookie("lang", "", -1, "/", "", false, true)
}
