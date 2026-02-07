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

// setupThemeSelectorTestRouter creates a test router with theme endpoints.
func setupThemeSelectorTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	// Initialize i18n - GetInstance() loads translations automatically
	inst := i18n.GetInstance()
	if inst == nil {
		t.Fatal("i18n instance not available")
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

	// Register the theme API handlers
	r.GET("/api/themes", HandleGetAvailableThemes)
	r.POST("/api/themes", HandleSetPreLoginTheme)

	// Register login page handler
	r.GET("/login", handleLoginPage)

	// Register middleware for auth bypass testing
	registry := routing.NewHandlerRegistry()
	routing.RegisterExistingHandlers(registry)

	return r
}

func TestGetAvailableThemes(t *testing.T) {
	r := setupThemeSelectorTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/themes", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Expected 200 OK for /api/themes")

	var response struct {
		Success      bool   `json:"success"`
		CurrentTheme string `json:"current_theme"`
		CurrentMode  string `json:"current_mode"`
		Available    []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"available"`
	}

	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err, "Should parse JSON response")

	assert.True(t, response.Success, "Response should indicate success")
	assert.NotEmpty(t, response.Available, "Should return available themes")

	// Verify we have the expected themes
	themeIDs := make(map[string]bool)
	for _, theme := range response.Available {
		themeIDs[theme.ID] = true
		assert.NotEmpty(t, theme.Name, "Theme should have a name")
	}

	// Check for expected themes
	expectedThemes := []string{"synthwave", "goatflow-classic", "seventies-vibes"}
	for _, id := range expectedThemes {
		assert.True(t, themeIDs[id], "Should include theme: %s", id)
	}
}

func TestSetPreLoginTheme(t *testing.T) {
	r := setupThemeSelectorTestRouter(t)

	// Test setting theme to seventies-vibes
	body := strings.NewReader(`{"theme":"seventies-vibes"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/themes", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Expected 200 OK for POST /api/themes")

	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}

	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err, "Should parse JSON response")
	assert.True(t, response.Success, "Response should indicate success")

	// Verify cookie was set
	cookies := w.Result().Cookies()
	var themeCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "goatflow_theme" {
			themeCookie = c
			break
		}
	}

	require.NotNil(t, themeCookie, "Should set goatflow_theme cookie")
	assert.Equal(t, "seventies-vibes", themeCookie.Value, "Cookie should have value 'seventies-vibes'")
	assert.Equal(t, "/", themeCookie.Path, "Cookie should be set for root path")
}

func TestSetPreLoginMode(t *testing.T) {
	r := setupThemeSelectorTestRouter(t)

	// Test setting mode to light
	body := strings.NewReader(`{"mode":"light"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/themes", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Expected 200 OK for POST /api/themes")

	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}

	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err, "Should parse JSON response")
	assert.True(t, response.Success, "Response should indicate success")

	// Verify cookie was set
	cookies := w.Result().Cookies()
	var modeCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "goatflow_mode" {
			modeCookie = c
			break
		}
	}

	require.NotNil(t, modeCookie, "Should set goatflow_mode cookie")
	assert.Equal(t, "light", modeCookie.Value, "Cookie should have value 'light'")
	assert.Equal(t, "/", modeCookie.Path, "Cookie should be set for root path")
}

func TestSetPreLoginThemeAndMode(t *testing.T) {
	r := setupThemeSelectorTestRouter(t)

	// Test setting both theme and mode
	body := strings.NewReader(`{"theme":"goatflow-classic","mode":"light"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/themes", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Expected 200 OK for POST /api/themes")

	// Verify both cookies were set
	cookies := w.Result().Cookies()
	var themeCookie, modeCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "goatflow_theme" {
			themeCookie = c
		}
		if c.Name == "goatflow_mode" {
			modeCookie = c
		}
	}

	require.NotNil(t, themeCookie, "Should set goatflow_theme cookie")
	assert.Equal(t, "goatflow-classic", themeCookie.Value)

	require.NotNil(t, modeCookie, "Should set goatflow_mode cookie")
	assert.Equal(t, "light", modeCookie.Value)
}

