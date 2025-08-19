package ldap

import (
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
)

// Provider implements LDAP/Active Directory authentication
type Provider struct {
	config *Config
	conn   *ldap.Conn
}

// Config holds LDAP configuration
type Config struct {
	// Connection settings
	Host     string `json:"host"`
	Port     int    `json:"port"`
	UseSSL   bool   `json:"use_ssl"`
	UseTLS   bool   `json:"use_tls"`
	SkipTLS  bool   `json:"skip_tls_verify"`
	Timeout  int    `json:"timeout_seconds"`
	
	// Bind settings
	BindDN       string `json:"bind_dn"`
	BindPassword string `json:"bind_password"`
	
	// User search settings
	BaseDN       string `json:"base_dn"`
	UserFilter   string `json:"user_filter"`   // e.g., "(uid=%s)" or "(sAMAccountName=%s)"
	UserBaseDN   string `json:"user_base_dn"`  // Optional, defaults to BaseDN
	
	// User attributes mapping
	EmailAttribute     string `json:"email_attribute"`      // e.g., "mail"
	FirstNameAttribute string `json:"first_name_attribute"` // e.g., "givenName"
	LastNameAttribute  string `json:"last_name_attribute"`  // e.g., "sn"
	DisplayNameAttribute string `json:"display_name_attribute"` // e.g., "displayName"
	
	// Group settings (optional)
	GroupBaseDN    string `json:"group_base_dn"`
	GroupFilter    string `json:"group_filter"`    // e.g., "(&(objectClass=group)(member=%s))"
	GroupAttribute string `json:"group_attribute"` // e.g., "cn"
	
	// Role mapping
	AdminGroups []string `json:"admin_groups"`
	AgentGroups []string `json:"agent_groups"`
	
	// Active Directory specific
	IsActiveDirectory bool   `json:"is_active_directory"`
	Domain           string `json:"domain"` // For AD, e.g., "company.com"
}

// User represents an LDAP user
type User struct {
	DN          string   `json:"dn"`
	Username    string   `json:"username"`
	Email       string   `json:"email"`
	FirstName   string   `json:"first_name"`
	LastName    string   `json:"last_name"`
	DisplayName string   `json:"display_name"`
	Groups      []string `json:"groups"`
	Role        string   `json:"role"` // Admin, Agent, Customer
}

