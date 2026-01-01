package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/i18n"
	"github.com/gotrs-io/gotrs-ce/internal/middleware"
)

// I18nHandlers handles internationalization-related requests.
type I18nHandlers struct {
	i18n *i18n.I18n
}

// NewI18nHandlers creates new i18n handlers.
func NewI18nHandlers() *I18nHandlers {
	return &I18nHandlers{
		i18n: i18n.GetInstance(),
	}
}

// @Router /api/v1/i18n/languages [get].
func (h *I18nHandlers) GetSupportedLanguages(c *gin.Context) {
	languages := h.i18n.GetSupportedLanguages()
	currentLang := middleware.GetLanguage(c)

	// Build language list with display names
	var languageList []LanguageInfo
	for _, lang := range languages {
		info := LanguageInfo{
			Code:       lang,
			Name:       h.getLanguageName(lang),
			NativeName: h.getLanguageNativeName(lang),
			Active:     lang == currentLang,
		}
		languageList = append(languageList, info)
	}

	c.JSON(http.StatusOK, LanguagesResponse{
		Languages: languageList,
		Current:   currentLang,
		Default:   h.i18n.GetDefaultLanguage(),
	})
}

// @Router /api/v1/i18n/language [post].
func (h *I18nHandlers) SetLanguage(c *gin.Context) {
	var req SetLanguageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": middleware.TranslateError(c, "bad_request"),
		})
		return
	}

	// Check if language is supported
	supported := false
	for _, lang := range h.i18n.GetSupportedLanguages() {
		if lang == req.Language {
			supported = true
			break
		}
	}

	if !supported {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   middleware.TranslateError(c, "unsupported"),
			"message": "Language not supported",
		})
		return
	}

	// Set language cookie
	middleware.SetLanguageCookie(c, req.Language)

	// If user is authenticated, save preference to database
	if userID, exists := c.Get("user_id"); exists {
		// TODO: Save user language preference to database
		_ = userID
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  middleware.TranslateSuccess(c, "saved"),
		"language": req.Language,
	})
}

// @Router /api/v1/i18n/translations/{lang} [get].
func (h *I18nHandlers) GetTranslations(c *gin.Context) {
	lang := c.Param("lang")

	// Check if language is supported
	supported := false
	for _, supportedLang := range h.i18n.GetSupportedLanguages() {
		if supportedLang == lang {
			supported = true
			break
		}
	}

	if !supported {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   middleware.TranslateError(c, "not_found"),
			"message": "Language not found",
		})
		return
	}

	translations := h.i18n.GetTranslations(lang)

	c.JSON(http.StatusOK, TranslationsResponse{
		Language:     lang,
		Translations: translations,
	})
}

// @Router /api/v1/i18n/translations [get].
func (h *I18nHandlers) GetCurrentTranslations(c *gin.Context) {
	lang := middleware.GetLanguage(c)
	translations := h.i18n.GetTranslations(lang)

	c.JSON(http.StatusOK, TranslationsResponse{
		Language:     lang,
		Translations: translations,
	})
}

// @Router /api/v1/i18n/translate [post].
func (h *I18nHandlers) Translate(c *gin.Context) {
	var req TranslateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": middleware.TranslateError(c, "bad_request"),
		})
		return
	}

	lang := middleware.GetLanguage(c)
	if req.Language != "" {
		lang = req.Language
	}

	// Convert args to interface slice
	var args []interface{}
	args = append(args, req.Args...)

	translation := h.i18n.T(lang, req.Key, args...)

	c.JSON(http.StatusOK, TranslateResponse{
		Key:         req.Key,
		Translation: translation,
		Language:    lang,
	})
}

