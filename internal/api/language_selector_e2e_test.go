package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/i18n"
	"github.com/goatkit/goatflow/internal/middleware"
	"github.com/goatkit/goatflow/internal/routing"
	"github.com/goatkit/goatflow/internal/shared"
)

// setupLanguageSelectorTestRouter creates a test router with i18n middleware and language endpoints.
func setupLanguageSelectorTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	// Initialize i18n - GetInstance() loads translations automatically
	// Just verify the instance is available
	inst := i18n.GetInstance()
	if inst == nil {
		t.Fatal("i18n instance not available")
	}
	langs := inst.GetSupportedLanguages()
	if len(langs) == 0 {
		t.Logf("Warning: No supported languages loaded")
	}

	// Initialize template renderer
	renderer, err := shared.NewTemplateRenderer("../../templates")
	if err != nil {
		t.Fatalf("Failed to create template renderer: %v", err)
	}
	shared.SetGlobalRenderer(renderer)

	r := gin.New()

	// Add i18n middleware (like production)
	i18nMW := middleware.NewI18nMiddleware()
	r.Use(i18nMW.Handle())

	// Register the language API handlers
	r.GET("/api/languages", HandleGetAvailableLanguages)
	r.POST("/api/languages", HandleSetPreLoginLanguage)

	// Register login page handler
	r.GET("/login", handleLoginPage)

	// Register middleware for auth bypass testing
	registry := routing.NewHandlerRegistry()
	routing.RegisterExistingHandlers(registry)

	return r
}

func TestGetAvailableLanguages(t *testing.T) {
	r := setupLanguageSelectorTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/languages", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Expected 200 OK for /api/languages")

	var response struct {
		Success   bool   `json:"success"`
		Current   string `json:"current"`
		Available []struct {
			Code       string `json:"code"`
			Name       string `json:"name"`
			NativeName string `json:"native_name"`
		} `json:"available"`
	}

	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err, "Should parse JSON response")

	assert.True(t, response.Success, "Response should indicate success")
	assert.NotEmpty(t, response.Available, "Should return available languages")

	// Verify we have the expected languages
	languageCodes := make(map[string]bool)
	for _, lang := range response.Available {
		languageCodes[lang.Code] = true
		assert.NotEmpty(t, lang.Name, "Language should have a name")
		assert.NotEmpty(t, lang.NativeName, "Language should have a native name")
	}

	// Check for key languages
	expectedLanguages := []string{"en", "de", "es", "fr", "pl", "ru", "zh", "ja", "ar"}
	for _, code := range expectedLanguages {
		assert.True(t, languageCodes[code], "Should include language: %s", code)
	}
}

func TestSetPreLoginLanguage(t *testing.T) {
	r := setupLanguageSelectorTestRouter(t)

	// Test setting language to Polish
	body := strings.NewReader(`{"value":"pl"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/languages", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Expected 200 OK for POST /api/languages")

	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}

	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err, "Should parse JSON response")
	assert.True(t, response.Success, "Response should indicate success")

	// Verify cookie was set
	cookies := w.Result().Cookies()
	var langCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "goatflow_lang" {
			langCookie = c
			break
		}
	}

	require.NotNil(t, langCookie, "Should set goatflow_lang cookie")
	assert.Equal(t, "pl", langCookie.Value, "Cookie should have value 'pl'")
	assert.Equal(t, "/", langCookie.Path, "Cookie should be set for root path")
}

func TestSetPreLoginLanguageInvalidRequest(t *testing.T) {
	r := setupLanguageSelectorTestRouter(t)

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			// Empty value is allowed - clears preference to use browser default
			name:       "empty value (allowed - clears preference)",
			body:       `{"value":""}`,
			wantStatus: http.StatusOK,
		},
		{
			// Missing value is treated as empty, which is allowed
			name:       "missing value (allowed - clears preference)",
			body:       `{}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid json",
			body:       `{invalid}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			// Unsupported language should be rejected
			name:       "unsupported language",
			body:       `{"value":"xyz"}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := strings.NewReader(tc.body)
			req := httptest.NewRequest(http.MethodPost, "/api/languages", body)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tc.wantStatus, w.Code, "Expected status %d for %s", tc.wantStatus, tc.name)
		})
	}
}

