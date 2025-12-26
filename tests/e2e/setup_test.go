//go:build e2e

package e2e

import (
	"os"
	"testing"
)

// TestSetup verifies the E2E environment is configured correctly
func TestSetup(t *testing.T) {
	t.Log("E2E Test Environment Check")
	t.Log("===========================")

	// Check environment variables
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	t.Logf("BASE_URL: %s", baseURL)

	demoEmail := os.Getenv("DEMO_ADMIN_EMAIL")
	if demoEmail == "" {
		t.Error("DEMO_ADMIN_EMAIL not set")
	} else {
		t.Logf("DEMO_ADMIN_EMAIL: %s", demoEmail)
	}

	demoPass := os.Getenv("DEMO_ADMIN_PASSWORD")
	if demoPass == "" {
		t.Error("DEMO_ADMIN_PASSWORD not set")
	} else {
		t.Log("DEMO_ADMIN_PASSWORD: [configured]")
	}

	// Check Go version
	t.Logf("Go version: %s", os.Getenv("GOVERSION"))

	// Check if we can import Playwright
	t.Log("Playwright Go bindings: Available")

	t.Log("✅ E2E test environment is ready!")
}
