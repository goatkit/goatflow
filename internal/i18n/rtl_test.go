package i18n

import (
	"testing"
)

func TestLanguageDirectionConstants(t *testing.T) {
	if LTR != "ltr" {
		t.Errorf("LTR = %q, want %q", LTR, "ltr")
	}
	if RTL != "rtl" {
		t.Errorf("RTL = %q, want %q", RTL, "rtl")
	}
}

func TestSupportedLanguages_RTL(t *testing.T) {
	// Test that required languages exist in metadata
	required := []string{"en", "de", "ar", "he", "fa", "es", "fr"}
	for _, code := range required {
		if _, exists := SupportedLanguages[code]; !exists {
			t.Errorf("SupportedLanguages missing %q", code)
		}
	}

	// Test English configuration
	en := SupportedLanguages["en"]
	if en.Code != "en" {
		t.Errorf("en.Code = %q, want %q", en.Code, "en")
	}
	if en.Name != "English" {
		t.Errorf("en.Name = %q, want %q", en.Name, "English")
	}
	if en.Direction != LTR {
		t.Errorf("en.Direction = %q, want %q", en.Direction, LTR)
	}

	// English should be enabled (has JSON file)
	enabledLangs := GetEnabledLanguages()
	foundEnglish := false
	for _, lang := range enabledLangs {
		if lang.Code == "en" {
			foundEnglish = true
			break
		}
	}
	if !foundEnglish {
		t.Error("English should be enabled (en.json must exist)")
	}
}

func TestGetLanguageConfig(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		wantName string
		wantOK   bool
	}{
		{"English exists", "en", "English", true},
		{"German exists", "de", "German", true},
		{"Arabic exists", "ar", "Arabic", true},
		{"Unknown language", "xyz", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, ok := GetLanguageConfig(tt.code)
			if ok != tt.wantOK {
				t.Errorf("GetLanguageConfig(%q) ok = %v, want %v", tt.code, ok, tt.wantOK)
			}
			if ok && config.Name != tt.wantName {
				t.Errorf("GetLanguageConfig(%q).Name = %q, want %q", tt.code, config.Name, tt.wantName)
			}
		})
	}
}