func TestLoginPageTranslationWithLanguageCookie(t *testing.T) {
	r := setupLanguageSelectorTestRouter(t)

	tests := []struct {
		name           string
		langCookie     string
		expectedTitle  string
		expectedLabels []string
	}{
		{
			name:          "English (default)",
			langCookie:    "en",
			expectedTitle: "Login",
			expectedLabels: []string{
				"Username",
				"Password",
			},
		},
		{
			name:          "Polish",
			langCookie:    "pl",
			expectedTitle: "Logowanie",
			expectedLabels: []string{
				"Nazwa użytkownika",
				"Hasło",
			},
		},
		{
			name:          "German",
			langCookie:    "de",
			expectedTitle: "Anmelden",
			expectedLabels: []string{
				"Benutzername",
				"Passwort",
			},
		},
		{
			name:          "Spanish",
			langCookie:    "es",
			expectedTitle: "Iniciar Sesión",
			expectedLabels: []string{
				"Nombre de Usuario",
				"Contraseña",
			},
		},
		{
			name:          "French",
			langCookie:    "fr",
			expectedTitle: "Connexion",
			expectedLabels: []string{
				// Note: apostrophe may be HTML-encoded as &#39;
				"Nom d",       // partial match to handle encoding
				"utilisateur", // partial match
				"Mot de passe",
			},
		},
		{
			name:          "Russian",
			langCookie:    "ru",
			expectedTitle: "Вход",
			expectedLabels: []string{
				"Имя пользователя",
				"Пароль",
			},
		},
		{
			name:          "Japanese",
			langCookie:    "ja",
			expectedTitle: "ログイン",
			expectedLabels: []string{
				"ユーザー名",
				"パスワード",
			},
		},
		{
			name:          "Chinese",
			langCookie:    "zh",
			expectedTitle: "登录",
			expectedLabels: []string{
				"用户名",
				"密码",
			},
		},
		{
			name:          "Arabic",
			langCookie:    "ar",
			expectedTitle: "تسجيل الدخول",
			expectedLabels: []string{
				"اسم المستخدم",
				"كلمة المرور",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/login", nil)
			req.AddCookie(&http.Cookie{Name: "goatflow_lang", Value: tc.langCookie})
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code, "Expected 200 OK for /login with %s", tc.langCookie)

			body := w.Body.String()

			// Check title contains expected translation
			assert.Contains(t, body, tc.expectedTitle,
				"Login page with lang=%s should contain title '%s'", tc.langCookie, tc.expectedTitle)

			// Check for translated labels
			for _, label := range tc.expectedLabels {
				assert.Contains(t, body, label,
					"Login page with lang=%s should contain label '%s'", tc.langCookie, label)
			}
		})
	}
}

func TestLoginPageTranslationWithQueryParam(t *testing.T) {
	r := setupLanguageSelectorTestRouter(t)

	// Test that ?lang= query parameter changes the language
	req := httptest.NewRequest(http.MethodGet, "/login?lang=de", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Expected 200 OK")

	body := w.Body.String()
	assert.Contains(t, body, "Anmelden", "Should show German title when ?lang=de")
	assert.Contains(t, body, "Benutzername", "Should show German username label")
	assert.Contains(t, body, "Passwort", "Should show German password label")

	// Verify lang cookie was set
	cookies := w.Result().Cookies()
	var langCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "lang" {
			langCookie = c
			break
		}
	}
	require.NotNil(t, langCookie, "Should set lang cookie from query param")
	assert.Equal(t, "de", langCookie.Value, "Cookie should have value 'de'")
}

