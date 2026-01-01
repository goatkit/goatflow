// Package ldap provides LDAP authentication and directory service integration.
package ldap

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ConfigManager handles LDAP configuration loading and saving.
type ConfigManager struct {
	configPath string
}

// NewConfigManager creates a new config manager.
func NewConfigManager(configPath string) *ConfigManager {
	return &ConfigManager{
		configPath: configPath,
	}
}

// LoadFromEnvironment loads LDAP configuration from environment variables.
func LoadFromEnvironment() (*Config, error) {
	// Check if LDAP is enabled
	enabled := strings.ToLower(os.Getenv("LDAP_ENABLED")) == "true"
	if !enabled {
		return nil, nil // LDAP is disabled
	}

	host := os.Getenv("LDAP_HOST")
	if host == "" {
		return nil, fmt.Errorf("LDAP_HOST is required when LDAP is enabled")
	}

	portStr := os.Getenv("LDAP_PORT")
	port := 389 // Default LDAP port
	if portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	config := &Config{
		Host:                 host,
		Port:                 port,
		UseSSL:               strings.ToLower(os.Getenv("LDAP_USE_SSL")) == "true",
		UseTLS:               strings.ToLower(os.Getenv("LDAP_USE_TLS")) == "true",
		SkipTLS:              strings.ToLower(os.Getenv("LDAP_SKIP_TLS_VERIFY")) == "true",
		BindDN:               os.Getenv("LDAP_BIND_DN"),
		BindPassword:         os.Getenv("LDAP_BIND_PASSWORD"),
		BaseDN:               os.Getenv("LDAP_BASE_DN"),
		UserFilter:           os.Getenv("LDAP_USER_FILTER"),
		UserBaseDN:           os.Getenv("LDAP_USER_BASE_DN"),
		EmailAttribute:       os.Getenv("LDAP_EMAIL_ATTRIBUTE"),
		FirstNameAttribute:   os.Getenv("LDAP_FIRST_NAME_ATTRIBUTE"),
		LastNameAttribute:    os.Getenv("LDAP_LAST_NAME_ATTRIBUTE"),
		DisplayNameAttribute: os.Getenv("LDAP_DISPLAY_NAME_ATTRIBUTE"),
		GroupBaseDN:          os.Getenv("LDAP_GROUP_BASE_DN"),
		GroupFilter:          os.Getenv("LDAP_GROUP_FILTER"),
		GroupAttribute:       os.Getenv("LDAP_GROUP_ATTRIBUTE"),
		IsActiveDirectory:    strings.ToLower(os.Getenv("LDAP_IS_ACTIVE_DIRECTORY")) == "true",
		Domain:               os.Getenv("LDAP_DOMAIN"),
	}

	// Set timeout
	if timeoutStr := os.Getenv("LDAP_TIMEOUT"); timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil {
			config.Timeout = timeout
		} else {
			config.Timeout = 30 // Default timeout
		}
	} else {
		config.Timeout = 30
	}

	// Parse admin groups
	if adminGroups := os.Getenv("LDAP_ADMIN_GROUPS"); adminGroups != "" {
		config.AdminGroups = strings.Split(adminGroups, ",")
		for i, group := range config.AdminGroups {
			config.AdminGroups[i] = strings.TrimSpace(group)
		}
	}

	// Parse agent groups
	if agentGroups := os.Getenv("LDAP_AGENT_GROUPS"); agentGroups != "" {
		config.AgentGroups = strings.Split(agentGroups, ",")
		for i, group := range config.AgentGroups {
			config.AgentGroups[i] = strings.TrimSpace(group)
		}
	}

	// Apply defaults based on LDAP type
	ldapType := strings.ToLower(os.Getenv("LDAP_TYPE"))
	if template, exists := DefaultConfigs[ldapType]; exists {
		if config.UserFilter == "" {
			config.UserFilter = template.UserFilter
		}
		if config.EmailAttribute == "" {
			config.EmailAttribute = template.EmailAttribute
		}
		if config.FirstNameAttribute == "" {
			config.FirstNameAttribute = template.FirstNameAttribute
		}
		if config.LastNameAttribute == "" {
			config.LastNameAttribute = template.LastNameAttribute
		}
		if config.DisplayNameAttribute == "" {
			config.DisplayNameAttribute = template.DisplayNameAttribute
		}
		if config.GroupFilter == "" {
			config.GroupFilter = template.GroupFilter
		}
		if config.GroupAttribute == "" {
			config.GroupAttribute = template.GroupAttribute
		}
	}

	// Validate required fields
	if config.BaseDN == "" {
		return nil, fmt.Errorf("LDAP_BASE_DN is required")
	}
	if config.UserFilter == "" {
		return nil, fmt.Errorf("LDAP_USER_FILTER is required")
	}
	if config.EmailAttribute == "" {
		return nil, fmt.Errorf("LDAP_EMAIL_ATTRIBUTE is required")
	}

	return config, nil
}

// SaveToFile saves LDAP configuration to JSON file.
func (cm *ConfigManager) SaveToFile(config *Config) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(cm.configPath, data, 0600)
}

