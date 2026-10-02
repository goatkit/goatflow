package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/sysconfig"
)

const portalSettingsPassword = "Portal-Settings-1"

// portalSettingsCustomer inserts a customer user of customerID who can sign in
// with portalSettingsPassword and returns its id.
func portalSettingsCustomer(t *testing.T, db *sql.DB, login, customerID string, validID int) int64 {
	t.Helper()
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO customer_user (login, email, customer_id, pw, first_name, last_name, valid_id,
			create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, 'Portal', 'Settings', ?, NOW(), 1, NOW(), 1) RETURNING id`),
		login, login, customerID, otrsSHA2Hash(portalSettingsPassword), validID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM customer_user WHERE id = ?`), id) //nolint:errcheck // cleanup
	})
	return id
}

func portalSettingsCompanyOverride(t *testing.T, db *sql.DB, customerID string, cfg sysconfig.CustomerPortalConfig) {
	t.Helper()
	require.NoError(t, sysconfig.SaveCustomerPortalConfigForCompany(db, customerID, cfg, 1))
	t.Cleanup(func() {
		for _, key := range []string{"enabled", "login", "title", "footer", "landing"} {
			_ = sysconfig.DeleteCustomerPortalConfigKeyForCompany(db, customerID, key) //nolint:errcheck // cleanup
		}
	})
}

func portalSettingsLogin(t *testing.T, r http.Handler, login string) (int, map[string]any) {
	t.Helper()
	body, err := json.Marshal(map[string]string{"login": login, "password": portalSettingsPassword})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/customer/login", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp), w.Body.String())
	return w.Code, resp
}

// Per-company portal settings (CustomerPortal::<Setting>::<customer_id>) apply
// to that company's signed-in customers; everybody else gets the global values.
func TestCustomerPortalAppliesCompanySettings(t *testing.T) {
	db := getTestDB(t)
	r := portalPagesRouter(t)
	valid := portalValidID(t, db, "valid")

	orig, err := sysconfig.LoadCustomerPortalConfig(db)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sysconfig.SaveCustomerPortalConfig(db, orig, 1) }) //nolint:errcheck // restore

	sfx := strconv.FormatInt(time.Now().UnixNano(), 36)
	global := sysconfig.CustomerPortalConfig{
		Enabled: true, LoginRequired: true,
		Title: "Global Portal " + sfx, FooterText: "Global Footer " + sfx, LandingPage: "/customer/tickets",
	}
	require.NoError(t, sysconfig.SaveCustomerPortalConfig(db, global, 1))

	coA, coB, coC := "psA-"+sfx, "psB-"+sfx, "psC-"+sfx
	portalInsertCompany(t, db, coA, "Alpha Settings "+sfx, "https://alpha.example.com", valid)
	portalInsertCompany(t, db, coB, "Beta Settings "+sfx, "https://beta.example.com", valid)
	portalInsertCompany(t, db, coC, "Gamma Settings "+sfx, "https://gamma.example.com", valid)
	portalSettingsCompanyOverride(t, db, coA, sysconfig.CustomerPortalConfig{
		Enabled: true, LoginRequired: true,
		Title: "Alpha Portal " + sfx, FooterText: "Alpha Footer " + sfx, LandingPage: "/customer/tickets/new",
	})
	portalSettingsCompanyOverride(t, db, coB, sysconfig.CustomerPortalConfig{
		Enabled: false, LoginRequired: true,
		Title: "Beta Portal " + sfx, FooterText: "Beta Footer " + sfx, LandingPage: global.LandingPage,
	})

	alice := "alice-" + sfx + "@alpha.example.com"
	bob := "bob-" + sfx + "@beta.example.com"
	carol := "carol-" + sfx + "@gamma.example.com"
	aliceToken := portalCustomerToken(t, portalSettingsCustomer(t, db, alice, coA, valid), alice)
	bobToken := portalCustomerToken(t, portalSettingsCustomer(t, db, bob, coB, valid), bob)
	carolToken := portalCustomerToken(t, portalSettingsCustomer(t, db, carol, coC, valid), carol)

	t.Run("pages use the customer's company title and footer", func(t *testing.T) {
		w := portalGet(t, r, "/customer/company", aliceToken)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), "Alpha Portal "+sfx+"</title>")
		assert.Contains(t, w.Body.String(), "Alpha Footer "+sfx)
		assert.NotContains(t, w.Body.String(), "Global Footer "+sfx)

		w = portalGet(t, r, "/customer/company", carolToken)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), "Global Portal "+sfx+"</title>", "a company without overrides inherits the global title")
		assert.Contains(t, w.Body.String(), "Global Footer "+sfx)
	})

	t.Run("a company with the portal switched off is refused", func(t *testing.T) {
		w := portalGet(t, r, "/customer/tickets", bobToken)
		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		assert.Contains(t, w.Body.String(), "Beta Portal "+sfx+" is currently disabled")

		w = portalGet(t, r, "/customer/tickets", carolToken)
		assert.Equal(t, http.StatusOK, w.Code, "other companies keep the portal: %s", w.Body.String())
	})

	t.Run("a company override keeps the portal open when it is off globally", func(t *testing.T) {
		off := global
		off.Enabled = false
		require.NoError(t, sysconfig.SaveCustomerPortalConfig(db, off, 1))
		t.Cleanup(func() { require.NoError(t, sysconfig.SaveCustomerPortalConfig(db, global, 1)) })

		w := portalGet(t, r, "/customer/tickets", aliceToken)
		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

		w = portalGet(t, r, "/customer/tickets", carolToken)
		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		assert.Contains(t, w.Body.String(), "Global Portal "+sfx+" is currently disabled")

		w = portalGet(t, r, "/customer/tickets", "")
		assert.Equal(t, http.StatusServiceUnavailable, w.Code, "visitors without a session get the global setting")
	})

	t.Run("sign-in sends the customer to their company's landing page", func(t *testing.T) {
		code, resp := portalSettingsLogin(t, r, alice)
		require.Equal(t, http.StatusOK, code, resp)
		assert.Equal(t, "/customer/tickets/new", resp["redirect"])

		code, resp = portalSettingsLogin(t, r, carol)
		require.Equal(t, http.StatusOK, code, resp)
		assert.Equal(t, "/customer/tickets", resp["redirect"], "a company without overrides uses the global landing page")
	})

	t.Run("a signed-in customer opening the login page goes to the landing page", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/customer/login", nil)
		req.Header.Set("Accept", "text/html")
		req.AddCookie(&http.Cookie{Name: "customer_access_token", Value: aliceToken})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusFound, w.Code)
		assert.Equal(t, "/customer/tickets/new", w.Header().Get("Location"))
	})

	t.Run("site root on a customer-only instance goes to the global landing page", func(t *testing.T) {
		t.Setenv("CUSTOMER_FE_ONLY", "true")
		t.Setenv("ROOT_REDIRECT_PATH", "")
		landing := global
		landing.LandingPage = "/customer/company"
		require.NoError(t, sysconfig.SaveCustomerPortalConfig(db, landing, 1))
		t.Cleanup(func() { require.NoError(t, sysconfig.SaveCustomerPortalConfig(db, global, 1)) })

		assert.Equal(t, "/customer/company", RootRedirectTarget())
	})
}
