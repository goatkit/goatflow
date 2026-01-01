// Package lookups provides static lookup data including country codes.
package lookups

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

var (
	countryMu    sync.RWMutex
	countryCache []string
)

//go:embed countries_embedded.yaml
var embeddedCountries []byte

func init() {
	if err := loadCountriesFromBytes(embeddedCountries); err != nil {
		panic(fmt.Sprintf("failed to load embedded countries: %v", err))
	}
}

// LoadCountries loads the country list from the provided config directory overriding the embedded defaults.
func LoadCountries(configDir string) error {
	if configDir == "" {
		return errors.New("config directory is required")
	}
	path := filepath.Join(configDir, "lookups", "countries.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read countries lookup: %w", err)
	}
	return loadCountriesFromBytes(data)
}

// Countries returns the currently loaded list of ISO country names.
func Countries() []string {
	countryMu.RLock()
	defer countryMu.RUnlock()
	out := make([]string, len(countryCache))
	copy(out, countryCache)
	return out
}

func loadCountriesFromBytes(data []byte) error {
	var payload struct {
		Countries []string `yaml:"countries"`
	}
	if err := yaml.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("parse countries yaml: %w", err)
	}
	clean := make([]string, 0, len(payload.Countries))
	seen := make(map[string]struct{})
	for _, country := range payload.Countries {
		country = strings.TrimSpace(country)
		if country == "" {
			continue
		}
		key := strings.ToLower(country)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		clean = append(clean, country)
	}
	if len(clean) == 0 {
		return errors.New("country list is empty")
	}
	countryMu.Lock()
	countryCache = clean
	countryMu.Unlock()
	return nil
}
