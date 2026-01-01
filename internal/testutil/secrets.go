// Package testutil provides testing utilities and test environment setup.
package testutil

import (
	"os"
	"strings"
	"testing"
)

// Tests should use the synthesized .env with APP_ENV=test.
func SetupTestEnvironment(t *testing.T) {
	t.Helper()

	// Set test environment if not already set
	if os.Getenv("APP_ENV") == "" {
		t.Setenv("APP_ENV", "test")
	}

	// Load .env.test if it exists, otherwise .env
	envFile := ".env.test"
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		envFile = ".env"
	}

	// Note: In a real implementation, you'd load the env file here
	// For now, tests should run `make synthesize` with APP_ENV=test first
}

// SetupTestSecret sets a single environment variable for testing.
func SetupTestSecret(t *testing.T, key, value string) {
	t.Helper()
	t.Setenv(key, value)
}

// IsTestSecret checks if a value has a test/development prefix.
func IsTestSecret(value string) bool {
	testPrefixes := []string{
		"test-",
		"mock-",
		"dummy-",
		"example-",
		"demo-",
		"dev-",
		"stage-",
	}

	valueLower := strings.ToLower(value)
	for _, prefix := range testPrefixes {
		if strings.HasPrefix(valueLower, prefix) {
			return true
		}
	}

	return false
}
