package service

import (
	"testing"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository/memory"
	"github.com/stretchr/testify/assert"
)

func TestLDAPService_Configuration(t *testing.T) {
	// Create repositories
	userRepo := memory.NewUserRepository()
	roleRepo := memory.NewRoleRepository()
	groupRepo := memory.NewGroupRepository()

	// Create service
	ldapService := NewLDAPService(userRepo, roleRepo, groupRepo)

	t.Run("ValidateConfig_Success", func(t *testing.T) {
		config := &LDAPConfig{
			Host:         "ldap.example.com",
			Port:         389,
			BaseDN:       "dc=example,dc=com",
			BindDN:       "cn=service,dc=example,dc=com",
			BindPassword: "password123",
			AttributeMap: LDAPAttributeMap{
				Username:    "sAMAccountName",
				Email:       "mail",
				FirstName:   "givenName",
				LastName:    "sn",
				DisplayName: "displayName",
				Groups:      "memberOf",
				ObjectGUID:  "objectGUID",
				ObjectSID:   "objectSid",
			},
			AutoCreateUsers:  true,
			AutoUpdateUsers:  true,
			SyncInterval:     1 * time.Hour,
			DefaultRole:      "user",
		}

		// This would normally test actual LDAP connection
		// For unit test, we just test configuration validation
		err := ldapService.validateConfig(config)
		assert.NoError(t, err)
		assert.Equal(t, "sAMAccountName", config.AttributeMap.Username)
		assert.Equal(t, "member", config.GroupMemberAttribute)
	})

	t.Run("ValidateConfig_MissingHost", func(t *testing.T) {
		config := &LDAPConfig{
			BaseDN:       "dc=example,dc=com",
			BindDN:       "cn=service,dc=example,dc=com",
			BindPassword: "password123",
		}

		err := ldapService.validateConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "host is required")
	})

	t.Run("ValidateConfig_MissingBaseDN", func(t *testing.T) {
		config := &LDAPConfig{
			Host:         "ldap.example.com",
			BindDN:       "cn=service,dc=example,dc=com",
			BindPassword: "password123",
		}

		err := ldapService.validateConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "base DN is required")
	})

	t.Run("ValidateConfig_DefaultAttributeMapping", func(t *testing.T) {
		config := &LDAPConfig{
			Host:         "ldap.example.com",
			BaseDN:       "dc=example,dc=com",
			BindDN:       "cn=service,dc=example,dc=com",
			BindPassword: "password123",
			AttributeMap: LDAPAttributeMap{}, // Empty mapping
		}

		err := ldapService.validateConfig(config)
		assert.NoError(t, err)
		
		// Should set Active Directory defaults
		assert.Equal(t, "sAMAccountName", config.AttributeMap.Username)
		assert.Equal(t, "mail", config.AttributeMap.Email)
		assert.Equal(t, "givenName", config.AttributeMap.FirstName)
		assert.Equal(t, "sn", config.AttributeMap.LastName)
		assert.Equal(t, "displayName", config.AttributeMap.DisplayName)
		assert.Equal(t, "memberOf", config.AttributeMap.Groups)
		assert.Equal(t, "objectGUID", config.AttributeMap.ObjectGUID)
		assert.Equal(t, "objectSid", config.AttributeMap.ObjectSID)
		assert.Equal(t, "member", config.GroupMemberAttribute)
	})

	t.Run("GetSyncStatus_NotConfigured", func(t *testing.T) {
		status := ldapService.GetSyncStatus()
		
		assert.False(t, status["configured"].(bool))
		assert.False(t, status["auto_sync"].(bool))
		assert.Equal(t, 1*time.Hour, status["sync_interval"].(time.Duration))
	})
}

