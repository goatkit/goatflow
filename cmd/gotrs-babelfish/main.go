// Package main provides translation file processing.
package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/gotrs-io/gotrs-ce/internal/i18n"
)

func showBabelFish() {
	fmt.Println(`
    ><(((º>  GOTRS-BABELFISH  <º)))><
    The Universal Translation Tool
    
    "The Babel fish is small, yellow, leech-like, and probably the oddest
     thing in the Universe. If you stick one in your ear, you can instantly
     understand anything said to you in any form of language."
                                        - The Hitchhiker's Guide to the Galaxy
    
    DON'T PANIC! This tool helps manage GOTRS translations across the galaxy.
    `)
}

func main() {
	var (
		action  = flag.String("action", "coverage", "Action to perform: coverage, missing, validate, export, import")
		lang    = flag.String("lang", "", "Language code (required for some actions)")
		format  = flag.String("format", "text", "Output format: text, json, csv")
		file    = flag.String("file", "", "File path for import/export")
		verbose = flag.Bool("v", false, "Verbose output")
		quiet   = flag.Bool("q", false, "Quiet mode (no ASCII art)")
		help    = flag.Bool("help", false, "Show help with usage examples")
	)

	flag.Usage = func() {
		if !*quiet {
			showBabelFish()
		}
		fmt.Fprintf(os.Stderr, "Usage: gotrs-babelfish [options]\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  gotrs-babelfish -action=coverage                    # Show coverage for all languages\n")
		fmt.Fprintf(os.Stderr, "  gotrs-babelfish -action=missing -lang=es            # Show missing Spanish translations\n")
		fmt.Fprintf(os.Stderr, "  gotrs-babelfish -action=validate -lang=de           # Validate German translations\n")
		fmt.Fprintf(os.Stderr, "  gotrs-babelfish -action=export -lang=en -file=en.csv -format=csv\n")
		fmt.Fprintf(os.Stderr, "  gotrs-babelfish -action=import -lang=fr -file=fr.json\n")
		fmt.Fprintf(os.Stderr, "\nSupported languages: en, es, fr, de, pt, ja, zh, ar, ru, it, nl, tlh (Klingon!)\n")
		fmt.Fprintf(os.Stderr, "\n✨ Remember: The answer is 42, but what's the question?\n")
	}

	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	// Show ASCII art unless in quiet mode or non-text format
	if !*quiet && *format == "text" && *action == "coverage" {
		showBabelFish()
	}

	// Initialize i18n
	if err := i18n.Initialize(&i18n.Config{
		DefaultLanguage:    "en",
		SupportedLanguages: []string{"en", "es", "fr", "de", "pt", "ja", "zh", "ar", "ru", "it", "nl", "tlh"},
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing i18n: %v\n", err)
		os.Exit(1)
	}

	i18nInstance := i18n.GetInstance()

	switch *action {
	case "coverage":
		showCoverage(i18nInstance, *format, *verbose)
	case "missing":
		if *lang == "" {
			fmt.Fprintf(os.Stderr, "Language code required for missing action\n")
			os.Exit(1)
		}
		showMissing(i18nInstance, *lang, *format, *verbose)
	case "validate":
		if *lang == "" {
			validateAll(i18nInstance, *verbose)
		} else {
			validateLanguage(i18nInstance, *lang, *verbose)
		}
	case "export":
		if *lang == "" || *file == "" {
			fmt.Fprintf(os.Stderr, "Language code and file path required for export\n")
			os.Exit(1)
		}
		exportTranslations(i18nInstance, *lang, *file, *format)
	case "import":
		if *lang == "" || *file == "" {
			fmt.Fprintf(os.Stderr, "Language code and file path required for import\n")
			os.Exit(1)
		}
		importTranslations(i18nInstance, *lang, *file, *format)
	default:
		fmt.Fprintf(os.Stderr, "Unknown action: %s\n", *action)
		os.Exit(1)
	}
}

func showCoverage(i18n *i18n.I18n, format string, verbose bool) {
	baseKeys := i18n.GetAllKeys("en")
	totalKeys := len(baseKeys)

	type Coverage struct {
		Language       string
		Code           string
		TotalKeys      int
		TranslatedKeys int
		MissingKeys    int
		Coverage       float64
	}

	supportedLangs := i18n.GetSupportedLanguages()
	coverages := make([]Coverage, 0, len(supportedLangs))

	for _, lang := range supportedLangs {
		langKeys := i18n.GetAllKeys(lang)
		translatedKeys := len(langKeys)
		coverage := float64(translatedKeys) / float64(totalKeys) * 100.0

		coverages = append(coverages, Coverage{
			Language:       getLanguageName(lang),
			Code:           lang,
			TotalKeys:      totalKeys,
			TranslatedKeys: translatedKeys,
			MissingKeys:    totalKeys - translatedKeys,
			Coverage:       coverage,
		})
	}

	switch format {
	case "json":
		output, err := json.MarshalIndent(coverages, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(output))
	case "csv":
		w := csv.NewWriter(os.Stdout)
		if err := w.Write([]string{"Language", "Code", "Total Keys", "Translated", "Missing", "Coverage %"}); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing CSV header: %v\n", err)
			os.Exit(1)
		}
		for _, c := range coverages {
			if err := w.Write([]string{
				c.Language,
				c.Code,
				fmt.Sprintf("%d", c.TotalKeys),
				fmt.Sprintf("%d", c.TranslatedKeys),
				fmt.Sprintf("%d", c.MissingKeys),
				fmt.Sprintf("%.1f", c.Coverage),
			}); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing CSV row: %v\n", err)
				os.Exit(1)
			}
		}
		w.Flush()
		if err := w.Error(); err != nil {
			fmt.Fprintf(os.Stderr, "Error flushing CSV: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Println("Translation Coverage Report")
		fmt.Println("===========================")
		fmt.Printf("Total Keys: %d\n\n", totalKeys)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(w, "Language\tCode\tTranslated\tMissing\tCoverage"); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
			os.Exit(1)
		}
		if _, err := fmt.Fprintln(w, "--------\t----\t----------\t-------\t--------"); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
			os.Exit(1)
		}

		for _, c := range coverages {
			status := "✅"
			if c.Coverage < 100 {
				status = "🚧"
			}
			if c.Coverage < 50 {
				status = "⚠️"
			}
			if _, err := fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%.1f%% %s\n",
				c.Language, c.Code, c.TranslatedKeys, c.MissingKeys, c.Coverage, status); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
				os.Exit(1)
			}
		}
		if err := w.Flush(); err != nil {
			fmt.Fprintf(os.Stderr, "Error flushing output: %v\n", err)
			os.Exit(1)
		}
	}
}