func TestLanguageSelectorE2EFlow(t *testing.T) {
	r := setupLanguageSelectorTestRouter(t)

	// Step 1: Get available languages
	req1 := httptest.NewRequest(http.MethodGet, "/api/languages", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	assert.Equal(t, http.StatusOK, w1.Code)

	var langResponse struct {
		Available []struct {
			Code       string `json:"code"`
			NativeName string `json:"native_name"`
		} `json:"available"`
	}
	err := json.Unmarshal(w1.Body.Bytes(), &langResponse)
	require.NoError(t, err)

	// Find Polish in the list
	var polishFound bool
	for _, lang := range langResponse.Available {
		if lang.Code == "pl" {
			polishFound = true
			assert.Equal(t, "Polski", lang.NativeName, "Polish native name should be 'Polski'")
			break
		}
	}
	assert.True(t, polishFound, "Polish should be in available languages")

	// Step 2: Set language preference to Polish
	body := strings.NewReader(`{"value":"pl"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/api/languages", body)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)

	// Get the cookie that was set
	var setCookie string
	for _, c := range w2.Result().Cookies() {
		if c.Name == "goatflow_lang" {
			setCookie = c.Value
			break
		}
	}
	assert.Equal(t, "pl", setCookie, "Cookie should be set to 'pl'")

	// Step 3: Verify login page now shows Polish
	req3 := httptest.NewRequest(http.MethodGet, "/login", nil)
	req3.AddCookie(&http.Cookie{Name: "goatflow_lang", Value: "pl"})
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)

	assert.Equal(t, http.StatusOK, w3.Code)

	htmlBody := w3.Body.String()
	assert.Contains(t, htmlBody, "Logowanie", "Page should show Polish title 'Logowanie'")
	assert.Contains(t, htmlBody, "Nazwa użytkownika", "Page should show Polish username label")
	assert.Contains(t, htmlBody, "Hasło", "Page should show Polish password label")

	// Verify Content-Language header is set
	contentLang := w3.Header().Get("Content-Language")
	assert.Equal(t, "pl", contentLang, "Content-Language header should be 'pl'")
}

func TestLanguagePersistenceAfterPageReload(t *testing.T) {
	r := setupLanguageSelectorTestRouter(t)

	// Simulate: User selects German, then reloads the page multiple times
	// The language should persist via cookie

	// First request: set language to German
	body := strings.NewReader(`{"value":"de"}`)
	req1 := httptest.NewRequest(http.MethodPost, "/api/languages", body)
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	assert.Equal(t, http.StatusOK, w1.Code)

	// Simulate multiple page reloads with the cookie
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/login", nil)
		req.AddCookie(&http.Cookie{Name: "goatflow_lang", Value: "de"})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Reload %d should succeed", i+1)

		htmlBody := w.Body.String()
		assert.Contains(t, htmlBody, "Anmelden",
			"Reload %d: Page should still show German title", i+1)
		assert.Contains(t, htmlBody, "Benutzername",
			"Reload %d: Page should still show German username label", i+1)
	}
}

func TestLanguageSelectorWithBothCookies(t *testing.T) {
	r := setupLanguageSelectorTestRouter(t)

	// Test priority: lang cookie should take precedence over goatflow_lang
	// (This matches the middleware behavior)

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	req.AddCookie(&http.Cookie{Name: "lang", Value: "fr"})
	req.AddCookie(&http.Cookie{Name: "goatflow_lang", Value: "de"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	htmlBody := w.Body.String()
	// lang cookie should win, so we expect French
	assert.Contains(t, htmlBody, "Connexion", "Should show French (lang cookie takes precedence)")
}

func TestLoginPageHasLanguageSelector(t *testing.T) {
	r := setupLanguageSelectorTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	htmlBody := w.Body.String()

	// Check that the page includes the language selector partial
	// and makes fetch calls to /api/languages
	assert.Contains(t, htmlBody, "/api/languages",
		"Login page should reference /api/languages endpoint")
}