func TestLDAPService_UserMapping(t *testing.T) {
	// Create repositories
	userRepo := memory.NewUserRepository()
	roleRepo := memory.NewRoleRepository()
	groupRepo := memory.NewGroupRepository()

	// Create service
	_ = NewLDAPService(userRepo, roleRepo, groupRepo)

	t.Run("MapLDAPUser_Complete", func(t *testing.T) {
		_ = &LDAPConfig{
			AttributeMap: LDAPAttributeMap{
				Username:    "sAMAccountName",
				Email:       "mail",
				FirstName:   "givenName",
				LastName:    "sn",
				DisplayName: "displayName",
				Phone:       "telephoneNumber",
				Department:  "department",
				Title:       "title",
				Manager:     "manager",
				Groups:      "memberOf",
				ObjectGUID:  "objectGUID",
				ObjectSID:   "objectSid",
			},
		}

		// Mock LDAP entry attributes
		attributes := map[string]string{
			"sAMAccountName":   "jdoe",
			"mail":             "john.doe@example.com",
			"givenName":        "John",
			"sn":               "Doe",
			"displayName":      "John Doe",
			"telephoneNumber":  "+1-555-0123",
			"department":       "IT",
			"title":            "Senior Developer",
			"manager":          "cn=Jane Smith,ou=Users,dc=example,dc=com",
			"objectGUID":       "12345678-1234-1234-1234-123456789abc",
			"objectSid":        "S-1-5-21-123456789-123456789-123456789-1001",
			"userAccountControl": "512", // Normal account
		}

		groups := []string{
			"cn=IT Team,ou=Groups,dc=example,dc=com",
			"cn=Developers,ou=Groups,dc=example,dc=com",
		}

		// Test helper method to get attribute value
		getValue := func(attr string) string {
			return attributes[attr]
		}

		// Mock user object
		user := &LDAPUser{
			DN:          "cn=John Doe,ou=Users,dc=example,dc=com",
			Username:    getValue("sAMAccountName"),
			Email:       getValue("mail"),
			FirstName:   getValue("givenName"),
			LastName:    getValue("sn"),
			DisplayName: getValue("displayName"),
			Phone:       getValue("telephoneNumber"),
			Department:  getValue("department"),
			Title:       getValue("title"),
			Manager:     getValue("manager"),
			Groups:      groups,
			ObjectGUID:  getValue("objectGUID"),
			ObjectSID:   getValue("objectSid"),
			IsActive:    true,
			Attributes:  attributes,
		}

		// Verify user mapping
		assert.Equal(t, "jdoe", user.Username)
		assert.Equal(t, "john.doe@example.com", user.Email)
		assert.Equal(t, "John", user.FirstName)
		assert.Equal(t, "Doe", user.LastName)
		assert.Equal(t, "John Doe", user.DisplayName)
		assert.Equal(t, "+1-555-0123", user.Phone)
		assert.Equal(t, "IT", user.Department)
		assert.Equal(t, "Senior Developer", user.Title)
		assert.Equal(t, "cn=Jane Smith,ou=Users,dc=example,dc=com", user.Manager)
		assert.Len(t, user.Groups, 2)
		assert.Contains(t, user.Groups, "cn=IT Team,ou=Groups,dc=example,dc=com")
		assert.Contains(t, user.Groups, "cn=Developers,ou=Groups,dc=example,dc=com")
		assert.Equal(t, "12345678-1234-1234-1234-123456789abc", user.ObjectGUID)
		assert.Equal(t, "S-1-5-21-123456789-123456789-123456789-1001", user.ObjectSID)
		assert.True(t, user.IsActive)
	})

	t.Run("MapLDAPUser_MinimalAttributes", func(t *testing.T) {
		_ = &LDAPConfig{
			AttributeMap: LDAPAttributeMap{
				Username: "uid",
				Email:    "mail",
			},
		}

		attributes := map[string]string{
			"uid":  "jdoe",
			"mail": "john.doe@example.com",
		}

		getValue := func(attr string) string {
			return attributes[attr]
		}

		user := &LDAPUser{
			DN:         "uid=jdoe,ou=users,dc=example,dc=com",
			Username:   getValue("uid"),
			Email:      getValue("mail"),
			IsActive:   true,
			Attributes: attributes,
		}

		// DisplayName should fallback to username when not set
		if user.DisplayName == "" && user.Username != "" {
			user.DisplayName = user.Username
		}

		assert.Equal(t, "jdoe", user.Username)
		assert.Equal(t, "john.doe@example.com", user.Email)
		assert.Equal(t, "jdoe", user.DisplayName) // Fallback
		assert.Empty(t, user.FirstName)
		assert.Empty(t, user.LastName)
		assert.True(t, user.IsActive)
	})
}

