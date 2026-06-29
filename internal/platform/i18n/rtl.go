package i18n

// LanguageDirection represents text direction.
type LanguageDirection string

const (
	// LTR represents left-to-right text direction.
	LTR LanguageDirection = "ltr"
	// RTL represents right-to-left text direction.
	RTL LanguageDirection = "rtl"
)

// LanguageConfig contains configuration for each language.
type LanguageConfig struct {
	Code         string            `json:"code"`
	Name         string            `json:"name"`
	NativeName   string            `json:"native_name"`
	Direction    LanguageDirection `json:"direction"`
	DateFormat   string            `json:"date_format"`
	TimeFormat   string            `json:"time_format"`
	NumberFormat NumberFormat      `json:"number_format"`
	Currency     CurrencyFormat    `json:"currency"`
	Enabled      bool              `json:"enabled"`
}

// NumberFormat represents number formatting configuration.
type NumberFormat struct {
	DecimalSeparator  string `json:"decimal_separator"`
	ThousandSeparator string `json:"thousand_separator"`
	Digits            string `json:"digits"` // For languages with different digit systems
}

// CurrencyFormat represents currency formatting configuration.
type CurrencyFormat struct {
	Symbol           string `json:"symbol"`
	Code             string `json:"code"`
	Position         string `json:"position"` // before or after
	DecimalPlaces    int    `json:"decimal_places"`
	SpaceAfterSymbol bool   `json:"space_after_symbol"`
}

