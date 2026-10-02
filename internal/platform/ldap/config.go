// Package ldap is GoatFlow's LDAP / Active Directory directory client. It finds
// a user with a service-account (or anonymous) search and verifies the password
// by binding as that user, the same way OTRS's Kernel::System::Auth::LDAP does.
package ldap

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
)

// Config holds the LDAP settings, read from LDAP_* environment variables.
type Config struct {
	// Connection
	Host          string
	Port          int
	UseSSL        bool // ldaps://
	UseTLS        bool // StartTLS on ldap://
	SkipTLSVerify bool
	CACertFile    string
	Timeout       time.Duration

	// Service account used for the user search; empty BindDN searches anonymously.
	BindDN       string
	BindPassword string

	// User search
	BaseDN            string
	UserBaseDN        string // defaults to BaseDN
	UserFilter        string // exactly one %s, replaced by the escaped login
	IsActiveDirectory bool
	Domain            string // AD: also match userPrincipalName=<login>@<Domain>

	// Attribute mapping
	UsernameAttribute    string // value becomes the GoatFlow login; empty keeps the typed login
	EmailAttribute       string
	FirstNameAttribute   string
	LastNameAttribute    string
	DisplayNameAttribute string

	// Groups
	GroupBaseDN      string
	GroupFilter      string // exactly one %s, replaced by the escaped member value
	GroupAttribute   string
	GroupMemberValue string // "dn" or "username": what replaces %s in GroupFilter
	AdminGroups      []string
	AgentGroups      []string

	// GoatFlow agent account mapping
	AutoCreateUsers bool
	AutoUpdateUsers bool
	InitialGroups   []string
}

// Group member values accepted by LDAP_GROUP_MEMBER_VALUE.
const (
	GroupMemberDN       = "dn"
	GroupMemberUsername = "username"
)

const (
	defaultPort     = 389
	defaultSSLPort  = 636
	defaultTimeout  = 10 * time.Second
	maxTimeout      = 5 * time.Minute
	defaultGroupAtt = "cn"
)

// typeDefaults fills settings left empty for a known directory type (LDAP_TYPE).
var typeDefaults = map[string]Config{
	"active_directory": {
		UserFilter:           "(&(objectClass=user)(sAMAccountName=%s))",
		UsernameAttribute:    "sAMAccountName",
		EmailAttribute:       "mail",
		FirstNameAttribute:   "givenName",
		LastNameAttribute:    "sn",
		DisplayNameAttribute: "displayName",
		GroupFilter:          "(&(objectClass=group)(member=%s))",
		IsActiveDirectory:    true,
	},
	"openldap": {
		UserFilter:           "(&(objectClass=inetOrgPerson)(uid=%s))",
		UsernameAttribute:    "uid",
		EmailAttribute:       "mail",
		FirstNameAttribute:   "givenName",
		LastNameAttribute:    "sn",
		DisplayNameAttribute: "cn",
		GroupFilter:          "(&(objectClass=groupOfNames)(member=%s))",
	},
	"389ds": {
		UserFilter:           "(&(objectClass=inetOrgPerson)(uid=%s))",
		UsernameAttribute:    "uid",
		EmailAttribute:       "mail",
		FirstNameAttribute:   "givenName",
		LastNameAttribute:    "sn",
		DisplayNameAttribute: "cn",
		GroupFilter:          "(&(objectClass=groupOfUniqueNames)(uniqueMember=%s))",
	},
}

// Enabled reports whether LDAP_ENABLED is set to a true value.
func Enabled() bool {
	v, err := strconv.ParseBool(strings.TrimSpace(os.Getenv("LDAP_ENABLED")))
	return err == nil && v
}

// LoadFromEnvironment reads and validates the LDAP_* environment variables.
// The error lists every invalid setting by variable name.
func LoadFromEnvironment() (*Config, error) {
	return loadConfig(os.Getenv)
}

// envReader collects parse errors while reading variables.
type envReader struct {
	getenv func(string) string
	errs   []error
}

func (r *envReader) str(key string) string { return strings.TrimSpace(r.getenv(key)) }

func (r *envReader) boolean(key string) bool {
	v := r.str(key)
	if v == "" {
		return false
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		r.errs = append(r.errs, fmt.Errorf("%s=%q is not a boolean (use true or false)", key, v))
	}
	return b
}

