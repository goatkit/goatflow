package service

import (
	"log"
	"slices"

	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/ldap"
)

// ValidateAuthProviders checks provider settings at startup. Invalid LDAP
// settings (LDAP_ENABLED=true) are returned as an error naming each bad
// variable; an LDAP_ENABLED flag that disagrees with Auth::Providers is logged.
func ValidateAuthProviders() error {
	listed := slices.Contains(getConfiguredProviderOrder(), "ldap")
	switch enabled := ldap.Enabled(); {
	case enabled && !listed:
		log.Printf("WARNING: LDAP_ENABLED=true but Auth::Providers does not list ldap; LDAP logins are off")
	case listed && !enabled:
		log.Printf("WARNING: Auth::Providers lists ldap but LDAP_ENABLED is not true; LDAP logins are off")
	}
	return auth.LDAPStartupCheck()
}