// SupportedLanguages contains configuration for all supported languages.
var SupportedLanguages = map[string]LanguageConfig{
	"en": {
		Code:       "en",
		Name:       "English",
		NativeName: "English",
		Direction:  LTR,
		DateFormat: "Jan 2, 2006",
		TimeFormat: "3:04 PM",
		NumberFormat: NumberFormat{
			DecimalSeparator:  ".",
			ThousandSeparator: ",",
			Digits:            "0123456789",
		},
		Currency: CurrencyFormat{
			Symbol:           "$",
			Code:             "USD",
			Position:         "before",
			DecimalPlaces:    2,
			SpaceAfterSymbol: false,
		},
			},
	"es": {
		Code:       "es",
		Name:       "Spanish",
		NativeName: "Español",
		Direction:  LTR,
		DateFormat: "2 de Jan de 2006",
		TimeFormat: "15:04",
		NumberFormat: NumberFormat{
			DecimalSeparator:  ",",
			ThousandSeparator: ".",
			Digits:            "0123456789",
		},
		Currency: CurrencyFormat{
			Symbol:           "€",
			Code:             "EUR",
			Position:         "after",
			DecimalPlaces:    2,
			SpaceAfterSymbol: true,
		},
			},
	"fr": {
		Code:       "fr",
		Name:       "French",
		NativeName: "Français",
		Direction:  LTR,
		DateFormat: "2 Jan 2006",
		TimeFormat: "15:04",
		NumberFormat: NumberFormat{
			DecimalSeparator:  ",",
			ThousandSeparator: " ",
			Digits:            "0123456789",
		},
		Currency: CurrencyFormat{
			Symbol:           "€",
			Code:             "EUR",
			Position:         "after",
			DecimalPlaces:    2,
			SpaceAfterSymbol: true,
		},
			},
	"de": {
		Code:       "de",
		Name:       "German",
		NativeName: "Deutsch",
		Direction:  LTR,
		DateFormat: "2. Jan 2006",
		TimeFormat: "15:04",
		NumberFormat: NumberFormat{
			DecimalSeparator:  ",",
			ThousandSeparator: ".",
			Digits:            "0123456789",
		},
		Currency: CurrencyFormat{
			Symbol:           "€",
			Code:             "EUR",
			Position:         "after",
			DecimalPlaces:    2,
			SpaceAfterSymbol: true,
		},
			},
	"ar": {
		Code:       "ar",
		Name:       "Arabic",
		NativeName: "العربية",
		Direction:  RTL,
		DateFormat: "2 Jan 2006",
		TimeFormat: "15:04",
		NumberFormat: NumberFormat{
			DecimalSeparator:  "٫",
			ThousandSeparator: "٬",
			Digits:            "٠١٢٣٤٥٦٧٨٩", // Arabic-Indic digits
		},
		Currency: CurrencyFormat{
			Symbol:           "ر.س",
			Code:             "SAR",
			Position:         "after",
			DecimalPlaces:    2,
			SpaceAfterSymbol: true,
		},
			},
	"he": {
		Code:       "he",
		Name:       "Hebrew",
		NativeName: "עברית",
		Direction:  RTL,
		DateFormat: "2 Jan 2006",
		TimeFormat: "15:04",
		NumberFormat: NumberFormat{
			DecimalSeparator:  ".",
			ThousandSeparator: ",",
			Digits:            "0123456789",
		},
		Currency: CurrencyFormat{
			Symbol:           "₪",
			Code:             "ILS",
			Position:         "before",
			DecimalPlaces:    2,
			SpaceAfterSymbol: true,
		},
			},
	"fa": {
		Code:       "fa",
		Name:       "Persian",
		NativeName: "فارسی",
		Direction:  RTL,
		DateFormat: "2 Jan 2006",
		TimeFormat: "15:04",
		NumberFormat: NumberFormat{
			DecimalSeparator:  "٫",
			ThousandSeparator: "٬",
			Digits:            "۰۱۲۳۴۵۶۷۸۹", // Persian digits
		},
		Currency: CurrencyFormat{
			Symbol:           "﷼",
			Code:             "IRR",
			Position:         "after",
			DecimalPlaces:    0,
			SpaceAfterSymbol: true,
		},
			},
	"ur": {
		Code:       "ur",
		Name:       "Urdu",
		NativeName: "اردو",
		Direction:  RTL,
		DateFormat: "2 Jan 2006",
		TimeFormat: "15:04",
		NumberFormat: NumberFormat{
			DecimalSeparator:  ".",
			ThousandSeparator: ",",
			Digits:            "۰۱۲۳۴۵۶۷۸۹", // Urdu digits
		},
		Currency: CurrencyFormat{
			Symbol:           "Rs",
			Code:             "PKR",
			Position:         "before",
			DecimalPlaces:    2,
			SpaceAfterSymbol: true,
		},
			},
	"ja": {
		Code:       "ja",
		Name:       "Japanese",
		NativeName: "日本語",
		Direction:  LTR,
		DateFormat: "2006年1月2日",
		TimeFormat: "15:04",
		NumberFormat: NumberFormat{
			DecimalSeparator:  ".",
			ThousandSeparator: ",",
			Digits:            "0123456789",
		},
		Currency: CurrencyFormat{
			Symbol:           "¥",
			Code:             "JPY",
			Position:         "before",
			DecimalPlaces:    0,
			SpaceAfterSymbol: false,
		},
			},
	"zh": {
		Code:       "zh",
		Name:       "Chinese",
		NativeName: "中文",
		Direction:  LTR,
		DateFormat: "2006年1月2日",
		TimeFormat: "15:04",
		NumberFormat: NumberFormat{
			DecimalSeparator:  ".",
			ThousandSeparator: ",",
			Digits:            "0123456789",
		},
		Currency: CurrencyFormat{
			Symbol:           "¥",
			Code:             "CNY",
			Position:         "before",
			DecimalPlaces:    2,
			SpaceAfterSymbol: false,
		},
			},
	"pl": {
		Code:       "pl",
		Name:       "Polish",
		NativeName: "Polski",
		Direction:  LTR,
		DateFormat: "2 Jan 2006",
		TimeFormat: "15:04",
		NumberFormat: NumberFormat{
			DecimalSeparator:  ",",
			ThousandSeparator: " ",
			Digits:            "0123456789",
		},
		Currency: CurrencyFormat{
			Symbol:           "zł",
			Code:             "PLN",
			Position:         "after",
			DecimalPlaces:    2,
			SpaceAfterSymbol: true,
		},
			},
	"pt": {
		Code:       "pt",
		Name:       "Portuguese",
		NativeName: "Português",
		Direction:  LTR,
		DateFormat: "2 de Jan de 2006",
		TimeFormat: "15:04",
		NumberFormat: NumberFormat{
			DecimalSeparator:  ",",
			ThousandSeparator: ".",
			Digits:            "0123456789",
		},
		Currency: CurrencyFormat{
			Symbol:           "R$",
			Code:             "BRL",
			Position:         "before",
			DecimalPlaces:    2,
			SpaceAfterSymbol: true,
		},
			},
	"tlh": {
		Code:       "tlh",
		Name:       "Klingon",
		NativeName: "tlhIngan Hol",
		Direction:  LTR,
		DateFormat: "2 Jan 2006",
		TimeFormat: "15:04",
		NumberFormat: NumberFormat{
			DecimalSeparator:  ".",
			ThousandSeparator: ",",
			Digits:            "0123456789", // Klingon uses standard digits in practice
		},
		Currency: CurrencyFormat{
			Symbol:           "DarSeq",
			Code:             "DRK",
			Position:         "after",
			DecimalPlaces:    2,
			SpaceAfterSymbol: true,
		},
			},
	"ru": {
		Code:       "ru",
		Name:       "Russian",
		NativeName: "Русский",
		Direction:  LTR,
		DateFormat: "2 Jan 2006",
		TimeFormat: "15:04",
		NumberFormat: NumberFormat{
			DecimalSeparator:  ",",
			ThousandSeparator: " ",
			Digits:            "0123456789",
		},
		Currency: CurrencyFormat{
			Symbol:           "₽",
			Code:             "RUB",
			Position:         "after",
			DecimalPlaces:    2,
			SpaceAfterSymbol: true,
		},
			},
	"uk": {
		Code:       "uk",
		Name:       "Ukrainian",
		NativeName: "Українська",
		Direction:  LTR,
		DateFormat: "2 Jan 2006",
		TimeFormat: "15:04",
		NumberFormat: NumberFormat{
			DecimalSeparator:  ",",
			ThousandSeparator: " ",
			Digits:            "0123456789",
		},
		Currency: CurrencyFormat{
			Symbol:           "₴",
			Code:             "UAH",
			Position:         "after",
			DecimalPlaces:    2,
			SpaceAfterSymbol: true,
		},
			},
}

