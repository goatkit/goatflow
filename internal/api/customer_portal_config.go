package api

import (
	"database/sql"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/sysconfig"
)

// Alias helpers to shared sysconfig implementations to avoid duplicate logic.
type customerPortalConfig = sysconfig.CustomerPortalConfig

func loadCustomerPortalConfig(db *sql.DB) customerPortalConfig {
	if cfg, err := sysconfig.LoadCustomerPortalConfig(db); err == nil {
		return cfg
	}
	return sysconfig.DefaultCustomerPortalConfig()
}

func saveCustomerPortalConfig(db *sql.DB, cfg customerPortalConfig, userID int) error {
	return sysconfig.SaveCustomerPortalConfig(db, cfg, userID)
}

func loadCustomerPortalConfigForCustomer(db *sql.DB, customerID string) customerPortalConfig {
	if cfg, err := sysconfig.LoadCustomerPortalConfigForCompany(db, customerID); err == nil {
		return cfg
	}
	if cfg, err := sysconfig.LoadCustomerPortalConfig(db); err == nil {
		return cfg
	}
	return sysconfig.DefaultCustomerPortalConfig()
}

func saveCustomerPortalConfigForCustomer(db *sql.DB, customerID string, cfg customerPortalConfig, userID int) error {
	return sysconfig.SaveCustomerPortalConfigForCompany(db, customerID, cfg, userID)
}

// loadCustomerPortalConfigForLogin returns the settings that apply to a
// signed-in customer: their company's overrides on top of the global values.
// An empty or unknown login gets the global values.
func loadCustomerPortalConfigForLogin(db *sql.DB, login string) customerPortalConfig {
	if cfg, err := sysconfig.LoadCustomerPortalConfigForCustomerUser(db, login); err == nil {
		return cfg
	}
	return loadCustomerPortalConfig(db)
}

// customerLandingRedirect returns where a customer is sent when entering the
// portal (after signing in): the captive plugin's landing page when their
// organisation is captive to one, otherwise their company's portal landing page.
func customerLandingRedirect(login string) string {
	if target := resolveCustomerCaptiveRedirect(login); target != "" {
		return target
	}
	db, err := database.GetDB()
	if err != nil {
		db = nil
	}
	return loadCustomerPortalConfigForLogin(db, login).LandingPath()
}
