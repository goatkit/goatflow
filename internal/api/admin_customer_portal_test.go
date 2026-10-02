package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/sysconfig"
)

// Note: Uses centralized GetTestAuthToken() and AddTestAuthCookie() from test_helpers.go

func TestAdminCustomerPortalSettingsUpdate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token := GetTestAuthToken(t)

	db := getTestDB(t)
	// Note: Do not close singleton DB connection

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

// Unticking a per-field override must delete the company row so the field
// inherits the global value again.
func TestAdminCustomerCompanyPortalSettingsClearsUntickedOverride(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token := GetTestAuthToken(t)
	db := getTestDB(t)

	origCfg, err := sysconfig.LoadCustomerPortalConfig(db)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sysconfig.SaveCustomerPortalConfig(db, origCfg, 1) })

	customerID := fmt.Sprintf("TESTPORTALCLR%d", time.Now().UnixNano()%1e9)
	clearCompanyPortalEntries(t, db, customerID)
	t.Cleanup(func() { clearCompanyPortalEntries(t, db, customerID) })

	globalCfg := origCfg
	globalCfg.Title = "Global Title " + customerID
	require.NoError(t, sysconfig.SaveCustomerPortalConfig(db, globalCfg, 1))

	require.NoError(t, sysconfig.SaveCustomerPortalConfigForCompany(db, customerID, sysconfig.CustomerPortalConfig{
		Enabled:     true,
		Title:       "Company Title " + customerID,
		FooterText:  "Company Footer " + customerID,
		LandingPage: "/customer/tickets",
	}, 1))
	require.True(t, sysconfig.CustomerPortalOverrides(db, customerID)["title"])

	form := url.Values{}
	form.Set("override_footer_text", "1")
	form.Set("footer_text", "Kept Footer "+customerID)
	req := httptest.NewRequest(http.MethodPost, "/admin/customer/companies/"+customerID+"/portal-settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	AddTestAuthCookie(req, token)
	w := httptest.NewRecorder()
	NewSimpleRouterWithDB(db).ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	overrides := sysconfig.CustomerPortalOverrides(db, customerID)
	assert.False(t, overrides["title"], "unticked title override must be removed")
	assert.True(t, overrides["footer"])

	var rows int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		`SELECT COUNT(*) FROM sysconfig_modified WHERE name = ?`), "CustomerPortal::Title::"+customerID).Scan(&rows))
	assert.Equal(t, 0, rows)

	cfg, err := sysconfig.LoadCustomerPortalConfigForCompany(db, customerID)
	require.NoError(t, err)
	assert.Equal(t, globalCfg.Title, cfg.Title)
	assert.Equal(t, "Kept Footer "+customerID, cfg.FooterText)
}

