package api

import (
	"os"
	"path/filepath"
	"testing"
)

// TestTemplatesManifest validates that all YAML-referenced templates resolve and parse.
// Non-strict by default (logs only). Set SSR_SMOKE_STRICT=1 to fail on issues.
func TestTemplatesManifest(t *testing.T) {
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()

	root := findProjectRoot(t)
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir root failed: %v", err)
	}
	_ = os.Setenv("APP_ENV", "test")

	templatesDir := filepath.Join(root, "templates")
	strict := os.Getenv("SSR_SMOKE_STRICT") == "1"

	failures, err := ValidateTemplatesReferencedInRoutes("./routes", templatesDir)
	if err != nil {
		t.Fatalf("validate templates from routes: %v", err)
	}
	if len(failures) > 0 {
		if strict {
			for _, f := range failures { t.Errorf("template invalid/missing: %s", f) }
			t.Fatalf("%d template(s) invalid or missing (from YAML)", len(failures))
		}
		for _, f := range failures { t.Logf("template invalid/missing (non-strict): %s", f) }
	}
}
