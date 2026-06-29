package template

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestHardcodedTextScanner runs the check-hardcoded-text.sh script
// and reports any hardcoded UI text found in templates.
//
// This test helps catch i18n compliance issues before they reach production.
// It scans for:
// - Hardcoded text in JavaScript alert/confirm calls
// - Common hardcoded phrases that should use translations
// - Hardcoded title and placeholder attributes
func TestHardcodedTextScanner(t *testing.T) {
	// Find the script relative to the project root
	scriptPath := findScript(t)
	if scriptPath == "" {
		t.Skip("check-hardcoded-text.sh script not found")
	}

	// Run the scanner in verbose mode to get full output
	cmd := exec.Command("bash", scriptPath, "--fix")
	output, err := cmd.CombinedOutput()

	// The script exits 0 even when it finds issues (unless --strict)
	// We just want to capture and report the findings
	if err != nil {
		// Check if it's a permission error or actual script failure
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Logf("Scanner exited with code %d", exitErr.ExitCode())
		} else {
			t.Fatalf("Failed to run scanner: %v", err)
		}
	}

	outputStr := string(output)

	// Log all findings for visibility
	if len(outputStr) > 0 {
		t.Logf("Hardcoded text scanner output:\n%s", outputStr)
	}

	// Count issues found by looking for the summary line
	if strings.Contains(outputStr, "potential hardcoded text issue") {
		// Extract the count
		lines := strings.Split(outputStr, "\n")
		for _, line := range lines {
			if strings.Contains(line, "Found") && strings.Contains(line, "potential hardcoded text issue") {
				t.Logf("Scanner found issues: %s", strings.TrimSpace(line))
				// Note: We don't fail the test, just report.
				// Use --strict mode in CI if you want failures.
			}
		}
	}
}

// TestHardcodedTextScannerStrict runs the scanner in strict mode.
// This test is skipped by default and can be enabled in CI.
// To run: go test -run TestHardcodedTextScannerStrict -tags=strict
func TestHardcodedTextScannerStrict(t *testing.T) {
	if os.Getenv("CI_STRICT_I18N") != "true" {
		t.Skip("Skipping strict i18n check (set CI_STRICT_I18N=true to enable)")
	}

	scriptPath := findScript(t)
	if scriptPath == "" {
		t.Skip("check-hardcoded-text.sh script not found")
	}

	cmd := exec.Command("bash", scriptPath, "--strict")
	output, err := cmd.CombinedOutput()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			t.Fatalf("Hardcoded text found (strict mode):\n%s", string(output))
		}
		t.Fatalf("Failed to run scanner: %v\nOutput: %s", err, string(output))
	}

	t.Log("No hardcoded text found - i18n compliance check passed")
}

// findScript locates the check-hardcoded-text.sh script
func findScript(t *testing.T) string {
	t.Helper()

	// Try common locations
	candidates := []string{
		"../../scripts/check-hardcoded-text.sh",
		"../../../scripts/check-hardcoded-text.sh",
		"scripts/check-hardcoded-text.sh",
	}

	// Also try finding from GOMOD
	if modRoot := findModuleRoot(); modRoot != "" {
		candidates = append(candidates, filepath.Join(modRoot, "scripts", "check-hardcoded-text.sh"))
	}

	for _, path := range candidates {
		absPath, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		if _, err := os.Stat(absPath); err == nil {
			return absPath
		}
	}

	return ""
}

// findModuleRoot finds the Go module root by looking for go.mod
func findModuleRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// TestCommonHardcodedPatterns checks for specific hardcoded patterns
// that are known to cause i18n issues.
func TestCommonHardcodedPatterns(t *testing.T) {
	templatesDir := findTemplatesDir(t)
	if templatesDir == "" {
		t.Skip("templates directory not found")
	}

	// Patterns that are commonly missed in i18n
	criticalPatterns := []struct {
		pattern     string
		description string
	}{
		{"Loading...", "Loading indicator text"},
		{"Please wait", "Wait message"},
		{"Are you sure", "Confirmation dialog"},
		{"No results found", "Empty state message"},
		{"Error:", "Error prefix"},
		{"Success!", "Success message"},
	}

	var issues []string

	err := filepath.Walk(templatesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".pongo2") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		contentStr := string(content)
		relPath, _ := filepath.Rel(templatesDir, path)

		for _, p := range criticalPatterns {
			if strings.Contains(contentStr, p.pattern) {
				// Check if it's already wrapped in i18n
				// Look for the pattern NOT inside a t() call
				lines := strings.Split(contentStr, "\n")
				for lineNum, line := range lines {
					if strings.Contains(line, p.pattern) {
						// Skip if line has t( or default:
						if strings.Contains(line, "t(") || strings.Contains(line, "default:") {
							continue
						}
						issues = append(issues,
							strings.Join([]string{relPath, ":", string(rune(lineNum+1)), ": ", p.description, " - '", p.pattern, "'"}, ""))
					}
				}
			}
		}

		return nil
	})

	if err != nil {
		t.Fatalf("Failed to walk templates: %v", err)
	}

	if len(issues) > 0 {
		t.Logf("Found %d potential i18n issues:", len(issues))
		for _, issue := range issues {
			t.Logf("  %s", issue)
		}
		// Note: Not failing the test, just reporting
		// Enable strict mode in CI if you want failures
	}
}

// findTemplatesDir locates the templates directory
func findTemplatesDir(t *testing.T) string {
	t.Helper()

	candidates := []string{
		"../../templates",
		"../../../templates",
		"templates",
	}

	if modRoot := findModuleRoot(); modRoot != "" {
		candidates = append(candidates, filepath.Join(modRoot, "templates"))
	}

	for _, path := range candidates {
		absPath, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		if info, err := os.Stat(absPath); err == nil && info.IsDir() {
			return absPath
		}
	}

	return ""
}