// AuthResult represents authentication result
type AuthResult struct {
	Success     bool   `json:"success"`
	User        *User  `json:"user,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// DefaultConfigs provides common LDAP configurations
var DefaultConfigs = map[string]*Config{
	"active_directory": {
		Port:                 389,
		UseSSL:               false,
		UseTLS:               true,
		UserFilter:           "(sAMAccountName=%s)",
		EmailAttribute:       "mail",
		FirstNameAttribute:   "givenName",
		LastNameAttribute:    "sn",
		DisplayNameAttribute: "displayName",
		GroupFilter:          "(&(objectClass=group)(member=%s))",
		GroupAttribute:       "cn",
		IsActiveDirectory:    true,
		Timeout:              30,
	},
	"openldap": {
		Port:                 389,
		UseSSL:               false,
		UseTLS:               true,
		UserFilter:           "(uid=%s)",
		EmailAttribute:       "mail",
		FirstNameAttribute:   "givenName",
		LastNameAttribute:    "sn",
		DisplayNameAttribute: "cn",
		GroupFilter:          "(&(objectClass=groupOfNames)(member=%s))",
		GroupAttribute:       "cn",
		IsActiveDirectory:    false,
		Timeout:              30,
	},
	"389ds": {
		Port:                 389,
		UseSSL:               false,
		UseTLS:               true,
		UserFilter:           "(uid=%s)",
		EmailAttribute:       "mail",
		FirstNameAttribute:   "givenName",
		LastNameAttribute:    "sn",
		DisplayNameAttribute: "cn",
		GroupFilter:          "(&(objectClass=groupOfUniqueNames)(uniqueMember=%s))",
		GroupAttribute:       "cn",
		IsActiveDirectory:    false,
		Timeout:              30,
	},
}

// NewProvider creates a new LDAP provider
func NewProvider(config *Config) *Provider {
	return &Provider{
		config: config,
	}
}

// Connect establishes connection to LDAP server
func (p *Provider) Connect() error {
	var err error
	address := fmt.Sprintf("%s:%d", p.config.Host, p.config.Port)
	
	if p.config.UseSSL {
		tlsConfig := &tls.Config{
			ServerName:         p.config.Host,
			InsecureSkipVerify: p.config.SkipTLS,
		}
		p.conn, err = ldap.DialTLS("tcp", address, tlsConfig)
	} else {
		p.conn, err = ldap.Dial("tcp", address)
	}
	
	if err != nil {
		return fmt.Errorf("failed to connect to LDAP server: %w", err)
	}
	
	// Set timeout
	if p.config.Timeout > 0 {
		p.conn.SetTimeout(time.Duration(p.config.Timeout) * time.Second)
	}
	
	// Start TLS if requested
	if p.config.UseTLS && !p.config.UseSSL {
		tlsConfig := &tls.Config{
			ServerName:         p.config.Host,
			InsecureSkipVerify: p.config.SkipTLS,
		}
		err = p.conn.StartTLS(tlsConfig)
		if err != nil {
			p.conn.Close()
			return fmt.Errorf("failed to start TLS: %w", err)
		}
	}
	
	// Bind with service account if provided
	if p.config.BindDN != "" && p.config.BindPassword != "" {
		err = p.conn.Bind(p.config.BindDN, p.config.BindPassword)
		if err != nil {
			p.conn.Close()
			return fmt.Errorf("failed to bind with service account: %w", err)
		}
	}
	
	return nil
}

// Close closes the LDAP connection
func (p *Provider) Close() {
	if p.conn != nil {
		p.conn.Close()
		p.conn = nil
	}
}

// Authenticate authenticates a user with username and password
func (p *Provider) Authenticate(username, password string) *AuthResult {
	if p.conn == nil {
		if err := p.Connect(); err != nil {
			return &AuthResult{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Connection failed: %v", err),
			}
		}
		defer p.Close()
	}
	
	// Search for user
	user, err := p.findUser(username)
	if err != nil {
		return &AuthResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("User lookup failed: %v", err),
		}
	}
	
	if user == nil {
		return &AuthResult{
			Success:      false,
			ErrorMessage: "User not found",
		}
	}
	
	// Try to bind as the user to verify password
	err = p.conn.Bind(user.DN, password)
	if err != nil {
		// Re-bind with service account for subsequent operations
		if p.config.BindDN != "" {
			p.conn.Bind(p.config.BindDN, p.config.BindPassword)
		}
		
		return &AuthResult{
			Success:      false,
			ErrorMessage: "Invalid credentials",
		}
	}
	
	// Re-bind with service account for group lookup
	if p.config.BindDN != "" {
		p.conn.Bind(p.config.BindDN, p.config.BindPassword)
	}
	
	// Get user groups and determine role
	groups, err := p.getUserGroups(user.DN)
	if err != nil {
		// Don't fail authentication for group lookup errors
		groups = []string{}
	}
	
	user.Groups = groups
	user.Role = p.determineRole(groups)
	
	return &AuthResult{
		Success: true,
		User:    user,
	}
}

// findUser searches for a user in LDAP
func (p *Provider) findUser(username string) (*User, error) {
	searchBaseDN := p.config.UserBaseDN
	if searchBaseDN == "" {
		searchBaseDN = p.config.BaseDN
	}
	
	// Build search filter
	filter := fmt.Sprintf(p.config.UserFilter, ldap.EscapeFilter(username))
	
	// For Active Directory, also try userPrincipalName format
	if p.config.IsActiveDirectory {
		if !strings.Contains(username, "@") && p.config.Domain != "" {
			upnFilter := fmt.Sprintf("(userPrincipalName=%s@%s)", 
				ldap.EscapeFilter(username), ldap.EscapeFilter(p.config.Domain))
			filter = fmt.Sprintf("(|%s%s)", filter, upnFilter)
		}
	}
	
	// Build attributes to retrieve
	attributes := []string{
		p.config.EmailAttribute,
		p.config.FirstNameAttribute,
		p.config.LastNameAttribute,
		p.config.DisplayNameAttribute,
	}
	
	searchRequest := ldap.NewSearchRequest(
		searchBaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		1, // Size limit - we only want one user
		int(time.Duration(p.config.Timeout)*time.Second/time.Second),
		false,
		filter,
		attributes,
		nil,
	)
	
	result, err := p.conn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}
	
	if len(result.Entries) == 0 {
		return nil, nil // User not found
	}
	
	entry := result.Entries[0]
	
	// Extract user information
	user := &User{
		DN:          entry.DN,
		Username:    username,
		Email:       entry.GetAttributeValue(p.config.EmailAttribute),
		FirstName:   entry.GetAttributeValue(p.config.FirstNameAttribute),
		LastName:    entry.GetAttributeValue(p.config.LastNameAttribute),
		DisplayName: entry.GetAttributeValue(p.config.DisplayNameAttribute),
	}
	
	// Use email as username if no explicit username
	if user.Email != "" && user.Username == "" {
		user.Username = user.Email
	}
	
	// Generate display name if not provided
	if user.DisplayName == "" {
		if user.FirstName != "" && user.LastName != "" {
			user.DisplayName = user.FirstName + " " + user.LastName
		} else if user.FirstName != "" {
			user.DisplayName = user.FirstName
		} else {
			user.DisplayName = user.Username
		}
	}
	
	return user, nil
}

// getUserGroups retrieves groups for a user
func (p *Provider) getUserGroups(userDN string) ([]string, error) {
	if p.config.GroupBaseDN == "" || p.config.GroupFilter == "" {
		return []string{}, nil
	}
	
	// Build group search filter
	filter := fmt.Sprintf(p.config.GroupFilter, ldap.EscapeFilter(userDN))
	
	searchRequest := ldap.NewSearchRequest(
		p.config.GroupBaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0, // No size limit
		int(time.Duration(p.config.Timeout)*time.Second/time.Second),
		false,
		filter,
		[]string{p.config.GroupAttribute},
		nil,
	)
	
	result, err := p.conn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("group search failed: %w", err)
	}
	
	groups := make([]string, 0, len(result.Entries))
	for _, entry := range result.Entries {
		groupName := entry.GetAttributeValue(p.config.GroupAttribute)
		if groupName != "" {
			groups = append(groups, groupName)
		}
	}
	
	return groups, nil
}

// determineRole determines user role based on group membership
func (p *Provider) determineRole(groups []string) string {
	// Check for admin role first
	for _, group := range groups {
		for _, adminGroup := range p.config.AdminGroups {
			if strings.EqualFold(group, adminGroup) {
				return "Admin"
			}
		}
	}
	
	// Check for agent role
	for _, group := range groups {
		for _, agentGroup := range p.config.AgentGroups {
			if strings.EqualFold(group, agentGroup) {
				return "Agent"
			}
		}
	}
	
	// Default to customer role
	return "Customer"
}

// TestConnection tests LDAP connection and authentication
func (p *Provider) TestConnection() error {
	err := p.Connect()
	if err != nil {
		return err
	}
	defer p.Close()
	
	// Try a simple search to verify connection works
	searchRequest := ldap.NewSearchRequest(
		p.config.BaseDN,
		ldap.ScopeBaseObject,
		ldap.NeverDerefAliases,
		1,
		5, // 5 second timeout for test
		false,
		"(objectClass=*)",
		[]string{"1.1"}, // No attributes needed
		nil,
	)
	
	_, err = p.conn.Search(searchRequest)
	if err != nil {
		return fmt.Errorf("test search failed: %w", err)
	}
	
	return nil
}

// ValidateConfig validates LDAP configuration
func ValidateConfig(config *Config) []string {
	var errors []string
	
	if config.Host == "" {
		errors = append(errors, "LDAP host is required")
	}
	
	if config.Port <= 0 || config.Port > 65535 {
		errors = append(errors, "LDAP port must be between 1 and 65535")
	}
	
	if config.BaseDN == "" {
		errors = append(errors, "Base DN is required")
	}
	
	if config.UserFilter == "" {
		errors = append(errors, "User filter is required")
	}
	
	if !strings.Contains(config.UserFilter, "%s") {
		errors = append(errors, "User filter must contain %s placeholder")
	}
	
	if config.EmailAttribute == "" {
		errors = append(errors, "Email attribute is required")
	}
	
	if config.UseSSL && config.UseTLS {
		errors = append(errors, "Cannot use both SSL and StartTLS")
	}
	
	// Validate group configuration if provided
	if config.GroupBaseDN != "" && config.GroupFilter == "" {
		errors = append(errors, "Group filter is required when group base DN is specified")
	}
	
	if config.GroupFilter != "" && !strings.Contains(config.GroupFilter, "%s") {
		errors = append(errors, "Group filter must contain %s placeholder")
	}
	
	return errors
}

// GetConfigTemplate returns a configuration template for a specific LDAP type
func GetConfigTemplate(ldapType string) (*Config, error) {
	template, exists := DefaultConfigs[ldapType]
	if !exists {
		return nil, fmt.Errorf("unknown LDAP type: %s", ldapType)
	}
	
	// Return a copy to avoid modifying the template
	config := *template
	return &config, nil
}