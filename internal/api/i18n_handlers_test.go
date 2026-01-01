package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupI18nTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	return router
}

func TestGetTranslationCoverage(t *testing.T) {
	router := setupI18nTestRouter()
	handlers := NewI18nHandlers()

	api := router.Group("/api/v1")
	handlers.RegisterRoutes(api)

	tests := []struct {
		name           string
		expectedStatus int
		checkResponse  func(t *testing.T, body []byte)
	}{
		{
			name:           "Should return coverage statistics for all languages",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var response CoverageResponse
				err := json.Unmarshal(body, &response)
				require.NoError(t, err)

				// Should have coverage data for all supported languages
				assert.NotEmpty(t, response.Languages)

				// Check that English is at 100%
				found := false
				for _, lang := range response.Languages {
					if lang.Code == "en" {
						found = true
						assert.Equal(t, 100.0, lang.Coverage)
						assert.Equal(t, 0, lang.MissingCount)
					}

					// All languages should have valid data
					assert.NotEmpty(t, lang.Code)
					assert.NotEmpty(t, lang.Name)
					assert.GreaterOrEqual(t, lang.TotalKeys, 0)
					assert.GreaterOrEqual(t, lang.TranslatedKeys, 0)
					assert.GreaterOrEqual(t, lang.Coverage, 0.0)
					// Coverage can exceed 100% if language has extra keys not in base
				}
				assert.True(t, found, "English language not found in coverage")

				// Should have summary statistics
				assert.Greater(t, response.Summary.TotalKeys, 0)
				assert.GreaterOrEqual(t, response.Summary.AverageCoverage, 0.0)
				// Average coverage can exceed 100% if languages have extra keys
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/i18n/coverage", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.Bytes())
			}
		})
	}
}

func TestGetMissingTranslations(t *testing.T) {
	router := setupI18nTestRouter()
	handlers := NewI18nHandlers()

	api := router.Group("/api/v1")
	handlers.RegisterRoutes(api)

	tests := []struct {
		name           string
		lang           string
		expectedStatus int
		checkResponse  func(t *testing.T, body []byte)
	}{
		{
			name:           "Should return empty list for English",
			lang:           "en",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var response MissingKeysResponse
				err := json.Unmarshal(body, &response)
				require.NoError(t, err)

				assert.Equal(t, "en", response.Language)
				assert.Empty(t, response.MissingKeys)
				assert.Equal(t, 0, response.Count)
			},
		},
		{
			name:           "Should return missing keys for incomplete language",
			lang:           "es",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var response MissingKeysResponse
				err := json.Unmarshal(body, &response)
				require.NoError(t, err)

				assert.Equal(t, "es", response.Language)
				// Spanish might have missing keys
				assert.GreaterOrEqual(t, response.Count, 0)
				assert.Equal(t, len(response.MissingKeys), response.Count)

				// Each missing key should be valid
				for _, key := range response.MissingKeys {
					assert.NotEmpty(t, key.Key)
					assert.NotEmpty(t, key.EnglishValue)
					assert.NotEmpty(t, key.Category) // e.g., "tickets", "admin"
				}
			},
		},
		{
			name:           "Should return 404 for unsupported language",
			lang:           "xx",
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				err := json.Unmarshal(body, &response)
				require.NoError(t, err)

				assert.Contains(t, response, "error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/i18n/missing/"+tt.lang, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.Bytes())
			}
		})
	}
}

func TestExportTranslations(t *testing.T) {
	router := setupI18nTestRouter()
	handlers := NewI18nHandlers()

	api := router.Group("/api/v1")
	handlers.RegisterRoutes(api)

	tests := []struct {
		name           string
		lang           string
		format         string
		expectedStatus int
		checkResponse  func(t *testing.T, body []byte, contentType string)
	}{
		{
			name:           "Should export as JSON",
			lang:           "en",
			format:         "json",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte, contentType string) {
				assert.Contains(t, contentType, "application/json")

				var translations map[string]interface{}
				err := json.Unmarshal(body, &translations)
				require.NoError(t, err)

				// Should have standard sections
				assert.Contains(t, translations, "app")
				assert.Contains(t, translations, "auth")
				assert.Contains(t, translations, "navigation")
				assert.Contains(t, translations, "tickets")
			},
		},
		{
			name:           "Should export as CSV",
			lang:           "en",
			format:         "csv",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte, contentType string) {
				assert.Contains(t, contentType, "text/csv")

				// CSV should have headers
				content := string(body)
				assert.Contains(t, content, "key,value")
				assert.Contains(t, content, "app.name")
				assert.Contains(t, content, "GOTRS")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/i18n/export/"+tt.lang+"?format="+tt.format, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.Bytes(), w.Header().Get("Content-Type"))
			}
		})
	}
}

func TestValidateTranslations(t *testing.T) {
	router := setupI18nTestRouter()
	handlers := NewI18nHandlers()

	api := router.Group("/api/v1")
	handlers.RegisterRoutes(api)

	tests := []struct {
		name           string
		lang           string
		expectedStatus int
		checkResponse  func(t *testing.T, body []byte)
	}{
		{
			name:           "Should validate complete language",
			lang:           "en",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var response ValidationResponse
				err := json.Unmarshal(body, &response)
				require.NoError(t, err)

				assert.Equal(t, "en", response.Language)
				assert.True(t, response.IsValid)
				assert.True(t, response.IsComplete)
				assert.Empty(t, response.Errors)
				assert.Empty(t, response.Warnings)
				assert.Equal(t, 100.0, response.Coverage)
			},
		},
		{
			name:           "Should validate incomplete language with warnings",
			lang:           "es",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var response ValidationResponse
				err := json.Unmarshal(body, &response)
				require.NoError(t, err)

				assert.Equal(t, "es", response.Language)
				// May have warnings but should be valid JSON
				assert.True(t, response.IsValid)
				// May not be complete
				if response.Coverage < 100.0 {
					assert.False(t, response.IsComplete)
					assert.NotEmpty(t, response.Warnings)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/i18n/validate/"+tt.lang, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.Bytes())
			}
		})
	}
}