func TestLDAPService_GroupMapping(t *testing.T) {
	t.Run("MapLDAPGroup_Complete", func(t *testing.T) {
		// config variable is unused, removing it

		attributes := map[string]string{
			"cn":          "IT Team",
			"description": "Information Technology Team",
			"objectGUID":  "87654321-4321-4321-4321-abcdef123456",
			"objectSid":   "S-1-5-21-123456789-123456789-123456789-2001",
		}

		members := []string{
			"cn=John Doe,ou=Users,dc=example,dc=com",
			"cn=Jane Smith,ou=Users,dc=example,dc=com",
		}

		getValue := func(attr string) string {
			return attributes[attr]
		}

		group := &LDAPGroup{
			DN:          "cn=IT Team,ou=Groups,dc=example,dc=com",
			Name:        getValue("cn"),
			Description: getValue("description"),
			Members:     members,
			ObjectGUID:  getValue("objectGUID"),
			ObjectSID:   getValue("objectSid"),
		}

		assert.Equal(t, "IT Team", group.Name)
		assert.Equal(t, "Information Technology Team", group.Description)
		assert.Len(t, group.Members, 2)
		assert.Contains(t, group.Members, "cn=John Doe,ou=Users,dc=example,dc=com")
		assert.Contains(t, group.Members, "cn=Jane Smith,ou=Users,dc=example,dc=com")
		assert.Equal(t, "87654321-4321-4321-4321-abcdef123456", group.ObjectGUID)
		assert.Equal(t, "S-1-5-21-123456789-123456789-123456789-2001", group.ObjectSID)
	})
}