func TestSetPreLoginThemeInvalidRequest(t *testing.T) {
	r := setupThemeSelectorTestRouter(t)

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "invalid json",
			body:       `{invalid}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unsupported theme",
			body:       `{"theme":"nonexistent-theme"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid mode",
			body:       `{"mode":"twilight"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			// Empty request is OK (no cookies set)
			name:       "empty request",
			body:       `{}`,
			wantStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := strings.NewReader(tc.body)
			req := httptest.NewRequest(http.MethodPost, "/api/themes", body)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tc.wantStatus, w.Code, "Expected status %d for %s", tc.wantStatus, tc.name)
		})
	}
}

func TestGetAvailableThemesWithExistingCookies(t *testing.T) {
	r := setupThemeSelectorTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/themes", nil)
	req.AddCookie(&http.Cookie{Name: "goatflow_theme", Value: "seventies-vibes"})
	req.AddCookie(&http.Cookie{Name: "goatflow_mode", Value: "light"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Success      bool   `json:"success"`
		CurrentTheme string `json:"current_theme"`
		CurrentMode  string `json:"current_mode"`
	}

	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response.Success)
	assert.Equal(t, "seventies-vibes", response.CurrentTheme, "Should return current theme from cookie")
	assert.Equal(t, "light", response.CurrentMode, "Should return current mode from cookie")
}

func TestThemeSelectorE2EFlow(t *testing.T) {
	r := setupThemeSelectorTestRouter(t)

	// Step 1: Get available themes
	req1 := httptest.NewRequest(http.MethodGet, "/api/themes", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	assert.Equal(t, http.StatusOK, w1.Code)

	var themeResponse struct {
		Available []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"available"`
	}
	err := json.Unmarshal(w1.Body.Bytes(), &themeResponse)
	require.NoError(t, err)

	// Find seventies-vibes in the list
	var seventiesFound bool
	for _, theme := range themeResponse.Available {
		if theme.ID == "seventies-vibes" {
			seventiesFound = true
			break
		}
	}
	assert.True(t, seventiesFound, "Seventies-vibes should be in available themes")

	// Step 2: Set theme preference to seventies-vibes with light mode
	body := strings.NewReader(`{"theme":"seventies-vibes","mode":"light"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/api/themes", body)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)

	// Verify cookies were set
	var themeCookie, modeCookie string
	for _, c := range w2.Result().Cookies() {
		if c.Name == "goatflow_theme" {
			themeCookie = c.Value
		}
		if c.Name == "goatflow_mode" {
			modeCookie = c.Value
		}
	}
	assert.Equal(t, "seventies-vibes", themeCookie, "Theme cookie should be set")
	assert.Equal(t, "light", modeCookie, "Mode cookie should be set")

	// Step 3: Verify GET returns the set values
	req3 := httptest.NewRequest(http.MethodGet, "/api/themes", nil)
	req3.AddCookie(&http.Cookie{Name: "goatflow_theme", Value: "seventies-vibes"})
	req3.AddCookie(&http.Cookie{Name: "goatflow_mode", Value: "light"})
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)

	assert.Equal(t, http.StatusOK, w3.Code)

	var verifyResponse struct {
		CurrentTheme string `json:"current_theme"`
		CurrentMode  string `json:"current_mode"`
	}
	err = json.Unmarshal(w3.Body.Bytes(), &verifyResponse)
	require.NoError(t, err)

	assert.Equal(t, "seventies-vibes", verifyResponse.CurrentTheme)
	assert.Equal(t, "light", verifyResponse.CurrentMode)
}

func TestLoginPageHasThemeSelector(t *testing.T) {
	r := setupThemeSelectorTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	htmlBody := w.Body.String()

	// Check that the page includes the theme selector partial
	// and makes fetch calls to /api/themes
	assert.Contains(t, htmlBody, "/api/themes",
		"Login page should reference /api/themes endpoint")
}
