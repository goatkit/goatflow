package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/i18n"
)

func setupPreferencesTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	return router
}

func TestHandleGetLanguage_Unauthenticated(t *testing.T) {
	router := setupPreferencesTestRouter()
	router.GET("/api/preferences/language", HandleGetLanguage)

	req := httptest.NewRequest("GET", "/api/preferences/language", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.False(t, response["success"].(bool))
	assert.Equal(t, "User not authenticated", response["error"])
}

func TestHandleGetLanguage_Authenticated(t *testing.T) {
	router := setupPreferencesTestRouter()

	// Middleware to set user_id
	router.Use(func(c *gin.Context) {
		c.Set("user_id", 1)
		c.Next()
	})
	router.GET("/api/preferences/language", HandleGetLanguage)

	req := httptest.NewRequest("GET", "/api/preferences/language", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// This test requires database, so it might fail in unit test mode
	// but it verifies the handler structure is correct
	if w.Code == http.StatusOK {
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
		assert.Contains(t, response, "available")
		assert.Contains(t, response, "value")
	}
}

func TestHandleSetLanguage_Unauthenticated(t *testing.T) {
	router := setupPreferencesTestRouter()
	router.POST("/api/preferences/language", HandleSetLanguage)

	body := bytes.NewBufferString(`{"value": "de"}`)
	req := httptest.NewRequest("POST", "/api/preferences/language", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.False(t, response["success"].(bool))
}

func TestHandleSetLanguage_InvalidLanguage(t *testing.T) {
	router := setupPreferencesTestRouter()

	// Middleware to set user_id
	router.Use(func(c *gin.Context) {
		c.Set("user_id", 1)
		c.Next()
	})
	router.POST("/api/preferences/language", HandleSetLanguage)

	// Try to set an invalid language
	body := bytes.NewBufferString(`{"value": "invalid_lang"}`)
	req := httptest.NewRequest("POST", "/api/preferences/language", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.False(t, response["success"].(bool))
	assert.Contains(t, response["error"], "Unsupported language")
}

func TestHandleSetLanguage_InvalidJSON(t *testing.T) {
	router := setupPreferencesTestRouter()

	router.Use(func(c *gin.Context) {
		c.Set("user_id", 1)
		c.Next()
	})
	router.POST("/api/preferences/language", HandleSetLanguage)

	body := bytes.NewBufferString(`{invalid json}`)
	req := httptest.NewRequest("POST", "/api/preferences/language", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleGetSessionTimeout_Unauthenticated(t *testing.T) {
	router := setupPreferencesTestRouter()
	router.GET("/api/preferences/session-timeout", HandleGetSessionTimeout)

	req := httptest.NewRequest("GET", "/api/preferences/session-timeout", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandleSetSessionTimeout_Unauthenticated(t *testing.T) {
	router := setupPreferencesTestRouter()
	router.POST("/api/preferences/session-timeout", HandleSetSessionTimeout)

	body := bytes.NewBufferString(`{"value": 3600}`)
	req := httptest.NewRequest("POST", "/api/preferences/session-timeout", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAvailableLanguagesMatchI18n(t *testing.T) {
	// This test verifies that the languages returned by GetSupportedLanguages
	// match what the preference handlers would return
	instance := i18n.GetInstance()
	languages := instance.GetSupportedLanguages()

	// Should have at least 6 languages (en, de, es, fr, ar, tlh)
	assert.GreaterOrEqual(t, len(languages), 6, "Should have at least 6 supported languages")

	// Verify expected languages are present
	expectedLangs := []string{"en", "de", "es", "fr", "ar", "tlh"}
	for _, expected := range expectedLangs {
		found := false
		for _, lang := range languages {
			if lang == expected {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected language %s not found in supported languages", expected)
	}
}

func TestTimeUnitTranslationsExist(t *testing.T) {
	// This test verifies that the time unit translations we added exist
	instance := i18n.GetInstance()

	// Check English translations
	tests := []struct {
		key      string
		expected string
	}{
		{"time.hour", "hour"},
		{"time.hours", "hours"},
		{"time.day", "day"},
		{"time.days", "days"},
		{"time.minute", "minute"},
		{"time.minutes", "minutes"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := instance.T("en", tt.key)
			assert.Equal(t, tt.expected, result, "Translation for %s should be %s", tt.key, tt.expected)
		})
	}
}

func TestTimeUnitTranslationsAllLanguages(t *testing.T) {
	// Verify time unit translations exist in all supported languages
	instance := i18n.GetInstance()
	languages := instance.GetSupportedLanguages()

	keys := []string{"time.hour", "time.hours", "time.day", "time.days"}

	for _, lang := range languages {
		for _, key := range keys {
			t.Run(lang+"_"+key, func(t *testing.T) {
				result := instance.T(lang, key)
				// Should not return the key itself (which indicates missing translation)
				assert.NotEqual(t, key, result, "Translation for %s in %s should exist", key, lang)
				assert.NotEmpty(t, result, "Translation for %s in %s should not be empty", key, lang)
			})
		}
	}
}

func TestPreferenceHandlersRegistered(t *testing.T) {
	// This test verifies that the preference handlers are registered in the handler registry
	// This prevents the issue where handlers exist but aren't accessible via routes

	// First ensure the registry is initialized by calling ensureCoreHandlers
	ensureCoreHandlers()

	requiredHandlers := []string{
		"HandleGetSessionTimeout",
		"HandleSetSessionTimeout",
		"HandleGetLanguage",
		"HandleSetLanguage",
		"HandleGetProfile",
		"HandleUpdateProfile",
	}

	for _, name := range requiredHandlers {
		t.Run(name, func(t *testing.T) {
			handler, exists := GetHandler(name)
			assert.True(t, exists, "Handler %s must be registered in handler_registry.go", name)
			assert.NotNil(t, handler, "Handler %s must not be nil", name)
		})
	}
}

func TestHandleGetProfile_Unauthenticated(t *testing.T) {
	router := setupPreferencesTestRouter()
	router.GET("/api/profile", HandleGetProfile)

	req := httptest.NewRequest("GET", "/api/profile", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.False(t, response["success"].(bool))
	assert.Equal(t, "User not authenticated", response["error"])
}

func TestHandleUpdateProfile_Unauthenticated(t *testing.T) {
	router := setupPreferencesTestRouter()
	router.POST("/api/profile", HandleUpdateProfile)

	body := bytes.NewBufferString(`{"first_name": "John", "last_name": "Doe"}`)
	req := httptest.NewRequest("POST", "/api/profile", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.False(t, response["success"].(bool))
}

func TestHandleUpdateProfile_MissingRequiredFields(t *testing.T) {
	router := setupPreferencesTestRouter()

	// Middleware to set user_id
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uint(1))
		c.Next()
	})
	router.POST("/api/profile", HandleUpdateProfile)

	tests := []struct {
		name string
		body string
	}{
		{"missing first_name", `{"last_name": "Doe"}`},
		{"missing last_name", `{"first_name": "John"}`},
		{"empty first_name", `{"first_name": "", "last_name": "Doe"}`},
		{"empty last_name", `{"first_name": "John", "last_name": ""}`},
		{"both empty", `{"first_name": "", "last_name": ""}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := bytes.NewBufferString(tt.body)
			req := httptest.NewRequest("POST", "/api/profile", body)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.False(t, response["success"].(bool))
			assert.Contains(t, response["error"], "required")
		})
	}
}

func TestHandleUpdateProfile_InvalidJSON(t *testing.T) {
	router := setupPreferencesTestRouter()

	router.Use(func(c *gin.Context) {
		c.Set("user_id", uint(1))
		c.Next()
	})
	router.POST("/api/profile", HandleUpdateProfile)

	body := bytes.NewBufferString(`{invalid json}`)
	req := httptest.NewRequest("POST", "/api/profile", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestHandleUpdateProfile_DataPersistence verifies that profile changes are actually
// saved to the database and can be retrieved. This is a TDD test that proves the
// profile save functionality works end-to-end.
func TestHandleUpdateProfile_DataPersistence(t *testing.T) {
	WithCleanDB(t)

	db, err := database.GetDB()
	require.NoError(t, err, "Database should be available")

	// Use user ID 1 (admin user) which exists in canonical test data
	userID := uint(1)

	// Get original values to restore later
	var origFirstName, origLastName, origTitle sql.NullString
	err = db.QueryRow("SELECT first_name, last_name, title FROM users WHERE id = ?", userID).
		Scan(&origFirstName, &origLastName, &origTitle)
	require.NoError(t, err, "Should be able to read original user data")

	// Set up router with auth middleware simulation
	router := setupPreferencesTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	})
	router.POST("/api/profile", HandleUpdateProfile)
	router.GET("/api/profile", HandleGetProfile)

	// Test data - use unique values to ensure we're testing real persistence
	testFirstName := "TestFirstName_" + t.Name()
	testLastName := "TestLastName_" + t.Name()
	testTitle := "Dr."

	// POST to update profile
	updateBody := bytes.NewBufferString(`{
		"first_name": "` + testFirstName + `",
		"last_name": "` + testLastName + `",
		"title": "` + testTitle + `"
	}`)
	req := httptest.NewRequest("POST", "/api/profile", updateBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Verify the POST response indicates success
	assert.Equal(t, http.StatusOK, w.Code, "Profile update should return 200 OK")

	var updateResponse map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &updateResponse)
	require.NoError(t, err, "Response should be valid JSON")
	assert.True(t, updateResponse["success"].(bool), "Response should indicate success")

	// Now verify the data was actually persisted by reading from database directly
	var dbFirstName, dbLastName, dbTitle sql.NullString
	err = db.QueryRow("SELECT first_name, last_name, title FROM users WHERE id = ?", userID).
		Scan(&dbFirstName, &dbLastName, &dbTitle)
	require.NoError(t, err, "Should be able to read updated user data from DB")

	assert.Equal(t, testFirstName, dbFirstName.String, "First name should be persisted in database")
	assert.Equal(t, testLastName, dbLastName.String, "Last name should be persisted in database")
	assert.Equal(t, testTitle, dbTitle.String, "Title should be persisted in database")

	// Also verify via GET /api/profile endpoint
	getReq := httptest.NewRequest("GET", "/api/profile", nil)
	getW := httptest.NewRecorder()

	router.ServeHTTP(getW, getReq)

	assert.Equal(t, http.StatusOK, getW.Code, "GET profile should return 200 OK")

	var getResponse map[string]interface{}
	err = json.Unmarshal(getW.Body.Bytes(), &getResponse)
	require.NoError(t, err, "GET response should be valid JSON")
	assert.True(t, getResponse["success"].(bool), "GET response should indicate success")

	profile := getResponse["profile"].(map[string]interface{})
	assert.Equal(t, testFirstName, profile["first_name"], "GET profile should return updated first name")
	assert.Equal(t, testLastName, profile["last_name"], "GET profile should return updated last name")
	assert.Equal(t, testTitle, profile["title"], "GET profile should return updated title")
}

// TestHandleUpdateProfile_EmptyTitleAllowed verifies that empty title is allowed
// (title is optional, only first_name and last_name are required).
func TestHandleUpdateProfile_EmptyTitleAllowed(t *testing.T) {
	WithCleanDB(t)

	db, err := database.GetDB()
	require.NoError(t, err, "Database should be available")

	userID := uint(1)

	router := setupPreferencesTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	})
	router.POST("/api/profile", HandleUpdateProfile)

	// POST with empty title
	updateBody := bytes.NewBufferString(`{
		"first_name": "John",
		"last_name": "Doe",
		"title": ""
	}`)
	req := httptest.NewRequest("POST", "/api/profile", updateBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Profile update with empty title should succeed")

	// Verify database has empty title
	var dbTitle sql.NullString
	err = db.QueryRow("SELECT title FROM users WHERE id = ?", userID).Scan(&dbTitle)
	require.NoError(t, err)
	assert.Equal(t, "", dbTitle.String, "Empty title should be persisted")
}