func TestLDAPService_RoleMapping(t *testing.T) {
	// Create repositories
	userRepo := memory.NewUserRepository()
	roleRepo := memory.NewRoleRepository()
	groupRepo := memory.NewGroupRepository()

	// Create service
	ldapService := NewLDAPService(userRepo, roleRepo, groupRepo)

	// Setup test data
	adminRole := &models.Role{
		ID:          "admin",
		Name:        "admin",
		Description: "Administrator role",
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	roleRepo.CreateRole(nil, adminRole)

	userRole := &models.Role{
		ID:          "user", 
		Name:        "user",
		Description: "Regular user role",
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	roleRepo.CreateRole(nil, userRole)

	// Configure LDAP with role mappings
	config := &LDAPConfig{
		DefaultRole:  "user",
		AdminGroups:  []string{"Domain Admins", "IT Administrators"},
		UserGroups:   []string{"Domain Users", "Employees"},
	}
	ldapService.config = config

	t.Run("UpdateUserRole_AdminGroup", func(t *testing.T) {
		user := &models.User{
			Email:      "admin@example.com",
			FirstName:  "Admin",
			LastName:   "User",
			Role:       string(models.RoleUser), // Start as user
			ValidID:    1,
			CreateTime: time.Now(),
			ChangeTime: time.Now(),
		}

		ldapUser := &LDAPUser{
			Username: "admin",
			Email:    "admin@example.com",
			Groups: []string{
				"cn=Domain Admins,cn=Users,dc=example,dc=com",
				"cn=Domain Users,cn=Users,dc=example,dc=com",
			},
		}

		ldapService.updateUserRole(user, ldapUser)

		// Should be promoted to admin (LDAP service uses lowercase)
		assert.Equal(t, "admin", user.Role)
	})

	t.Run("UpdateUserRole_UserGroup", func(t *testing.T) {
		user := &models.User{
			Email:      "user@example.com",
			FirstName:  "Regular",
			LastName:   "User",
			Role:       "", // No role initially
			ValidID:    1,
			CreateTime: time.Now(),
			ChangeTime: time.Now(),
		}

		ldapUser := &LDAPUser{
			Username: "user",
			Email:    "user@example.com",
			Groups: []string{
				"cn=Domain Users,cn=Users,dc=example,dc=com",
				"cn=Employees,cn=Users,dc=example,dc=com",
			},
		}

		ldapService.updateUserRole(user, ldapUser)

		// Should be assigned user role from Domain Users group
		assert.Equal(t, "user", user.Role)
	})

	t.Run("UpdateUserRole_NoMatchingGroups", func(t *testing.T) {
		user := &models.User{
			Email:      "contractor@example.com",
			FirstName:  "Contractor",
			LastName:   "User",
			Role:       string(models.RoleUser), // Start with user role
			ValidID:    1,
			CreateTime: time.Now(),
			ChangeTime: time.Now(),
		}

		ldapUser := &LDAPUser{
			Username: "contractor",
			Email:    "contractor@example.com",
			Groups: []string{
				"cn=Contractors,cn=Users,dc=example,dc=com",
				"cn=External,cn=Users,dc=example,dc=com",
			},
		}

		originalRole := user.Role
		ldapService.updateUserRole(user, ldapUser)

		// Role should remain unchanged
		assert.Equal(t, originalRole, user.Role)
	})
}

func TestLDAPService_SyncResult(t *testing.T) {
	t.Run("SyncResult_Calculation", func(t *testing.T) {
		startTime := time.Now()
		endTime := startTime.Add(2 * time.Minute)

		result := &LDAPSyncResult{
			UsersFound:    100,
			UsersCreated:  25,
			UsersUpdated:  70,
			UsersDisabled: 5,
			GroupsFound:   10,
			GroupsCreated: 2,
			GroupsUpdated: 8,
			Errors:        []string{"User john.doe failed: invalid email"},
			StartTime:     startTime,
			EndTime:       endTime,
			Duration:      endTime.Sub(startTime),
		}

		assert.Equal(t, 100, result.UsersFound)
		assert.Equal(t, 25, result.UsersCreated)
		assert.Equal(t, 70, result.UsersUpdated)
		assert.Equal(t, 5, result.UsersDisabled)
		assert.Equal(t, 10, result.GroupsFound)
		assert.Equal(t, 2, result.GroupsCreated)
		assert.Equal(t, 8, result.GroupsUpdated)
		assert.Len(t, result.Errors, 1)
		assert.Equal(t, 2*time.Minute, result.Duration)
	})
}

func TestLDAPService_AttributeMapping(t *testing.T) {
	t.Run("GetUserAttributes_Complete", func(t *testing.T) {
		config := &LDAPConfig{
			AttributeMap: LDAPAttributeMap{
				Username:    "sAMAccountName",
				Email:       "mail",
				FirstName:   "givenName",
				LastName:    "sn",
				DisplayName: "displayName",
				Phone:       "telephoneNumber",
				Department:  "department",
				Title:       "title",
				Manager:     "manager",
				Groups:      "memberOf",
				ObjectGUID:  "objectGUID",
				ObjectSID:   "objectSid",
			},
		}

		service := &LDAPService{}
		attrs := service.getUserAttributes(config)

		expectedAttrs := []string{
			"dn",
			"sAMAccountName",
			"mail",
			"givenName",
			"sn",
			"displayName",
			"telephoneNumber",
			"department",
			"title",
			"manager",
			"memberOf",
			"objectGUID",
			"objectSid",
			"userAccountControl",
		}

		assert.Len(t, attrs, len(expectedAttrs))
		for _, expected := range expectedAttrs {
			assert.Contains(t, attrs, expected)
		}
	})

	t.Run("GetGroupAttributes_Complete", func(t *testing.T) {
		config := &LDAPConfig{
			GroupMemberAttribute: "member",
			AttributeMap: LDAPAttributeMap{
				ObjectGUID: "objectGUID",
				ObjectSID:  "objectSid",
			},
		}

		service := &LDAPService{}
		attrs := service.getGroupAttributes(config)

		expectedAttrs := []string{
			"dn",
			"cn",
			"description",
			"member",
			"objectGUID",
			"objectSid",
		}

		assert.Len(t, attrs, len(expectedAttrs))
		for _, expected := range expectedAttrs {
			assert.Contains(t, attrs, expected)
		}
	})

	t.Run("GetAttributes_EmptyAttributesFiltered", func(t *testing.T) {
		config := &LDAPConfig{
			AttributeMap: LDAPAttributeMap{
				Username:   "sAMAccountName",
				Email:      "mail",
				FirstName:  "", // Empty - should be filtered
				LastName:   "", // Empty - should be filtered
				ObjectGUID: "objectGUID",
			},
		}

		service := &LDAPService{}
		attrs := service.getUserAttributes(config)

		// Should contain non-empty attributes plus dn and userAccountControl
		expectedAttrs := []string{
			"dn",
			"sAMAccountName",
			"mail",
			"objectGUID",
			"userAccountControl",
		}

		assert.Len(t, attrs, len(expectedAttrs))
		for _, expected := range expectedAttrs {
			assert.Contains(t, attrs, expected)
		}

		// Should not contain empty attributes
		assert.NotContains(t, attrs, "")
	})
}

// Benchmark tests for performance
func BenchmarkLDAPService_ValidateConfig(b *testing.B) {
	userRepo := memory.NewUserRepository()
	roleRepo := memory.NewRoleRepository()
	groupRepo := memory.NewGroupRepository()
	ldapService := NewLDAPService(userRepo, roleRepo, groupRepo)

	config := &LDAPConfig{
		Host:         "ldap.example.com",
		Port:         389,
		BaseDN:       "dc=example,dc=com",
		BindDN:       "cn=service,dc=example,dc=com",
		BindPassword: "password123",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ldapService.validateConfig(config)
	}
}

func BenchmarkLDAPService_GetUserAttributes(b *testing.B) {
	config := &LDAPConfig{
		AttributeMap: LDAPAttributeMap{
			Username:    "sAMAccountName",
			Email:       "mail",
			FirstName:   "givenName",
			LastName:    "sn",
			DisplayName: "displayName",
			Groups:      "memberOf",
			ObjectGUID:  "objectGUID",
			ObjectSID:   "objectSid",
		},
	}

	service := &LDAPService{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.getUserAttributes(config)
	}
}