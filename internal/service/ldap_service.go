package service

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/go-ldap/ldap/v3"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/repository/memory"
)

// LDAPService handles LDAP/Active Directory integration.
type LDAPService struct {
	userRepo     *memory.UserRepository
	roleRepo     *memory.RoleRepository
	groupRepo    *memory.GroupRepository
	mu           sync.RWMutex
	connections  map[string]*ldap.Conn
	config       *LDAPConfig
	syncInterval time.Duration
	lastSync     time.Time
	stopChan     chan bool
	authLogs     []AuthLog
}

// AuthLog represents an authentication attempt log entry.
type AuthLog struct {
	Username  string    `json:"username"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
	IPAddress string    `json:"ip_address,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// LDAPConfig represents LDAP configuration.
type LDAPConfig struct {
	Host                 string           `json:"host"`
	Port                 int              `json:"port"`
	BaseDN               string           `json:"base_dn"`
	BindDN               string           `json:"bind_dn"`
	BindPassword         string           `json:"bind_password"`
	UserFilter           string           `json:"user_filter"`
	UserSearchBase       string           `json:"user_search_base,omitempty"`
	GroupFilter          string           `json:"group_filter,omitempty"`
	GroupSearchBase      string           `json:"group_search_base,omitempty"`
	UseTLS               bool             `json:"use_tls"`
	StartTLS             bool             `json:"start_tls"`
	InsecureSkipVerify   bool             `json:"insecure_skip_verify"`
	AttributeMap         LDAPAttributeMap `json:"attribute_map"`
	GroupMemberAttribute string           `json:"group_member_attribute"`
	AutoCreateUsers      bool             `json:"auto_create_users"`
	AutoUpdateUsers      bool             `json:"auto_update_users"`
	AutoCreateGroups     bool             `json:"auto_create_groups"`
	SyncInterval         time.Duration    `json:"sync_interval"`
	DefaultRole          string           `json:"default_role"`
	AdminGroups          []string         `json:"admin_groups,omitempty"`
	UserGroups           []string         `json:"user_groups,omitempty"`
}

// LDAPAttributeMap maps LDAP attributes to GoatFlow user fields.
type LDAPAttributeMap struct {
	Username    string `json:"username"`     // sAMAccountName, uid
	Email       string `json:"email"`        // mail, userPrincipalName
	FirstName   string `json:"first_name"`   // givenName
	LastName    string `json:"last_name"`    // sn
	DisplayName string `json:"display_name"` // displayName, cn
	Phone       string `json:"phone"`        // telephoneNumber
	Department  string `json:"department"`   // department
	Title       string `json:"title"`        // title
	Manager     string `json:"manager"`      // manager
	Groups      string `json:"groups"`       // memberOf
	ObjectGUID  string `json:"object_guid"`  // objectGUID
	ObjectSID   string `json:"object_sid"`   // objectSid
}

// LDAPUser represents a user from LDAP.
type LDAPUser struct {
	DN          string            `json:"dn"`
	Username    string            `json:"username"`
	Email       string            `json:"email"`
	FirstName   string            `json:"first_name"`
	LastName    string            `json:"last_name"`
	DisplayName string            `json:"display_name"`
	Phone       string            `json:"phone"`
	Department  string            `json:"department"`
	Title       string            `json:"title"`
	Manager     string            `json:"manager"`
	Groups      []string          `json:"groups"`
	Attributes  map[string]string `json:"attributes"`
	ObjectGUID  string            `json:"object_guid"`
	ObjectSID   string            `json:"object_sid"`
	LastLogin   time.Time         `json:"last_login"`
	IsActive    bool              `json:"is_active"`
}

// LDAPGroup represents a group from LDAP.
type LDAPGroup struct {
	DN          string   `json:"dn"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Members     []string `json:"members"`
	ObjectGUID  string   `json:"object_guid"`
	ObjectSID   string   `json:"object_sid"`
}

// LDAPSyncResult represents the result of an LDAP sync operation.
type LDAPSyncResult struct {
	UsersFound    int           `json:"users_found"`
	UsersCreated  int           `json:"users_created"`
	UsersUpdated  int           `json:"users_updated"`
	UsersDisabled int           `json:"users_disabled"`
	GroupsFound   int           `json:"groups_found"`
	GroupsCreated int           `json:"groups_created"`
	GroupsUpdated int           `json:"groups_updated"`
	Errors        []string      `json:"errors"`
	StartTime     time.Time     `json:"start_time"`
	EndTime       time.Time     `json:"end_time"`
	Duration      time.Duration `json:"duration"`
	DryRun        bool          `json:"dry_run"`
}

// NewLDAPService creates a new LDAP service.
func NewLDAPService(userRepo *memory.UserRepository, roleRepo *memory.RoleRepository, groupRepo *memory.GroupRepository) *LDAPService {
	return &LDAPService{
		userRepo:     userRepo,
		roleRepo:     roleRepo,
		groupRepo:    groupRepo,
		connections:  make(map[string]*ldap.Conn),
		syncInterval: 1 * time.Hour, // Default sync interval
		stopChan:     make(chan bool),
		authLogs:     make([]AuthLog, 0, 1000), // Keep last 1000 auth logs
	}
}

// ConfigureLDAP configures LDAP integration.
func (s *LDAPService) ConfigureLDAP(config *LDAPConfig) error {
	// Validate configuration
	if err := s.validateConfig(config); err != nil {
		return fmt.Errorf("invalid LDAP configuration: %w", err)
	}

	// Test connection
	conn, err := s.connect(config)
	if err != nil {
		return fmt.Errorf("failed to connect to LDAP server: %w", err)
	}
	defer conn.Close()

	// Test authentication
	if err := conn.Bind(config.BindDN, config.BindPassword); err != nil {
		return fmt.Errorf("failed to authenticate with LDAP server: %w", err)
	}

	s.mu.Lock()
	s.config = config
	s.syncInterval = config.SyncInterval
	if s.syncInterval == 0 {
		s.syncInterval = 1 * time.Hour
	}
	s.mu.Unlock()

	// Start background sync if enabled
	if config.AutoUpdateUsers {
		go s.startSyncScheduler()
	}

	return nil
}

// AuthenticateUser authenticates a user against LDAP.
func (s *LDAPService) AuthenticateUser(username, password string) (*LDAPUser, error) {
	s.mu.RLock()
	config := s.config
	s.mu.RUnlock()

	if config == nil {
		return nil, fmt.Errorf("LDAP not configured")
	}

	// Connect to LDAP
	conn, err := s.connect(config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to LDAP: %w", err)
	}
	defer conn.Close()

	// Search for user
	ldapUser, err := s.searchUser(conn, config, username)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Authenticate user
	if err := conn.Bind(ldapUser.DN, password); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Update last login
	ldapUser.LastLogin = time.Now()

	// Create or update user in database if auto-create is enabled
	if config.AutoCreateUsers {
		if err := s.createOrUpdateUser(ldapUser); err != nil {
			log.Printf("Failed to create/update user from LDAP: %v", err)
		}
	}

	return ldapUser, nil
}

// SyncUsers synchronizes users from LDAP.
func (s *LDAPService) SyncUsers() (*LDAPSyncResult, error) {
	s.mu.RLock()
	config := s.config
	s.mu.RUnlock()

	if config == nil {
		return nil, fmt.Errorf("LDAP not configured")
	}

	result := &LDAPSyncResult{
		StartTime: time.Now(),
		Errors:    make([]string, 0),
	}

	// Connect to LDAP
	conn, err := s.connect(config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to LDAP: %w", err)
	}
	defer conn.Close()

	// Bind with service account
	if err := conn.Bind(config.BindDN, config.BindPassword); err != nil {
		return nil, fmt.Errorf("failed to bind to LDAP: %w", err)
	}

	// Search for users
	users, err := s.searchAllUsers(conn, config)
	if err != nil {
		return nil, fmt.Errorf("failed to search users: %w", err)
	}

	result.UsersFound = len(users)

	// Process each user
	for _, ldapUser := range users {
		if err := s.createOrUpdateUser(ldapUser); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("User %s: %v", ldapUser.Username, err))
			continue
		}

		// Determine if user was created or updated
		existingUser, err := s.userRepo.GetByEmail(ldapUser.Email)
		if err != nil {
			result.UsersCreated++
		} else if existingUser != nil {
			result.UsersUpdated++
		}
	}

	// Sync groups if enabled
	if config.AutoCreateGroups {
		groupResult, err := s.syncGroups(conn, config)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Group sync failed: %v", err))
		} else {
			result.GroupsFound = groupResult.GroupsFound
			result.GroupsCreated = groupResult.GroupsCreated
			result.GroupsUpdated = groupResult.GroupsUpdated
		}
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	s.lastSync = result.EndTime

	return result, nil
}

// GetUser retrieves a user from LDAP.
func (s *LDAPService) GetUser(username string) (*LDAPUser, error) {
	s.mu.RLock()
	config := s.config
	s.mu.RUnlock()

	if config == nil {
		return nil, fmt.Errorf("LDAP not configured")
	}

	conn, err := s.connect(config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to LDAP: %w", err)
	}
	defer conn.Close()

	if err := conn.Bind(config.BindDN, config.BindPassword); err != nil {
		return nil, fmt.Errorf("failed to bind to LDAP: %w", err)
	}

	return s.searchUser(conn, config, username)
}

// GetGroups retrieves groups from LDAP.
func (s *LDAPService) GetGroups() ([]*LDAPGroup, error) {
	s.mu.RLock()
	config := s.config
	s.mu.RUnlock()

	if config == nil {
		return nil, fmt.Errorf("LDAP not configured")
	}

	conn, err := s.connect(config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to LDAP: %w", err)
	}
	defer conn.Close()

	if err := conn.Bind(config.BindDN, config.BindPassword); err != nil {
		return nil, fmt.Errorf("failed to bind to LDAP: %w", err)
	}

	return s.searchAllGroups(conn, config)
}

// TestConnection tests the LDAP connection.
func (s *LDAPService) TestConnection(config *LDAPConfig) error {
	conn, err := s.connect(config)
	if err != nil {
		return err
	}
	defer conn.Close()

	return conn.Bind(config.BindDN, config.BindPassword)
}

// GetSyncStatus returns the status of the last sync.
func (s *LDAPService) GetSyncStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"last_sync":     s.lastSync,
		"sync_interval": s.syncInterval,
		"configured":    s.config != nil,
		"auto_sync":     s.config != nil && s.config.AutoUpdateUsers,
	}
}

// Stop stops the LDAP service.
func (s *LDAPService) Stop() {
	close(s.stopChan)
}

// Private methods

// connect establishes connection to LDAP server.
func (s *LDAPService) connect(config *LDAPConfig) (*ldap.Conn, error) {
	// Prefer URL-based dialing with timeout
	scheme := "ldap"
	if config.UseTLS {
		scheme = "ldaps"
	}
	url := fmt.Sprintf("%s://%s:%d", scheme, config.Host, config.Port)
	dialer := &net.Dialer{Timeout: 10 * time.Second}

	conn, err := ldap.DialURL(url, ldap.DialWithDialer(dialer))
	if err != nil {
		return nil, err
	}

	if config.StartTLS && !config.UseTLS {
		tlsConfig := &tls.Config{InsecureSkipVerify: config.InsecureSkipVerify}
		if err := conn.StartTLS(tlsConfig); err != nil {
			conn.Close()
			return nil, err
		}
	}

	return conn, nil
}

// searchUser searches for a single user in LDAP.
func (s *LDAPService) searchUser(conn *ldap.Conn, config *LDAPConfig, username string) (*LDAPUser, error) {
	searchBase := config.UserSearchBase
	if searchBase == "" {
		searchBase = config.BaseDN
	}

	filter := config.UserFilter
	if filter == "" {
		filter = fmt.Sprintf("(&(objectClass=user)(%s=%s))", config.AttributeMap.Username, username)
	} else {
		filter = strings.ReplaceAll(filter, "{username}", username)
	}

	searchRequest := ldap.NewSearchRequest(
		searchBase,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		filter,
		s.getUserAttributes(config),
		nil,
	)

	sr, err := conn.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	if len(sr.Entries) == 0 {
		return nil, fmt.Errorf("user not found")
	}

	if len(sr.Entries) > 1 {
		return nil, fmt.Errorf("multiple users found")
	}

	return s.mapLDAPUser(sr.Entries[0], config), nil
}

// searchAllUsers searches for all users in LDAP.
func (s *LDAPService) searchAllUsers(conn *ldap.Conn, config *LDAPConfig) ([]*LDAPUser, error) {
	searchBase := config.UserSearchBase
	if searchBase == "" {
		searchBase = config.BaseDN
	}

	filter := "(&(objectClass=user)(!(objectClass=computer)))"
	if config.UserFilter != "" && !strings.Contains(config.UserFilter, "{username}") {
		filter = config.UserFilter
	}

	searchRequest := ldap.NewSearchRequest(
		searchBase,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		filter,
		s.getUserAttributes(config),
		nil,
	)

	sr, err := conn.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	users := make([]*LDAPUser, len(sr.Entries))
	for i, entry := range sr.Entries {
		users[i] = s.mapLDAPUser(entry, config)
	}

	return users, nil
}

// searchAllGroups searches for all groups in LDAP.
func (s *LDAPService) searchAllGroups(conn *ldap.Conn, config *LDAPConfig) ([]*LDAPGroup, error) {
	if config.GroupFilter == "" {
		return nil, nil // Groups not configured
	}

	searchBase := config.GroupSearchBase
	if searchBase == "" {
		searchBase = config.BaseDN
	}

	searchRequest := ldap.NewSearchRequest(
		searchBase,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		config.GroupFilter,
		s.getGroupAttributes(config),
		nil,
	)

	sr, err := conn.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	groups := make([]*LDAPGroup, len(sr.Entries))
	for i, entry := range sr.Entries {
		groups[i] = s.mapLDAPGroup(entry, config)
	}

	return groups, nil
}

// getUserAttributes returns the list of user attributes to retrieve.
func (s *LDAPService) getUserAttributes(config *LDAPConfig) []string {
	attrs := []string{
		"dn",
		config.AttributeMap.Username,
		config.AttributeMap.Email,
		config.AttributeMap.FirstName,
		config.AttributeMap.LastName,
		config.AttributeMap.DisplayName,
		config.AttributeMap.Phone,
		config.AttributeMap.Department,
		config.AttributeMap.Title,
		config.AttributeMap.Manager,
		config.AttributeMap.Groups,
		config.AttributeMap.ObjectGUID,
		config.AttributeMap.ObjectSID,
		"userAccountControl", // For checking if account is enabled
	}

	// Remove empty attributes
	var result []string
	for _, attr := range attrs {
		if attr != "" {
			result = append(result, attr)
		}
	}

	return result
}

// getGroupAttributes returns the list of group attributes to retrieve.
func (s *LDAPService) getGroupAttributes(config *LDAPConfig) []string {
	attrs := []string{
		"dn",
		"cn",
		"description",
		config.GroupMemberAttribute,
		config.AttributeMap.ObjectGUID,
		config.AttributeMap.ObjectSID,
	}

	// Remove empty attributes
	var result []string
	for _, attr := range attrs {
		if attr != "" {
			result = append(result, attr)
		}
	}

	return result
}

// mapLDAPUser maps an LDAP entry to an LDAPUser.
func (s *LDAPService) mapLDAPUser(entry *ldap.Entry, config *LDAPConfig) *LDAPUser {
	user := &LDAPUser{
		DN:         entry.DN,
		Attributes: make(map[string]string),
		IsActive:   true, // Default to active
	}

	// Map standard attributes
	user.Username = s.getAttributeValue(entry, config.AttributeMap.Username)
	user.Email = s.getAttributeValue(entry, config.AttributeMap.Email)
	user.FirstName = s.getAttributeValue(entry, config.AttributeMap.FirstName)
	user.LastName = s.getAttributeValue(entry, config.AttributeMap.LastName)
	user.DisplayName = s.getAttributeValue(entry, config.AttributeMap.DisplayName)
	user.Phone = s.getAttributeValue(entry, config.AttributeMap.Phone)
	user.Department = s.getAttributeValue(entry, config.AttributeMap.Department)
	user.Title = s.getAttributeValue(entry, config.AttributeMap.Title)
	user.Manager = s.getAttributeValue(entry, config.AttributeMap.Manager)
	user.ObjectGUID = s.getAttributeValue(entry, config.AttributeMap.ObjectGUID)
	user.ObjectSID = s.getAttributeValue(entry, config.AttributeMap.ObjectSID)

	// Parse groups
	if config.AttributeMap.Groups != "" {
		user.Groups = entry.GetAttributeValues(config.AttributeMap.Groups)
	}

	// Check if account is enabled (Active Directory specific)
	if uac := s.getAttributeValue(entry, "userAccountControl"); uac != "" {
		// UserAccountControl flags: 0x2 = ACCOUNTDISABLE
		user.IsActive = !strings.Contains(uac, "514") && !strings.Contains(uac, "546")
	}

	// Set display name fallback
	if user.DisplayName == "" {
		if user.FirstName != "" && user.LastName != "" {
			user.DisplayName = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
		} else if user.Username != "" {
			user.DisplayName = user.Username
		}
	}

	// Store all attributes
	for _, attr := range entry.Attributes {
		user.Attributes[attr.Name] = strings.Join(attr.Values, ", ")
	}

	return user
}

// mapLDAPGroup maps an LDAP entry to an LDAPGroup.
func (s *LDAPService) mapLDAPGroup(entry *ldap.Entry, config *LDAPConfig) *LDAPGroup {
	group := &LDAPGroup{
		DN:          entry.DN,
		Name:        s.getAttributeValue(entry, "cn"),
		Description: s.getAttributeValue(entry, "description"),
		ObjectGUID:  s.getAttributeValue(entry, config.AttributeMap.ObjectGUID),
		ObjectSID:   s.getAttributeValue(entry, config.AttributeMap.ObjectSID),
	}

	// Parse members
	if config.GroupMemberAttribute != "" {
		group.Members = entry.GetAttributeValues(config.GroupMemberAttribute)
	}

	return group
}

// getAttributeValue safely gets an attribute value from LDAP entry.
func (s *LDAPService) getAttributeValue(entry *ldap.Entry, attribute string) string {
	if attribute == "" {
		return ""
	}
	values := entry.GetAttributeValues(attribute)
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

// createOrUpdateUser creates or updates a user from LDAP data.
func (s *LDAPService) createOrUpdateUser(ldapUser *LDAPUser) error {
	// Check if user exists
	existingUser, err := s.userRepo.GetByEmail(ldapUser.Email)
	if err != nil && err.Error() != "user not found" {
		return fmt.Errorf("failed to check existing user: %w", err)
	}

	user := &models.User{
		Email:     ldapUser.Email,
		FirstName: ldapUser.FirstName,
		LastName:  ldapUser.LastName,
		Title:     ldapUser.Title,
		Login:     ldapUser.Username,
		ValidID:   1, // Active by default, will be set to 2 if ldapUser.IsActive is false
	}

	if existingUser == nil {
		// Create new user
		user.CreateTime = time.Now()
		user.ChangeTime = time.Now()
		if !ldapUser.IsActive {
			user.ValidID = 2 // Set to invalid if LDAP user is not active
		} else {
			user.ValidID = 1 // Set to valid if LDAP user is active
		}

		// Set default role
		if s.config.DefaultRole != "" {
			role, err := s.roleRepo.GetByName(context.Background(), s.config.DefaultRole)
			if err == nil {
				user.Role = role.Name
			}
		}

		return s.userRepo.Create(user)
	} else {
		// Update existing user
		existingUser.FirstName = user.FirstName
		existingUser.LastName = user.LastName
		existingUser.Title = user.Title
		if !ldapUser.IsActive {
			existingUser.ValidID = 2 // Set to invalid if LDAP user is not active
		} else {
			existingUser.ValidID = 1 // Set to valid if LDAP user is active
		}
		existingUser.ChangeTime = time.Now()

		// Update role based on group membership
		s.updateUserRole(existingUser, ldapUser)

		return s.userRepo.Update(existingUser)
	}
}

// updateUserRole updates user role based on LDAP group membership.
func (s *LDAPService) updateUserRole(user *models.User, ldapUser *LDAPUser) {
	if s.config == nil {
		return
	}

	// Check admin groups
	for _, adminGroup := range s.config.AdminGroups {
		for _, userGroup := range ldapUser.Groups {
			if strings.Contains(strings.ToLower(userGroup), strings.ToLower(adminGroup)) {
				if role, err := s.roleRepo.GetByName(context.Background(), "admin"); err == nil {
					user.Role = role.Name
					return
				}
			}
		}
	}

	// Check user groups
	for _, userGroup := range s.config.UserGroups {
		for _, ldapGroup := range ldapUser.Groups {
			if strings.Contains(strings.ToLower(ldapGroup), strings.ToLower(userGroup)) {
				if role, err := s.roleRepo.GetByName(context.Background(), "user"); err == nil {
					user.Role = role.Name
					return
				}
			}
		}
	}
}

// syncGroups synchronizes groups from LDAP.
func (s *LDAPService) syncGroups(conn *ldap.Conn, config *LDAPConfig) (*LDAPSyncResult, error) {
	result := &LDAPSyncResult{
		StartTime: time.Now(),
		Errors:    make([]string, 0),
	}

	groups, err := s.searchAllGroups(conn, config)
	if err != nil {
		return nil, err
	}

	result.GroupsFound = len(groups)

	for _, ldapGroup := range groups {
		// Check if group exists
		existingGroup, err := s.groupRepo.GetByName(context.Background(), ldapGroup.Name)
		if err != nil && err.Error() != "group not found" {
			result.Errors = append(result.Errors, fmt.Sprintf("Group %s: %v", ldapGroup.Name, err))
			continue
		}

		if existingGroup == nil {
			// Create new group
			group := &models.Group{
				ID:          ldapGroup.Name, // Use name as ID for LDAP groups
				Name:        ldapGroup.Name,
				Description: ldapGroup.Description,
				Type:        models.GroupTypeLDAP,
				DN:          ldapGroup.DN,
				ValidID:     1, // Active
				CreateTime:  time.Now(),
				ChangeTime:  time.Now(),
			}

			if err := s.groupRepo.CreateGroup(context.Background(), group); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Group %s: %v", ldapGroup.Name, err))
			} else {
				result.GroupsCreated++
			}
		} else {
			// Update existing group
			existingGroup.Description = ldapGroup.Description
			existingGroup.UpdatedAt = time.Now()

			if err := s.groupRepo.UpdateGroup(context.Background(), existingGroup); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Group %s: %v", ldapGroup.Name, err))
			} else {
				result.GroupsUpdated++
			}
		}
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// validateConfig validates LDAP configuration.
func (s *LDAPService) validateConfig(config *LDAPConfig) error {
	if config.Host == "" {
		return fmt.Errorf("host is required")
	}

	if config.Port == 0 {
		config.Port = 389
		if config.UseTLS {
			config.Port = 636
		}
	}

	if config.BaseDN == "" {
		return fmt.Errorf("base DN is required")
	}

	if config.BindDN == "" {
		return fmt.Errorf("bind DN is required")
	}

	if config.BindPassword == "" {
		return fmt.Errorf("bind password is required")
	}

	// Set default attribute mappings for Active Directory
	if config.AttributeMap.Username == "" {
		config.AttributeMap.Username = "sAMAccountName"
	}
	if config.AttributeMap.Email == "" {
		config.AttributeMap.Email = "mail"
	}
	if config.AttributeMap.FirstName == "" {
		config.AttributeMap.FirstName = "givenName"
	}
	if config.AttributeMap.LastName == "" {
		config.AttributeMap.LastName = "sn"
	}
	if config.AttributeMap.DisplayName == "" {
		config.AttributeMap.DisplayName = "displayName"
	}
	if config.AttributeMap.Groups == "" {
		config.AttributeMap.Groups = "memberOf"
	}
	if config.AttributeMap.ObjectGUID == "" {
		config.AttributeMap.ObjectGUID = "objectGUID"
	}
	if config.AttributeMap.ObjectSID == "" {
		config.AttributeMap.ObjectSID = "objectSid"
	}

	if config.GroupMemberAttribute == "" {
		config.GroupMemberAttribute = "member"
	}

	return nil
}

// startSyncScheduler starts the background sync scheduler.
func (s *LDAPService) startSyncScheduler() {
	ticker := time.NewTicker(s.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Println("Starting scheduled LDAP sync...")
			if result, err := s.SyncUsers(); err != nil {
				log.Printf("LDAP sync failed: %v", err)
			} else {
				log.Printf("LDAP sync completed: %d users found, %d created, %d updated",
					result.UsersFound, result.UsersCreated, result.UsersUpdated)
			}
		case <-s.stopChan:
			log.Println("LDAP sync scheduler stopped")
			return
		}
	}
}

// ImportUsers imports specific users from LDAP.
func (s *LDAPService) ImportUsers(usernames []string, dryRun bool) (*LDAPSyncResult, error) {
	if s.config == nil {
		return nil, fmt.Errorf("LDAP not configured")
	}

	result := &LDAPSyncResult{
		StartTime: time.Now(),
		DryRun:    dryRun,
	}

	for _, username := range usernames {
		ldapUser, err := s.GetUser(username)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("User %s: %v", username, err))
			continue
		}

		if !dryRun {
			if err := s.createOrUpdateUser(ldapUser); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("User %s: %v", username, err))
			} else {
				result.UsersCreated++
			}
		} else {
			result.UsersFound++
		}
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	return result, nil
}

// GetConfig returns the current LDAP configuration (without sensitive data).
func (s *LDAPService) GetConfig() *LDAPConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// GetAuthLogs returns authentication logs.
func (s *LDAPService) GetAuthLogs(limit int) []AuthLog {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > len(s.authLogs) {
		limit = len(s.authLogs)
	}

	logs := make([]AuthLog, limit)
	startIdx := len(s.authLogs) - limit
	copy(logs, s.authLogs[startIdx:])
	return logs
}
