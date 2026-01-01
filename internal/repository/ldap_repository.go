package repository

import (
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// LDAPRepository defines the interface for LDAP configuration and sync data.
type LDAPRepository interface {
	// Configuration management
	CreateConfig(config *models.LDAPConfiguration) error
	UpdateConfig(config *models.LDAPConfiguration) error
	GetConfigByID(id int) (*models.LDAPConfiguration, error)
	GetActiveConfig() (*models.LDAPConfiguration, error)
	ListConfigs() ([]*models.LDAPConfiguration, error)
	DeleteConfig(id int) error

	// Sync history
	CreateSyncHistory(history *models.LDAPSyncHistory) error
	UpdateSyncHistory(history *models.LDAPSyncHistory) error
	GetSyncHistoryByID(id int) (*models.LDAPSyncHistory, error)
	ListSyncHistory(configID int, limit, offset int) ([]*models.LDAPSyncHistory, error)
	GetLatestSyncHistory(configID int) (*models.LDAPSyncHistory, error)
	DeleteOldSyncHistory(configID int, keepDays int) error

	// User mappings
	CreateUserMapping(mapping *models.LDAPUserMapping) error
	UpdateUserMapping(mapping *models.LDAPUserMapping) error
	GetUserMappingByUserID(userID int) (*models.LDAPUserMapping, error)
	GetUserMappingByLDAPDN(configID int, dn string) (*models.LDAPUserMapping, error)
	GetUserMappingByLDAPGUID(configID int, guid string) (*models.LDAPUserMapping, error)
	ListUserMappings(configID int) ([]*models.LDAPUserMapping, error)
	DeleteUserMapping(id int) error

	// Group mappings
	CreateGroupMapping(mapping *models.LDAPGroupMapping) error
	UpdateGroupMapping(mapping *models.LDAPGroupMapping) error
	GetGroupMappingByGroupID(groupID int) (*models.LDAPGroupMapping, error)
	GetGroupMappingByLDAPDN(configID int, dn string) (*models.LDAPGroupMapping, error)
	GetGroupMappingByLDAPGUID(configID int, guid string) (*models.LDAPGroupMapping, error)
	ListGroupMappings(configID int) ([]*models.LDAPGroupMapping, error)
	DeleteGroupMapping(id int) error

	// Authentication logs
	CreateAuthLog(log *models.LDAPAuthenticationLog) error
	GetAuthLogsByUsername(username string, limit int) ([]*models.LDAPAuthenticationLog, error)
	GetAuthLogsByTimeRange(configID int, startTime, endTime time.Time) ([]*models.LDAPAuthenticationLog, error)
	DeleteOldAuthLogs(configID int, keepDays int) error

	// Statistics
	GetSyncStatistics(configID int) (*models.LDAPSyncStatistics, error)
	GetAuthStatistics(configID int, days int) (map[string]interface{}, error)

	// Testing
	RecordConnectionTest(test *models.LDAPConnectionTest) error
}