func showMissing(i18n *i18n.I18n, lang string, format string, verbose bool) {
	baseKeys := i18n.GetAllKeys("en")
	langKeys := i18n.GetAllKeys(lang)

	// Create map for quick lookup
	langKeyMap := make(map[string]bool)
	for _, key := range langKeys {
		langKeyMap[key] = true
	}

	// Find missing keys
	var missing []string
	for _, key := range baseKeys {
		if !langKeyMap[key] {
			missing = append(missing, key)
		}
	}

	sort.Strings(missing)

	switch format {
	case "json":
		type MissingKey struct {
			Key   string `json:"key"`
			Value string `json:"english_value"`
		}
		missingKeys := make([]MissingKey, 0, len(missing))
		for _, key := range missing {
			missingKeys = append(missingKeys, MissingKey{
				Key:   key,
				Value: i18n.T("en", key),
			})
		}
		output, err := json.MarshalIndent(missingKeys, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(output))
	case "csv":
		w := csv.NewWriter(os.Stdout)
		if err := w.Write([]string{"Key", "English Value"}); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing CSV header: %v\n", err)
			os.Exit(1)
		}
		for _, key := range missing {
			if err := w.Write([]string{key, i18n.T("en", key)}); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing CSV row: %v\n", err)
				os.Exit(1)
			}
		}
		w.Flush()
		if err := w.Error(); err != nil {
			fmt.Fprintf(os.Stderr, "Error flushing CSV: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Printf("Missing Translations for %s (%s)\n", getLanguageName(lang), lang)
		fmt.Println("=====================================")
		fmt.Printf("Total Missing: %d\n\n", len(missing))

		if verbose || len(missing) <= 20 {
			for _, key := range missing {
				englishValue := i18n.T("en", key)
				fmt.Printf("  %s\n    => %s\n", key, englishValue)
			}
		} else {
			// Show first 20
			for i := 0; i < 20 && i < len(missing); i++ {
				englishValue := i18n.T("en", missing[i])
				fmt.Printf("  %s\n    => %s\n", missing[i], englishValue)
			}
			fmt.Printf("\n  ... and %d more\n", len(missing)-20)
			fmt.Println("\nUse -v flag to see all missing keys")
		}
	}
}

func validateLanguage(i18n *i18n.I18n, lang string, verbose bool) {
	baseKeys := i18n.GetAllKeys("en")
	langKeys := i18n.GetAllKeys(lang)

	totalKeys := len(baseKeys)
	translatedKeys := len(langKeys)
	coverage := float64(translatedKeys) / float64(totalKeys) * 100.0

	// Check for missing keys
	langKeyMap := make(map[string]bool)
	for _, key := range langKeys {
		langKeyMap[key] = true
	}

	var missing []string
	for _, key := range baseKeys {
		if !langKeyMap[key] {
			missing = append(missing, key)
		}
	}

	// Check for extra keys
	baseKeyMap := make(map[string]bool)
	for _, key := range baseKeys {
		baseKeyMap[key] = true
	}

	var extra []string
	for _, key := range langKeys {
		if !baseKeyMap[key] {
			extra = append(extra, key)
		}
	}

	fmt.Printf("Validation Report for %s (%s)\n", getLanguageName(lang), lang)
	fmt.Println("=====================================")
	fmt.Printf("Coverage: %.1f%% (%d/%d keys)\n", coverage, translatedKeys, totalKeys)
	fmt.Printf("Missing Keys: %d\n", len(missing))
	fmt.Printf("Extra Keys: %d\n", len(extra))

	if coverage == 100.0 && len(extra) == 0 {
		fmt.Println("\n✅ Language is complete and valid!")
	} else {
		fmt.Println("\n⚠️  Issues found:")
		if len(missing) > 0 {
			fmt.Printf("  - Missing %d translations\n", len(missing))
			if verbose {
				fmt.Println("\n  Missing keys:")
				for _, key := range missing {
					fmt.Printf("    - %s\n", key)
				}
			}
		}
		if len(extra) > 0 {
			fmt.Printf("  - %d extra keys not in base language\n", len(extra))
			if verbose {
				fmt.Println("\n  Extra keys:")
				for _, key := range extra {
					fmt.Printf("    - %s\n", key)
				}
			}
		}
		if !verbose && (len(missing) > 0 || len(extra) > 0) {
			fmt.Println("\n  Use -v flag for detailed list")
		}
	}
}