// @Router /api/v1/i18n/stats [get].
func (h *I18nHandlers) GetLanguageStats(c *gin.Context) {
	// In a real implementation, this would fetch from database
	stats := LanguageStatsResponse{
		TotalUsers: 1000,
		Languages: []LanguageStats{
			{Code: "en", Users: 600, Percentage: 60.0},
			{Code: "es", Users: 200, Percentage: 20.0},
			{Code: "fr", Users: 100, Percentage: 10.0},
			{Code: "de", Users: 50, Percentage: 5.0},
			{Code: "pt", Users: 30, Percentage: 3.0},
			{Code: "ja", Users: 10, Percentage: 1.0},
			{Code: "zh", Users: 10, Percentage: 1.0},
		},
		DefaultLanguage: h.i18n.GetDefaultLanguage(),
	}

	c.JSON(http.StatusOK, stats)
}

// @Router /api/v1/i18n/coverage [get].
func (h *I18nHandlers) GetTranslationCoverage(c *gin.Context) {
	baseKeys := h.i18n.GetAllKeys("en")
	totalKeys := len(baseKeys)

	var languages []LanguageCoverage
	var totalCoverage float64

	for _, lang := range h.i18n.GetSupportedLanguages() {
		langKeys := h.i18n.GetAllKeys(lang)
		translatedKeys := len(langKeys)
		missingCount := totalKeys - translatedKeys
		coverage := float64(translatedKeys) / float64(totalKeys) * 100.0

		languages = append(languages, LanguageCoverage{
			Code:           lang,
			Name:           h.getLanguageName(lang),
			TotalKeys:      totalKeys,
			TranslatedKeys: translatedKeys,
			MissingCount:   missingCount,
			Coverage:       coverage,
		})

		totalCoverage += coverage
	}

	response := CoverageResponse{
		Languages: languages,
	}
	response.Summary.TotalKeys = totalKeys
	response.Summary.AverageCoverage = totalCoverage / float64(len(languages))

	c.JSON(http.StatusOK, response)
}

// @Router /api/v1/i18n/missing/{lang} [get].
func (h *I18nHandlers) GetMissingTranslations(c *gin.Context) {
	lang := c.Param("lang")

	// Check if language is supported
	supported := false
	for _, supportedLang := range h.i18n.GetSupportedLanguages() {
		if supportedLang == lang {
			supported = true
			break
		}
	}

	if !supported {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   middleware.TranslateError(c, "not_found"),
			"message": "Language not found",
		})
		return
	}

	baseKeys := h.i18n.GetAllKeys("en")
	langKeys := h.i18n.GetAllKeys(lang)

	// Create a map for quick lookup
	langKeyMap := make(map[string]bool)
	for _, key := range langKeys {
		langKeyMap[key] = true
	}

	// Find missing keys
	var missingKeys []MissingKey
	for _, key := range baseKeys {
		if !langKeyMap[key] {
			// Get the English value
			englishValue := h.i18n.T("en", key)

			// Extract category from key (e.g., "tickets.title" -> "tickets")
			category := ""
			if idx := strings.Index(key, "."); idx > 0 {
				category = key[:idx]
			}

			missingKeys = append(missingKeys, MissingKey{
				Key:          key,
				EnglishValue: englishValue,
				Category:     category,
			})
		}
	}

	c.JSON(http.StatusOK, MissingKeysResponse{
		Language:    lang,
		MissingKeys: missingKeys,
		Count:       len(missingKeys),
	})
}

// @Router /api/v1/i18n/export/{lang} [get].
func (h *I18nHandlers) ExportTranslations(c *gin.Context) {
	lang := c.Param("lang")
	format := c.DefaultQuery("format", "json")

	// Check if language is supported
	supported := false
	for _, supportedLang := range h.i18n.GetSupportedLanguages() {
		if supportedLang == lang {
			supported = true
			break
		}
	}

	if !supported {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   middleware.TranslateError(c, "not_found"),
			"message": "Language not found",
		})
		return
	}

	translations := h.i18n.GetTranslations(lang)

	switch format {
	case "csv":
		// Export as CSV
		var csvContent strings.Builder
		csvContent.WriteString("key,value\n")

		// Flatten translations for CSV
		h.flattenTranslations(translations, "", &csvContent)

		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.csv\"", lang))
		c.String(http.StatusOK, csvContent.String())

	default:
		// Export as JSON
		c.Header("Content-Type", "application/json")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.json\"", lang))
		c.JSON(http.StatusOK, translations)
	}
}

