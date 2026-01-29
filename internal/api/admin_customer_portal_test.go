package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/sysconfig"
)

// Note: Uses centralized GetTestAuthToken() and AddTestAuthCookie() from test_helpers.go

func TestAdminCustomerPortalSettingsUpdate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token := GetTestAuthToken(t)

	db := getTestDB(t)
	// Note: Do not close singleton DB connection

	if !portalSysconfigAvailable(t, db) {
		t.Skip("sysconfig tables not present in test database")
	}

	origCfg, err := sysconfig.LoadCustomerPortalConfig(db)
	require.NoError(t, err)
	defer sysconfig.SaveCustomerPortalConfig(db, origCfg, 1)

	router := NewSimpleRouterWithDB(db)

	form := url.Values{}
	form.Set("enabled", "1")
	form.Set("login_required", "1")
	title := "Portal Title " + time.Now().Format("150405")
	form.Set("title", title)
	form.Set("footer_text", "Footer for test portal")
	form.Set("landing_page", "/customer/tickets/new")

	req := httptest.NewRequest(http.MethodPost, "/admin/customer/portal/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	AddTestAuthCookie(req, token)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["success"])
	assert.Equal(t, "/admin/customer/portal/settings", resp["redirect"])

	updated, err := sysconfig.LoadCustomerPortalConfig(db)
	require.NoError(t, err)
	assert.True(t, updated.Enabled)
	assert.True(t, updated.LoginRequired)
	assert.Equal(t, title, updated.Title)
	assert.Equal(t, "Footer for test portal", updated.FooterText)
	assert.Equal(t, "/customer/tickets/new", updated.LandingPage)
}

func TestAdminCustomerCompanyPortalSettingsUpdate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token := GetTestAuthToken(t)

	db := getTestDB(t)
	// Note: Do not close singleton DB connection

	if !portalSysconfigAvailable(t, db) {
		t.Skip("sysconfig tables not present in test database")
	}

	origCfg, err := sysconfig.LoadCustomerPortalConfig(db)
	require.NoError(t, err)
	defer sysconfig.SaveCustomerPortalConfig(db, origCfg, 1)

	customerID := "TESTPORTAL" + time.Now().Format("150405")
	clearCompanyPortalEntries(t, db, customerID)

	router := NewSimpleRouterWithDB(db)

	form := url.Values{}
	form.Set("enabled", "0")
	form.Set("login_required", "0")
	title := "Company Portal " + customerID
	form.Set("title", title)
	footer := "Footer " + customerID
	form.Set("footer_text", footer)
	landing := "/customer/tickets/" + strings.ToLower(customerID)
	form.Set("landing_page", landing)

	req := httptest.NewRequest(http.MethodPost, "/admin/customer/companies/"+customerID+"/portal-settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	AddTestAuthCookie(req, token)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["success"])

	updated, err := sysconfig.LoadCustomerPortalConfigForCompany(db, customerID)
	require.NoError(t, err)
	assert.False(t, updated.Enabled)
	assert.False(t, updated.LoginRequired)
	assert.Equal(t, title, updated.Title)
	assert.Equal(t, footer, updated.FooterText)
	assert.Equal(t, landing, updated.LandingPage)

	clearCompanyPortalEntries(t, db, customerID)
}

func TestAdminCustomerCompanyPortalSettingsDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := getTestDB(t)
	// Note: Do not close singleton DB connection

	if !portalSysconfigAvailable(t, db) {
		t.Skip("sysconfig tables not present in test database")
	}

	origCfg, err := sysconfig.LoadCustomerPortalConfig(db)
	require.NoError(t, err)
	defer sysconfig.SaveCustomerPortalConfig(db, origCfg, 1)

	customerID := "TESTPORTALDEL" + time.Now().Format("150405")
	clearCompanyPortalEntries(t, db, customerID)

	globalCfg := origCfg
	globalCfg.Title = "Global Portal " + customerID
	globalCfg.FooterText = "Global Footer " + customerID
	globalCfg.LandingPage = "/customer/tickets"
	require.NoError(t, sysconfig.SaveCustomerPortalConfig(db, globalCfg, 1))

	companyCfg := sysconfig.CustomerPortalConfig{
		Enabled:       false,
		LoginRequired: false,
		Title:         "Company Title " + customerID,
		FooterText:    "Company Footer " + customerID,
		LandingPage:   "/customer/custom-" + strings.ToLower(customerID),
	}
	require.NoError(t, sysconfig.SaveCustomerPortalConfigForCompany(db, customerID, companyCfg, 1))

	overridden, err := sysconfig.LoadCustomerPortalConfigForCompany(db, customerID)
	require.NoError(t, err)
	assert.Equal(t, companyCfg.Title, overridden.Title)
	assert.Equal(t, companyCfg.FooterText, overridden.FooterText)

	clearCompanyPortalEntries(t, db, customerID)

	after, err := sysconfig.LoadCustomerPortalConfigForCompany(db, customerID)
	require.NoError(t, err)
	assert.Equal(t, globalCfg.Title, after.Title)
	assert.Equal(t, globalCfg.FooterText, after.FooterText)
	assert.Equal(t, globalCfg.LandingPage, after.LandingPage)
	assert.Equal(t, globalCfg.Enabled, after.Enabled)
	assert.Equal(t, globalCfg.LoginRequired, after.LoginRequired)
}

func portalSysconfigAvailable(t *testing.T, db *sql.DB) bool {
	query := `SELECT COUNT(*) FROM information_schema.tables WHERE table_name IN ('sysconfig_default','sysconfig_modified')`
	if database.IsMySQL() {
		query = `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name IN ('sysconfig_default','sysconfig_modified')`
	}

	var count int
	err := db.QueryRow(query).Scan(&count)
	require.NoError(t, err)
	return count >= 2
}

func clearCompanyPortalEntries(t *testing.T, db *sql.DB, customerID string) {
	t.Helper()
	pattern := "CustomerPortal::%::" + customerID
	_, err := db.Exec(database.ConvertPlaceholders(`DELETE FROM sysconfig_modified WHERE name LIKE ?`), pattern)
	require.NoError(t, err)
	_, err = db.Exec(database.ConvertPlaceholders(`DELETE FROM sysconfig_default WHERE name LIKE ?`), pattern)
	require.NoError(t, err)
}
