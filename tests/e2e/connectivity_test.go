//go:build e2e

package e2e

import (
	"net/http"
	"os"
	"testing"
	"time"
)

// TestConnectivity verifies we can reach the backend
func TestConnectivity(t *testing.T) {
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Test login page is accessible
	resp, err := client.Get(baseURL + "/login")
	if err != nil {
		t.Fatalf("Failed to connect to %s/login: %v", baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	t.Logf("✅ Successfully connected to backend at %s", baseURL)
	t.Logf("   Login page status: %d", resp.StatusCode)
}