// flattenTranslations flattens nested translations for CSV export.
func (h *I18nHandlers) flattenTranslations(m map[string]interface{}, prefix string, writer *strings.Builder) {
	for key, value := range m {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		if nestedMap, ok := value.(map[string]interface{}); ok {
			h.flattenTranslations(nestedMap, fullKey, writer)
		} else if str, ok := value.(string); ok {
			// Escape quotes in CSV
			escaped := strings.ReplaceAll(str, "\"", "\"\"")
			writer.WriteString(fmt.Sprintf("\"%s\",\"%s\"\n", fullKey, escaped))
		}
	}
}

// @Router /api/v1/i18n/validate/{lang} [get].
func (h *I18nHandlers) ValidateTranslations(c *gin.Context) {
	lang := c.Param("lang")

	// Check if language is supported
	supported := false
	for _, supportedLang := range h.i18n.GetSupportedLanguages() {
		if supportedLang == lang {
			supported = true
			break
		}
	}

	if !supported {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   middleware.TranslateError(c, "not_found"),
			"message": "Language not found",
		})
		return
	}

	baseKeys := h.i18n.GetAllKeys("en")
	langKeys := h.i18n.GetAllKeys(lang)

	totalKeys := len(baseKeys)
	translatedKeys := len(langKeys)
	coverage := float64(translatedKeys) / float64(totalKeys) * 100.0

	var errors []string
	var warnings []string

	// Check for missing keys
	langKeyMap := make(map[string]bool)
	for _, key := range langKeys {
		langKeyMap[key] = true
	}

	missingCount := 0
	for _, key := range baseKeys {
		if !langKeyMap[key] {
			missingCount++
			if missingCount <= 10 { // Only show first 10 missing keys
				warnings = append(warnings, fmt.Sprintf("Missing key: %s", key))
			}
		}
	}

	if missingCount > 10 {
		warnings = append(warnings, fmt.Sprintf("... and %d more missing keys", missingCount-10))
	}

	// Check for extra keys not in base language
	baseKeyMap := make(map[string]bool)
	for _, key := range baseKeys {
		baseKeyMap[key] = true
	}

	extraCount := 0
	for _, key := range langKeys {
		if !baseKeyMap[key] {
			extraCount++
			if extraCount <= 5 {
				warnings = append(warnings, fmt.Sprintf("Extra key not in base language: %s", key))
			}
		}
	}

	if extraCount > 5 {
		warnings = append(warnings, fmt.Sprintf("... and %d more extra keys", extraCount-5))
	}

	isValid := len(errors) == 0
	isComplete := coverage == 100.0

	c.JSON(http.StatusOK, ValidationResponse{
		Language:   lang,
		IsValid:    isValid,
		IsComplete: isComplete,
		Coverage:   coverage,
		Errors:     errors,
		Warnings:   warnings,
	})
}

// Helper methods

func (h *I18nHandlers) getLanguageName(code string) string {
	names := map[string]string{
		"en":  "English",
		"es":  "Spanish",
		"fr":  "French",
		"de":  "German",
		"pt":  "Portuguese",
		"ja":  "Japanese",
		"zh":  "Chinese",
		"ar":  "Arabic",
		"he":  "Hebrew",
		"fa":  "Persian",
		"ur":  "Urdu",
		"tlh": "Klingon",
	}
	if name, ok := names[code]; ok {
		return name
	}
	return code
}

func (h *I18nHandlers) getLanguageNativeName(code string) string {
	names := map[string]string{
		"en":  "English",
		"es":  "Español",
		"fr":  "Français",
		"de":  "Deutsch",
		"pt":  "Português",
		"ja":  "日本語",
		"zh":  "中文",
		"ar":  "العربية",
		"he":  "עברית",
		"fa":  "فارسی",
		"ur":  "اردو",
		"tlh": "tlhIngan Hol",
	}
	if name, ok := names[code]; ok {
		return name
	}
	return code
}

