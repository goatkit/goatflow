package yamlmgmt

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGetConfigValueValuePrecedence ensures 'value' overrides 'default'.
func TestGetConfigValueValuePrecedence(t *testing.T) {
	dir := t.TempDir()
	// Minimal Config.yaml content
	cfg := `version: "1.0"
settings:
  - name: "Ticket::NumberGenerator"
    type: "select"
    default: "Date"
    value: "Increment"
`
	if err := os.WriteFile(filepath.Join(dir, "Config.yaml"), []byte(cfg), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	vm := NewVersionManager(dir)
	adapter := NewConfigAdapter(vm)
	if err := adapter.ImportConfigYAML(filepath.Join(dir, "Config.yaml")); err != nil {
		t.Fatalf("import: %v", err)
	}

	v, err := adapter.GetConfigValue("Ticket::NumberGenerator")
	if err != nil {
		t.Fatalf("GetConfigValue: %v", err)
	}
	if v.(string) != "Increment" {
		t.Fatalf("expected 'Increment', got %v", v)
	}
}
