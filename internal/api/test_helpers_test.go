package api

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/goatkit/goatflow/internal/platform/shared"
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

// GetTestAuthConfig returns the identity used for test tokens: root@localhost (user 1), admin.
func GetTestAuthConfig() TestAuthConfig {
	return TestAuthConfig{
		UserID:  1,
		Email:   "root@localhost",
		Role:    "Admin",
		IsAdmin: true,
	}
}

// GetTestAuthToken generates a valid JWT token for testing authenticated routes.
// The identity comes from GetTestAuthConfig.
// This is the single source of truth for test authentication - all tests should use this.
func GetTestAuthToken(t *testing.T) string {
	t.Helper()
	config := GetTestAuthConfig()
	return testSessionToken(t, config.UserID, config.Email, config.Email, config.Role, config.IsAdmin, 0)
}

// testSessionToken creates a sessions row for the identity and returns an
// access token bound to it (the sid claim); the row is killed at cleanup.
// Access tokens are only accepted while their session exists, so every test
// token must come from here or from a login handler.
func testSessionToken(t *testing.T, userID uint, login, email, role string, isAdmin bool, tenantID uint) string {
	t.Helper()
	sessionID := testSession(t, int(userID), login, role)
	token, err := shared.GetJWTManager().GenerateTokenWithLogin(sessionID, userID, login, email, role, isAdmin, tenantID)
	if err != nil {
		t.Fatalf("Failed to generate test auth token: %v", err)
	}
	return token
}

// testSession creates a sessions row and returns its id; killed at cleanup.
func testSession(t *testing.T, userID int, login, userType string) string {
	t.Helper()
	sessions := shared.GetSessionService()
	if sessions == nil {
		t.Fatal("session service not available - ensure the database is up")
	}
	sessionID, err := sessions.CreateSession(userID, login, userType, "127.0.0.1", "go-test")
	if err != nil {
		t.Fatalf("Failed to create test session: %v", err)
	}
	t.Cleanup(func() { _ = sessions.KillSession(sessionID) })
	return sessionID
}

// AddTestAuthCookie adds the authentication cookie to a request.
// This is the standard way to add auth to test requests.
func AddTestAuthCookie(req *http.Request, token string) {
	req.AddCookie(&http.Cookie{
		Name:  "auth_token",
		Value: token,
	})
}
