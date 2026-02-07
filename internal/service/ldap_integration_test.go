package service

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/goatkit/goatflow/internal/repository/memory"
)

// LDAPIntegrationTestSuite runs integration tests against a real OpenLDAP server.
type LDAPIntegrationTestSuite struct {
	suite.Suite
	ldapService *LDAPService
	config      *LDAPConfig
}

// SetupSuite sets up the test suite.
func (suite *LDAPIntegrationTestSuite) SetupSuite() {
	// Skip integration tests unless explicitly enabled
	if os.Getenv("LDAP_INTEGRATION_TESTS") != "true" {
		suite.T().Skip("LDAP integration tests disabled. Set LDAP_INTEGRATION_TESTS=true to enable.")
	}

	// Create repositories
	userRepo := memory.NewUserRepository()
	roleRepo := memory.NewRoleRepository()
	groupRepo := memory.NewGroupRepository()

	// Create service
	suite.ldapService = NewLDAPService(userRepo, roleRepo, groupRepo)

	// Setup test configuration
	suite.config = &LDAPConfig{
		Host:                 getEnvOrDefault("LDAP_HOST", "localhost"),
		Port:                 389,
		BaseDN:               "dc=goatflow,dc=local",
		BindDN:               "cn=readonly,dc=goatflow,dc=local",
		BindPassword:         getEnvOrDefault("LDAP_READONLY_PASSWORD", "readonly123"),
		UserSearchBase:       "ou=Users,dc=goatflow,dc=local",
		UserFilter:           "(&(objectClass=inetOrgPerson)(uid={username}))",
		GroupSearchBase:      "ou=Groups,dc=goatflow,dc=local",
		GroupFilter:          "(objectClass=groupOfNames)",
		UseTLS:               false,
		StartTLS:             false,
		InsecureSkipVerify:   true,
		AutoCreateUsers:      true,
		AutoUpdateUsers:      true,
		AutoCreateGroups:     true,
		SyncInterval:         1 * time.Hour,
		DefaultRole:          "user",
		AdminGroups:          []string{"Domain Admins", "IT Team"},
		UserGroups:           []string{"Users"},
		GroupMemberAttribute: "member",
		AttributeMap: LDAPAttributeMap{
			Username:    "uid",
			Email:       "mail",
			FirstName:   "givenName",
			LastName:    "sn",
			DisplayName: "displayName",
			Phone:       "telephoneNumber",
			Department:  "departmentNumber",
			Title:       "title",
			Manager:     "manager",
			Groups:      "memberOf",
		},
	}

	// Wait for LDAP server to be ready
	suite.waitForLDAPServer()
}

// TestConnection tests LDAP server connection.
func (suite *LDAPIntegrationTestSuite) TestConnection() {
	err := suite.ldapService.TestConnection(suite.config)
	require.NoError(suite.T(), err, "Should be able to connect to LDAP server")
}

// TestConfiguration tests LDAP configuration.
func (suite *LDAPIntegrationTestSuite) TestConfiguration() {
	err := suite.ldapService.ConfigureLDAP(suite.config)
	require.NoError(suite.T(), err, "Should be able to configure LDAP")

	status := suite.ldapService.GetSyncStatus()
	assert.True(suite.T(), status["configured"].(bool))
	assert.True(suite.T(), status["auto_sync"].(bool))
}