// LoadFromFile loads LDAP configuration from JSON file.
func (cm *ConfigManager) LoadFromFile() (*Config, error) {
	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Config file doesn't exist
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

// GetEnvironmentTemplate returns environment variables template for LDAP configuration.
func GetEnvironmentTemplate(ldapType string) map[string]string {
	template := map[string]string{
		"LDAP_ENABLED":         "true",
		"LDAP_TYPE":            ldapType,
		"LDAP_HOST":            "ldap.example.com",
		"LDAP_PORT":            "389",
		"LDAP_USE_SSL":         "false",
		"LDAP_USE_TLS":         "true",
		"LDAP_SKIP_TLS_VERIFY": "false",
		"LDAP_TIMEOUT":         "30",
		"LDAP_BIND_DN":         "cn=binduser,dc=example,dc=com",
		"LDAP_BIND_PASSWORD":   "bindpassword",
		"LDAP_BASE_DN":         "dc=example,dc=com",
		"LDAP_USER_BASE_DN":    "", // Optional, uses BASE_DN if empty
		"LDAP_GROUP_BASE_DN":   "ou=groups,dc=example,dc=com",
		"LDAP_ADMIN_GROUPS":    "Domain Admins,GOTRS Admins",
		"LDAP_AGENT_GROUPS":    "Support Team,Helpdesk",
	}

	// Add type-specific defaults
	if config, exists := DefaultConfigs[ldapType]; exists {
		template["LDAP_USER_FILTER"] = config.UserFilter
		template["LDAP_EMAIL_ATTRIBUTE"] = config.EmailAttribute
		template["LDAP_FIRST_NAME_ATTRIBUTE"] = config.FirstNameAttribute
		template["LDAP_LAST_NAME_ATTRIBUTE"] = config.LastNameAttribute
		template["LDAP_DISPLAY_NAME_ATTRIBUTE"] = config.DisplayNameAttribute
		template["LDAP_GROUP_FILTER"] = config.GroupFilter
		template["LDAP_GROUP_ATTRIBUTE"] = config.GroupAttribute

		if config.IsActiveDirectory {
			template["LDAP_IS_ACTIVE_DIRECTORY"] = "true"
			template["LDAP_DOMAIN"] = "example.com"
		} else {
			template["LDAP_IS_ACTIVE_DIRECTORY"] = "false"
			template["LDAP_DOMAIN"] = ""
		}
	}

	return template
}

// ExampleConfigs provides example configurations for documentation.
var ExampleConfigs = map[string]string{
	"active_directory": `# Active Directory Configuration
LDAP_ENABLED=true
LDAP_TYPE=active_directory
LDAP_HOST=dc.company.com
LDAP_PORT=389
LDAP_USE_TLS=true
LDAP_BIND_DN=cn=gotrs-service,ou=Service Accounts,dc=company,dc=com
LDAP_BIND_PASSWORD=your-bind-password
LDAP_BASE_DN=dc=company,dc=com
LDAP_USER_FILTER=(sAMAccountName=%s)
LDAP_GROUP_BASE_DN=ou=Groups,dc=company,dc=com
LDAP_GROUP_FILTER=(&(objectClass=group)(member=%s))
LDAP_EMAIL_ATTRIBUTE=mail
LDAP_FIRST_NAME_ATTRIBUTE=givenName
LDAP_LAST_NAME_ATTRIBUTE=sn
LDAP_DISPLAY_NAME_ATTRIBUTE=displayName
LDAP_GROUP_ATTRIBUTE=cn
LDAP_IS_ACTIVE_DIRECTORY=true
LDAP_DOMAIN=company.com
LDAP_ADMIN_GROUPS=Domain Admins,GOTRS Administrators
LDAP_AGENT_GROUPS=Support Team,IT Helpdesk`,

	"openldap": `# OpenLDAP Configuration
LDAP_ENABLED=true
LDAP_TYPE=openldap
LDAP_HOST=ldap.company.com
LDAP_PORT=389
LDAP_USE_TLS=true
LDAP_BIND_DN=cn=gotrs,ou=system,dc=company,dc=com
LDAP_BIND_PASSWORD=your-bind-password
LDAP_BASE_DN=dc=company,dc=com
LDAP_USER_FILTER=(uid=%s)
LDAP_GROUP_BASE_DN=ou=groups,dc=company,dc=com
LDAP_GROUP_FILTER=(&(objectClass=groupOfNames)(member=%s))
LDAP_EMAIL_ATTRIBUTE=mail
LDAP_FIRST_NAME_ATTRIBUTE=givenName
LDAP_LAST_NAME_ATTRIBUTE=sn
LDAP_DISPLAY_NAME_ATTRIBUTE=cn
LDAP_GROUP_ATTRIBUTE=cn
LDAP_ADMIN_GROUPS=gotrs-admins,system-admins
LDAP_AGENT_GROUPS=gotrs-agents,support-team`,

	"389ds": `# 389 Directory Server Configuration
LDAP_ENABLED=true
LDAP_TYPE=389ds
LDAP_HOST=ldap.company.com
LDAP_PORT=389
LDAP_USE_TLS=true
LDAP_BIND_DN=uid=gotrs,cn=sysaccounts,cn=etc,dc=company,dc=com
LDAP_BIND_PASSWORD=your-bind-password
LDAP_BASE_DN=dc=company,dc=com
LDAP_USER_FILTER=(uid=%s)
LDAP_GROUP_BASE_DN=cn=groups,cn=accounts,dc=company,dc=com
LDAP_GROUP_FILTER=(&(objectClass=groupOfUniqueNames)(uniqueMember=%s))
LDAP_EMAIL_ATTRIBUTE=mail
LDAP_FIRST_NAME_ATTRIBUTE=givenName
LDAP_LAST_NAME_ATTRIBUTE=sn
LDAP_DISPLAY_NAME_ATTRIBUTE=cn
LDAP_GROUP_ATTRIBUTE=cn
LDAP_ADMIN_GROUPS=gotrs-admins,admins
LDAP_AGENT_GROUPS=gotrs-agents,support`,
}
