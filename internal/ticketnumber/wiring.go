package ticketnumber

import (
	"log"
	"path/filepath"
	"strconv"

	"github.com/goatkit/goatflow/internal/yamlmgmt"
)

type SetupResult struct {
	Generator Generator
	SystemID  string
	Name      string
}

// SetupFromConfig resolves SystemID and Ticket::NumberGenerator from config and returns a generator (no DB store binding yet).
func SetupFromConfig(configDir string) SetupResult {
	systemID := "10"
	generatorName := "DateChecksum"
	vm := yamlmgmt.NewVersionManager(configDir)
	adapter := yamlmgmt.NewConfigAdapter(vm)
	// Ensure settings imported at least once
	settings, _ := adapter.GetConfigSettings()
	if len(settings) == 0 {
		_ = adapter.ImportConfigYAML(filepath.Join(configDir, "Config.yaml"))
		settings, _ = adapter.GetConfigSettings()
	}
	var rawDefault, rawValue interface{}
	for _, s := range settings {
		if name, ok := s["name"].(string); ok && name == "Ticket::NumberGenerator" {
			rawDefault = s["default"]
			rawValue = s["value"]
			break
		}
	}
	if v, err := adapter.GetConfigValue("SystemID"); err == nil {
		switch tv := v.(type) {
		case int:
			systemID = strconv.Itoa(tv)
		case int64:
			systemID = strconv.FormatInt(tv, 10)
		case float64:
			systemID = strconv.Itoa(int(tv))
		case string:
			if tv != "" {
				systemID = tv
			}
		}
	}
	if v, err := adapter.GetConfigValue("Ticket::NumberGenerator"); err == nil {
		if s, ok := v.(string); ok && s != "" {
			generatorName = s
		}
	}
	log.Printf("🧾 Config Ticket::NumberGenerator raw default=%v value=%v selected=%s", rawDefault, rawValue, generatorName)
	g, err := Resolve(generatorName, systemID, nil)
	if err != nil {
		log.Printf("⚠️  Unknown Ticket::NumberGenerator '%s', falling back to DateChecksum: %v", generatorName, err)
		generatorName = "DateChecksum"
		g, _ = Resolve("DateChecksum", systemID, nil)
	}
	log.Printf("🧾 Ticket number generator selected: %s (SystemID=%s)", generatorName, systemID)
	return SetupResult{Generator: g, SystemID: systemID, Name: generatorName}
}
