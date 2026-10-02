package ldap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func baseEnv() map[string]string {
	return map[string]string{
		"LDAP_HOST":          "ldap.example.com",
		"LDAP_BASE_DN":       "dc=example,dc=com",
		"LDAP_TYPE":          "openldap",
		"LDAP_BIND_DN":       "cn=svc,dc=example,dc=com",
		"LDAP_BIND_PASSWORD": "secret",
	}
}

func TestLoadConfig_DefaultsFromType(t *testing.T) {
	cfg, err := loadConfig(envOf(baseEnv()))
	require.NoError(t, err)
	assert.Equal(t, 389, cfg.Port)
	assert.Equal(t, 10*time.Second, cfg.Timeout)
	assert.Equal(t, "(&(objectClass=inetOrgPerson)(uid=%s))", cfg.UserFilter)
	assert.Equal(t, "uid", cfg.UsernameAttribute)
	assert.Equal(t, "cn", cfg.GroupAttribute)
	assert.Equal(t, GroupMemberDN, cfg.GroupMemberValue)
	assert.Equal(t, []string{"users"}, cfg.InitialGroups)
	assert.False(t, cfg.UseTLS)
	assert.False(t, cfg.SkipTLSVerify)
	assert.False(t, cfg.AutoCreateUsers)

	env := baseEnv()
	env["LDAP_TYPE"] = "active_directory"
	env["LDAP_USE_SSL"] = "true"
	env["LDAP_USER_FILTER"] = "(sAMAccountName=%s)"
	cfg, err = loadConfig(envOf(env))
	require.NoError(t, err)
	assert.Equal(t, 636, cfg.Port, "ldaps defaults to 636")
	assert.True(t, cfg.IsActiveDirectory)
	assert.Equal(t, "(sAMAccountName=%s)", cfg.UserFilter, "explicit filter wins over the type default")
}

