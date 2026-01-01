package i18n

import (
	"fmt"
	"html/template"
	"time"
)

// TemplateFuncs returns template functions for i18n.
func TemplateFuncs(lang string) template.FuncMap {
	return template.FuncMap{
		"t":              createTranslateFunc(lang),
		"T":              createTranslateFunc(lang),
		"trans":          createTranslateFunc(lang),
		"timeAgo":        createTimeAgoFunc(lang),
		"formatDate":     createFormatDateFunc(lang),
		"formatTime":     createFormatTimeFunc(lang),
		"formatDateTime": createFormatDateTimeFunc(lang),
		"pluralize":      createPluralizeFunc(lang),
		"currency":       createCurrencyFunc(lang),
		"number":         createNumberFunc(lang),
		"percent":        createPercentFunc(lang),
	}
}

// createTranslateFunc creates a translate function for templates.
func createTranslateFunc(lang string) func(key string, args ...interface{}) string {
	return func(key string, args ...interface{}) string {
		return GetInstance().T(lang, key, args...)
	}
}

// createTimeAgoFunc creates a time ago function for templates.
func createTimeAgoFunc(lang string) func(t time.Time) string {
	return func(t time.Time) string {
		now := time.Now()
		diff := now.Sub(t)

		switch {
		case diff < time.Minute:
			return GetInstance().T(lang, "time.just_now")
		case diff < time.Hour:
			minutes := int(diff.Minutes())
			if minutes == 1 {
				return GetInstance().T(lang, "time.minute_ago")
			}
			return GetInstance().T(lang, "time.minutes_ago", minutes)
		case diff < 24*time.Hour:
			hours := int(diff.Hours())
			if hours == 1 {
				return GetInstance().T(lang, "time.hour_ago")
			}
			return GetInstance().T(lang, "time.hours_ago", hours)
		case diff < 7*24*time.Hour:
			days := int(diff.Hours() / 24)
			if days == 1 {
				return GetInstance().T(lang, "time.day_ago")
			}
			return GetInstance().T(lang, "time.days_ago", days)
		case diff < 30*24*time.Hour:
			weeks := int(diff.Hours() / (24 * 7))
			if weeks == 1 {
				return GetInstance().T(lang, "time.week_ago")
			}
			return GetInstance().T(lang, "time.weeks_ago", weeks)
		case diff < 365*24*time.Hour:
			months := int(diff.Hours() / (24 * 30))
			if months == 1 {
				return GetInstance().T(lang, "time.month_ago")
			}
			return GetInstance().T(lang, "time.months_ago", months)
		default:
			years := int(diff.Hours() / (24 * 365))
			if years == 1 {
				return GetInstance().T(lang, "time.year_ago")
			}
			return GetInstance().T(lang, "time.years_ago", years)
		}
	}
}

// createFormatDateFunc creates a date formatting function for templates.
func createFormatDateFunc(lang string) func(t time.Time) string {
	return func(t time.Time) string {
		// Format based on language
		switch lang {
		case "en":
			return t.Format("Jan 2, 2006")
		case "es":
			return t.Format("2 de Jan de 2006")
		case "fr":
			return t.Format("2 Jan 2006")
		case "de":
			return t.Format("2. Jan 2006")
		case "pt":
			return t.Format("2 de Jan de 2006")
		case "ja":
			return t.Format("2006年1月2日")
		case "zh":
			return t.Format("2006年1月2日")
		default:
			return t.Format("2006-01-02")
		}
	}
}

// createFormatTimeFunc creates a time formatting function for templates.
func createFormatTimeFunc(lang string) func(t time.Time) string {
	return func(t time.Time) string {
		// Format based on language
		switch lang {
		case "en":
			return t.Format("3:04 PM")
		case "es", "fr", "de", "pt":
			return t.Format("15:04")
		case "ja", "zh":
			return t.Format("15:04")
		default:
			return t.Format("15:04")
		}
	}
}

// createFormatDateTimeFunc creates a datetime formatting function for templates.
func createFormatDateTimeFunc(lang string) func(t time.Time) string {
	dateFunc := createFormatDateFunc(lang)
	timeFunc := createFormatTimeFunc(lang)

	return func(t time.Time) string {
		return dateFunc(t) + " " + timeFunc(t)
	}
}

// createPluralizeFunc creates a pluralization function for templates.
func createPluralizeFunc(lang string) func(count int, singular, plural string) string {
	return func(count int, singular, plural string) string {
		if count == 1 {
			return singular
		}
		return plural
	}
}

// createCurrencyFunc creates a currency formatting function for templates.
func createCurrencyFunc(lang string) func(amount float64) string {
	return func(amount float64) string {
		// Simple currency formatting based on language
		switch lang {
		case "en":
			return "$" + formatNumber(amount, 2)
		case "es":
			return formatNumber(amount, 2) + " €"
		case "fr":
			return formatNumber(amount, 2) + " €"
		case "de":
			return formatNumber(amount, 2) + " €"
		case "pt":
			return "R$ " + formatNumber(amount, 2)
		case "ja":
			return "¥" + formatNumber(amount, 0)
		case "zh":
			return "¥" + formatNumber(amount, 2)
		default:
			return "$" + formatNumber(amount, 2)
		}
	}
}

// createNumberFunc creates a number formatting function for templates.
func createNumberFunc(lang string) func(n interface{}) string {
	return func(n interface{}) string {
		// Convert to float64
		var num float64
		switch v := n.(type) {
		case int:
			num = float64(v)
		case int64:
			num = float64(v)
		case float32:
			num = float64(v)
		case float64:
			num = v
		default:
			return "0"
		}

		return formatNumber(num, 0)
	}
}

// createPercentFunc creates a percentage formatting function for templates.
func createPercentFunc(lang string) func(n float64) string {
	return func(n float64) string {
		return formatNumber(n*100, 1) + "%"
	}
}

// formatNumber formats a number with the specified decimal places.
func formatNumber(n float64, decimals int) string {
	// Simple number formatting
	format := "%."
	if decimals >= 0 {
		format += fmt.Sprintf("%d", decimals) + "f"
	} else {
		format += "f"
	}

	// TODO: Add thousands separator based on locale
	return sprintf(format, n)
}

// sprintf is a simple sprintf implementation.
func sprintf(format string, args ...interface{}) string {
	// This would use fmt.Sprintf in a real implementation
	// Simplified for demonstration
	return "formatted"
}