// The customer portal logo upload endpoint was a fake (it saved nothing and
// answered success with an invented URL) and had no UI caller: it is not served.
func TestAdminCustomerPortalLogoUploadNotServed(t *testing.T) {
	router := newProductionRouter(t)
	for _, rt := range allRoutes(router) {
		assert.NotEqual(t, "/admin/customer/portal/logo/upload", rt.Path, "%s %s is still registered", rt.Method, rt.Path)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/customer/portal/logo/upload", strings.NewReader(""))
	AddTestAuthCookie(req, GetTestAuthToken(t))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

// postCompanyPortalForm posts the per-company portal form the way the browser
// does (repeated keys: hidden "0" then the ticked checkbox "1").
func postCompanyPortalForm(t *testing.T, db *sql.DB, customerID string, form url.Values, accept string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/admin/customer/companies/"+customerID+"/portal-settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", accept)
	AddTestAuthCookie(req, GetTestAuthToken(t))
	w := httptest.NewRecorder()
	NewSimpleRouterWithDB(db).ServeHTTP(w, req)
	return w
}

// The company edit form always posts every override_* flag as a hidden "0"
// followed by the checkbox "1" when ticked; the ticked state must win, and an
// all-unticked form must not create overrides from the always-posted hidden
// enabled/login_required inputs.
func TestAdminCustomerCompanyPortalSettingsFormOverrideFlags(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := getTestDB(t)

	origCfg, err := sysconfig.LoadCustomerPortalConfig(db)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sysconfig.SaveCustomerPortalConfig(db, origCfg, 1) })
	globalCfg := origCfg
	globalCfg.Enabled = true
	globalCfg.LoginRequired = true
	globalCfg.Title = "Global Title"
	require.NoError(t, sysconfig.SaveCustomerPortalConfig(db, globalCfg, 1))

	customerID := fmt.Sprintf("TESTPORTALFORM%d", time.Now().UnixNano()%1e9)
	clearCompanyPortalEntries(t, db, customerID)
	t.Cleanup(func() { clearCompanyPortalEntries(t, db, customerID) })

	t.Run("ticked overrides are saved", func(t *testing.T) {
		form := url.Values{
			"override_enabled":        {"0", "1"},
			"enabled":                 {"0"},
			"override_login_required": {"0"},
			"login_required":          {"0"},
			"override_title":          {"0", "1"},
			"title":                   {"Company Title " + customerID},
			"override_footer_text":    {"0"},
			"override_landing_page":   {"0"},
		}
		w := postCompanyPortalForm(t, db, customerID, form, "application/json")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		overrides := sysconfig.CustomerPortalOverrides(db, customerID)
		assert.True(t, overrides["enabled"], "ticked enabled override must be stored")
		assert.True(t, overrides["title"], "ticked title override must be stored")
		assert.False(t, overrides["login"])
		assert.False(t, overrides["footer"])
		assert.False(t, overrides["landing"])

		cfg, err := sysconfig.LoadCustomerPortalConfigForCompany(db, customerID)
		require.NoError(t, err)
		assert.False(t, cfg.Enabled, "company portal disabled by its override")
		assert.True(t, cfg.LoginRequired, "login_required inherits the global default")
		assert.Equal(t, "Company Title "+customerID, cfg.Title)
	})

	t.Run("nothing ticked inherits every default", func(t *testing.T) {
		form := url.Values{
			"override_enabled":        {"0"},
			"enabled":                 {"0"},
			"override_login_required": {"0"},
			"login_required":          {"0"},
			"override_title":          {"0"},
			"override_footer_text":    {"0"},
			"override_landing_page":   {"0"},
		}
		w := postCompanyPortalForm(t, db, customerID, form, "application/json")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		var rows int
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
			`SELECT COUNT(*) FROM sysconfig_modified WHERE name LIKE ?`), "CustomerPortal::%::"+customerID).Scan(&rows))
		assert.Equal(t, 0, rows, "no per-company override rows may remain")

		cfg, err := sysconfig.LoadCustomerPortalConfigForCompany(db, customerID)
		require.NoError(t, err)
		assert.True(t, cfg.Enabled)
		assert.True(t, cfg.LoginRequired)
		assert.Equal(t, "Global Title", cfg.Title)
	})

	t.Run("browser form post returns to the portal tab", func(t *testing.T) {
		form := url.Values{"override_title": {"0", "1"}, "title": {"Tab Title"}}
		w := postCompanyPortalForm(t, db, customerID, form, "text/html,application/xhtml+xml")
		require.Equal(t, http.StatusSeeOther, w.Code, w.Body.String())
		assert.Equal(t, "/admin/customer/companies/"+customerID+"/edit?tab=portal&success=1", w.Header().Get("Location"))
		assert.True(t, sysconfig.CustomerPortalOverrides(db, customerID)["title"])
	})
}

func clearCompanyPortalEntries(t *testing.T, db *sql.DB, customerID string) {
	t.Helper()
	pattern := "CustomerPortal::%::" + customerID
	_, err := db.Exec(database.ConvertPlaceholders(`DELETE FROM sysconfig_modified WHERE name LIKE ?`), pattern)
	require.NoError(t, err)
	_, err = db.Exec(database.ConvertPlaceholders(`DELETE FROM sysconfig_default WHERE name LIKE ?`), pattern)
	require.NoError(t, err)
}