func TestLoadConfig_RejectsInvalidSettings(t *testing.T) {
	cases := []struct {
		name string
		set  map[string]string
		want string
	}{
		{"missing host", map[string]string{"LDAP_HOST": ""}, "LDAP_HOST is required"},
		{"host with scheme", map[string]string{"LDAP_HOST": "ldaps://dc.example.com"}, "LDAP_HOST"},
		{"missing base dn", map[string]string{"LDAP_BASE_DN": ""}, "LDAP_BASE_DN is required"},
		{"bad base dn", map[string]string{"LDAP_BASE_DN": "not a dn"}, "LDAP_BASE_DN"},
		{"bad port", map[string]string{"LDAP_PORT": "70000"}, "LDAP_PORT"},
		{"bad timeout", map[string]string{"LDAP_TIMEOUT": "0"}, "LDAP_TIMEOUT"},
		{"bad bool", map[string]string{"LDAP_USE_TLS": "yes please"}, "LDAP_USE_TLS"},
		{"unknown type", map[string]string{"LDAP_TYPE": "novell"}, "LDAP_TYPE"},
		{"ssl and starttls", map[string]string{"LDAP_USE_SSL": "true", "LDAP_USE_TLS": "true"}, "cannot both be true"},
		{"skip verify without tls", map[string]string{"LDAP_SKIP_TLS_VERIFY": "true"}, "LDAP_SKIP_TLS_VERIFY"},
		{"ca file without tls", map[string]string{"LDAP_TLS_CA_FILE": "/etc/ca.pem"}, "LDAP_TLS_CA_FILE"},
		{"bind dn without password", map[string]string{"LDAP_BIND_PASSWORD": ""}, "unauthenticated bind"},
		{"bind password without dn", map[string]string{"LDAP_BIND_DN": ""}, "LDAP_BIND_DN is required"},
		{"filter without placeholder", map[string]string{"LDAP_USER_FILTER": "(uid=admin)"}, "exactly one %s"},
		{"filter with two placeholders", map[string]string{"LDAP_USER_FILTER": "(|(uid=%s)(mail=%s))"}, "exactly one %s"},
		{"filter with other verb", map[string]string{"LDAP_USER_FILTER": "(uid=%s%d)"}, "must not contain %"},
		{"unbalanced filter", map[string]string{"LDAP_USER_FILTER": "(uid=%s"}, "not a valid LDAP filter"},
		{"agent groups without group base", map[string]string{"LDAP_AGENT_GROUPS": "helpdesk"}, "LDAP_GROUP_BASE_DN is required"},
		{"bad group filter", map[string]string{"LDAP_GROUP_BASE_DN": "ou=groups,dc=example,dc=com", "LDAP_GROUP_FILTER": "(member=x)"}, "LDAP_GROUP_FILTER"},
		{"bad member value", map[string]string{"LDAP_GROUP_MEMBER_VALUE": "uid"}, "LDAP_GROUP_MEMBER_VALUE"},
		{"domain without AD", map[string]string{"LDAP_DOMAIN": "example.com"}, "LDAP_DOMAIN"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := baseEnv()
			for k, v := range tc.set {
				env[k] = v
			}
			_, err := loadConfig(envOf(env))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestLoadConfig_ReportsEveryProblem(t *testing.T) {
	_, err := loadConfig(envOf(map[string]string{"LDAP_PORT": "abc"}))
	require.Error(t, err)
	for _, key := range []string{"LDAP_HOST", "LDAP_BASE_DN", "LDAP_USER_FILTER", "LDAP_PORT"} {
		assert.Contains(t, err.Error(), key)
	}
}

func TestNewClient_CAFile(t *testing.T) {
	env := baseEnv()
	env["LDAP_USE_TLS"] = "true"
	env["LDAP_TLS_CA_FILE"] = filepath.Join(t.TempDir(), "missing.pem")
	cfg, err := loadConfig(envOf(env))
	require.NoError(t, err)
	_, err = NewClient(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LDAP_TLS_CA_FILE")

	notPEM := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(notPEM, []byte("not a certificate"), 0o600))
	cfg.CACertFile = notPEM
	_, err = NewClient(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no PEM certificates")

	cfg.CACertFile = ""
	c, err := NewClient(cfg)
	require.NoError(t, err)
	assert.False(t, c.tlsConfig.InsecureSkipVerify, "certificate verification is on unless LDAP_SKIP_TLS_VERIFY=true")
	assert.Equal(t, "ldap.example.com", c.tlsConfig.ServerName)
}

func TestUserFilter_EscapesInjection(t *testing.T) {
	cfg, err := loadConfig(envOf(baseEnv()))
	require.NoError(t, err)
	c, err := NewClient(cfg)
	require.NoError(t, err)

	cases := map[string]string{
		"jdoe":         `(&(objectClass=inetOrgPerson)(uid=jdoe))`,
		"*":            `(&(objectClass=inetOrgPerson)(uid=\2a))`,
		"*)(uid=*":     `(&(objectClass=inetOrgPerson)(uid=\2a\29\28uid=\2a))`,
		"admin)(|(a=*": `(&(objectClass=inetOrgPerson)(uid=admin\29\28|\28a=\2a))`,
		`back\slash`:   `(&(objectClass=inetOrgPerson)(uid=back\5cslash))`,
		"nul\x00byte":  `(&(objectClass=inetOrgPerson)(uid=nul\00byte))`,
		"jürgen":       `(&(objectClass=inetOrgPerson)(uid=j\c3\bcrgen))`,
		"%s":           `(&(objectClass=inetOrgPerson)(uid=%s))`,
	}
	for login, want := range cases {
		got := c.UserFilter(login)
		assert.Equal(t, want, got, "login %q", login)
		assert.NoError(t, checkCompiles(got), "filter for %q must stay a single valid filter", login)
	}
}

func TestUserFilter_ActiveDirectoryUPN(t *testing.T) {
	env := baseEnv()
	env["LDAP_TYPE"] = "active_directory"
	env["LDAP_DOMAIN"] = "corp.example.com"
	cfg, err := loadConfig(envOf(env))
	require.NoError(t, err)
	c, err := NewClient(cfg)
	require.NoError(t, err)

	assert.Equal(t,
		`(|(&(objectClass=user)(sAMAccountName=jdoe))(userPrincipalName=jdoe@corp.example.com))`,
		c.UserFilter("jdoe"))
	assert.Equal(t,
		`(|(&(objectClass=user)(sAMAccountName=jdoe@other.com))(userPrincipalName=jdoe@other.com))`,
		c.UserFilter("jdoe@other.com"))
	assert.Equal(t,
		`(|(&(objectClass=user)(sAMAccountName=\2a))(userPrincipalName=\2a@corp.example.com))`,
		c.UserFilter("*"))
}

func TestGroupFilter_EscapesMemberValue(t *testing.T) {
	env := baseEnv()
	env["LDAP_GROUP_BASE_DN"] = "ou=groups,dc=example,dc=com"
	cfg, err := loadConfig(envOf(env))
	require.NoError(t, err)
	c, err := NewClient(cfg)
	require.NoError(t, err)

	u := &User{DN: `cn=Smith\, John (IT),ou=people,dc=example,dc=com`, Username: "j*"}
	assert.Equal(t, `(&(objectClass=groupOfNames)(member=cn=Smith\5c, John \28IT\29,ou=people,dc=example,dc=com))`, c.GroupFilter(u))

	cfg.GroupMemberValue = GroupMemberUsername
	cfg.GroupFilter = "(&(objectClass=posixGroup)(memberUid=%s))"
	assert.Equal(t, `(&(objectClass=posixGroup)(memberUid=j\2a))`, c.GroupFilter(u))
}

func TestAuthenticate_EmptyPasswordNeverContactsServer(t *testing.T) {
	env := baseEnv()
	env["LDAP_HOST"] = "ldap.invalid" // any network use would fail with a DNS error, not ErrInvalidCredentials
	cfg, err := loadConfig(envOf(env))
	require.NoError(t, err)
	c, err := NewClient(cfg)
	require.NoError(t, err)

	for _, tc := range []struct{ login, password string }{{"jdoe", ""}, {"", "pw"}, {"   ", "pw"}} {
		_, err := c.Authenticate(t.Context(), tc.login, tc.password)
		assert.ErrorIs(t, err, ErrInvalidCredentials, "login %q password %q", tc.login, tc.password)
	}

	_, err = c.Authenticate(t.Context(), "jdoe", "pw")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrInvalidCredentials)
	assert.True(t, strings.Contains(err.Error(), "connect"), "non-empty credentials reach the network: %v", err)
}

func checkCompiles(filter string) error {
	_, err := ldap.CompileFilter(filter)
	return err
}
