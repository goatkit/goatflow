package models

import "time"

// LDAPConfiguration represents LDAP configuration stored in database.
type LDAPConfiguration struct {
	ID                   int        `json:"id" db:"id"`
	Name                 string     `json:"name" db:"name"`
	Host                 string     `json:"host" db:"host"`
	Port                 int        `json:"port" db:"port"`
	BaseDN               string     `json:"base_dn" db:"base_dn"`
	BindDN               string     `json:"bind_dn" db:"bind_dn"`
	BindPassword         string     `json:"bind_password" db:"bind_password"`
	UserFilter           string     `json:"user_filter" db:"user_filter"`
	UserSearchBase       string     `json:"user_search_base" db:"user_search_base"`
	GroupFilter          string     `json:"group_filter" db:"group_filter"`
	GroupSearchBase      string     `json:"group_search_base" db:"group_search_base"`
	UseTLS               bool       `json:"use_tls" db:"use_tls"`
	StartTLS             bool       `json:"start_tls" db:"start_tls"`
	InsecureSkipVerify   bool       `json:"insecure_skip_verify" db:"insecure_skip_verify"`
	AttributeMapping     string     `json:"attribute_mapping" db:"attribute_mapping"` // JSON
	GroupMemberAttribute string     `json:"group_member_attribute" db:"group_member_attribute"`
	AutoCreateUsers      bool       `json:"auto_create_users" db:"auto_create_users"`
	AutoUpdateUsers      bool       `json:"auto_update_users" db:"auto_update_users"`
	AutoCreateGroups     bool       `json:"auto_create_groups" db:"auto_create_groups"`
	SyncIntervalMinutes  int        `json:"sync_interval_minutes" db:"sync_interval_minutes"`
	DefaultRoleID        int        `json:"default_role_id" db:"default_role_id"`
	AdminGroups          string     `json:"admin_groups" db:"admin_groups"` // JSON array
	UserGroups           string     `json:"user_groups" db:"user_groups"`   // JSON array
	IsActive             bool       `json:"is_active" db:"is_active"`
	TestMode             bool       `json:"test_mode" db:"test_mode"`
	LastSyncAt           *time.Time `json:"last_sync_at" db:"last_sync_at"`
	SyncStatus           string     `json:"sync_status" db:"sync_status"`
	SyncMessage          string     `json:"sync_message" db:"sync_message"`
	CreatedAt            time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at" db:"updated_at"`
	CreatedBy            int        `json:"created_by" db:"created_by"`
	UpdatedBy            int        `json:"updated_by" db:"updated_by"`
}

// LDAPSyncHistory represents LDAP sync history.
type LDAPSyncHistory struct {
	ID            int        `json:"id" db:"id"`
	ConfigID      int        `json:"config_id" db:"config_id"`
	StartTime     time.Time  `json:"start_time" db:"start_time"`
	EndTime       *time.Time `json:"end_time" db:"end_time"`
	Status        string     `json:"status" db:"status"` // running, completed, failed
	UsersFound    int        `json:"users_found" db:"users_found"`
	UsersCreated  int        `json:"users_created" db:"users_created"`
	UsersUpdated  int        `json:"users_updated" db:"users_updated"`
	UsersDisabled int        `json:"users_disabled" db:"users_disabled"`
	GroupsFound   int        `json:"groups_found" db:"groups_found"`
	GroupsCreated int        `json:"groups_created" db:"groups_created"`
	GroupsUpdated int        `json:"groups_updated" db:"groups_updated"`
	ErrorCount    int        `json:"error_count" db:"error_count"`
	ErrorLog      string     `json:"error_log" db:"error_log"`       // JSON array of errors
	Duration      int64      `json:"duration" db:"duration"`         // Milliseconds
	TriggeredBy   string     `json:"triggered_by" db:"triggered_by"` // manual, scheduled, api
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
}