func TestIsRTL(t *testing.T) {
	tests := []struct {
		name string
		code string
		want bool
	}{
		{"English is LTR", "en", false},
		{"German is LTR", "de", false},
		{"French is LTR", "fr", false},
		{"Spanish is LTR", "es", false},
		{"Arabic is RTL", "ar", true},
		{"Hebrew is RTL", "he", true},
		{"Persian is RTL", "fa", true},
		{"Urdu is RTL", "ur", true},
		{"Unknown defaults to LTR", "xyz", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRTL(tt.code)
			if got != tt.want {
				t.Errorf("IsRTL(%q) = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
}

func TestGetDirection(t *testing.T) {
	tests := []struct {
		name string
		code string
		want LanguageDirection
	}{
		{"English", "en", LTR},
		{"German", "de", LTR},
		{"Arabic", "ar", RTL},
		{"Hebrew", "he", RTL},
		{"Unknown", "xyz", LTR},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetDirection(tt.code)
			if got != tt.want {
				t.Errorf("GetDirection(%q) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

func TestGetEnabledLanguages(t *testing.T) {
	enabled := GetEnabledLanguages()

	if len(enabled) == 0 {
		t.Error("GetEnabledLanguages returned empty list")
	}

	// Check that all returned languages are enabled
	for _, lang := range enabled {
		if !lang.Enabled {
			t.Errorf("GetEnabledLanguages returned disabled language: %s", lang.Code)
		}
	}

	// Check that English is in the list
	hasEnglish := false
	for _, lang := range enabled {
		if lang.Code == "en" {
			hasEnglish = true
			break
		}
	}
	if !hasEnglish {
		t.Error("English should be in enabled languages")
	}
}

func TestConvertDigits(t *testing.T) {
	tests := []struct {
		name   string
		number string
		lang   string
		want   string
	}{
		{"English keeps digits", "12345", "en", "12345"},
		{"German keeps digits", "12345", "de", "12345"},
		{"Unknown lang keeps digits", "12345", "xyz", "12345"},
		{"Arabic converts digits", "0123456789", "ar", "٠١٢٣٤٥٦٧٨٩"},
		{"Persian converts digits", "0123456789", "fa", "۰۱۲۳۴۵۶۷۸۹"},
		{"Mixed content preserved", "abc123", "en", "abc123"},
		{"Empty string", "", "en", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertDigits(tt.number, tt.lang)
			if got != tt.want {
				t.Errorf("ConvertDigits(%q, %q) = %q, want %q", tt.number, tt.lang, got, tt.want)
			}
		})
	}
}

func TestGetCSSClass(t *testing.T) {
	tests := []struct {
		name     string
		lang     string
		contains []string
	}{
		{
			name:     "English",
			lang:     "en",
			contains: []string{"lang-en", "ltr"},
		},
		{
			name:     "Arabic",
			lang:     "ar",
			contains: []string{"lang-ar", "rtl"},
		},
		{
			name:     "Hebrew",
			lang:     "he",
			contains: []string{"lang-he", "rtl"},
		},
		{
			name:     "Unknown defaults to en",
			lang:     "xyz",
			contains: []string{"lang-en", "ltr"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetCSSClass(tt.lang)
			for _, want := range tt.contains {
				if !containsStr(got, want) {
					t.Errorf("GetCSSClass(%q) = %q, should contain %q", tt.lang, got, want)
				}
			}
		})
	}
}

func TestGetHTMLAttributes(t *testing.T) {
	tests := []struct {
		name    string
		lang    string
		wantDir string
	}{
		{"English", "en", "ltr"},
		{"German", "de", "ltr"},
		{"Arabic", "ar", "rtl"},
		{"Hebrew", "he", "rtl"},
		{"Unknown defaults to en", "xyz", "ltr"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs := GetHTMLAttributes(tt.lang)

			if attrs["dir"] != tt.wantDir {
				t.Errorf("GetHTMLAttributes(%q)[dir] = %q, want %q", tt.lang, attrs["dir"], tt.wantDir)
			}

			if _, hasLang := attrs["lang"]; !hasLang {
				t.Errorf("GetHTMLAttributes(%q) missing 'lang' attribute", tt.lang)
			}
		})
	}
}

func TestLanguageConfig_NumberFormat(t *testing.T) {
	// Test German number format
	de := SupportedLanguages["de"]
	if de.NumberFormat.DecimalSeparator != "," {
		t.Errorf("German decimal separator = %q, want %q", de.NumberFormat.DecimalSeparator, ",")
	}
	if de.NumberFormat.ThousandSeparator != "." {
		t.Errorf("German thousand separator = %q, want %q", de.NumberFormat.ThousandSeparator, ".")
	}

	// Test English number format
	en := SupportedLanguages["en"]
	if en.NumberFormat.DecimalSeparator != "." {
		t.Errorf("English decimal separator = %q, want %q", en.NumberFormat.DecimalSeparator, ".")
	}
	if en.NumberFormat.ThousandSeparator != "," {
		t.Errorf("English thousand separator = %q, want %q", en.NumberFormat.ThousandSeparator, ",")
	}
}

func TestLanguageConfig_CurrencyFormat(t *testing.T) {
	// Test English (USD)
	en := SupportedLanguages["en"]
	if en.Currency.Symbol != "$" {
		t.Errorf("English currency symbol = %q, want %q", en.Currency.Symbol, "$")
	}
	if en.Currency.Position != "before" {
		t.Errorf("English currency position = %q, want %q", en.Currency.Position, "before")
	}

	// Test German (EUR)
	de := SupportedLanguages["de"]
	if de.Currency.Symbol != "€" {
		t.Errorf("German currency symbol = %q, want %q", de.Currency.Symbol, "€")
	}
	if de.Currency.Position != "after" {
		t.Errorf("German currency position = %q, want %q", de.Currency.Position, "after")
	}
}

func TestFormatCurrency(t *testing.T) {
	// Basic test that it doesn't panic
	result := FormatCurrency(100.00, "en")
	if result == "" {
		t.Error("FormatCurrency returned empty string")
	}

	resultDe := FormatCurrency(100.00, "de")
	if resultDe == "" {
		t.Error("FormatCurrency for German returned empty string")
	}

	// Unknown language should default to English-like behavior
	resultUnknown := FormatCurrency(100.00, "xyz")
	if resultUnknown == "" {
		t.Error("FormatCurrency for unknown language returned empty string")
	}
}

func TestFormatNumber(t *testing.T) {
	// Basic test that it doesn't panic
	result := FormatNumber(1234.56, "en", 2)
	if result == "" {
		t.Error("FormatNumber returned empty string")
	}

	// Test with unknown language (should default)
	resultUnknown := FormatNumber(1234.56, "xyz", 2)
	if resultUnknown == "" {
		t.Error("FormatNumber for unknown language returned empty string")
	}
}

// Helper function.
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findSubstr(s, substr))
}

func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
