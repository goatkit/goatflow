package api

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/gotrs-io/gotrs-ce/internal/shared"
)

var (
	testRendererOnce sync.Once
	testRendererErr  error
)

// SetupTestTemplateRenderer initializes the global template renderer for tests.
// This MUST be called by any test that exercises handlers calling shared.GetGlobalRenderer().
// Safe to call multiple times - initialization happens only once.
func SetupTestTemplateRenderer(t *testing.T) {
	t.Helper()

	testRendererOnce.Do(func() {
		// Find templates directory relative to this file
		_, file, _, ok := runtime.Caller(0)
		if !ok {
			testRendererErr = nil // Can't determine path, handlers will use fallback
			return
		}

		// This file is at internal/api/test_helpers.go
		// Templates are at templates/ (project root)
		apiDir := filepath.Dir(file)
		internalDir := filepath.Dir(apiDir)
		projectRoot := filepath.Dir(internalDir)
		templateDir := filepath.Join(projectRoot, "templates")

		// Check if templates directory exists
		if _, err := os.Stat(templateDir); os.IsNotExist(err) {
			// Templates not available - handlers will use fallback
			testRendererErr = nil
			return
		}

		renderer, err := shared.NewTemplateRenderer(templateDir)
		if err != nil {
			testRendererErr = err
			return
		}
		shared.SetGlobalRenderer(renderer)
	})

	if testRendererErr != nil {
		t.Logf("Warning: Could not initialize template renderer: %v (handlers will use fallback)", testRendererErr)
	}
}

// GetTestConfig returns test configuration from environment variables with safe defaults.
type TestConfig struct {
	UserLogin     string
	UserFirstName string
	UserLastName  string
	UserEmail     string
	UserGroups    []string
	QueueName     string
	GroupName     string
	CompanyName   string
}

// GetTestConfig retrieves parameterized test configuration.
func GetTestConfig() TestConfig {
	config := TestConfig{
		UserLogin:     getEnvOrDefault("TEST_USER_LOGIN", "testuser"),
		UserFirstName: getEnvOrDefault("TEST_USER_FIRSTNAME", "Test"),
		UserLastName:  getEnvOrDefault("TEST_USER_LASTNAME", "Agent"),
		UserEmail:     getEnvOrDefault("TEST_USER_EMAIL", "testuser@example.test"),
		QueueName:     getEnvOrDefault("TEST_QUEUE_NAME", "Postmaster"),
		GroupName:     getEnvOrDefault("TEST_GROUP_NAME", "users"),
		CompanyName:   getEnvOrDefault("TEST_COMPANY_NAME", "Test Company Alpha"),
	}

	// Parse groups from comma-separated list
	groupsStr := getEnvOrDefault("TEST_USER_GROUPS", "users,admin")
	config.UserGroups = strings.Split(groupsStr, ",")
	for i := range config.UserGroups {
		config.UserGroups[i] = strings.TrimSpace(config.UserGroups[i])
	}

	return config
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
