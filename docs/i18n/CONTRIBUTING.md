# i18n Contributing Guide

This guide explains how to add new language support or improve existing translations in GOTRS.

## Table of Contents
- [Quick Start](#quick-start)
- [Translation Structure](#translation-structure)
- [Adding a New Language](#adding-a-new-language)
- [Testing Translations](#testing-translations)
- [Best Practices](#best-practices)

## Quick Start

To add or improve translations:

1. Navigate to `internal/i18n/translations/`
2. Edit existing language files or create new ones
3. Test your translations using `make test-i18n`
4. Submit a PR with 100% coverage

### Check Current Coverage

Before starting, check the current translation coverage for a language:

```bash
# Check missing keys for any language (requires authentication)
make http-call ENDPOINT=/api/v1/i18n/missing/xx

# Example: check German coverage
make http-call ENDPOINT=/api/v1/i18n/missing/de
# {"language":"de","missing_keys":null,"count":0}  # 0 = complete
```

## Translation Structure

Translation files are JSON files with nested structure located in `internal/i18n/translations/`:

```json
{
  "app": {
    "name": "GOTRS",
    "title": "GOTRS - Ticketing System"
  },
  "navigation": {
    "dashboard": "Dashboard",
    "tickets": "Tickets"
  }
}
```

### Key Naming Convention
- Use lowercase with underscores: `ticket_created`
- Group related keys: `tickets.new_ticket`
- Use consistent prefixes: `button.save`, `label.email`

## Adding a New Language

Adding a new language requires two steps:

### Step 1: Create Translation File

1. Copy `en.json` as template:
```bash
cp internal/i18n/translations/en.json internal/i18n/translations/xx.json
```

2. Replace `xx` with your language code (ISO 639-1, e.g., `pl` for Polish)

3. Translate all keys in the new file

### Step 2: Add Language Configuration

Add your language to the `SupportedLanguages` map in `internal/i18n/rtl.go`:

```go
"xx": {
    Code:       "xx",
    Name:       "Language Name",      // English name
    NativeName: "Native Name",        // Name in the language itself
    Direction:  LTR,                  // or RTL for right-to-left languages
    DateFormat: "2 Jan 2006",
    TimeFormat: "15:04",
    NumberFormat: NumberFormat{
        DecimalSeparator:  ".",
        ThousandSeparator: ",",
        Digits:            "0123456789",
    },
    Currency: CurrencyFormat{
        Symbol:           "$",
        Code:             "XXX",      // ISO 4217 currency code
        Position:         "before",   // or "after"
        DecimalPlaces:    2,
        SpaceAfterSymbol: false,
    }
}
```

**Important:** `rtl.go` is the single source of truth for language metadata. The `Name` and `NativeName` fields are used throughout the application (API responses, UI dropdowns, etc.). Do not duplicate language names elsewhere.

### Step 3: Validate Completeness

Run the i18n tests to ensure 100% coverage:

```bash
# Run i18n validation tests
make test-i18n

# Or run directly via Docker
docker compose run --rm toolbox go test ./internal/i18n/... -v
```

To verify the language is now complete via the API:

```bash
# Verify no missing keys (requires authentication)
make http-call ENDPOINT=/api/v1/i18n/missing/xx

# Expected output for a complete language:
# {"language":"xx","missing_keys":null,"count":0}
```

### Step 4: Rebuild and Test

```bash
# Rebuild to embed new translations
make build

# Restart to apply changes
make restart

# Test in the UI by changing language in profile settings
```

## Testing Translations

### Running Tests

```bash
# Run all i18n tests
make test-i18n

# Run specific test
docker compose run --rm toolbox go test ./internal/i18n/... -run TestTranslationCompleteness -v
```

### Coverage Requirements

- **100% coverage required** - All English keys must have translations
- **Extra keys allowed** - Languages may have additional keys for locale-specific content
- **Format consistency** - Maintain placeholder formatting (%s, %d, {{variable}})

### Test Output Example

```
=== RUN   TestTranslationCompleteness/pl
    validation_test.go:105: Language pl coverage: 100.0% (1587/1587 keys)
--- PASS: TestTranslationCompleteness/pl (0.00s)
```

## Best Practices

### 1. Context Matters
Consider where text appears when translating:
- Button text should be concise
- Help text can be more descriptive
- Error messages should be clear and actionable

### 2. Maintain Consistency
- Use consistent terminology throughout
- Follow the glossary for technical terms
- Keep formatting consistent with English

### 3. Handle Placeholders
Preserve placeholders in translations:
```json
"min_length": "Minimum length is %d characters"
"welcome": "Welcome, {{name}}!"
```

### 4. Cultural Adaptation
- Use appropriate date/time formats (configured in `rtl.go`)
- Consider text direction (RTL languages like Arabic, Hebrew)
- Adapt idioms and examples appropriately

### 5. Testing Process
1. Complete all translations
2. Run `make test-i18n`
3. Rebuild with `make build`
4. Test in application UI
5. Review with native speakers if possible

## Translation Guidelines

### General Rules
- Keep translations natural and fluent
- Don't translate literally if it sounds unnatural
- Maintain professional tone
- Use formal/informal address consistently

### Technical Terms
Some terms may remain in English depending on locale conventions:
- API
- URL
- Email (or localized equivalent)
- SLA

### Common Patterns

#### Status Messages
```json
"status": {
  "new": "New",
  "open": "Open",
  "closed": "Closed"
}
```

#### Form Labels
```json
"labels": {
  "email": "Email Address",
  "phone": "Phone Number",
  "required": "Required Field"
}
```

#### Error Messages
```json
"errors": {
  "not_found": "Resource not found",
  "unauthorized": "You are not authorized",
  "try_again": "Please try again later"
}
```

## Contributing Process

1. **Fork the repository**
2. **Create feature branch**: `git checkout -b i18n/add-xx-language`
3. **Add translation file**: `internal/i18n/translations/xx.json`
4. **Add language config**: Update `internal/i18n/rtl.go`
5. **Test thoroughly**: Run `make test-i18n`
6. **Submit PR**: Include test output in description

### PR Checklist
- [ ] Translation file created with all keys from `en.json`
- [ ] Language config added to `rtl.go` with correct metadata
- [ ] `make test-i18n` passes with 100% coverage
- [ ] `make build` succeeds
- [ ] UI tested with new language
- [ ] Native speaker review (preferred)

## File Structure

```
internal/i18n/
├── i18n.go              # Core i18n functionality
├── rtl.go               # Language configs (source of truth for names/metadata)
├── validation_test.go   # Translation coverage tests
└── translations/
    ├── en.json          # English (base language)
    ├── ar.json          # Arabic (RTL)
    ├── de.json          # German
    ├── es.json          # Spanish
    ├── fa.json          # Persian (RTL)
    ├── fr.json          # French
    ├── he.json          # Hebrew (RTL)
    ├── ja.json          # Japanese
    ├── pl.json          # Polish
    ├── pt.json          # Portuguese
    ├── ru.json          # Russian
    ├── tlh.json         # Klingon
    ├── uk.json          # Ukrainian
    ├── ur.json          # Urdu (RTL)
    └── zh.json          # Chinese
```

## Language Status

Current language support (15 languages, 12 complete):

| Language | Code | Direction | Status |
|----------|------|-----------|--------|
| English | en | LTR | ✅ Base Language |
| Arabic | ar | RTL | ✅ Complete |
| German | de | LTR | ✅ Complete |
| Spanish | es | LTR | ✅ Complete |
| French | fr | LTR | ✅ Complete |
| Japanese | ja | LTR | ✅ Complete |
| Polish | pl | LTR | ✅ Complete |
| Portuguese | pt | LTR | ✅ Complete |
| Russian | ru | LTR | ✅ Complete |
| Ukrainian | uk | LTR | ✅ Complete |
| Urdu | ur | RTL | ✅ Complete |
| Klingon | tlh | LTR | ✅ Complete |
| Hebrew | he | RTL | ⚠️ 99.4% |
| Chinese | zh | LTR | ⚠️ 98.4% |
| Persian | fa | RTL | ⚠️ 91.3% |

## Getting Help

- **GitHub Issues**: Report translation bugs or suggestions
- **Pull Requests**: Submit improvements or new languages

## Architecture Notes

### Single Source of Truth

Language metadata is centralized in `internal/i18n/rtl.go`:

- `SupportedLanguages` map contains all language configurations
- `GetLanguageConfig(code)` returns config for a language
- Used by API handlers, templates, and CLI tools

This prevents duplication of language names across:
- Backend API responses
- Frontend templates
- CLI tools (gotrs-babelfish)

### Embedded Translations

Translation JSON files are embedded at compile time using Go's `//go:embed` directive. After modifying translation files, you must rebuild:

```bash
make build
```
