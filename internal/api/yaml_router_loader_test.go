package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestFallbackAuthGuard_BypassDisabledJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("GOTRS_DISABLE_TEST_AUTH_BYPASS", "1")
	t.Setenv("APP_ENV", "")

	router := gin.New()
	router.GET("/admin/users", fallbackAuthGuard(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, resp.Code)
	}
}

func TestFallbackAuthGuard_BypassDisabledHTML(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("GOTRS_DISABLE_TEST_AUTH_BYPASS", "1")
	t.Setenv("APP_ENV", "")

	router := gin.New()
	router.GET("/admin/users", fallbackAuthGuard(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.Header.Set("Accept", "text/html")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, resp.Code)
	}
	if loc := resp.Header().Get("Location"); loc != "/login" {
		t.Fatalf("expected redirect to /login, got %q", loc)
	}
}

func TestFallbackAuthGuard_BypassAllowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("GOTRS_DISABLE_TEST_AUTH_BYPASS", "0")
	t.Setenv("APP_ENV", "test")

	router := gin.New()
	router.GET("/admin/users", fallbackAuthGuard(), func(c *gin.Context) {
		if _, ok := c.Get("user_id"); !ok {
			t.Fatalf("expected user context to be populated")
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.Code)
	}
}