// GetLanguageConfig returns configuration for a language.
func GetLanguageConfig(code string) (LanguageConfig, bool) {
	config, exists := SupportedLanguages[code]
	return config, exists
}

// IsRTL checks if a language is right-to-left.
func IsRTL(code string) bool {
	if config, exists := SupportedLanguages[code]; exists {
		return config.Direction == RTL
	}
	return false
}

// GetDirection returns the text direction for a language.
func GetDirection(code string) LanguageDirection {
	if config, exists := SupportedLanguages[code]; exists {
		return config.Direction
	}
	return LTR
}

// GetEnabledLanguages returns languages that have translation files.
// A language is considered enabled if its JSON file exists in the embedded translations.
func GetEnabledLanguages() []LanguageConfig {
	var enabled []LanguageConfig
	for code, config := range SupportedLanguages {
		if translationFileExists(code) {
			config.Enabled = true
			enabled = append(enabled, config)
		}
	}
	return enabled
}

// translationFileExists checks if xx.json exists in embedded translations.
func translationFileExists(code string) bool {
	_, err := translationsFS.ReadFile("translations/" + code + ".json")
	return err == nil
}

// ConvertDigits converts Western digits to locale-specific digits.
func ConvertDigits(number string, lang string) string {
	config, exists := SupportedLanguages[lang]
	if !exists || config.NumberFormat.Digits == "0123456789" {
		return number
	}

	result := ""
	for _, char := range number {
		if char >= '0' && char <= '9' {
			index := int(char - '0')
			// For Arabic/Persian/Urdu digits
			result += string([]rune(config.NumberFormat.Digits)[index])
		} else {
			result += string(char)
		}
	}

	return result
}

// FormatNumber formats a number according to language configuration.
func FormatNumber(value float64, lang string, decimals int) string {
	_, exists := SupportedLanguages[lang]
	if !exists {
		lang = "en"
	}

	// This is a simplified implementation
	// In production, use a proper number formatting library

	// Format the number
	// TODO: Implement proper number formatting with separators

	return ConvertDigits("formatted_number", lang)
}

// FormatCurrency formats currency according to language configuration.
func FormatCurrency(amount float64, lang string) string {
	config, exists := SupportedLanguages[lang]
	if !exists {
		config = SupportedLanguages["en"]
	}

	// Format the number part
	numberStr := FormatNumber(amount, lang, config.Currency.DecimalPlaces)

	// Apply currency symbol
	if config.Currency.Position == "before" {
		if config.Currency.SpaceAfterSymbol {
			return config.Currency.Symbol + " " + numberStr
		}
		return config.Currency.Symbol + numberStr
	} else {
		if config.Currency.SpaceAfterSymbol {
			return numberStr + " " + config.Currency.Symbol
		}
		return numberStr + config.Currency.Symbol
	}
}

// GetCSSClass returns CSS classes for language-specific styling.
func GetCSSClass(lang string) string {
	config, exists := SupportedLanguages[lang]
	if !exists {
		return "lang-en ltr"
	}

	classes := "lang-" + lang
	if config.Direction == RTL {
		classes += " rtl"
	} else {
		classes += " ltr"
	}

	return classes
}

// GetHTMLAttributes returns HTML attributes for language support.
func GetHTMLAttributes(lang string) map[string]string {
	config, exists := SupportedLanguages[lang]
	if !exists {
		config = SupportedLanguages["en"]
	}

	return map[string]string{
		"lang": config.Code,
		"dir":  string(config.Direction),
	}
}