// TestUserAuthentication tests user authentication against LDAP.
func (suite *LDAPIntegrationTestSuite) TestUserAuthentication() {
	// Configure LDAP first
	err := suite.ldapService.ConfigureLDAP(suite.config)
	require.NoError(suite.T(), err)

	testCases := []struct {
		name        string
		username    string
		password    string
		shouldPass  bool
		expectedErr string
	}{
		{
			name:       "Valid admin user",
			username:   "jadmin",
			password:   "password123",
			shouldPass: true,
		},
		{
			name:       "Valid regular user",
			username:   "mwilson",
			password:   "password123",
			shouldPass: true,
		},
		{
			name:        "Invalid password",
			username:    "jadmin",
			password:    "wrongpassword",
			shouldPass:  false,
			expectedErr: "authentication failed",
		},
		{
			name:        "Non-existent user",
			username:    "nonexistent",
			password:    "password123",
			shouldPass:  false,
			expectedErr: "user not found",
		},
		{
			name:        "Empty username",
			username:    "",
			password:    "password123",
			shouldPass:  false,
			expectedErr: "user not found",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			user, err := suite.ldapService.AuthenticateUser(tc.username, tc.password)

			if tc.shouldPass {
				assert.NoError(suite.T(), err, "Authentication should succeed")
				assert.NotNil(suite.T(), user, "User object should be returned")
				assert.Equal(suite.T(), tc.username, user.Username)
				assert.NotEmpty(suite.T(), user.Email)
				assert.NotEmpty(suite.T(), user.DisplayName)
				assert.True(suite.T(), user.IsActive)
			} else {
				assert.Error(suite.T(), err, "Authentication should fail")
				assert.Nil(suite.T(), user, "No user object should be returned")
				if tc.expectedErr != "" {
					assert.Contains(suite.T(), err.Error(), tc.expectedErr)
				}
			}
		})
	}
}

// TestUserLookup tests individual user lookup.
func (suite *LDAPIntegrationTestSuite) TestUserLookup() {
	// Configure LDAP first
	err := suite.ldapService.ConfigureLDAP(suite.config)
	require.NoError(suite.T(), err)

	testUsers := []struct {
		username      string
		expectedEmail string
		expectedName  string
		expectedTitle string
		expectedDept  string
	}{
		{
			username:      "jadmin",
			expectedEmail: "john.admin@goatflow.local",
			expectedName:  "John Admin",
			expectedTitle: "System Administrator",
			expectedDept:  "IT",
		},
		{
			username:      "smitchell",
			expectedEmail: "sarah.mitchell@goatflow.local",
			expectedName:  "Sarah Mitchell",
			expectedTitle: "IT Manager",
			expectedDept:  "IT",
		},
		{
			username:      "mwilson",
			expectedEmail: "mike.wilson@goatflow.local",
			expectedName:  "Mike Wilson",
			expectedTitle: "Senior Support Agent",
			expectedDept:  "Support",
		},
	}

	for _, testUser := range testUsers {
		suite.Run(fmt.Sprintf("Lookup_%s", testUser.username), func() {
			user, err := suite.ldapService.GetUser(testUser.username)
			require.NoError(suite.T(), err, "Should be able to find user")
			require.NotNil(suite.T(), user, "User should not be nil")

			assert.Equal(suite.T(), testUser.username, user.Username)
			assert.Equal(suite.T(), testUser.expectedEmail, user.Email)
			assert.Equal(suite.T(), testUser.expectedName, user.DisplayName)
			assert.Equal(suite.T(), testUser.expectedTitle, user.Title)
			assert.Equal(suite.T(), testUser.expectedDept, user.Department)
			assert.True(suite.T(), user.IsActive)
			assert.NotEmpty(suite.T(), user.DN)
		})
	}
}

// TestGroupLookup tests group retrieval.
func (suite *LDAPIntegrationTestSuite) TestGroupLookup() {
	// Configure LDAP first
	err := suite.ldapService.ConfigureLDAP(suite.config)
	require.NoError(suite.T(), err)

	groups, err := suite.ldapService.GetGroups()
	require.NoError(suite.T(), err, "Should be able to retrieve groups")
	require.NotEmpty(suite.T(), groups, "Should have groups")

	// Check for expected groups
	expectedGroups := []string{
		"Domain Admins",
		"IT Team",
		"Support Team",
		"Agents",
		"Managers",
		"Users",
		"Developers",
		"QA Team",
	}

	groupNames := make([]string, len(groups))
	for i, group := range groups {
		groupNames[i] = group.Name
	}

	for _, expectedGroup := range expectedGroups {
		assert.Contains(suite.T(), groupNames, expectedGroup,
			fmt.Sprintf("Should contain group: %s", expectedGroup))
	}

	// Verify group structure
	for _, group := range groups {
		assert.NotEmpty(suite.T(), group.DN, "Group should have DN")
		assert.NotEmpty(suite.T(), group.Name, "Group should have name")
		// Members may be empty for some groups, that's OK
	}
}