// Request and response types

// LanguageInfo represents information about a language.
type LanguageInfo struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	NativeName string `json:"native_name"`
	Active     bool   `json:"active"`
}

// LanguagesResponse represents the response for supported languages.
type LanguagesResponse struct {
	Languages []LanguageInfo `json:"languages"`
	Current   string         `json:"current"`
	Default   string         `json:"default"`
}

// SetLanguageRequest represents a request to set language.
type SetLanguageRequest struct {
	Language string `json:"language" binding:"required"`
}

// TranslationsResponse represents the response for translations.
type TranslationsResponse struct {
	Language     string                 `json:"language"`
	Translations map[string]interface{} `json:"translations"`
}

// TranslateRequest represents a request to translate a key.
type TranslateRequest struct {
	Key      string        `json:"key" binding:"required"`
	Language string        `json:"language"`
	Args     []interface{} `json:"args"`
}

// TranslateResponse represents the response for translation.
type TranslateResponse struct {
	Key         string `json:"key"`
	Translation string `json:"translation"`
	Language    string `json:"language"`
}

// LanguageStats represents language usage statistics.
type LanguageStats struct {
	Code       string  `json:"code"`
	Users      int     `json:"users"`
	Percentage float64 `json:"percentage"`
}

// LanguageStatsResponse represents language statistics response.
type LanguageStatsResponse struct {
	TotalUsers      int             `json:"total_users"`
	Languages       []LanguageStats `json:"languages"`
	DefaultLanguage string          `json:"default_language"`
}

// LanguageCoverage represents coverage data for a language.
type LanguageCoverage struct {
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	TotalKeys      int     `json:"total_keys"`
	TranslatedKeys int     `json:"translated_keys"`
	MissingCount   int     `json:"missing_count"`
	Coverage       float64 `json:"coverage"`
}

// CoverageResponse represents the coverage statistics response.
type CoverageResponse struct {
	Languages []LanguageCoverage `json:"languages"`
	Summary   struct {
		TotalKeys       int     `json:"total_keys"`
		AverageCoverage float64 `json:"average_coverage"`
	} `json:"summary"`
}

// MissingKey represents a missing translation key.
type MissingKey struct {
	Key          string `json:"key"`
	EnglishValue string `json:"english_value"`
	Category     string `json:"category"`
}

// MissingKeysResponse represents missing keys response.
type MissingKeysResponse struct {
	Language    string       `json:"language"`
	MissingKeys []MissingKey `json:"missing_keys"`
	Count       int          `json:"count"`
}

// ValidationResponse represents translation validation response.
type ValidationResponse struct {
	Language   string   `json:"language"`
	IsValid    bool     `json:"is_valid"`
	IsComplete bool     `json:"is_complete"`
	Coverage   float64  `json:"coverage"`
	Errors     []string `json:"errors"`
	Warnings   []string `json:"warnings"`
}

// RegisterRoutes registers i18n routes.
func (h *I18nHandlers) RegisterRoutes(router *gin.RouterGroup) {
	i18n := router.Group("/i18n")
	{
		i18n.GET("/languages", h.GetSupportedLanguages)
		i18n.POST("/language", h.SetLanguage)
		i18n.GET("/translations", h.GetCurrentTranslations)
		i18n.GET("/translations/:lang", h.GetTranslations)
		i18n.POST("/translate", h.Translate)
		i18n.GET("/stats", h.GetLanguageStats)
		i18n.GET("/coverage", h.GetTranslationCoverage)
		i18n.GET("/missing/:lang", h.GetMissingTranslations)
		i18n.GET("/export/:lang", h.ExportTranslations)
		i18n.GET("/validate/:lang", h.ValidateTranslations)
	}
}
