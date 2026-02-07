package api

import (
	"database/sql"

	"github.com/goatkit/goatflow/internal/sysconfig"
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