// LDAPUserMapping represents mapping between LDAP and GoatFlow users.
type LDAPUserMapping struct {
	ID             int       `json:"id" db:"id"`
	UserID         int       `json:"user_id" db:"user_id"`
	ConfigID       int       `json:"config_id" db:"config_id"`
	LDAPUserDN     string    `json:"ldap_user_dn" db:"ldap_user_dn"`
	LDAPUsername   string    `json:"ldap_username" db:"ldap_username"`
	LDAPObjectGUID string    `json:"ldap_object_guid" db:"ldap_object_guid"`
	LDAPObjectSID  string    `json:"ldap_object_sid" db:"ldap_object_sid"`
	LDAPAttributes string    `json:"ldap_attributes" db:"ldap_attributes"` // JSON
	LastSyncAt     time.Time `json:"last_sync_at" db:"last_sync_at"`
	IsActive       bool      `json:"is_active" db:"is_active"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// LDAPGroupMapping represents mapping between LDAP and GoatFlow groups.
type LDAPGroupMapping struct {
	ID             int       `json:"id" db:"id"`
	GroupID        int       `json:"group_id" db:"group_id"`
	ConfigID       int       `json:"config_id" db:"config_id"`
	LDAPGroupDN    string    `json:"ldap_group_dn" db:"ldap_group_dn"`
	LDAPGroupName  string    `json:"ldap_group_name" db:"ldap_group_name"`
	LDAPObjectGUID string    `json:"ldap_object_guid" db:"ldap_object_guid"`
	LDAPObjectSID  string    `json:"ldap_object_sid" db:"ldap_object_sid"`
	RoleMapping    string    `json:"role_mapping" db:"role_mapping"` // JSON
	LastSyncAt     time.Time `json:"last_sync_at" db:"last_sync_at"`
	IsActive       bool      `json:"is_active" db:"is_active"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// LDAPAuthenticationLog represents LDAP authentication attempts.
type LDAPAuthenticationLog struct {
	ID           int       `json:"id" db:"id"`
	ConfigID     int       `json:"config_id" db:"config_id"`
	Username     string    `json:"username" db:"username"`
	UserID       *int      `json:"user_id" db:"user_id"`
	Success      bool      `json:"success" db:"success"`
	ErrorMessage string    `json:"error_message" db:"error_message"`
	IPAddress    string    `json:"ip_address" db:"ip_address"`
	UserAgent    string    `json:"user_agent" db:"user_agent"`
	AuthTime     time.Time `json:"auth_time" db:"auth_time"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// Constants for LDAP sync status.
const (
	LDAPSyncStatusPending   = "pending"
	LDAPSyncStatusRunning   = "running"
	LDAPSyncStatusCompleted = "completed"
	LDAPSyncStatusFailed    = "failed"
	LDAPSyncStatusCancelled = "cancelled"
)

// Constants for LDAP sync triggers.
const (
	LDAPSyncTriggerManual    = "manual"
	LDAPSyncTriggerScheduled = "scheduled"
	LDAPSyncTriggerAPI       = "api"
	LDAPSyncTriggerStartup   = "startup"
)

// LDAPSyncStatistics represents aggregated sync statistics.
type LDAPSyncStatistics struct {
	ConfigID           int        `json:"config_id"`
	TotalSyncs         int        `json:"total_syncs"`
	SuccessfulSyncs    int        `json:"successful_syncs"`
	FailedSyncs        int        `json:"failed_syncs"`
	LastSyncAt         *time.Time `json:"last_sync_at"`
	LastSuccessfulSync *time.Time `json:"last_successful_sync"`
	AverageDuration    int64      `json:"average_duration"` // Milliseconds
	TotalUsersCreated  int        `json:"total_users_created"`
	TotalUsersUpdated  int        `json:"total_users_updated"`
	TotalGroupsCreated int        `json:"total_groups_created"`
	TotalGroupsUpdated int        `json:"total_groups_updated"`
	ErrorRate          float64    `json:"error_rate"`
}

// LDAPConnectionTest represents a test connection result.
type LDAPConnectionTest struct {
	Success      bool      `json:"success"`
	ErrorMessage string    `json:"error_message,omitempty"`
	ResponseTime int64     `json:"response_time"` // Milliseconds
	ServerInfo   string    `json:"server_info,omitempty"`
	UserCount    int       `json:"user_count,omitempty"`
	GroupCount   int       `json:"group_count,omitempty"`
	TestedAt     time.Time `json:"tested_at"`
}