func (r *envReader) list(key string) []string {
	var out []string
	for _, item := range strings.Split(r.getenv(key), ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func loadConfig(getenv func(string) string) (*Config, error) {
	r := &envReader{getenv: getenv}
	cfg := &Config{
		Host:                 r.str("LDAP_HOST"),
		UseSSL:               r.boolean("LDAP_USE_SSL"),
		UseTLS:               r.boolean("LDAP_USE_TLS"),
		SkipTLSVerify:        r.boolean("LDAP_SKIP_TLS_VERIFY"),
		CACertFile:           r.str("LDAP_TLS_CA_FILE"),
		BindDN:               r.str("LDAP_BIND_DN"),
		BindPassword:         getenv("LDAP_BIND_PASSWORD"),
		BaseDN:               r.str("LDAP_BASE_DN"),
		UserBaseDN:           r.str("LDAP_USER_BASE_DN"),
		UserFilter:           r.str("LDAP_USER_FILTER"),
		IsActiveDirectory:    r.boolean("LDAP_IS_ACTIVE_DIRECTORY"),
		Domain:               r.str("LDAP_DOMAIN"),
		UsernameAttribute:    r.str("LDAP_USERNAME_ATTRIBUTE"),
		EmailAttribute:       r.str("LDAP_EMAIL_ATTRIBUTE"),
		FirstNameAttribute:   r.str("LDAP_FIRST_NAME_ATTRIBUTE"),
		LastNameAttribute:    r.str("LDAP_LAST_NAME_ATTRIBUTE"),
		DisplayNameAttribute: r.str("LDAP_DISPLAY_NAME_ATTRIBUTE"),
		GroupBaseDN:          r.str("LDAP_GROUP_BASE_DN"),
		GroupFilter:          r.str("LDAP_GROUP_FILTER"),
		GroupAttribute:       r.str("LDAP_GROUP_ATTRIBUTE"),
		GroupMemberValue:     strings.ToLower(r.str("LDAP_GROUP_MEMBER_VALUE")),
		AdminGroups:          r.list("LDAP_ADMIN_GROUPS"),
		AgentGroups:          r.list("LDAP_AGENT_GROUPS"),
		AutoCreateUsers:      r.boolean("LDAP_AUTO_CREATE_USERS"),
		AutoUpdateUsers:      r.boolean("LDAP_AUTO_UPDATE_USERS"),
		InitialGroups:        r.list("LDAP_INITIAL_GROUPS"),
	}

	cfg.Port = defaultPort
	if cfg.UseSSL {
		cfg.Port = defaultSSLPort
	}
	if v := r.str("LDAP_PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil || p < 1 || p > 65535 {
			r.errs = append(r.errs, fmt.Errorf("LDAP_PORT=%q must be a port number between 1 and 65535", v))
		}
		cfg.Port = p
	}

	cfg.Timeout = defaultTimeout
	if v := r.str("LDAP_TIMEOUT"); v != "" {
		s, err := strconv.Atoi(v)
		if err != nil || s < 1 || time.Duration(s)*time.Second > maxTimeout {
			r.errs = append(r.errs, fmt.Errorf("LDAP_TIMEOUT=%q must be a whole number of seconds between 1 and %d", v, int(maxTimeout.Seconds())))
		}
		cfg.Timeout = time.Duration(s) * time.Second
	}

	if t := strings.ToLower(r.str("LDAP_TYPE")); t != "" {
		defaults, ok := typeDefaults[t]
		if !ok {
			r.errs = append(r.errs, fmt.Errorf("LDAP_TYPE=%q is unknown (use active_directory, openldap or 389ds)", t))
		} else {
			cfg.applyDefaults(defaults, getenv("LDAP_IS_ACTIVE_DIRECTORY") == "")
		}
	}
	if cfg.GroupAttribute == "" {
		cfg.GroupAttribute = defaultGroupAtt
	}
	if cfg.GroupMemberValue == "" {
		cfg.GroupMemberValue = GroupMemberDN
	}
	if len(cfg.InitialGroups) == 0 {
		cfg.InitialGroups = []string{"users"}
	}

	errs := append(r.errs, cfg.validate()...)
	if len(errs) > 0 {
		return nil, fmt.Errorf("invalid LDAP configuration: %w", errors.Join(errs...))
	}
	return cfg, nil
}

func (c *Config) applyDefaults(d Config, setAD bool) {
	fill := func(dst *string, v string) {
		if *dst == "" {
			*dst = v
		}
	}
	fill(&c.UserFilter, d.UserFilter)
	fill(&c.UsernameAttribute, d.UsernameAttribute)
	fill(&c.EmailAttribute, d.EmailAttribute)
	fill(&c.FirstNameAttribute, d.FirstNameAttribute)
	fill(&c.LastNameAttribute, d.LastNameAttribute)
	fill(&c.DisplayNameAttribute, d.DisplayNameAttribute)
	fill(&c.GroupFilter, d.GroupFilter)
	if setAD {
		c.IsActiveDirectory = d.IsActiveDirectory
	}
}

// validate checks the semantic rules; each error names the variable to fix.
func (c *Config) validate() []error {
	var errs []error
	add := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

	switch {
	case c.Host == "":
		add("LDAP_HOST is required")
	case strings.Contains(c.Host, "://") || strings.ContainsAny(c.Host, "/ "):
		add("LDAP_HOST=%q must be a bare host name (set LDAP_USE_SSL / LDAP_USE_TLS and LDAP_PORT separately)", c.Host)
	}
	if c.UseSSL && c.UseTLS {
		add("LDAP_USE_SSL (ldaps) and LDAP_USE_TLS (StartTLS) cannot both be true")
	}
	if !c.UseSSL && !c.UseTLS {
		if c.SkipTLSVerify {
			add("LDAP_SKIP_TLS_VERIFY needs LDAP_USE_SSL or LDAP_USE_TLS")
		}
		if c.CACertFile != "" {
			add("LDAP_TLS_CA_FILE needs LDAP_USE_SSL or LDAP_USE_TLS")
		}
	}

	if c.BindDN != "" && c.BindPassword == "" {
		add("LDAP_BIND_PASSWORD is required when LDAP_BIND_DN is set (an empty password would be an unauthenticated bind)")
	}
	if c.BindDN == "" && c.BindPassword != "" {
		add("LDAP_BIND_DN is required when LDAP_BIND_PASSWORD is set")
	}
	if c.BindDN != "" {
		if _, err := ldap.ParseDN(c.BindDN); err != nil {
			add("LDAP_BIND_DN=%q is not a valid DN: %v", c.BindDN, err)
		}
	}

	if c.BaseDN == "" {
		add("LDAP_BASE_DN is required")
	}
	for key, dn := range map[string]string{"LDAP_BASE_DN": c.BaseDN, "LDAP_USER_BASE_DN": c.UserBaseDN, "LDAP_GROUP_BASE_DN": c.GroupBaseDN} {
		if dn == "" {
			continue
		}
		if _, err := ldap.ParseDN(dn); err != nil {
			add("%s=%q is not a valid DN: %v", key, dn, err)
		}
	}

	if c.UserFilter == "" {
		add("LDAP_USER_FILTER is required (or set LDAP_TYPE for a default)")
	} else if err := checkFilterTemplate(c.UserFilter); err != nil {
		add("LDAP_USER_FILTER=%q: %v", c.UserFilter, err)
	}
	if c.Domain != "" && !c.IsActiveDirectory {
		add("LDAP_DOMAIN is only used with Active Directory (LDAP_IS_ACTIVE_DIRECTORY=true or LDAP_TYPE=active_directory)")
	}

	usesGroups := len(c.AdminGroups) > 0 || len(c.AgentGroups) > 0
	if usesGroups && c.GroupBaseDN == "" {
		add("LDAP_GROUP_BASE_DN is required when LDAP_ADMIN_GROUPS or LDAP_AGENT_GROUPS is set")
	}
	if c.GroupBaseDN != "" {
		if c.GroupFilter == "" {
			add("LDAP_GROUP_FILTER is required when LDAP_GROUP_BASE_DN is set (or set LDAP_TYPE for a default)")
		} else if err := checkFilterTemplate(c.GroupFilter); err != nil {
			add("LDAP_GROUP_FILTER=%q: %v", c.GroupFilter, err)
		}
	}
	if c.GroupMemberValue != GroupMemberDN && c.GroupMemberValue != GroupMemberUsername {
		add("LDAP_GROUP_MEMBER_VALUE=%q must be dn or username", c.GroupMemberValue)
	}
	return errs
}

// checkFilterTemplate requires exactly one %s placeholder and a filter that
// compiles once the placeholder is filled.
func checkFilterTemplate(f string) error {
	if strings.Count(f, "%s") != 1 {
		return errors.New("must contain exactly one %s placeholder")
	}
	if strings.Contains(strings.Replace(f, "%s", "", 1), "%") {
		return errors.New("must not contain % other than the single %s placeholder")
	}
	if _, err := ldap.CompileFilter(fillFilter(f, "x")); err != nil {
		return fmt.Errorf("is not a valid LDAP filter: %w", err)
	}
	return nil
}

// fillFilter substitutes value (already escaped) into the %s placeholder.
func fillFilter(template, escapedValue string) string {
	return strings.Replace(template, "%s", escapedValue, 1)
}
