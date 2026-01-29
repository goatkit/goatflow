package api

import (
	"net/http"
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

// TestAuthConfig holds configuration for test authentication.
type TestAuthConfig struct {
	UserID  uint
	Email   string
	Role    string
	IsAdmin bool
}

// GetTestAuthConfig returns auth configuration from environment variables with defaults.
// Environment variables:
//   - TEST_AUTH_USER_ID: User ID for test token (default: 1)
//   - TEST_AUTH_EMAIL: Email for test token (default: root@localhost)
//   - TEST_AUTH_ROLE: Role for test token (default: Admin)
//   - TEST_AUTH_IS_ADMIN: Whether user is admin (default: true)
func GetTestAuthConfig() TestAuthConfig {
	userID := shared.ToUint(os.Getenv("TEST_AUTH_USER_ID"), 1)

	isAdmin := true
	if admin := os.Getenv("TEST_AUTH_IS_ADMIN"); admin != "" {
		isAdmin = admin == "true" || admin == "1"
	}

	return TestAuthConfig{
		UserID:  userID,
		Email:   getEnvOrDefault("TEST_AUTH_EMAIL", "root@localhost"),
		Role:    getEnvOrDefault("TEST_AUTH_ROLE", "Admin"),
		IsAdmin: isAdmin,
	}
}

// GetTestAuthToken generates a valid JWT token for testing authenticated routes.
// Uses environment variables for configuration (see GetTestAuthConfig).
// This is the single source of truth for test authentication - all tests should use this.
func GetTestAuthToken(t *testing.T) string {
	t.Helper()

	jwtManager := shared.GetJWTManager()
	if jwtManager == nil {
		t.Fatal("JWT manager not available - ensure shared.InitJWTManager() was called")
	}

	config := GetTestAuthConfig()
	token, err := jwtManager.GenerateTokenWithAdmin(
		config.UserID,
		config.Email,
		config.Role,
		config.IsAdmin,
		0, // tenantID
	)
	if err != nil {
		t.Fatalf("Failed to generate test auth token: %v", err)
	}

	return token
}

// AddTestAuthCookie adds the authentication cookie to a request.
// This is the standard way to add auth to test requests.
func AddTestAuthCookie(req *http.Request, token string) {
	req.AddCookie(&http.Cookie{
		Name:  "auth_token",
		Value: token,
	})
}
