package api

import (
	"os"
	"strings"

	"github.com/goatkit/goatflow/internal/platform/config"
	"github.com/goatkit/goatflow/internal/platform/database"
)

// resolveRootRedirect returns where "/" redirects: ROOT_REDIRECT_PATH when set,
// otherwise the customer portal landing page on a customer-only instance
// (CUSTOMER_FE_ONLY) and /login everywhere else. "/" carries no customer
// session, so the global CustomerPortal::LandingPage applies.
func resolveRootRedirect() string {
	v := strings.TrimSpace(os.Getenv("ROOT_REDIRECT_PATH"))
	if v == "" {
		if config.CustomerFEOnly() {
			db, err := database.GetDB()
			if err != nil {
				db = nil
			}
			return loadCustomerPortalConfig(db).LandingPath()
		}
		return "/login"
	}
	if !strings.HasPrefix(v, "/") {
		return "/" + v
	}
	return v
}

func RootRedirectTarget() string {
	return resolveRootRedirect()
}