// TestUserSynchronization tests full user sync.
func (suite *LDAPIntegrationTestSuite) TestUserSynchronization() {
	// Configure LDAP first
	err := suite.ldapService.ConfigureLDAP(suite.config)
	require.NoError(suite.T(), err)

	// Perform sync
	result, err := suite.ldapService.SyncUsers()
	require.NoError(suite.T(), err, "Sync should succeed")
	require.NotNil(suite.T(), result, "Sync result should not be nil")

	// Verify sync results
	assert.Greater(suite.T(), result.UsersFound, 0, "Should find users")
	assert.Greater(suite.T(), result.Duration, time.Duration(0), "Should have positive duration")
	assert.NotNil(suite.T(), result.StartTime, "Should have start time")
	assert.NotNil(suite.T(), result.EndTime, "Should have end time")

	// Log sync results for debugging
	suite.T().Logf("Sync completed: Found %d users, Created %d, Updated %d, Errors: %d",
		result.UsersFound, result.UsersCreated, result.UsersUpdated, len(result.Errors))

	if len(result.Errors) > 0 {
		for _, err := range result.Errors {
			suite.T().Logf("Sync error: %s", err)
		}
	}
}

// TestUserGroupMembership tests group membership resolution.
func (suite *LDAPIntegrationTestSuite) TestUserGroupMembership() {
	// Configure LDAP first
	err := suite.ldapService.ConfigureLDAP(suite.config)
	require.NoError(suite.T(), err)

	testCases := []struct {
		username       string
		expectedGroups []string
	}{
		{
			username: "jadmin",
			expectedGroups: []string{
				"Domain Admins",
				"IT Team",
				"Users",
			},
		},
		{
			username: "smitchell",
			expectedGroups: []string{
				"IT Team",
				"Agents",
				"Managers",
				"Users",
			},
		},
		{
			username: "mwilson",
			expectedGroups: []string{
				"Support Team",
				"Agents",
				"Managers",
				"Users",
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Groups_%s", tc.username), func() {
			user, err := suite.ldapService.GetUser(tc.username)
			require.NoError(suite.T(), err)
			require.NotNil(suite.T(), user)

			// Check group memberships
			for _, expectedGroup := range tc.expectedGroups {
				found := false
				for _, userGroup := range user.Groups {
					if contains(userGroup, expectedGroup) {
						found = true
						break
					}
				}
				assert.True(suite.T(), found,
					fmt.Sprintf("User %s should be member of group %s", tc.username, expectedGroup))
			}
		})
	}
}

// TestLDAPPerformance tests LDAP performance with multiple operations.
func (suite *LDAPIntegrationTestSuite) TestLDAPPerformance() {
	// Configure LDAP first
	err := suite.ldapService.ConfigureLDAP(suite.config)
	require.NoError(suite.T(), err)

	usernames := []string{"jadmin", "smitchell", "mwilson", "lchen", "djohnson"}

	// Test authentication performance
	suite.Run("AuthenticationPerformance", func() {
		startTime := time.Now()

		for _, username := range usernames {
			_, err := suite.ldapService.AuthenticateUser(username, "password123")
			assert.NoError(suite.T(), err)
		}

		duration := time.Since(startTime)
		avgTime := duration / time.Duration(len(usernames))

		suite.T().Logf("Authentication performance: %d users in %v (avg: %v per user)",
			len(usernames), duration, avgTime)

		// Should complete within reasonable time
		assert.Less(suite.T(), avgTime, 2*time.Second, "Authentication should be fast")
	})

	// Test user lookup performance
	suite.Run("LookupPerformance", func() {
		startTime := time.Now()

		for _, username := range usernames {
			_, err := suite.ldapService.GetUser(username)
			assert.NoError(suite.T(), err)
		}

		duration := time.Since(startTime)
		avgTime := duration / time.Duration(len(usernames))

		suite.T().Logf("Lookup performance: %d users in %v (avg: %v per user)",
			len(usernames), duration, avgTime)

		// Should complete within reasonable time
		assert.Less(suite.T(), avgTime, 1*time.Second, "User lookup should be fast")
	})
}

