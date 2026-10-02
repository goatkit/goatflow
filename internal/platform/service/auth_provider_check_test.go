package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderOrder_EnvOverridesConfig(t *testing.T) {
	SetConfigAdapter(testConfigAdapter(t, []string{"database"}))
	t.Cleanup(func() { SetConfigAdapter(nil) })

	t.Setenv("AUTH_PROVIDERS", "")
	assert.Equal(t, []string{"database"}, getConfiguredProviderOrder())

	t.Setenv("AUTH_PROVIDERS", " LDAP , database,, ")
	assert.Equal(t, []string{"ldap", "database"}, getConfiguredProviderOrder())
}

func TestValidateAuthProviders_LDAP(t *testing.T) {
	t.Setenv("AUTH_PROVIDERS", "ldap,database")

	t.Setenv("LDAP_ENABLED", "false")
	require.NoError(t, ValidateAuthProviders(), "disabled LDAP is not validated")

	t.Setenv("LDAP_ENABLED", "true")
	t.Setenv("LDAP_HOST", "")
	t.Setenv("LDAP_BASE_DN", "dc=example,dc=com")
	t.Setenv("LDAP_USER_FILTER", "(uid=%s)")
	t.Setenv("LDAP_BIND_DN", "cn=svc,dc=example,dc=com")
	t.Setenv("LDAP_BIND_PASSWORD", "")
	err := ValidateAuthProviders()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LDAP_HOST is required")
	assert.Contains(t, err.Error(), "LDAP_BIND_PASSWORD is required")

	t.Setenv("LDAP_HOST", "ldap.example.com")
	t.Setenv("LDAP_BIND_PASSWORD", "secret")
	require.NoError(t, ValidateAuthProviders())
}