func validateAll(i18n *i18n.I18n, verbose bool) {
	fmt.Println("Validating All Languages")
	fmt.Println("========================")

	for _, lang := range i18n.GetSupportedLanguages() {
		validateLanguage(i18n, lang, false)
		fmt.Println()
	}
}

func exportTranslations(i18n *i18n.I18n, lang string, filePath string, format string) {
	translations := i18n.GetTranslations(lang)
	if translations == nil {
		fmt.Fprintf(os.Stderr, "Language %s not found\n", lang)
		os.Exit(1)
	}

	file, err := os.Create(filePath) //nolint:gosec // G304 CLI tool
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating file: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			fmt.Fprintf(os.Stderr, "Error closing file: %v\n", cerr)
		}
	}()

	switch format {
	case "csv":
		w := csv.NewWriter(file)
		if err := w.Write([]string{"key", "value"}); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing CSV header: %v\n", err)
			os.Exit(1)
		}
		if err := exportToCSV(translations, "", w); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing CSV: %v\n", err)
			os.Exit(1)
		}
		w.Flush()
		if err := w.Error(); err != nil {
			fmt.Fprintf(os.Stderr, "Error flushing CSV: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Exported %s translations to %s (CSV format)\n", lang, filePath)
	default:
		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(translations); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Exported %s translations to %s (JSON format)\n", lang, filePath)
	}
}

func exportToCSV(m map[string]interface{}, prefix string, w *csv.Writer) error {
	for key, value := range m {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		if nestedMap, ok := value.(map[string]interface{}); ok {
			if err := exportToCSV(nestedMap, fullKey, w); err != nil {
				return err
			}
		} else if str, ok := value.(string); ok {
			if err := w.Write([]string{fullKey, str}); err != nil {
				return err
			}
		}
	}
	return nil
}

func importTranslations(i18n *i18n.I18n, lang string, filePath string, format string) {
	file, err := os.Open(filePath) //nolint:gosec // G304 CLI tool
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			fmt.Fprintf(os.Stderr, "Error closing file: %v\n", cerr)
		}
	}()

	switch format {
	case "csv":
		r := csv.NewReader(file)
		records, err := r.ReadAll()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading CSV: %v\n", err)
			os.Exit(1)
		}

		translations := make(map[string]interface{})
		for i, record := range records {
			if i == 0 && record[0] == "key" {
				continue // Skip header
			}
			if len(record) >= 2 {
				setNestedValue(translations, record[0], record[1])
			}
		}

		// Save to file
		outputPath := filepath.Join("internal", "i18n", "translations", lang+".json")
		saveTranslations(translations, outputPath)
		fmt.Printf("Imported translations from %s to %s\n", filePath, outputPath)

	default:
		var translations map[string]interface{}
		decoder := json.NewDecoder(file)
		if err := decoder.Decode(&translations); err != nil {
			fmt.Fprintf(os.Stderr, "Error decoding JSON: %v\n", err)
			os.Exit(1)
		}

		// Save to file
		outputPath := filepath.Join("internal", "i18n", "translations", lang+".json")
		saveTranslations(translations, outputPath)
		fmt.Printf("Imported translations from %s to %s\n", filePath, outputPath)
	}
}

func setNestedValue(m map[string]interface{}, key string, value string) {
	parts := strings.Split(key, ".")
	current := m

	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = value
		} else {
			if next, ok := current[part].(map[string]interface{}); ok {
				current = next
			} else {
				next := make(map[string]interface{})
				current[part] = next
				current = next
			}
		}
	}
}

func saveTranslations(translations map[string]interface{}, path string) {
	file, err := os.Create(path) //nolint:gosec // G304 CLI tool
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating file: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			fmt.Fprintf(os.Stderr, "Error closing file: %v\n", cerr)
		}
	}()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(translations); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
		os.Exit(1)
	}
}

func getLanguageName(code string) string {
	if config, exists := i18n.GetLanguageConfig(code); exists {
		return config.Name
	}
	return code
}
