package api

import (
	"bytes"
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
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func createTestProvider(db *sql.DB, name, providerType, clientID string) uint {
	query := database.ConvertPlaceholders(`
		INSERT INTO gk_identity_provider (name, provider_type, client_id, discovery_url, scopes,
			user_claim_email, user_claim_name, user_claim_groups,
			enabled, auto_provision, auto_add_to_group,
			create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	_, err := db.Exec(query, name, providerType, clientID, "", "openid email profile",
		"email", "name", "groups", true, false, false,
		time.Now(), 1, time.Now(), 1,
	)
	if err != nil {
		return 0
	}
	var id uint
	db.QueryRow(database.ConvertPlaceholders(`SELECT id FROM gk_identity_provider WHERE name = ?`), name).Scan(&id)
	return id
}

func cleanupProvider(db *sql.DB, id uint) {
	query := database.ConvertPlaceholders(`DELETE FROM gk_identity_provider WHERE id = ?`)
	db.Exec(query, id)
}

// setDB sets the global test DB so handlers can call database.GetDB().
func setDB(db *sql.DB) {
	database.SetDB(db)
}

// restoreDB restores the DB after SetDB was called.
func restoreDB() {
	database.ResetDB()
}
func TestHandleAdminIdentityProviders(t *testing.T) {
	setupTemplateRenderer(t)
	db := getTestDB(t)
	t.Run("GET list page returns 200 with auth", func(t *testing.T) {
		defer restoreDB()
		token := GetTestAuthToken(t)
		setDB(db)
		router := NewSimpleRouterWithDB(db)
		req := httptest.NewRequest(http.MethodGet, "/admin/identity-providers", nil)
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "List page should return 200, got %d: %s", w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), "Identity Providers")
	})
}
func TestHandleAdminIdentityProviderNew(t *testing.T) {
	setupTemplateRenderer(t)
	t.Run("GET new form returns 200 with auth", func(t *testing.T) {
		db := getTestDB(t)
		defer restoreDB()
		token := GetTestAuthToken(t)
		setDB(db)
		router := NewSimpleRouterWithDB(db)
		req := httptest.NewRequest(http.MethodGet, "/admin/identity-providers/new", nil)
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "New form should return 200, got %d", w.Code)
		assert.Contains(t, w.Body.String(), "Name")
	})
}

func TestHandleAdminIdentityProviderCreate(t *testing.T) {
	setupTemplateRenderer(t)
	t.Run("POST creates provider successfully", func(t *testing.T) {
		db := getTestDB(t)
		token := GetTestAuthToken(t)
		setDB(db)
		defer restoreDB()
		testName := fmt.Sprintf("TestOIDC_%d", time.Now().UnixNano())
		router := NewSimpleRouterWithDB(db)
		formData := url.Values{
			"name":             {testName},
			"provider_type":    {"oidc"},
			"client_id":        {"test-client-id"},
			"discovery_url":    {"https://example.com/.well-known/openid-configuration"},
			"scopes":           {"openid email profile"},
			"user_claim_email": {"email"},
			"user_claim_name":  {"name"},
			"enabled":          {"1"},
			"auto_provision":   {"0"},
		}
		req := httptest.NewRequest(http.MethodPost, "/admin/identity-providers", bytes.NewBufferString(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "Create should succeed: %s", w.Body.String())
		var resp map[string]interface{}
		json.NewDecoder(w.Body).Decode(&resp)
		assert.True(t, resp["success"] == true)
		assert.NotEmpty(t, resp["id"])
		var id, enabled, autoProvision int
		err := db.QueryRow(database.ConvertPlaceholders(`SELECT id, enabled, auto_provision FROM gk_identity_provider WHERE name = ?`), testName).Scan(&id, &enabled, &autoProvision)
		require.NoError(t, err)
		assert.Equal(t, 1, enabled)
		assert.Equal(t, 0, autoProvision)
		cleanupProvider(db, uint(id))
	})
	t.Run("POST rejects missing required fields", func(t *testing.T) {
		db := getTestDB(t)
		token := GetTestAuthToken(t)
		setDB(db)
		defer restoreDB()
		router := NewSimpleRouterWithDB(db)
		formData := url.Values{"name": {"Test Provider"}}
		req := httptest.NewRequest(http.MethodPost, "/admin/identity-providers", bytes.NewBufferString(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
	t.Run("POST rejects invalid provider type", func(t *testing.T) {
		db := getTestDB(t)
		token := GetTestAuthToken(t)
		setDB(db)
		defer restoreDB()
		router := NewSimpleRouterWithDB(db)
		formData := url.Values{"name": {"Test Provider"}, "provider_type": {"invalid_type"}, "client_id": {"client-id"}}
		req := httptest.NewRequest(http.MethodPost, "/admin/identity-providers", bytes.NewBufferString(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
	t.Run("POST creates with org_id", func(t *testing.T) {
		db := getTestDB(t)
		token := GetTestAuthToken(t)
		setDB(db)
		defer restoreDB()
		testName := fmt.Sprintf("TestOrgProvider_%d", time.Now().UnixNano())
		router := NewSimpleRouterWithDB(db)
		formData := url.Values{"name": {testName}, "provider_type": {"google"}, "client_id": {"google-client-id"}, "org_id": {"42"}}
		req := httptest.NewRequest(http.MethodPost, "/admin/identity-providers", bytes.NewBufferString(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "Create with org_id should succeed: %s", w.Body.String())
		var orgIDVal int64
		err := db.QueryRow(database.ConvertPlaceholders(`SELECT org_id FROM gk_identity_provider WHERE name = ?`), testName).Scan(&orgIDVal)
		require.NoError(t, err)
		assert.NotZero(t, orgIDVal, "org_id should be set")
		assert.Equal(t, int64(42), orgIDVal)
		var id uint
		db.QueryRow(database.ConvertPlaceholders(`SELECT id FROM gk_identity_provider WHERE name = ?`), testName).Scan(&id)
		cleanupProvider(db, id)
	})
	t.Run("POST supports toggle fields", func(t *testing.T) {
		db := getTestDB(t)
		token := GetTestAuthToken(t)
		setDB(db)
		defer restoreDB()
		testName := fmt.Sprintf("TestToggle_%d", time.Now().UnixNano())
		router := NewSimpleRouterWithDB(db)
		formData := url.Values{"name": {testName}, "provider_type": {"oidc"}, "client_id": {"saml-client"}, "enabled": {"1"}, "auto_provision": {"1"}}
		req := httptest.NewRequest(http.MethodPost, "/admin/identity-providers", bytes.NewBufferString(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "Create with toggles should succeed: %s", w.Body.String())
		var enabled, autoProvision int
		err := db.QueryRow(database.ConvertPlaceholders(`SELECT enabled, auto_provision FROM gk_identity_provider WHERE name = ?`), testName).Scan(&enabled, &autoProvision)
		require.NoError(t, err)
		assert.Equal(t, 1, enabled)
		assert.Equal(t, 1, autoProvision)
		var id uint
		db.QueryRow(database.ConvertPlaceholders(`SELECT id FROM gk_identity_provider WHERE name = ?`), testName).Scan(&id)
		cleanupProvider(db, id)
	})
}

func TestHandleAdminIdentityProviderEdit(t *testing.T) {
	setupTemplateRenderer(t)
	t.Run("GET edit form for existing provider", func(t *testing.T) {
		db := getTestDB(t)
		token := GetTestAuthToken(t)
		setDB(db)
		defer restoreDB()
		testName := fmt.Sprintf("TestEdit_%d", time.Now().UnixNano())
		providerID := createTestProvider(db, testName, "oidc", "edit-client")
		defer cleanupProvider(db, providerID)
		router := NewSimpleRouterWithDB(db)
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/identity-providers/%d/edit", providerID), nil)
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "Edit form should return 200, got %d: %s", w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), "Edit Identity Provider")
	})
	t.Run("GET edit form for non-existent provider returns 404", func(t *testing.T) {
		db := getTestDB(t)
		token := GetTestAuthToken(t)
		setDB(db)
		defer restoreDB()
		router := NewSimpleRouterWithDB(db)
		req := httptest.NewRequest(http.MethodGet, "/admin/identity-providers/99999/edit", nil)
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
	t.Run("GET edit form with invalid ID returns 400", func(t *testing.T) {
		db := getTestDB(t)
		token := GetTestAuthToken(t)
		setDB(db)
		defer restoreDB()
		router := NewSimpleRouterWithDB(db)
		req := httptest.NewRequest(http.MethodGet, "/admin/identity-providers/abc/edit", nil)
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandleAdminIdentityProviderUpdate(t *testing.T) {
	setupTemplateRenderer(t)
	t.Run("PUT updates provider successfully", func(t *testing.T) {
		db := getTestDB(t)
		token := GetTestAuthToken(t)
		setDB(db)
		defer restoreDB()
		testName := fmt.Sprintf("TestUpdate_%d", time.Now().UnixNano())
		providerID := createTestProvider(db, testName, "oidc", "old-client-id")
		defer cleanupProvider(db, providerID)
		router := NewSimpleRouterWithDB(db)
		formData := url.Values{
			"name":           {testName},
			"provider_type":  {"google"},
			"client_id":      {"updated-client-id"},
			"discovery_url":  {"https://accounts.google.com/.well-known/openid-configuration"},
			"scopes":         {"openid email"},
			"enabled":        {"0"},
			"auto_provision": {"1"},
		}
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/admin/identity-providers/%d", providerID), bytes.NewBufferString(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "Update should succeed: %s", w.Body.String())
		var name, providerType, clientID string
		var enabled, autoProvision int
		err := db.QueryRow(database.ConvertPlaceholders(`SELECT name, provider_type, client_id, enabled, auto_provision FROM gk_identity_provider WHERE id = ?`), providerID).Scan(&name, &providerType, &clientID, &enabled, &autoProvision)
		require.NoError(t, err)
		assert.Equal(t, testName, name)
		assert.Equal(t, "google", providerType)
		assert.Equal(t, "updated-client-id", clientID)
		assert.Equal(t, 0, enabled)
		assert.Equal(t, 1, autoProvision)
	})
	t.Run("PUT preserves client_secret if not provided", func(t *testing.T) {
		db := getTestDB(t)
		token := GetTestAuthToken(t)
		setDB(db)
		defer restoreDB()
		testName := fmt.Sprintf("TestPreserveSecret_%d", time.Now().UnixNano())
		providerID := createTestProvider(db, testName, "oidc", "test-client")
		db.Exec(database.ConvertPlaceholders(`UPDATE gk_identity_provider SET client_secret = ? WHERE id = ?`), "old-secret", providerID)
		defer cleanupProvider(db, providerID)
		router := NewSimpleRouterWithDB(db)
		formData := url.Values{"name": {testName}, "provider_type": {"oidc"}, "client_id": {"test-client"}}
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/admin/identity-providers/%d", providerID), bytes.NewBufferString(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		var secret string
		err := db.QueryRow(database.ConvertPlaceholders(`SELECT client_secret FROM gk_identity_provider WHERE id = ?`), providerID).Scan(&secret)
		require.NoError(t, err)
		assert.Equal(t, "old-secret", secret)
	})
	t.Run("PUT rejects invalid provider type", func(t *testing.T) {
		db := getTestDB(t)
		token := GetTestAuthToken(t)
		setDB(db)
		defer restoreDB()
		testName := fmt.Sprintf("TestBadType_%d", time.Now().UnixNano())
		providerID := createTestProvider(db, testName, "oidc", "test-client")
		defer cleanupProvider(db, providerID)
		router := NewSimpleRouterWithDB(db)
		formData := url.Values{"name": {testName}, "provider_type": {"bad_type"}, "client_id": {"test-client"}}
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/admin/identity-providers/%d", providerID), bytes.NewBufferString(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
	t.Run("PUT updates to github type", func(t *testing.T) {
		db := getTestDB(t)
		token := GetTestAuthToken(t)
		setDB(db)
		defer restoreDB()
		for _, ptype := range []string{"github", "saml2"} {
			t.Run(fmt.Sprintf("update to %s", ptype), func(t *testing.T) {
				testName := fmt.Sprintf("Test%s_%s_%d", ptype, ptype, time.Now().UnixNano())
				providerID := createTestProvider(db, testName, "oidc", "test-client")
				defer cleanupProvider(db, providerID)
				router := NewSimpleRouterWithDB(db)
				formData := url.Values{"name": {testName}, "provider_type": {ptype}, "client_id": {"test-client"}}
				if ptype == "saml2" {
					formData["discovery_url"] = []string{"https://idp.example.com/metadata"}
					formData["signing_cert"] = []string{"-----BEGIN CERTIFICATE-----\nMIIBtest\n-----END CERTIFICATE-----"}
					formData["private_key"] = []string{"-----BEGIN PRIVATE KEY-----\nMIIBtestkey\n-----END PRIVATE KEY-----"}
				}
				req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/admin/identity-providers/%d", providerID), bytes.NewBufferString(formData.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.Header.Set("Accept", "application/json")
				AddTestAuthCookie(req, token)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
				assert.Equal(t, http.StatusOK, w.Code)
			})
		}
	})
	t.Run("PUT non-existent provider returns 404", func(t *testing.T) {
		db := getTestDB(t)
		token := GetTestAuthToken(t)
		setDB(db)
		defer restoreDB()
		router := NewSimpleRouterWithDB(db)
		formData := url.Values{"name": {"Test"}, "provider_type": {"oidc"}, "client_id": {"test"}}
		req := httptest.NewRequest(http.MethodPut, "/admin/identity-providers/99999", bytes.NewBufferString(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandleAdminIdentityProviderDelete(t *testing.T) {
	t.Run("DELETE removes provider successfully", func(t *testing.T) {
		db := getTestDB(t)
		token := GetTestAuthToken(t)
		setDB(db)
		defer restoreDB()
		testName := fmt.Sprintf("TestDelete_%d", time.Now().UnixNano())
		providerID := createTestProvider(db, testName, "oidc", "delete-client")
		router := NewSimpleRouterWithDB(db)
		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/admin/identity-providers/%d", providerID), nil)
		req.Header.Set("Accept", "application/json")
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "Delete should succeed: %s", w.Body.String())
		var resp map[string]interface{}
		json.NewDecoder(w.Body).Decode(&resp)
		assert.True(t, resp["success"] == true)
		var count int
		err := db.QueryRow(database.ConvertPlaceholders(`SELECT COUNT(*) FROM gk_identity_provider WHERE id = ?`), providerID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "Provider should be deleted from database")
	})
	t.Run("DELETE non-existent provider still succeeds", func(t *testing.T) {
		db := getTestDB(t)
		token := GetTestAuthToken(t)
		setDB(db)
		defer restoreDB()
		router := NewSimpleRouterWithDB(db)
		req := httptest.NewRequest(http.MethodDelete, "/admin/identity-providers/99999", nil)
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})
}

func TestHandleAdminIdentityProviderToggle(t *testing.T) {
	t.Run("Toggle enable provider", func(t *testing.T) {
		db := getTestDB(t)
		token := GetTestAuthToken(t)
		setDB(db)
		defer restoreDB()
		testName := fmt.Sprintf("TestEnable_%d", time.Now().UnixNano())
		providerID := createTestProvider(db, testName, "oidc", "enable-client")
		defer cleanupProvider(db, providerID)
		db.Exec(database.ConvertPlaceholders(`UPDATE gk_identity_provider SET enabled = 0 WHERE id = ?`), providerID)
		router := NewSimpleRouterWithDB(db)
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/identity-providers/%d/enable", providerID), nil)
		req.Header.Set("Accept", "application/json")
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]interface{}
		json.NewDecoder(w.Body).Decode(&resp)
		assert.True(t, resp["success"] == true)
		assert.Equal(t, true, resp["enabled"])
		var enabled int
		err := db.QueryRow(database.ConvertPlaceholders(`SELECT enabled FROM gk_identity_provider WHERE id = ?`), providerID).Scan(&enabled)
		require.NoError(t, err)
		assert.Equal(t, 1, enabled)
	})
	t.Run("Toggle disable provider", func(t *testing.T) {
		db := getTestDB(t)
		token := GetTestAuthToken(t)
		setDB(db)
		defer restoreDB()
		testName := fmt.Sprintf("TestDisable_%d", time.Now().UnixNano())
		providerID := createTestProvider(db, testName, "oidc", "disable-client")
		defer cleanupProvider(db, providerID)
		router := NewSimpleRouterWithDB(db)
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/identity-providers/%d/disable", providerID), nil)
		req.Header.Set("Accept", "application/json")
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]interface{}
		json.NewDecoder(w.Body).Decode(&resp)
		assert.True(t, resp["success"] == true)
		assert.Equal(t, false, resp["enabled"])
		var enabled int
		err := db.QueryRow(database.ConvertPlaceholders(`SELECT enabled FROM gk_identity_provider WHERE id = ?`), providerID).Scan(&enabled)
		require.NoError(t, err)
		assert.Equal(t, 0, enabled)
	})
	t.Run("Toggle invalid action returns 400", func(t *testing.T) {
		db := getTestDB(t)
		token := GetTestAuthToken(t)
		setDB(db)
		defer restoreDB()
		router := NewSimpleRouterWithDB(db)
		req := httptest.NewRequest(http.MethodPost, "/admin/identity-providers/1/invalid", nil)
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
	t.Run("Toggle non-existent provider returns 404", func(t *testing.T) {
		db := getTestDB(t)
		token := GetTestAuthToken(t)
		setDB(db)
		defer restoreDB()
		router := NewSimpleRouterWithDB(db)
		req := httptest.NewRequest(http.MethodPost, "/admin/identity-providers/99999/enable", nil)
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandleAdminIdentityProviderInvalidID(t *testing.T) {
	t.Run("Edit with non-numeric ID returns 400", func(t *testing.T) {
		db := getTestDB(t)
		token := GetTestAuthToken(t)
		setDB(db)
		defer restoreDB()
		router := NewSimpleRouterWithDB(db)
		req := httptest.NewRequest(http.MethodGet, "/admin/identity-providers/abc/edit", nil)
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
	t.Run("Update with non-numeric ID returns 400", func(t *testing.T) {
		db := getTestDB(t)
		token := GetTestAuthToken(t)
		setDB(db)
		defer restoreDB()
		router := NewSimpleRouterWithDB(db)
		req := httptest.NewRequest(http.MethodPut, "/admin/identity-providers/abc", nil)
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
	t.Run("Delete with non-numeric ID returns 400", func(t *testing.T) {
		db := getTestDB(t)
		token := GetTestAuthToken(t)
		setDB(db)
		defer restoreDB()
		router := NewSimpleRouterWithDB(db)
		req := httptest.NewRequest(http.MethodDelete, "/admin/identity-providers/abc", nil)
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
	t.Run("Toggle with non-numeric ID returns 400", func(t *testing.T) {
		db := getTestDB(t)
		token := GetTestAuthToken(t)
		setDB(db)
		defer restoreDB()
		router := NewSimpleRouterWithDB(db)
		req := httptest.NewRequest(http.MethodPost, "/admin/identity-providers/abc/enable", nil)
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandleAdminIdentityProviderEmptyFields(t *testing.T) {
	db := getTestDB(t)
	token := GetTestAuthToken(t)
	setDB(db)
	defer restoreDB()
	router := NewSimpleRouterWithDB(db)
	t.Run("Empty name returns 400", func(t *testing.T) {
		formData := url.Values{"name": {"   "}, "provider_type": {"oidc"}, "client_id": {"test"}}
		req := httptest.NewRequest(http.MethodPost, "/admin/identity-providers", bytes.NewBufferString(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
	t.Run("Empty provider_type returns 400", func(t *testing.T) {
		formData := url.Values{"name": {"Test"}, "provider_type": {""}, "client_id": {"test"}}
		req := httptest.NewRequest(http.MethodPost, "/admin/identity-providers", bytes.NewBufferString(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
	t.Run("Empty client_id returns 400", func(t *testing.T) {
		formData := url.Values{"name": {"Test"}, "provider_type": {"oidc"}, "client_id": {"  "}}
		req := httptest.NewRequest(http.MethodPost, "/admin/identity-providers", bytes.NewBufferString(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestAllProviderTypes(t *testing.T) {
	db := getTestDB(t)
	token := GetTestAuthToken(t)
	setDB(db)
	defer restoreDB()
	for _, ptype := range []string{"oidc", "google", "github", "saml2"} {
		t.Run(fmt.Sprintf("create_%s", ptype), func(t *testing.T) {
			testName := fmt.Sprintf("Test%s_%d", strings.ToUpper(ptype), time.Now().UnixNano())
			router := NewSimpleRouterWithDB(db)
			formData := url.Values{"name": {testName}, "provider_type": {ptype}, "client_id": {fmt.Sprintf("%s-client", ptype)}}
			if ptype == "saml2" {
				formData["discovery_url"] = []string{"https://idp.example.com/metadata"}
				formData["signing_cert"] = []string{"-----BEGIN CERTIFICATE-----\nMIIBtest\n-----END CERTIFICATE-----"}
				formData["private_key"] = []string{"-----BEGIN PRIVATE KEY-----\nMIIBtestkey\n-----END PRIVATE KEY-----"}
			}
			req := httptest.NewRequest(http.MethodPost, "/admin/identity-providers", bytes.NewBufferString(formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Accept", "application/json")
			AddTestAuthCookie(req, token)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code, "Create %s provider should succeed: %s", ptype, w.Body.String())
			var resp map[string]interface{}
			json.NewDecoder(w.Body).Decode(&resp)
			assert.True(t, resp["success"] == true)
			if id, ok := resp["id"].(float64); ok {
				cleanupProvider(db, uint(id))
			}
		})
	}
}

func TestCreateSAMLProviderWithMetadataURL(t *testing.T) {
	setupTemplateRenderer(t)
	t.Run("POST creates saml2 provider with valid metadata URL", func(t *testing.T) {
		db := getTestDB(t)
		token := GetTestAuthToken(t)
		setDB(db)
		defer restoreDB()
		testName := fmt.Sprintf("TestSAML_%d", time.Now().UnixNano())
		router := NewSimpleRouterWithDB(db)
		formData := url.Values{
			"name":          {testName},
			"provider_type": {"saml2"},
			"client_id":     {"saml-client-id"},
			"discovery_url": {"https://idp.example.com/metadata"},
			"signing_cert":  {"-----BEGIN CERTIFICATE-----\nMIIBtest\n-----END CERTIFICATE-----"},
			"private_key":   {"-----BEGIN PRIVATE KEY-----\nMIIBtestkey\n-----END PRIVATE KEY-----"},
			"enabled":       {"1"},
		}
		req := httptest.NewRequest(http.MethodPost, "/admin/identity-providers", bytes.NewBufferString(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "Create SAML provider should succeed: %s", w.Body.String())
		var resp map[string]interface{}
		json.NewDecoder(w.Body).Decode(&resp)
		assert.True(t, resp["success"] == true)
		assert.NotEmpty(t, resp["id"])
		var id int
		err := db.QueryRow(database.ConvertPlaceholders(`SELECT id FROM gk_identity_provider WHERE name = ?`), testName).Scan(&id)
		require.NoError(t, err)
		cleanupProvider(db, uint(id))
	})
	t.Run("POST rejects saml2 provider with invalid metadata URL", func(t *testing.T) {
		db := getTestDB(t)
		token := GetTestAuthToken(t)
		setDB(db)
		defer restoreDB()
		router := NewSimpleRouterWithDB(db)
		formData := url.Values{
			"name":          {"Test SAML"},
			"provider_type": {"saml2"},
			"client_id":     {"saml-client-id"},
			"discovery_url": {"not-a-valid-url"},
			"signing_cert":  {"-----BEGIN CERTIFICATE-----\nMIIBtest\n-----END CERTIFICATE-----"},
			"private_key":   {"-----BEGIN PRIVATE KEY-----\nMIIBtestkey\n-----END PRIVATE KEY-----"},
		}
		req := httptest.NewRequest(http.MethodPost, "/admin/identity-providers", bytes.NewBufferString(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
	t.Run("POST rejects saml2 provider without metadata URL", func(t *testing.T) {
		db := getTestDB(t)
		token := GetTestAuthToken(t)
		setDB(db)
		defer restoreDB()
		router := NewSimpleRouterWithDB(db)
		formData := url.Values{
			"name":          {"Test SAML"},
			"provider_type": {"saml2"},
			"client_id":     {"saml-client-id"},
			"discovery_url": {""},
			"signing_cert":  {"-----BEGIN CERTIFICATE-----\nMIIBtest\n-----END CERTIFICATE-----"},
			"private_key":   {"-----BEGIN PRIVATE KEY-----\nMIIBtestkey\n-----END PRIVATE KEY-----"},
		}
		req := httptest.NewRequest(http.MethodPost, "/admin/identity-providers", bytes.NewBufferString(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code, "SAML without metadata URL should be rejected: %s", w.Body.String())
	})
	t.Run("POST rejects saml2 provider without signing cert", func(t *testing.T) {
		db := getTestDB(t)
		token := GetTestAuthToken(t)
		setDB(db)
		defer restoreDB()
		router := NewSimpleRouterWithDB(db)
		formData := url.Values{
			"name":          {"Test SAML"},
			"provider_type": {"saml2"},
			"client_id":     {"saml-client-id"},
			"discovery_url": {"https://idp.example.com/metadata"},
			"signing_cert":  {""},
			"private_key":   {"-----BEGIN PRIVATE KEY-----\nMIIBtestkey\n-----END PRIVATE KEY-----"},
		}
		req := httptest.NewRequest(http.MethodPost, "/admin/identity-providers", bytes.NewBufferString(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code, "SAML without signing cert should be rejected: %s", w.Body.String())
	})
	t.Run("POST rejects saml2 provider without private key", func(t *testing.T) {
		db := getTestDB(t)
		token := GetTestAuthToken(t)
		setDB(db)
		defer restoreDB()
		router := NewSimpleRouterWithDB(db)
		formData := url.Values{
			"name":          {"Test SAML"},
			"provider_type": {"saml2"},
			"client_id":     {"saml-client-id"},
			"discovery_url": {"https://idp.example.com/metadata"},
			"signing_cert":  {"-----BEGIN CERTIFICATE-----\nMIIBtest\n-----END CERTIFICATE-----"},
			"private_key":   {""},
		}
		req := httptest.NewRequest(http.MethodPost, "/admin/identity-providers", bytes.NewBufferString(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code, "SAML without private key should be rejected: %s", w.Body.String())
	})
}

func TestListProvidersIncludesNewProvider(t *testing.T) {
	setupTemplateRenderer(t)
	db := getTestDB(t)
	token := GetTestAuthToken(t)
	setDB(db)
	defer restoreDB()
	testName := fmt.Sprintf("TestList_%d", time.Now().UnixNano())
	providerID := createTestProvider(db, testName, "oidc", "list-client")
	defer cleanupProvider(db, providerID)
	router := NewSimpleRouterWithDB(db)
	req := httptest.NewRequest(http.MethodGet, "/admin/identity-providers", nil)
	AddTestAuthCookie(req, token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, testName, "List page should show the new provider")
	assert.Contains(t, body, fmt.Sprintf("%d", providerID), "List page should show the provider ID")
}
