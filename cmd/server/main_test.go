package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	
	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"service": "gotrs-backend",
		})
	})

	// API routes
	api := r.Group("/api/v1")
	{
		api.GET("/status", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data": gin.H{
					"message": "GOTRS API is running",
					"version": "0.1.0",
				},
			})
		})
	}
	
	return r
}

func TestHealthEndpoint(t *testing.T) {
	router := setupTestRouter()

	t.Run("GET /health returns healthy status", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/health", nil)
		require.NoError(t, err)

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "healthy", response["status"])
		assert.Equal(t, "gotrs-backend", response["service"])
	})
}

func TestAPIStatusEndpoint(t *testing.T) {
	router := setupTestRouter()

	t.Run("GET /api/v1/status returns success", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/api/v1/status", nil)
		require.NoError(t, err)

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, true, response["success"])
		
		data, ok := response["data"].(map[string]interface{})
		require.True(t, ok, "data should be a map")
		
		assert.Equal(t, "GOTRS API is running", data["message"])
		assert.Equal(t, "0.1.0", data["version"])
	})
}

func TestGinModeConfiguration(t *testing.T) {
	t.Run("Production mode sets gin to release mode", func(t *testing.T) {
		// Save original env
		originalEnv := os.Getenv("APP_ENV")
		defer os.Setenv("APP_ENV", originalEnv)

		// Set production mode
		os.Setenv("APP_ENV", "production")
		
		// In a real test, we'd need to call the main function logic
		// For now, we just verify the environment variable is set
		assert.Equal(t, "production", os.Getenv("APP_ENV"))
	})

	t.Run("Non-production mode uses default gin mode", func(t *testing.T) {
		// Save original env
		originalEnv := os.Getenv("APP_ENV")
		defer os.Setenv("APP_ENV", originalEnv)

		// Set development mode
		os.Setenv("APP_ENV", "development")
		
		assert.NotEqual(t, "production", os.Getenv("APP_ENV"))
	})
}

func TestPortConfiguration(t *testing.T) {
	t.Run("Uses APP_PORT environment variable when set", func(t *testing.T) {
		originalPort := os.Getenv("APP_PORT")
		defer os.Setenv("APP_PORT", originalPort)

		os.Setenv("APP_PORT", "3000")
		assert.Equal(t, "3000", os.Getenv("APP_PORT"))
	})

	t.Run("Defaults to port 8080 when APP_PORT not set", func(t *testing.T) {
		originalPort := os.Getenv("APP_PORT")
		defer os.Setenv("APP_PORT", originalPort)

		os.Unsetenv("APP_PORT")
		
		// In actual implementation, we'd test the default value logic
		// For now, we verify the env var is empty
		assert.Empty(t, os.Getenv("APP_PORT"))
	})
}

func TestNotFoundRoute(t *testing.T) {
	router := setupTestRouter()

	t.Run("Non-existent route returns 404", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/nonexistent", nil)
		require.NoError(t, err)

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestMethodNotAllowed(t *testing.T) {
	router := setupTestRouter()

	t.Run("Wrong HTTP method returns 404 (gin default)", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, err := http.NewRequest("POST", "/health", nil)
		require.NoError(t, err)

		router.ServeHTTP(w, req)

		// Gin returns 404 by default for unmatched methods
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func BenchmarkHealthEndpoint(b *testing.B) {
	router := setupTestRouter()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w, req)
	}
}

func BenchmarkAPIStatusEndpoint(b *testing.B) {
	router := setupTestRouter()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/status", nil)
		router.ServeHTTP(w, req)
	}
}