package api

import (
	"os"
	"strings"
)

// GetTestConfig returns test configuration from environment variables with safe defaults
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

// GetTestConfig retrieves parameterized test configuration
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