// TestLDAPConnectionFailure tests connection failure scenarios.
func (suite *LDAPIntegrationTestSuite) TestLDAPConnectionFailure() {
	// Test with invalid host
	badConfig := *suite.config
	badConfig.Host = "invalid-host"

	err := suite.ldapService.TestConnection(&badConfig)
	assert.Error(suite.T(), err, "Should fail with invalid host")

	// Test with invalid port
	badConfig = *suite.config
	badConfig.Port = 9999

	err = suite.ldapService.TestConnection(&badConfig)
	assert.Error(suite.T(), err, "Should fail with invalid port")

	// Test with invalid credentials
	badConfig = *suite.config
	badConfig.BindPassword = "wrongpassword"

	err = suite.ldapService.TestConnection(&badConfig)
	assert.Error(suite.T(), err, "Should fail with invalid credentials")
}

// Helper functions

// waitForLDAPServer waits for the LDAP server to be ready.
func (suite *LDAPIntegrationTestSuite) waitForLDAPServer() {
	maxAttempts := 30
	for i := 0; i < maxAttempts; i++ {
		err := suite.ldapService.TestConnection(suite.config)
		if err == nil {
			suite.T().Log("LDAP server is ready")
			return
		}

		suite.T().Logf("Waiting for LDAP server... attempt %d/%d: %v", i+1, maxAttempts, err)
		time.Sleep(2 * time.Second)
	}

	suite.T().Fatal("LDAP server did not become ready in time")
}

// getEnvOrDefault gets environment variable or returns default.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// contains checks if a string contains a substring (case-insensitive).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			strings.Contains(strings.ToLower(s), strings.ToLower(substr))))
}

// TestLDAPIntegrationSuite runs the integration test suite.
func TestLDAPIntegrationSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping LDAP integration tests in short mode")
	}

	suite.Run(t, new(LDAPIntegrationTestSuite))
}

// TestLDAPServiceBenchmark provides benchmark tests for LDAP operations.
func BenchmarkLDAPOperations(b *testing.B) {
	if os.Getenv("LDAP_INTEGRATION_TESTS") != "true" {
		b.Skip("LDAP integration tests disabled")
	}

	// Setup
	userRepo := memory.NewUserRepository()
	roleRepo := memory.NewRoleRepository()
	groupRepo := memory.NewGroupRepository()
	ldapService := NewLDAPService(userRepo, roleRepo, groupRepo)

	config := &LDAPConfig{
		Host:           "localhost",
		Port:           389,
		BaseDN:         "dc=goatflow,dc=local",
		BindDN:         "cn=readonly,dc=goatflow,dc=local",
		BindPassword:   getEnvOrDefault("LDAP_READONLY_PASSWORD", "readonly123"),
		UserSearchBase: "ou=Users,dc=goatflow,dc=local",
		UserFilter:     "(&(objectClass=inetOrgPerson)(uid={username}))",
		AttributeMap: LDAPAttributeMap{
			Username: "uid",
			Email:    "mail",
		},
	}

	err := ldapService.ConfigureLDAP(config)
	if err != nil {
		b.Fatalf("Failed to configure LDAP: %v", err)
	}

	b.Run("Authentication", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := ldapService.AuthenticateUser("jadmin", "password123")
			if err != nil {
				b.Fatalf("Authentication failed: %v", err)
			}
		}
	})

	b.Run("UserLookup", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := ldapService.GetUser("jadmin")
			if err != nil {
				b.Fatalf("User lookup failed: %v", err)
			}
		}
	})

	b.Run("GroupLookup", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := ldapService.GetGroups()
			if err != nil {
				b.Fatalf("Group lookup failed: %v", err)
			}
		}
	})
}
